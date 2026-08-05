// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/distributedlog"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const _basisPointsDenom uint64 = 10_000

type delegateRewardRouting struct {
	owner                address.Address
	legacyRewardAddress  address.Address
	onchainRewardEnabled bool
	rewardAddressUpdated bool
	blockCommissionBPs   uint64
	epochCommissionBPs   uint64
	snapshot             *staking.CandidatePollSnapshot
}

func resolveDelegateRewardRouting(
	ctx context.Context,
	sr protocol.StateReader,
	candID address.Address,
) (*delegateRewardRouting, error) {
	g := genesis.MustExtractGenesisContext(ctx)
	live, err := staking.ReadCandidateRewardRouting(sr, candID, g.HermesRewardVaultAddresses)
	if err != nil {
		return nil, err
	}
	routing := &delegateRewardRouting{
		owner:                live.Owner,
		legacyRewardAddress:  live.LegacyRewardAddress,
		onchainRewardEnabled: live.OnchainRewardEnabled,
		rewardAddressUpdated: live.RewardAddressUpdated,
		blockCommissionBPs:   _basisPointsDenom,
		epochCommissionBPs:   _basisPointsDenom,
	}
	// No staleness check here, unlike freezeDelegateDrainWork. This runs at
	// EVERY epoch of an era, and the only authoritative source of the era's H
	// is the copy-on-write window — which is open for the freeze block and the
	// few drain blocks after the boundary, and closed (FreezeHeight 0) for the
	// rest of the era. Testing snapshot.FreezeHeight against it here would
	// declare every perfectly fresh snapshot stale for ~23 of an era's 24
	// epochs, and "stale" collapses to the ErrStateNotExist branch below, whose
	// 100%-commission default pays voters nothing. The guard belongs where the
	// window is guaranteed open, i.e. at the boundary.
	//
	// The exposure that leaves is bounded and strictly milder: a delegate
	// carrying a previous era's snapshot splits this era's epoch rewards at the
	// previous era's commission rate. Money is still conserved and still
	// reaches the voter pool; only the rate is off. The drain-side guard then
	// refuses to settle that pool on a mixed basis and rolls it forward.
	snap, err := staking.PollSnapshotFor(sr, candID)
	switch {
	case err == nil:
		routing.snapshot = snap
		routing.onchainRewardEnabled = snap.OnchainRewardEnabled
		if snap.OnchainRewardEnabled {
			routing.blockCommissionBPs = snap.BlockCommissionBasisPoints
			routing.epochCommissionBPs = snap.EpochCommissionBasisPoints
		}
	case errors.Is(err, state.ErrStateNotExist):
		// No snapshot at all. Commission stays at the 100% default and
		// onchainRewardEnabled stays on the LIVE value read above, which is the
		// pre-IIP-59 behaviour. FreezePollSnapshot is what keeps this branch
		// unreachable for a delegate that is opted in and present at H: it
		// freezes the poll list UNION the live opted-in set precisely so an
		// opted-in delegate cannot arrive here. A delegate registered after H
		// can still land here, and does so with all-to-delegate commission.
	default:
		return nil, err
	}
	return routing, nil
}

