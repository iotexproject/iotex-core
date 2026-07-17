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

// delegateAllocation is the pure split of a delegate's block+epoch reward
// streams into commission and voter pool. Produced once at Phase A by
// prepareDelegateAllocation and re-derived at Phase B (deterministic from
// the same frozen snapshot + amounts). Non-nil means the delegate is on
// the IIP-59 path: Phase A grants totalCommission, Phase B distributes
// voterPool across snap.Entries.
type delegateAllocation struct {
	snap            *staking.CandidatePollSnapshot
	totalWeight     *big.Int
	blockCommission *big.Int
	epochCommission *big.Int
	totalCommission *big.Int
	voterPool       *big.Int
	// epochVoterPool is the portion of totalCommission-carve-out that
	// came from the epoch stream. Phase B needs it separately so the
	// unclaimed-balance debit stays split between the two phases —
	// Phase A debits epochCommission, Phase B debits epochVoterPool,
	// sum = epochReward.
	epochVoterPool *big.Int
}

// prepareDelegateAllocation is the pure IIP-59 §3.2 split shared by Phase
// A (commission grant in GrantEpochReward) and Phase B (voter payouts +
// log in GrantVoterRewardChunk). It reads the frozen poll snapshot,
// runs the fork and opt-in / registration checks, and returns:
//
//   - (nil,   nil): fallback path. The fork is off, no snapshot exists,
//     the delegate opted out, the snapshot's DelegateProfile registration
//     is missing, the reward address is nil, or both amounts are zero.
//     Phase A pays the full amount to the reward address via legacy
//     grantToAccount; Phase B skips.
//   - (alloc, nil): IIP-59 path. Commission and voter pool computed.
//     Empty-voter fallback (snap present but voter list empty or total
//     weight zero) yields voterPool = 0 with totalCommission = full
//     amount, so Phase A grants everything as commission and Phase B
//     still emits the batched log with zero voter shares.
//   - (nil,   err): hard failure (state read error, invalid address).
//     Aborts the epoch grant.
func (p *Protocol) prepareDelegateAllocation(
	ctx context.Context,
	sm protocol.StateReader,
	cand *state.Candidate,
	rewardAddr address.Address,
	blockReward *big.Int,
	epochReward *big.Int,
) (*delegateAllocation, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NoVoterRewardDistribution {
		return nil, nil
	}
	if cand == nil {
		return nil, errors.New("rewarding: nil candidate for voter reward distribution")
	}
	if err := assertNonNegativeReward(blockReward); err != nil {
		return nil, err
	}
	if err := assertNonNegativeReward(epochReward); err != nil {
		return nil, err
	}
	if rewardAddr == nil {
		return nil, nil
	}
	if isNilOrZero(blockReward) && isNilOrZero(epochReward) {
		return nil, nil
	}
	candID, err := address.FromString(cand.Address)
	if err != nil {
		return nil, errors.Wrapf(err, "rewarding: invalid candidate address %q", cand.Address)
	}
	snap, err := staking.PollSnapshotFor(sm, candID)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "rewarding: read poll snapshot for %s", candID.String())
	}
	if !snap.VoterRewardOnchainOptIn || !snap.Registered {
		return nil, nil
	}

	totalWeight := new(big.Int)
	for _, e := range snap.Entries {
		if e.Weight != nil {
			totalWeight.Add(totalWeight, e.Weight)
		}
	}

	blockCommission, blockVoterPool := splitCommission(blockReward, snap.BlockCommissionBasisPoints)
	epochCommission, epochVoterPool := splitCommission(epochReward, snap.EpochCommissionBasisPoints)
	totalCommission := new(big.Int).Add(blockCommission, epochCommission)
	voterPool := new(big.Int).Add(blockVoterPool, epochVoterPool)
	if len(snap.Entries) == 0 || totalWeight.Sign() == 0 {
		// Empty-voter fallback: full pool becomes commission. Preserve
		// per-stream epoch attribution so the Phase A / Phase B
		// unclaimed-balance debits still net to epochReward.
		totalCommission = new(big.Int).Add(safeBig(blockReward), safeBig(epochReward))
		epochCommission = safeBig(epochReward)
		blockCommission = safeBig(blockReward)
		voterPool = new(big.Int)
		epochVoterPool = new(big.Int)
	}
	return &delegateAllocation{
		snap:            snap,
		totalWeight:     totalWeight,
		blockCommission: blockCommission,
		epochCommission: epochCommission,
		totalCommission: totalCommission,
		voterPool:       voterPool,
		epochVoterPool:  epochVoterPool,
	}, nil
}

// distributeVoterReward is retained as the epoch-only Phase B entry so
// legacy tests that never accumulate a block-reward pool exercise the
// same allocation → route → emit path used at production drain time.
// Returns (nil, false, nil) whenever prepareDelegateAllocation reports a
// fallback, matching the contract callers rely on to pay via the legacy
// grantToAccount path.
func (p *Protocol) distributeVoterReward(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	totalReward *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, bool, error) {
	return p.distributeVoterOnly(ctx, sm, cand, rewardAddr, nil, totalReward, blkHeight, actionHash)
}

