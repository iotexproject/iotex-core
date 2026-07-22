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
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/distributedlog"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
)

const _basisPointsDenom uint64 = 10_000

// splitDelegateEpochReward computes the epoch-reward commission / voter
// split for a delegate. It returns (amount, 0) — commission takes the
// full amount, voter share is zero — on any of:
//
//   - fork off (NoVoterRewardDistribution)
//   - nil candidate or zero amount
//   - no frozen poll snapshot for the candidate
//   - snapshot opted out or not registered
//   - empty voter list or total voter weight = 0
//
// Otherwise it returns splitCommission(amount, snap.EpochCommissionBasisPoints).
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
	snap, err := staking.PollSnapshotFor(sm, candID)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return safeBig(amount), new(big.Int), nil
		}
		return nil, nil, errors.Wrapf(err, "rewarding: read poll snapshot for %s", candID.String())
	}
	if !snap.VoterRewardOnchainOptIn || !snap.Registered {
		return safeBig(amount), new(big.Int), nil
	}
	totalWeight := safeBig(snap.TotalWeight)
	if len(snap.Entries) == 0 || totalWeight.Sign() == 0 {
		return safeBig(amount), new(big.Int), nil
	}
	commission, voterShare := splitCommission(amount, snap.EpochCommissionBasisPoints)
	return commission, voterShare, nil
}

