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

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
)

const _basisPointsDenom uint64 = 10_000

// delegateRewardRouting is the frozen reward policy used for one delegate in
// the current era. The candidate remains live only for its payout addresses;
// opt-in and commission rates come from the era snapshot.
type delegateRewardRouting struct {
	candidate            *staking.Candidate
	onchainRewardEnabled bool
	blockCommissionBPs   uint64
	epochCommissionBPs   uint64
}

func (r *delegateRewardRouting) PayoutAddress() address.Address {
	if r.onchainRewardEnabled {
		return r.candidate.Owner
	}
	return r.candidate.Reward
}

func resolveDelegateRewardRouting(
	sr protocol.StateReader,
	candID address.Address,
) (*delegateRewardRouting, error) {
	candidate, _, err := staking.NewCandidateByAddressReader(sr).CandidateByAddress(candID)
	if err != nil {
		return nil, err
	}
	routing := &delegateRewardRouting{
		candidate:          candidate,
		blockCommissionBPs: _basisPointsDenom,
		epochCommissionBPs: _basisPointsDenom,
	}

	// Snapshot absence means the delegate was not opted in at this era's
	// freeze. A live opt-in takes effect only when the next snapshot is frozen.
	snap, err := staking.CandidateRewardSnapshotFor(sr, candID)
	switch {
	case err == nil:
		routing.onchainRewardEnabled = true
		routing.blockCommissionBPs = snap.BlockCommissionBasisPoints
		routing.epochCommissionBPs = snap.EpochCommissionBasisPoints
	case errors.Is(err, state.ErrStateNotExist):
		routing.onchainRewardEnabled = false
	default:
		return nil, err
	}
	return routing, nil
}

// splitDelegateEpochReward applies an already-resolved era policy. It performs
// no state reads, so callers can reuse the routing needed for payout address
// selection instead of loading candidate and snapshot state twice.
func splitDelegateEpochReward(
	ctx context.Context,
	amount *big.Int,
	routing *delegateRewardRouting,
) (*big.Int, *big.Int, error) {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution || routing == nil || isNilOrZero(amount) {
		return safeBig(amount), new(big.Int), nil
	}
	if err := assertNonNegativeReward(amount); err != nil {
		return nil, nil, err
	}
	if !routing.onchainRewardEnabled {
		return safeBig(amount), new(big.Int), nil
	}
	commission, voterShare := splitCommission(amount, routing.epochCommissionBPs)
	return commission, voterShare, nil
}

// splitCommission returns (delegate commission, voter pool). Division rounds
// commission down, leaving the remainder to voters. Invalid rates above 100%
// are capped so they cannot overpay the delegate.
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
	return commission, new(big.Int).Sub(totalReward, commission)
}
