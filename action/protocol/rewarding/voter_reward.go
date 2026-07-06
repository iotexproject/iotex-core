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

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
)

// commissionRateDenominator is IIP-59's basis-points denominator; a
// commission rate of `n` means the delegate keeps n/10000 of the epoch
// reward, and the remaining (10000-n)/10000 flows to voters.
const commissionRateDenominator uint64 = 10000

// distributeVoterReward is IIP-59's per-delegate epoch reward split.
//
// It performs the following, in order:
//  1. reads the delegate's per-voter weights out of the frozen per-epoch
//     snapshot written by PR 2's SnapshotForEpochReward. The slice is
//     already sorted by voter address, so iteration order is deterministic.
//  2. moves the delegate's commission (rate * totalReward / 10000) to
//     rewardAddr.
//  3. distributes the remaining voterPool proportionally to voters,
//     emitting one VOTER_REWARD log per non-zero share.
//  4. folds the truncation dust from step 3 back to the delegate and
//     emits a single DELEGATE_COMMISSION log covering commission + dust.
//
// Return values:
//   - (logs, true, nil)  — IIP-59 path; distributeVoterReward has already
//     credited every recipient. The caller must NOT call grantToAccount
//     again for rewardAddr — just append the returned logs.
//   - (nil, false, nil)  — legacy fallback; the caller runs the pre-IIP-59
//     path (grantToAccount(rewardAddr, totalReward) + EPOCH_REWARD log).
//   - (nil, false, err)  — hard failure; propagates to block execution.
//
// Legacy fallback triggers:
//   - featureCtx.NoVoterRewardDistribution == true (pre-fork).
//   - cand.CommissionRate == 0 (delegate has not opted in; keep today's
//     100%-to-delegate behavior).
//   - cand.Identity is empty (schema pre-migration remnants). A non-empty
//     but unparseable Identity is treated as state corruption and errors.
func (p *Protocol) distributeVoterReward(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	totalReward *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, bool, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.NoVoterRewardDistribution {
		return nil, false, nil
	}
	if cand == nil || cand.CommissionRate == 0 {
		return nil, false, nil
	}
	if cand.CommissionRate > commissionRateDenominator {
		return nil, false, errors.Errorf(
			"invalid commission rate %d for candidate %s", cand.CommissionRate, cand.Identity)
	}
	if cand.Identity == "" {
		return nil, false, nil
	}
	candID, err := address.FromString(cand.Identity)
	if err != nil {
		return nil, false, errors.Wrapf(err,
			"failed to parse candidate identity %q", cand.Identity)
	}
	if rewardAddr == nil {
		return nil, false, errors.Errorf(
			"candidate %s has no reward address", cand.Identity)
	}

	// commission (unrounded floor) + voterPool = totalReward, exactly.
	commission := new(big.Int).Div(
		new(big.Int).Mul(totalReward, new(big.Int).SetUint64(cand.CommissionRate)),
		new(big.Int).SetUint64(commissionRateDenominator),
	)
	voterPool := new(big.Int).Sub(totalReward, commission)

	voters, err := staking.VoterWeightsFromSnapshot(sm, candID)
	if err != nil {
		return nil, false, errors.Wrapf(err,
			"failed to read voter weight snapshot for candidate %s", cand.Identity)
	}
	totalWeight := big.NewInt(0)
	for _, v := range voters {
		if v.Weight != nil && v.Weight.Sign() > 0 {
			totalWeight = new(big.Int).Add(totalWeight, v.Weight)
		}
	}

	// Degenerate cases where the delegate takes everything:
	//   - rate == 100% (voterPool == 0 by construction)
	//   - no voters / zero total weight — no legitimate voter recipient.
	if voterPool.Sign() == 0 || totalWeight.Sign() == 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, totalReward); err != nil {
			return nil, false, err
		}
		commissionLog, err := p.buildRewardLog(
			rewardingpb.RewardLog_DELEGATE_COMMISSION, rewardAddr, totalReward, blkHeight, actionHash)
		if err != nil {
			return nil, false, err
		}
		return []*action.Log{commissionLog}, true, nil
	}

	logs := make([]*action.Log, 0, len(voters)+1)

	if commission.Sign() > 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, commission); err != nil {
			return nil, false, err
		}
	}

	distributed := big.NewInt(0)
	for _, v := range voters {
		if v.Voter == nil || v.Weight == nil || v.Weight.Sign() == 0 {
			continue
		}
		share := new(big.Int).Div(new(big.Int).Mul(voterPool, v.Weight), totalWeight)
		if share.Sign() == 0 {
			continue
		}
		if err := p.grantToAccount(ctx, sm, v.Voter, share); err != nil {
			return nil, false, err
		}
		voterLog, err := p.buildRewardLog(
			rewardingpb.RewardLog_VOTER_REWARD, v.Voter, share, blkHeight, actionHash)
		if err != nil {
			return nil, false, err
		}
		logs = append(logs, voterLog)
		distributed = new(big.Int).Add(distributed, share)
	}

	// Fold truncation dust back to the delegate.
	dust := new(big.Int).Sub(voterPool, distributed)
	if dust.Sign() > 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, dust); err != nil {
			return nil, false, err
		}
	}
	delegatePayout := new(big.Int).Add(commission, dust)
	if delegatePayout.Sign() > 0 {
		commissionLog, err := p.buildRewardLog(
			rewardingpb.RewardLog_DELEGATE_COMMISSION, rewardAddr, delegatePayout, blkHeight, actionHash)
		if err != nil {
			return nil, false, err
		}
		logs = append(logs, commissionLog)
	}
	return logs, true, nil
}

func (p *Protocol) buildRewardLog(
	typ rewardingpb.RewardLog_RewardType,
	addr address.Address,
	amount *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) (*action.Log, error) {
	data, err := p.encodeRewardLog(typ, addr.String(), amount)
	if err != nil {
		return nil, err
	}
	return &action.Log{
		Address:     p.addr.String(),
		Topics:      nil,
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	}, nil
}