// distributeVoterOnly is IIP-59 §3.2's Phase B: allocate the delegate's
// frozen voter share (voterAmount) across the FULL snap.Entries list,
// then pay only voters in the [startVoter, startVoter+voterBudget)
// window; route each paid share to compound or credit via the AutoDeposit
// bridge, and emit a batched DelegateDistributed log with epochCommission
// attesting to the Phase A per-delegate commission.
//
// voterBudget == 0 means "no per-block voter cap for this call" — pay
// through the end of the list.
//
// Allocation is deterministic across chunks: the same snap.Entries +
// weights + voterAmount always produce the same share array. Splitting a
// delegate across K blocks produces byte-identical per-voter amounts to
// the un-split single-block run; the last-with-weight voter absorbs the
// dust once, whichever chunk reaches them.
//
// Post-C3, block-side commission is observable via the per-block
// BLOCK_REWARD logs and is intentionally omitted from TotalCommission;
// consumers who want an era total sum both streams themselves.
//
// Return contract:
//   - (logs, routed=true, paidThisChunk, compoundedThisChunk, consumedVoters, totalVoters, nil):
//     window paid, log emitted with the window's voters/amounts.
//   - (nil, routed=false, nil, nil, 0, 0, nil): snapshot missing or delegate
//     opted out / unregistered between Phase A and this chunk. Caller
//     advances past this delegate; the coda's orphan sweep drains the
//     residual pool entry.
//   - (nil, false, nil, nil, 0, 0, err): hard failure.
//
// Malformed on-chain data (bridge RPC error, bucket read error, ineligible
// bucket) still degrades individual voters to credit rather than halting
// the block; wiring errors (nil staking protocol, log-encoder failure)
// hard-fail.
//
// Post-5.5b the log's `TotalVoterPool` reflects the sum of `Amounts[]`
// paid in *this* chunk, not the delegate's era-wide frozen amount. Off-
// chain consumers must aggregate partial logs by (SnapshotHash, delegate,
// epoch) to recover era-wide totals.
func (p *Protocol) distributeVoterOnly(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	voterAmount *big.Int,
	epochCommission *big.Int,
	distributedBefore *big.Int,
	startVoter uint32,
	voterBudget uint32,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, bool, *big.Int, *big.Int, uint32, uint32, error) {
	if cand == nil || rewardAddr == nil {
		return nil, false, nil, nil, 0, 0, nil
	}
	if err := assertNonNegativeReward(voterAmount); err != nil {
		return nil, false, nil, nil, 0, 0, err
	}
	distributed := safeBig(distributedBefore)
	if err := assertNonNegativeReward(distributed); err != nil {
		return nil, false, nil, nil, 0, 0, errors.Wrap(err, "rewarding: invalid distributed voter amount")
	}
	if distributed.Cmp(safeBig(voterAmount)) > 0 {
		return nil, false, nil, nil, 0, 0, errors.New("rewarding: distributed voter amount exceeds frozen pool")
	}
	candidateID := candidateIdentifier(cand)
	candID, err := address.FromString(candidateID)
	if err != nil {
		return nil, false, nil, nil, 0, 0, errors.Wrapf(err, "rewarding: invalid candidate identity %q", candidateID)
	}
	snap, err := staking.PollSnapshotFor(sm, candID)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, false, nil, nil, 0, 0, nil
		}
		return nil, false, nil, nil, 0, 0, errors.Wrapf(err, "rewarding: read poll snapshot for %s", candID.String())
	}
	if !snap.VoterRewardOnchainOptIn || !snap.Registered {
		return nil, false, nil, nil, 0, 0, nil
	}
	totalVoters := uint32(len(snap.Entries))
	// endVoter is clamped to the list; a startVoter past the end is a
	// no-op window (0 voters paid), which is a legal "delegate is done"
	// state — the caller advances past this delegate.
	if startVoter > totalVoters {
		startVoter = totalVoters
	}
	endVoter := totalVoters
	if voterBudget > 0 && startVoter+voterBudget < endVoter {
		endVoter = startVoter + voterBudget
	}
	totalWeight := safeBig(snap.TotalWeight)

	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return nil, false, nil, nil, 0, 0, errors.New("rewarding: staking protocol not registered")
	}
	var bucketReader autodeposit.BucketReader
	if p.autoDepositBridge != nil {
		slotReader, srErr := evm.NewSlotReader(ctx, sm)
		if srErr != nil {
			return nil, false, nil, nil, 0, 0, errors.Wrap(srErr, "rewarding: build slot reader for autodeposit")
		}
		bucketReader, err = p.resolveAutoDepositBucketReader(slotReader)
		if err != nil {
			return nil, false, nil, nil, 0, 0, errors.Wrap(err, "rewarding: resolve autodeposit bucket reader")
		}
	}
	csr, err := staking.ConstructBaseView(sm)
	if err != nil {
		return nil, false, nil, nil, 0, 0, errors.Wrap(err, "rewarding: construct base view for compound routing")
	}

	voters, amounts, compoundBucketIDs, paid, compounded, err := p.allocateAndRouteVoters(
		ctx, sm, snap, totalWeight, safeBig(voterAmount), distributed,
		startVoter, endVoter,
		stakingProto, bucketReader, csr, candID,
	)
	if err != nil {
		return nil, false, nil, nil, 0, 0, err
	}

	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkHeight)
	// SnapshotHash covers the full frozen weight list, so it's stable
	// across chunks — off-chain consumers assemble partial logs by
	// (SnapshotHash, delegate, epoch).
	topics, data, err := distributedlog.Pack(distributedlog.EventArgs{
		Epoch:             epochNum,
		Delegate:          candID,
		RewardAddr:        rewardAddr,
		TotalCommission:   safeBig(epochCommission),
		TotalVoterPool:    safeBig(paid),
		SnapshotHash:      snapshotHashFull(snap),
		Voters:            voters,
		Amounts:           amounts,
		CompoundBucketIDs: compoundBucketIDs,
	})
	if err != nil {
		return nil, false, nil, nil, 0, 0, errors.Wrap(err, "rewarding: pack DelegateDistributed log")
	}
	return []*action.Log{{
		Address:     p.addr.String(),
		Topics:      topics,
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}}, true, paid, compounded, endVoter - startVoter, totalVoters, nil
}

// snapshotHashFull computes the deterministic SnapshotHash over the full
// frozen (voter, weight) list. Unlike a per-chunk hash, this value is
// stable across chunks and lets off-chain consumers assemble partial
// DelegateDistributed logs by (SnapshotHash, delegate, epoch).
func snapshotHashFull(snap *staking.CandidatePollSnapshot) hash.Hash256 {
	if snap == nil {
		return hash.ZeroHash256
	}
	if snap.SnapshotHash != hash.ZeroHash256 {
		return snap.SnapshotHash
	}
	voters := make([]address.Address, len(snap.Entries))
	weights := make([]*big.Int, len(snap.Entries))
	for i, e := range snap.Entries {
		voters[i] = e.Voter
		if e.Weight != nil {
			weights[i] = new(big.Int).Set(e.Weight)
		} else {
			weights[i] = new(big.Int)
		}
	}
	return distributedlog.SnapshotHash(voters, weights)
}

