// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

// voterRouting holds the collaborators the payout loop needs to decide
// compound-vs-credit for each voter. Resolved once per chunk because every one
// of them is a state read that would otherwise repeat per voter.
//
// It carries no candidate: the drain is voter-major, so one routing decision
// covers a voter's entire entitlement across every delegate they staked with.
type voterRouting struct {
	stakingProto *staking.Protocol
	bucketReader autodeposit.BucketReader
	csr          staking.CandidateStateReader
}

// voterCombinedPayout is the outcome of moving one voter's whole era
// entitlement. A voter is paid once, not once per delegate: the shares are
// summed first and a single transfer follows, so a voter with buckets on
// twenty delegates costs one destination lookup and one balance write instead
// of twenty.
type voterCombinedPayout struct {
	voter     address.Address
	recipient address.Address
	amount    *big.Int
	// compounded is the discriminator for "this payout went into a staking
	// bucket instead of a balance credit". It is a separate field, and not
	// `compoundBucketID != 0`, because native bucket index 0 is a real bucket:
	// a voter whose auto-deposit bucket is index 0 compounds successfully and
	// would otherwise be misreported as a direct credit -- emitting a spurious
	// CLAIM_FROM_REWARDING_FUND transaction log, omitting the amount from the
	// block's compound outflow, and telling off-chain consumers the voter was
	// paid to their reward address when they were not.
	//
	// compoundBucketID is only meaningful when compounded is true.
	compounded       bool
	compoundBucketID uint64
	shares           []voterDelegateShare
}