// splitDelegateEpochReward computes the epoch-reward commission / voter
// split for a delegate. Before the fork it returns (amount, 0). After the
// fork, only an enabled delegate is split. Missing profile data defaults to
// 100% delegate commission; disabled delegates stay on the legacy claim path.
//
//   - fork off (NoVoterRewardDistribution)
//   - nil candidate or zero amount
//
// Otherwise it returns splitCommission(amount, snap.EpochCommissionBasisPoints).
// Empty voter metadata still routes the amount to the pending pool; the cursor
// defers that pool until a later era has an eligible snapshot.
// The caller uses the voter portion as the epoch contribution to the
// per-delegate voter drain and the commission portion as the immediate
// per-delegate payout at Phase A.
func (p *Protocol) splitDelegateEpochReward(
	ctx context.Context,
	sm protocol.StateReader,
	cand *state.Candidate,
	amount *big.Int,
) (*big.Int, *big.Int, error) {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return safeBig(amount), new(big.Int), nil
	}
	if cand == nil || isNilOrZero(amount) {
		return safeBig(amount), new(big.Int), nil
	}
	if err := assertNonNegativeReward(amount); err != nil {
		return nil, nil, err
	}
	candidateID := candidateIdentifier(cand)
	candID, err := address.FromString(candidateID)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "rewarding: invalid candidate identity %q", candidateID)
	}
	routing, err := resolveDelegateRewardRouting(ctx, sm, candID)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "rewarding: resolve reward routing for %s", candID.String())
	}
	if !routing.onchainRewardEnabled {
		return safeBig(amount), new(big.Int), nil
	}
	commission, voterShare := splitCommission(amount, routing.epochCommissionBPs)
	return commission, voterShare, nil
}

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
// the whole sum is credited to their reward destination. Every fallback branch
// (no bridge, bridge RPC error, unreadable or ineligible bucket, self-stake
// role changed since the freeze) degrades to a direct credit rather than
// halting the block -- the share is still owed, only its destination changed.
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

	if routing.bucketReader != nil {
		stop := startIIP59Accumulation(&routeDurations.autoDepositLookup)
		bucketID, present, lookupErr := routing.bucketReader.LookupBucket(voter)
		stop()
		addIIP59Items("auto_deposit_lookup", 1)
		switch {
		case lookupErr != nil:
			log.L().Warn("autodeposit bucket lookup failed; routing voter share to credit",
				zap.String("voter", voter.String()), zap.Error(lookupErr))
		case present:
			stopRead := startIIP59Accumulation(&routeDurations.nativeBucketRead)
			bucket, bErr := routing.csr.NativeBucket(bucketID)
			stopRead()
			addIIP59Items("native_bucket_read", 1)
			if bErr != nil {
				log.L().Warn("bucket read for compound routing failed; routing voter share to credit",
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
	recipient, _, _, err := p.resolveVoterRewardDestination(ctx, sm, voter)
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

// compoundErrorIsChainDetermined classifies a failure returned by
// staking.AddDepositForCompound into the only two classes that matter to a
// system action running inside consensus.
//
// The split is NOT "recoverable vs unrecoverable". It is:
//
//	DETERMINED BY CHAIN STATE -> degrade to a direct credit.
//	    Every node executing this block sees the same committed state and
//	    therefore reaches the same verdict. Degrading is safe: proposer and
//	    validators all take the credit branch, produce the same writes, and the
//	    drain cursor advances. The voter is still paid -- only the destination
//	    changes -- so the era's accounting is unaffected.
//
//	INFRASTRUCTURE -> halt the chunk (return the error).
//	    A trie read that failed, a write that failed, a corrupted view. These
//	    are node-local: the proposer's disk can fail where a validator's does
//	    not. Degrading such an error would make the two nodes write different
//	    state from the same block, which is a consensus fork. Halting is loud
//	    and recoverable; the block fails, the operator sees it, and the cursor
//	    does not move.
//
// When in doubt an error goes to the halting class. Under-degrading stalls a
// drain, which an operator can see and fix; over-degrading forks the chain.
//
// The full error surface of AddDepositForCompound and its callees, in the order
// the function can raise them:
//
//	 #  site                                  error                                    class
//	 1  nil voter guard                       "staking: nil voter address"             HALT (unreachable: caller checks)
//	 2  nil amount guard                      "staking: nil compound amount"           HALT (unreachable: caller checks)
//	 3  non-positive amount guard             "staking: non-positive compound amount"  HALT (unreachable: caller checks)
//	 4  NewCandidateStateManagerWithContext   view/state read failure                  HALT  (infrastructure)
//	 5  fetchBucket -> NativeBucket           ErrStateNotExist  -> *handleError
//	                                          (ErrInvalidBucketIndex, 202)             DEGRADE (bucket genuinely absent)
//	 6  fetchBucket -> NativeBucket           any other read failure -> *handleError
//	                                          (ReceiptStatus_Failure, 0)               HALT  (infrastructure)
//	 7  owner re-check                        ErrCompoundBucketOwnerMismatch           DEGRADE (bucket ownership is chain state)
//	 8  GetByIdentifier == nil                errCandNotExist -> *handleError
//	                                          (ErrCandidateNotExist, 102)              DEGRADE (candidate genuinely absent)
//	 9  isSelfStakeBucket -> endorsement
//	    manager Height()/Status()             state read failure                       HALT  (infrastructure)
//	10  frozen-vs-live self-stake guard       ErrCompoundSelfStakeRoleChanged          DEGRADE (role is chain state; pre-existing case)
//	11  csm.updateBucket                      COW snapshot / PutState failure          HALT  (infrastructure)
//	12  subCandidateVotes (Candidate.SubVote) action.ErrInvalidAmount (votes < weight) DEGRADE (accumulator drift is chain state)
//	13  addCandidateVotes (Candidate.AddVote) action.ErrInvalidAmount                  DEGRADE (same)
//	14  candidate.AddSelfStake                action.ErrInvalidAmount                  DEGRADE (same)
//	15  csm.Upsert -> Validate/collision      *handleError via csmErrorToHandleError
//	                                          (ErrCandidateConflict 101 /
//	                                           ErrCandidateNotExist 102)               DEGRADE (name/operator/owner collisions are chain state)
//	16  csm.Upsert -> putCandidate            PutState failure (bare error)            HALT  (infrastructure)
//	17  csm.DebitBucketPool                   PutState failure                         HALT  (infrastructure)
//
// Rows 5, 8 and 15 share one mechanical rule: the staking protocol already
// classified them. Anything it wrapped in a *handleError carrying a SPECIFIC
// receipt status is an error it would have turned into a deterministic failure
// receipt for an equivalent user action -- by construction a verdict every node
// reproduces. The generic ReceiptStatus_Failure (0) carries no such promise;
// row 6 uses it precisely for an unclassified read failure, so it is excluded.
func compoundErrorIsChainDetermined(err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, staking.ErrCompoundSelfStakeRoleChanged): // row 10
		return true
	case errors.Is(err, staking.ErrCompoundBucketOwnerMismatch): // row 7
		return true
	case errors.Is(err, action.ErrInvalidAmount): // rows 12-14
		return true
	}
	// Rows 5, 8, 15. *handleError does not implement Unwrap, so the sentinel it
	// carries is unreachable by errors.Is; its receipt status is the classifier.
	var rErr staking.ReceiptError
	if errors.As(err, &rErr) {
		status := rErr.ReceiptStatus()
		return status != uint64(iotextypes.ReceiptStatus_Failure) &&
			status != uint64(iotextypes.ReceiptStatus_Success)
	}
	return false
}

// voterChunkSettleableError marks a drain-chunk failure that Handle may turn
// into a Failure receipt instead of failing the block.
//
// The default is the opposite: any error out of GrantVoterRewardChunk that is
// NOT marked halts. That inversion is the whole point of the type. Settling a
// Failure receipt writes a consensus-visible outcome -- "this block paid no
// voters and the cursor did not move" -- so it is only sound when every node
// executing the block reaches the same verdict.
//
// The line is not "which layer raised it". It is: is the fact that produced
// this error derivable from chain state that every node shares?
//
//   - Derivable, therefore markable: the dispatcher invariants below. Whether a
//     cursor exists, whether it is already completed, and whether the IIP-59
//     fork is active are all read from committed state and the block's own
//     feature set. Every node reads the same answer, so every node writes the
//     same Failure receipt.
//
//   - Not derivable, therefore never marked: everything the scan and read path
//     can raise. A ranged scan can fail because a particular working set does
//     not support ordered range scans at all -- and the proposer's derived
//     working set and a validator's freshly built one need not agree on that.
//     The proposer would write Failure with no payouts while validators write
//     Success with payouts: same block, two state roots. There is no sentinel
//     to test for either; `ErrNotSupported` exists twice with identical message
//     text (state/factory and db), both are reachable here, and an errors.Is
//     against one silently misses the other. So the capability class is not
//     detected -- it is simply never opted in.
//
// Per-item chain-state conditions inside the chunk (missing bucket, self-stake
// role change, invalid amount) are not affected by any of this: they never
// reach here, because compoundErrorIsChainDetermined above degrades them to a
// direct credit so the voter is paid and the cursor advances.
type voterChunkSettleableError struct{ error }

// settleableVoterChunkError builds an error Handle is allowed to settle as a
// Failure receipt. Use it only for a condition every node derives identically
// from committed state.
func settleableVoterChunkError(format string, args ...interface{}) error {
	return &voterChunkSettleableError{errors.Errorf(format, args...)}
}

// voterChunkErrorIsSettleable reports whether err was explicitly marked
// settleable. Unmarked errors -- including every error from a state read, a
// state write, or a range scan -- must propagate and fail the block.
func voterChunkErrorIsSettleable(err error) bool {
	var target *voterChunkSettleableError
	return errors.As(err, &target)
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
	return staking.FrozenSelfStake{FreezeHeight: work.FreezeHeight, BucketIdx: work.SelfStakeBucketIdx}
}

// delegateChunkLog accumulates the DelegateDistributed rows for one delegate
// across a whole chunk. The drain is voter-major, so a delegate is touched by
// many voters within one block; emitting a log per (voter, delegate) pair would
// multiply the log stream by the average delegate count per voter. One log per
// delegate per block preserves the off-chain aggregation contract, which keys
// on (SnapshotHash, delegate, epoch).
type delegateChunkLog struct {
	voters            []address.Address
	recipients        []address.Address
	amounts           []*big.Int
	compoundBucketIDs []uint64
	compounded        []bool
	paid              *big.Int
}

// recordVoterPayout files one voter's combined payout into the per-delegate
// log rows. The amount recorded against a delegate is that delegate's share,
// not the combined transfer, so the rows still sum to the delegate's pool.
func recordVoterPayout(logs []delegateChunkLog, payout voterCombinedPayout) {
	for _, share := range payout.shares {
		i := share.delegateIndex
		if i < 0 || i >= len(logs) {
			continue
		}
		logs[i].voters = append(logs[i].voters, payout.voter)
		logs[i].recipients = append(logs[i].recipients, payout.recipient)
		logs[i].amounts = append(logs[i].amounts, new(big.Int).Set(share.share))
		logs[i].compoundBucketIDs = append(logs[i].compoundBucketIDs, payout.compoundBucketID)
		logs[i].compounded = append(logs[i].compounded, payout.compounded)
		if logs[i].paid == nil {
			logs[i].paid = new(big.Int)
		}
		logs[i].paid.Add(logs[i].paid, share.share)
	}
}

// voterTransactionLog is the outflow record for a directly credited voter.
// Compounded amounts are not claims from the rewarding fund; they are settled
// once per block through settleCompoundOutflow.
// A compounded payout is identified by payout.compounded, never by a zero
// bucket ID: bucket 0 is a real native bucket, and treating it as "not
// compounded" would emit a CLAIM_FROM_REWARDING_FUND log for money that never
// left the fund toward the voter's account.
func voterTransactionLog(payout voterCombinedPayout) *action.TransactionLog {
	if payout.compounded || payout.recipient == nil || isNilOrZero(payout.amount) {
		return nil
	}
	return &action.TransactionLog{
		Type:      iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND,
		Sender:    address.RewardingPoolAddr,
		Recipient: payout.recipient.String(),
		Amount:    new(big.Int).Set(payout.amount),
	}
}

// packDelegateChunkLog builds the DelegateDistributed event for one delegate's
// slice of this chunk. Returns nil when the delegate paid no voter this block.
func (p *Protocol) packDelegateChunkLog(
	epochNum uint64,
	work epochDrainDelegateWork,
	payee cursorDelegatePayee,
	rows delegateChunkLog,
	blkHeight uint64,
	actionHash hash.Hash256,
) (*action.Log, error) {
	if len(rows.voters) == 0 {
		return nil, nil
	}
	candID, err := address.FromBytes(work.CandidateIdentifier)
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: decode cursor candidate identifier")
	}
	topics, data, err := distributedlog.Pack(distributedlog.EventArgs{
		Epoch:             epochNum,
		Delegate:          candID,
		RewardAddr:        payee.rewardAddr,
		TotalCommission:   safeBig(payee.epochCommission),
		TotalVoterPool:    safeBig(rows.paid),
		SnapshotHash:      hash.BytesToHash256(work.SnapshotHash),
		Voters:            rows.voters,
		Recipients:        rows.recipients,
		Amounts:           rows.amounts,
		CompoundBucketIDs: rows.compoundBucketIDs,
		Compounded:        rows.compounded,
	})
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: pack DelegateDistributed log")
	}
	return &action.Log{
		Address:     p.addr.String(),
		Topics:      topics,
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}, nil
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

// splitCommission returns (commission, voterPool) for a totalReward given
// a basis-points rate. Integer division truncates in favour of the voter
// pool: commission = totalReward * bps / 10_000. Rates above 100% are
// clamped to 100% so a malformed on-chain rate cannot over-pay commission.
func splitCommission(totalReward *big.Int, bps uint64) (*big.Int, *big.Int) {
	if totalReward == nil || totalReward.Sign() == 0 {
		return new(big.Int), new(big.Int)
	}
	if bps == 0 {
		return new(big.Int), new(big.Int).Set(totalReward)
	}
	if bps >= _basisPointsDenom {
		return new(big.Int).Set(totalReward), new(big.Int)
	}
	commission := new(big.Int).Mul(totalReward, new(big.Int).SetUint64(bps))
	commission.Div(commission, new(big.Int).SetUint64(_basisPointsDenom))
	voterPool := new(big.Int).Sub(totalReward, commission)
	return commission, voterPool
}

// assertNonNegativeReward rejects a negative reward amount. Nil is treated
// as zero so callers can opt one of the two streams out of a Phase B call.
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