// allocateAndRouteVoters splits voterPool across snap.Entries by frozen
// weight and applies the compound/credit routing for the voters in the
// [startVoter, endVoter) window. Returns the parallel slices the caller
// folds into the DelegateDistributed log — restricted to the window —
// plus the sum of amounts actually paid this chunk.
//
// Allocation enumerates the full snap.Entries every call so per-voter
// shares are deterministic across chunks: splitting one delegate across
// K blocks produces byte-identical amounts to a single-block run. The
// last weighted voter absorbs the modular-division dust so sum(shares)
// over the full list equals voterPool exactly; the dust lands in
// whichever chunk contains the last-weighted voter.
//
// All fallback branches (nil bridge, bridge RPC error, bucket ineligible)
// degrade the affected voter to credit rather than halting the block.
func (p *Protocol) allocateAndRouteVoters(
	ctx context.Context,
	sm protocol.StateManager,
	snap *staking.CandidatePollSnapshot,
	totalWeight *big.Int,
	voterPool *big.Int,
	distributedBefore *big.Int,
	startVoter uint32,
	endVoter uint32,
	stakingProto *staking.Protocol,
	bucketReader autodeposit.BucketReader,
	csr staking.CandidateStateReader,
	candID address.Address,
) ([]address.Address, []*big.Int, []uint64, *big.Int, *big.Int, error) {
	// Clamp the payout window to the frozen list.
	total := uint32(len(snap.Entries))
	if startVoter > total {
		startVoter = total
	}
	if endVoter > total {
		endVoter = total
	}
	if endVoter < startVoter {
		endVoter = startVoter
	}
	winLen := int(endVoter - startVoter)

	voters := make([]address.Address, winLen)
	amounts := make([]*big.Int, winLen)
	compoundBucketIDs := make([]uint64, winLen)
	paid := new(big.Int)
	compounded := new(big.Int)
	distributed := new(big.Int).Set(distributedBefore)

	for j := 0; j < winLen; j++ {
		i := int(startVoter) + j
		e := snap.Entries[i]
		voters[j] = e.Voter
		share := new(big.Int)
		if voterPool.Sign() > 0 && totalWeight.Sign() > 0 && e.Weight != nil && e.Weight.Sign() > 0 {
			if snap.HasWeightedEntries && uint32(i) == snap.LastWeightedIndex {
				share.Sub(voterPool, distributed)
				if share.Sign() < 0 {
					return nil, nil, nil, nil, nil, errors.New("rewarding: distributed voter amount exceeds frozen pool")
				}
			} else {
				share.Mul(voterPool, e.Weight)
				share.Div(share, totalWeight)
			}
		}
		amounts[j] = share
		distributed.Add(distributed, share)
		if share.Sign() == 0 {
			continue
		}
		if e.Voter == nil {
			// Malformed snapshot entry — should not happen. There is no
			// address to credit, so refuse rather than silently drop
			// the share.
			return nil, nil, nil, nil, nil, errors.Errorf("rewarding: nil voter address at snapshot index %d", i)
		}
		compoundBucketID := uint64(0)
		if bucketReader != nil {
			bucketID, present, lookupErr := bucketReader.LookupBucket(e.Voter)
			if lookupErr != nil {
				log.L().Warn("autodeposit bucket lookup failed; routing voter share to credit",
					zap.String("delegate", candID.String()),
					zap.String("voter", e.Voter.String()),
					zap.Error(lookupErr))
			} else if present {
				bucket, bErr := csr.NativeBucket(bucketID)
				if bErr != nil {
					log.L().Warn("bucket read for compound routing failed; routing voter share to credit",
						zap.String("delegate", candID.String()),
						zap.String("voter", e.Voter.String()),
						zap.Uint64("bucket", bucketID),
						zap.Error(bErr))
				} else if autodeposit.IsBucketEligibleForCompound(bucket, e.Voter) {
					if err := stakingProto.AddDepositForCompound(ctx, sm, e.Voter, bucketID, share); err != nil {
						return nil, nil, nil, nil, nil, errors.Wrapf(err,
							"rewarding: compound deposit failed for voter %s bucket %d",
							e.Voter.String(), bucketID)
					}
					compoundBucketID = bucketID
					compounded.Add(compounded, share)
				}
			}
		}
		if compoundBucketID == 0 {
			if err := p.grantToAccount(ctx, sm, e.Voter, share); err != nil {
				return nil, nil, nil, nil, nil, errors.Wrapf(err,
					"rewarding: credit voter %s failed", e.Voter.String())
			}
		}
		compoundBucketIDs[j] = compoundBucketID
		paid.Add(paid, share)
	}
	return voters, amounts, compoundBucketIDs, paid, compounded, nil
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