// payVoterCombined credits one voter the sum of their per-delegate shares.
//
// Routing is decided once for the combined amount. If the voter has an
// eligible auto-deposit bucket the whole sum is compounded into it; otherwise
// the whole sum is credited to their reward destination.
//
// Which branch a fault takes is not a matter of taste. A fallback degrades to
// a direct credit only when every validator derives it from the same committed
// state: no bridge configured, no registrant, a bucket that is absent,
// withdrawn or ineligible, a self-stake role changed since the freeze. The
// share is still owed in all of those, and only its destination changes.
//
// A read this node could not serve is the opposite case. Degrading it would
// let one validator credit the balance while another compounds into the
// bucket and moves candidate votes with it -- one block, two state roots. Those
// propagate.
func (p *Protocol) payVoterCombined(
	ctx context.Context,
	sm protocol.StateManager,
	routing voterRouting,
	in voterShareInputs,
	voter address.Address,
	shares voterShareSet,
	routeDurations *iip59RouteDurations,
) (voterCombinedPayout, error) {
	var none voterCombinedPayout
	if voter == nil {
		return none, errors.New("rewarding: nil voter address in drain")
	}
	total := safeBig(shares.total)
	if total.Sign() <= 0 {
		return none, nil
	}
	out := voterCombinedPayout{voter: voter, recipient: voter, amount: total, shares: shares.shares}

	failClosed := protocol.MustGetFeatureCtx(ctx).FixEpochSettlementFaultHandling

	if routing.bucketReader != nil {
		stop := startIIP59Accumulation(&routeDurations.autoDepositLookup)
		bucketID, present, lookupErr := routing.bucketReader.LookupBucket(voter)
		stop()
		addIIP59Items("auto_deposit_lookup", 1)
		switch {
		case lookupErr != nil:
			// Not degradable, despite reading like one. This lookup decides
			// where the share goes, and the two destinations are not
			// equivalent: a credit moves the voter's unclaimed balance, a
			// compound moves the bucket, the candidate's votes and the bucket
			// pool. SlotBucketReader reports every on-chain shape it can read
			// -- unset registrant, zero or malformed bucket id -- as
			// (0, false, nil), so a non-nil error here is the node failing to
			// serve a read, which is exactly the class that differs between
			// validators. Letting it pick the destination puts the credit
			// branch and the compound branch in the same block on different
			// nodes.
			//
			// Gated: a chain that activated IIP-59 before this height already
			// committed blocks where the fault produced a credit, and replay
			// has to reproduce them.
			if failClosed {
				return none, errors.Wrapf(lookupErr,
					"rewarding: auto-deposit bucket lookup for voter %s", voter.String())
			}
			log.L().Warn("autodeposit bucket lookup failed; routing voter share to credit",
				zap.String("voter", voter.String()), zap.Error(lookupErr))
		case present:
			stopRead := startIIP59Accumulation(&routeDurations.nativeBucketRead)
			bucket, bErr := routing.csr.NativeBucket(bucketID)
			stopRead()
			addIIP59Items("native_bucket_read", 1)
			if bErr != nil {
				// Same rule, one read later. "No such bucket" and "already
				// withdrawn" are committed state and degrade; anything else is
				// node-local and must fail the block rather than reroute money
				// on a fault only some validators saw.
				if failClosed && !bucketReadErrorIsChainDetermined(bErr) {
					return none, errors.Wrapf(bErr,
						"rewarding: read compound bucket %d for voter %s", bucketID, voter.String())
				}
				log.L().Warn("bucket unreadable for compound routing; routing voter share to credit",
					zap.String("voter", voter.String()),
					zap.Uint64("bucket", bucketID),
					zap.Error(bErr))
			} else if autodeposit.IsBucketEligibleForCompound(bucket, voter) {
				// The self-stake guard inside AddDepositForCompound compares
				// against the era that froze this bucket's delegate, so the era
				// has to come from the bucket's own candidate. Handing it some
				// other delegate's era would make the guard compare unrelated
				// bucket indices and either fire or stay silent by accident.
				era := frozenEraForBucket(in, bucket)
				// Snapshot before the deposit so a degradable failure can be
				// rolled back. AddDepositForCompound mutates in stages -- the
				// bucket is persisted before the candidate votes and the bucket
				// pool are touched -- so an error raised partway through leaves
				// state half-applied. Crediting the voter on top of a
				// half-applied deposit would pay the share twice.
				sid := sm.Snapshot()
				stopDeposit := startIIP59Accumulation(&routeDurations.compoundDeposit)
				err := routing.stakingProto.AddDepositForCompound(ctx, sm, voter, bucketID, total, era)
				stopDeposit()
				switch {
				case err == nil:
					out.compounded = true
					out.compoundBucketID = bucketID
					addIIP59Items("compound_voter", 1)
					return out, nil
				case compoundErrorIsChainDetermined(err):
					if rErr := sm.Revert(sid); rErr != nil {
						// Revert itself is infrastructure. If it fails there is
						// no consistent state to fall back to.
						return none, errors.Wrapf(rErr,
							"rewarding: revert failed compound deposit for voter %s bucket %d (deposit error: %v)",
							voter.String(), bucketID, err)
					}
					log.L().Warn("compound deposit rejected by chain state; routing voter share to credit",
						zap.String("voter", voter.String()),
						zap.Uint64("bucket", bucketID),
						zap.Error(err))
					addIIP59Items("compound_degraded", 1)
				default:
					return none, errors.Wrapf(err,
						"rewarding: compound deposit failed for voter %s bucket %d",
						voter.String(), bucketID)
				}
			}
		}
	}

	stopDestination := startIIP59Accumulation(&routeDurations.destinationLookup)
	recipient, err := p.effectiveVoterRewardDestination(ctx, sm, voter)
	stopDestination()
	addIIP59Items("reward_destination_lookup", 1)
	if err != nil {
		return none, errors.Wrapf(err, "rewarding: resolve reward destination for voter %s", voter.String())
	}
	out.recipient = recipient
	stopCredit := startIIP59Accumulation(&routeDurations.directCredit)
	err = creditPrimaryAccount(ctx, sm, recipient, total)
	stopCredit()
	if err != nil {
		return none, errors.Wrapf(err,
			"rewarding: credit voter %s recipient %s failed", voter.String(), recipient.String())
	}
	addIIP59Items("direct_voter", 1)
	return out, nil
}

