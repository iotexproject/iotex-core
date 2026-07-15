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

// distributeVoterReward is IIP-59 §3.2's per-delegate voter split for the
// epoch-reward stream. It reads the frozen poll snapshot (voter list +
// commission rate + opt-in flag), allocates the epoch share across voters
// in canonical order, routes each share to compound or credit via the
// AutoDeposit bridge, credits the delegate's commission to its reward
// address, and returns exactly one batched DelegateDistributed log per
// invocation.
//
// PR 4' folds in the block-reward stream via distributeCombinedReward,
// which is the entry point GrantEpochReward now uses. This function is
// kept as the pure epoch-only entry so callers that never accumulate a
// block-reward pool (e.g. tests) continue to work unchanged.
//
// Return contract (identical between distributeVoterReward and
// distributeCombinedReward):
//   - (logs,  true,  nil): IIP-59 path ran. Caller MUST NOT run the legacy
//     grantToAccount(rewardAddr, share) for this delegate — the split
//     already handled the full amount.
//   - (nil,   false, nil): fallback to legacy. The fork is off, the delegate
//     opted out, the snapshot's DelegateProfile registration is missing, or
//     no snapshot exists at all. Caller runs the legacy path unchanged.
//   - (nil,   false, err): hard failure (state read error, cross-protocol
//     wiring bug, encoder error). Aborts the epoch grant.
//
// The malformed-on-chain-data fallbacks (bridge RPC error, bucket read
// error, ineligible bucket) all downgrade the affected voter to credit
// rather than halting the block, per feedback-consensus-fallback-vs-halt.
// Wiring errors (nil staking protocol, log-encoder failure) still hard-fail.
func (p *Protocol) distributeVoterReward(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	totalReward *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, bool, error) {
	return p.distributeCombinedReward(ctx, sm, cand, rewardAddr, nil, totalReward, blkHeight, actionHash)
}

// distributeCombinedReward is IIP-59 §3.2 with both reward streams folded
// into one voter split. blockReward is the amount drained from the
// delegate's pending block-reward pool (may be nil / zero); epochReward is
// this epoch's share for the delegate (may be nil / zero). Commissions are
// computed independently against the block and epoch basis-points rates on
// the frozen snapshot, then the voter pools are summed and split once. A
// single DelegateDistributed log covers both streams.
//
// See distributeVoterReward for the (logs, handled, err) contract.
func (p *Protocol) distributeCombinedReward(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	blockReward *big.Int,
	epochReward *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, bool, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NoVoterRewardDistribution {
		return nil, false, nil
	}
	if cand == nil {
		return nil, false, errors.New("rewarding: nil candidate for voter reward distribution")
	}
	if err := assertNonNegativeReward(blockReward); err != nil {
		return nil, false, err
	}
	if err := assertNonNegativeReward(epochReward); err != nil {
		return nil, false, err
	}
	if rewardAddr == nil {
		// Delegate has no reward address configured; nothing to distribute.
		return nil, false, nil
	}
	if isNilOrZero(blockReward) && isNilOrZero(epochReward) {
		// Nothing to distribute for this delegate.
		return nil, false, nil
	}

	candID, err := address.FromString(cand.Address)
	if err != nil {
		return nil, false, errors.Wrapf(err, "rewarding: invalid candidate address %q", cand.Address)
	}

	snap, err := staking.PollSnapshotFor(sm, candID)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			// Pre-fork block, or delegate registered after the last freeze.
			// Rewarding falls back to legacy per PollSnapshotFor's contract.
			return nil, false, nil
		}
		return nil, false, errors.Wrapf(err, "rewarding: read poll snapshot for %s", candID.String())
	}
	if !snap.VoterRewardOnchainOptIn {
		return nil, false, nil
	}
	if !snap.Registered {
		// Opt-in without a DelegateProfile registration is a configuration
		// error, but not one that should halt the chain. The snapshot's
		// Registered=false contract requires the caller to fall back to
		// legacy Hermes distribution.
		return nil, false, nil
	}

	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkHeight)

	// Total-weight sum is used both to prorate voter shares and to detect
	// the "no voter weight known" degenerate case where the full pool
	// becomes commission (IIP-59 §3.2 fallback).
	totalWeight := new(big.Int)
	for _, e := range snap.Entries {
		if e.Weight != nil {
			totalWeight.Add(totalWeight, e.Weight)
		}
	}

	// Independent commission split per stream: block reward uses the
	// snapshot's BlockCommissionBasisPoints; epoch reward uses
	// EpochCommissionBasisPoints. The voter pools are summed and split
	// against the frozen weights below.
	blockCommission, blockVoterPool := splitCommission(blockReward, snap.BlockCommissionBasisPoints)
	epochCommission, epochVoterPool := splitCommission(epochReward, snap.EpochCommissionBasisPoints)
	totalCommission := new(big.Int).Add(blockCommission, epochCommission)
	voterPool := new(big.Int).Add(blockVoterPool, epochVoterPool)
	totalStreams := new(big.Int).Add(safeBig(blockReward), safeBig(epochReward))
	if len(snap.Entries) == 0 || totalWeight.Sign() == 0 {
		// Empty voter list or zero total weight: full pool becomes
		// commission. Still emit a log with empty voter arrays so
		// observers see the delegate's payout.
		totalCommission = totalStreams
		voterPool = new(big.Int)
	}

	// Per-voter allocation + routing. Extracted so the log-emission and
	// commission-grant steps stay in this outer function while the loop
	// logic is testable in isolation.
	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return nil, false, errors.New("rewarding: staking protocol not registered")
	}
	var bucketReader autodeposit.BucketReader
	if p.autoDepositBridge != nil {
		slotReader, srErr := evm.NewSlotReader(ctx, sm)
		if srErr != nil {
			return nil, false, errors.Wrap(srErr, "rewarding: build slot reader for autodeposit")
		}
		bucketReader, err = p.resolveAutoDepositBucketReader(slotReader)
		if err != nil {
			return nil, false, errors.Wrap(err, "rewarding: resolve autodeposit bucket reader")
		}
	}
	csr, err := staking.ConstructBaseView(sm)
	if err != nil {
		return nil, false, errors.Wrap(err, "rewarding: construct base view for compound routing")
	}

	voters, weights, amounts, routings, err := p.allocateAndRouteVoters(
		ctx, sm, snap, totalWeight, voterPool, stakingProto, bucketReader, csr, candID,
	)
	if err != nil {
		return nil, false, err
	}

	if totalCommission.Sign() > 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, totalCommission); err != nil {
			return nil, false, errors.Wrapf(err,
				"rewarding: credit commission to %s failed", rewardAddr.String())
		}
	}

	topics, data, err := distributedlog.Pack(distributedlog.EventArgs{
		Epoch:           epochNum,
		Delegate:        candID,
		RewardAddr:      rewardAddr,
		TotalCommission: totalCommission,
		TotalVoterPool:  voterPool,
		SnapshotHash:    distributedlog.SnapshotHash(voters, weights),
		Voters:          voters,
		Amounts:         amounts,
		Routings:        routings,
	})
	if err != nil {
		return nil, false, errors.Wrap(err, "rewarding: pack DelegateDistributed log")
	}
	return []*action.Log{{
		Address:     p.addr.String(),
		Topics:      topics,
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}}, true, nil
}