// distributeVoterOnly is IIP-59 §3.2's Phase B: allocate voterPool across
// snap.Entries, route each share to compound or credit via the AutoDeposit
// bridge, and emit a single batched DelegateDistributed log carrying
// totalCommission as attestation. It does NOT grant commission — Phase A
// (GrantEpochReward) is the sole granter of totalCommission to rewardAddr.
//
// Return contract mirrors the prior distributeCombinedReward:
//   - (logs,  true,  nil): IIP-59 path ran. Voter shares granted, log emitted.
//   - (nil,   false, nil): fallback — Phase A already paid this delegate.
//   - (nil,   false, err): hard failure.
//
// Malformed on-chain data (bridge RPC error, bucket read error, ineligible
// bucket) still degrades individual voters to credit rather than halting
// the block; wiring errors (nil staking protocol, log-encoder failure)
// hard-fail.
func (p *Protocol) distributeVoterOnly(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	blockReward *big.Int,
	epochReward *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, bool, error) {
	alloc, err := p.prepareDelegateAllocation(ctx, sm, cand, rewardAddr, blockReward, epochReward)
	if err != nil {
		return nil, false, err
	}
	if alloc == nil {
		return nil, false, nil
	}
	logs, err := p.distributeVoterFromAllocation(ctx, sm, cand, rewardAddr, alloc, blkHeight, actionHash)
	if err != nil {
		return nil, false, err
	}
	return logs, true, nil
}

// distributeVoterFromAllocation is the state-mutating tail of the Phase B
// path: given the deterministic allocation prepared once by
// prepareDelegateAllocation, run the per-voter route + credit loop and
// emit the batched DelegateDistributed log. Kept separate so callers that
// already hold the allocation (runVoterDistributionChunk consuming it for
// unclaimed-balance accounting) do not re-derive it.
func (p *Protocol) distributeVoterFromAllocation(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	alloc *delegateAllocation,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, error) {
	candID, err := address.FromString(cand.Address)
	if err != nil {
		return nil, errors.Wrapf(err, "rewarding: invalid candidate address %q", cand.Address)
	}

	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return nil, errors.New("rewarding: staking protocol not registered")
	}
	var reader autodeposit.ContractReader
	if p.autoDepositBridge != nil {
		reader = p.resolveAutoDepositReader(sm)
	}
	csr, err := staking.ConstructBaseView(sm)
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: construct base view for compound routing")
	}

	voters, weights, amounts, routings, err := p.allocateAndRouteVoters(
		ctx, sm, alloc.snap, alloc.totalWeight, alloc.voterPool, stakingProto, reader, csr, candID,
	)
	if err != nil {
		return nil, err
	}

	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(blkHeight)
	topics, data, err := distributedlog.Pack(distributedlog.EventArgs{
		Epoch:           epochNum,
		Delegate:        candID,
		RewardAddr:      rewardAddr,
		TotalCommission: alloc.totalCommission,
		TotalVoterPool:  alloc.voterPool,
		SnapshotHash:    distributedlog.SnapshotHash(voters, weights),
		Voters:          voters,
		Amounts:         amounts,
		Routings:        routings,
	})
	if err != nil {
		return nil, errors.Wrap(err, "rewarding: pack DelegateDistributed log")
	}
	return []*action.Log{{
		Address:     p.addr.String(),
		Topics:      topics,
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}}, nil
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
	reader autodeposit.ContractReader,
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
		if p.autoDepositBridge != nil {
			bucketID, present, lookupErr := p.autoDepositBridge.LookupBucket(ctx, reader, e.Voter)
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

// resolveAutoDepositReader returns the ContractReader used for a single
// distribution call. Tests may inject a factory via WithAutoDepositReader;
// production falls through to autoDepositContractReader which wraps
// evm.SimulateExecution.
func (p *Protocol) resolveAutoDepositReader(sm protocol.StateManager) autodeposit.ContractReader {
	if p.autoDepositReader != nil {
		return p.autoDepositReader(sm)
	}
	return autoDepositContractReader(sm)
}

// autoDepositContractReader mirrors the equivalent helper in poll/util.go
// (delegateProfileContractReader): build an unsigned Execution against the
// target contract with the zero-address caller and dispatch through
// evm.SimulateExecution. The pattern is deterministic (fixed caller, no
// gas billing) and reuses the existing view-call plumbing.
func autoDepositContractReader(sm protocol.StateManager) autodeposit.ContractReader {
	return autodeposit.ContractReaderFunc(func(ctx context.Context, contract string, callData []byte) ([]byte, error) {
		gasLimit := uint64(10_000_000)
		ex := action.NewExecution(contract, big.NewInt(0), callData)
		caller, err := address.FromString(address.ZeroAddress)
		if err != nil {
			return nil, err
		}
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasLimit(gasLimit).SetAction(ex).Build()
		ret, _, err := evm.SimulateExecution(ctx, sm, caller, elp)
		return ret, err
	})
}