// frozenEraForBucket returns the era view of the delegate a compound bucket
// votes for. A bucket whose candidate is not in the frozen work list yields the
// zero value, which disables the self-stake role check rather than checking it
// against the wrong delegate.
func frozenEraForBucket(in voterShareInputs, bucket *staking.VoteBucket) staking.FrozenSelfStake {
	if bucket == nil || bucket.Candidate == nil {
		return staking.FrozenSelfStake{}
	}
	i, ok := in.byCandidate[string(bucket.Candidate.Bytes())]
	if !ok || i >= len(in.delegates) {
		return staking.FrozenSelfStake{}
	}
	work := in.delegates[i]
	return staking.FrozenSelfStake{FreezeHeight: in.freezeHeight, BucketIdx: work.SelfStakeBucketIdx}
}

// resolveVoterRouting gathers the per-chunk collaborators for compound routing.
// A nil autoDepositBridge (the common case on a node that has not configured
// one) leaves bucketReader and csr nil, which the payout loop reads as
// "everyone gets credited directly".
func (p *Protocol) resolveVoterRouting(
	ctx context.Context,
	sm protocol.StateManager,
) (voterRouting, error) {
	var none voterRouting
	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return none, errors.New("rewarding: staking protocol not registered")
	}
	routing := voterRouting{stakingProto: stakingProto}
	if p.autoDepositBridge == nil {
		return routing, nil
	}
	slotReader, err := evm.NewSlotReader(ctx, sm)
	if err != nil {
		return none, errors.Wrap(err, "rewarding: build slot reader for autodeposit")
	}
	routing.bucketReader, err = p.resolveAutoDepositBucketReader(slotReader)
	if err != nil {
		return none, errors.Wrap(err, "rewarding: resolve autodeposit bucket reader")
	}
	if routing.bucketReader != nil {
		routing.csr, err = staking.ConstructBaseView(sm)
		if err != nil {
			return none, errors.Wrap(err, "rewarding: construct base view for compound routing")
		}
	}
	return routing, nil
}

// assertNonNegativeReward rejects a negative reward amount. Nil is treated
// as zero so callers can opt one of the two streams out of a voter reward drain call.
func assertNonNegativeReward(v *big.Int) error {
	if v == nil {
		return nil
	}
	if v.Sign() < 0 {
		return errors.New("rewarding: invalid total reward for voter distribution")
	}
	return nil
}

// safeBig returns v when non-nil, otherwise a fresh zero big.Int. Keeps
// nil-tolerance out of the arithmetic sites in the main body.
func safeBig(v *big.Int) *big.Int {
	if v == nil {
		return new(big.Int)
	}
	return v
}

// isNilOrZero returns true when v is nil or zero.
func isNilOrZero(v *big.Int) bool {
	return v == nil || v.Sign() == 0
}

// resolveAutoDepositBucketReader returns the per-drain BucketReader used
// against the AutoDeposit contract. Tests may inject a factory via
// WithAutoDepositBucketReader; production falls through to
// Bridge.NewSlotBucketReader which reads (registrants, buckets) directly
// from contract storage (see autodeposit/slot_reader.go).
func (p *Protocol) resolveAutoDepositBucketReader(slotReader autodeposit.SlotReader) (autodeposit.BucketReader, error) {
	if p.autoDepositBucketReaderFactory != nil {
		return p.autoDepositBucketReaderFactory(slotReader), nil
	}
	return p.autoDepositBridge.NewSlotBucketReader(slotReader)
}

func creditPrimaryAccount(
	ctx context.Context,
	sm protocol.StateManager,
	addr address.Address,
	amount *big.Int,
) error {
	opts := []state.AccountCreationOption{}
	if protocol.MustGetFeatureCtx(ctx).CreateLegacyNonceAccount {
		opts = append(opts, state.LegacyNonceAccountTypeOption())
	}
	account, err := accountutil.LoadOrCreateAccount(sm, addr, opts...)
	if err != nil {
		return err
	}
	if err := account.AddBalance(amount); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, addr, account)
}