// allocateAndRouteVoters splits voterPool across snap.Entries by frozen
// weight and applies the compound/credit routing for each share. Returns
// the parallel slices the caller folds into the DelegateDistributed log.
//
// Determinism: iteration follows the snapshot's canonical order; the last
// weighted voter absorbs the modular-division dust so sum(amounts) equals
// voterPool exactly. All fallback branches (nil bridge, bridge RPC error,
// bucket ineligible) degrade the affected voter to credit rather than
// halting the block.
func (p *Protocol) allocateAndRouteVoters(
	ctx context.Context,
	sm protocol.StateManager,
	snap *staking.CandidatePollSnapshot,
	totalWeight *big.Int,
	voterPool *big.Int,
	stakingProto *staking.Protocol,
	bucketReader autodeposit.BucketReader,
	csr staking.CandidateStateReader,
	candID address.Address,
) ([]address.Address, []*big.Int, []*big.Int, []autodeposit.Route, error) {
	shares := make([]*big.Int, len(snap.Entries))
	if voterPool.Sign() > 0 && totalWeight.Sign() > 0 {
		distributed := new(big.Int)
		for i, e := range snap.Entries {
			if e.Weight == nil || e.Weight.Sign() == 0 {
				shares[i] = new(big.Int)
				continue
			}
			share := new(big.Int).Mul(voterPool, e.Weight)
			share.Div(share, totalWeight)
			shares[i] = share
			distributed.Add(distributed, share)
		}
		if dust := new(big.Int).Sub(voterPool, distributed); dust.Sign() > 0 {
			for i := len(snap.Entries) - 1; i >= 0; i-- {
				if snap.Entries[i].Weight != nil && snap.Entries[i].Weight.Sign() > 0 {
					shares[i] = new(big.Int).Add(shares[i], dust)
					break
				}
			}
		}
	} else {
		for i := range shares {
			shares[i] = new(big.Int)
		}
	}

	voters := make([]address.Address, len(snap.Entries))
	weights := make([]*big.Int, len(snap.Entries))
	amounts := make([]*big.Int, len(snap.Entries))
	routings := make([]autodeposit.Route, len(snap.Entries))
	for i, e := range snap.Entries {
		voters[i] = e.Voter
		if e.Weight != nil {
			weights[i] = new(big.Int).Set(e.Weight)
		} else {
			weights[i] = new(big.Int)
		}
		amounts[i] = shares[i]
		routings[i] = autodeposit.RouteCredit
	}

	for i, e := range snap.Entries {
		share := shares[i]
		if share.Sign() == 0 {
			continue
		}
		if e.Voter == nil {
			// Malformed snapshot entry — should not happen. There is no
			// address to credit, so refuse rather than silently drop
			// the share.
			return nil, nil, nil, nil, errors.Errorf("rewarding: nil voter address at snapshot index %d", i)
		}
		route := autodeposit.RouteCredit
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
						return nil, nil, nil, nil, errors.Wrapf(err,
							"rewarding: compound deposit failed for voter %s bucket %d",
							e.Voter.String(), bucketID)
					}
					route = autodeposit.RouteCompound
				}
			}
		}
		if route == autodeposit.RouteCredit {
			if err := p.grantToAccount(ctx, sm, e.Voter, share); err != nil {
				return nil, nil, nil, nil, errors.Wrapf(err,
					"rewarding: credit voter %s failed", e.Voter.String())
			}
		}
		routings[i] = route
	}
	return voters, weights, amounts, routings, nil
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

// assertNonNegativeReward rejects a nil (only meaningful when it is the
// sole reward stream, checked separately) or negative reward amount.
// distributeCombinedReward allows a nil block or epoch reward so callers
// can opt out of one stream; nil is treated as zero.
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
