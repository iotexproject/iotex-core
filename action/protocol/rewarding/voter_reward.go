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

// distributeVoterReward splits a delegate's epoch reward between the
// delegate's commission and a proportional distribution to its voters,
// following IIP-59.
//
// Inputs come from the rewarding-protocol's per-candidate split in
// GrantEpochReward:
//   - cand: the per-epoch frozen state.Candidate snapshot, written by
//     PutPollResult mid-epoch. cand.CommissionRate is the rate that
//     applies to this distribution (NOT the latest staking.Candidate value
//     — that may have changed mid-epoch and only takes effect at the next
//     PutPollResult snapshot).
//   - rewardAddr: the address that receives the delegate's commission
//     (resolved by splitEpochReward from cand.RewardAddress).
//   - totalReward: the delegate's slice of the epoch reward.
//
// Returns:
//   - (nil, nil) when the IIP-59 feature is off OR when cand.CommissionRate
//     is 0, signalling the caller to fall back to the legacy
//     full-reward-to-delegate path. This is the explicit "opt-in only"
//     contract — pre-fork chains and delegates that never call
//     SetCommissionRate keep exactly today's payout behavior.
//   - (logs, nil) on a successful split, where logs contains one
//     VOTER_REWARD entry per voter (in sorted-voter order, ie. the same
//     order across every node) plus one COMMISSION_REWARD entry for the
//     delegate.
//
// Determinism notes:
//   - The voter slice comes from
//     staking.Protocol.SnapshotVoterWeightsByCandidate, which reads the
//     per-epoch frozen blob written by setCandidates at the prior
//     PutPollResult. Entries are sorted by voter address at write time.
//     We iterate the slice directly — no map iteration anywhere on this
//     path. This is the direct fix for PoC #4811 review finding #2
//     (map-iteration receipt drift).
//   - Reading from the snapshot (not the live view) pairs the voter set
//     with the same epoch-boundary CommissionRate frozen on cand. Live
//     stake activity between PutPollResult and the epoch's last block
//     does NOT shift the distribution.
//   - Rounding dust from the integer division is granted to the delegate's
//     reward account so the sum of all credits is exactly totalReward.
func (p *Protocol) distributeVoterReward(
	ctx context.Context,
	sm protocol.StateManager,
	cand *state.Candidate,
	rewardAddr address.Address,
	totalReward *big.Int,
	blkHeight uint64,
	actionHash hash.Hash256,
) ([]*action.Log, error) {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return nil, nil
	}
	if cand == nil || cand.CommissionRate == 0 {
		// Legacy: no auto-distribution. The caller falls back to the
		// existing grant path.
		return nil, nil
	}
	if rewardAddr == nil || totalReward == nil || totalReward.Sign() <= 0 {
		return nil, nil
	}

	commission := computeCommission(totalReward, cand.CommissionRate)
	voterPool := new(big.Int).Sub(totalReward, commission)

	// Look up the candidate's identifier address for the staking view
	// lookup. The poll snapshot stores it as a string for backward
	// compatibility with pre-IIP-59 RPCs; parse it once here.
	candIdentifier, err := address.FromString(cand.Identity)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid candidate identity %s", cand.Identity)
	}

	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProto == nil {
		return nil, errors.New("staking protocol not registered; cannot distribute voter rewards")
	}
	// Read from the per-epoch frozen snapshot, NOT the live view. The
	// snapshot was written by poll's setCandidates at the previous epoch's
	// mid-epoch PutPollResult — pairing the voter list with the
	// CommissionRate frozen on the same snapshot. Reading live would
	// introduce a stake/commission-rate asymmetry (rate frozen, weights
	// live) that could open arbitrage against late-epoch stake changes.
	voters, err := stakingProto.SnapshotVoterWeightsByCandidate(sm, candIdentifier)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read voter weight snapshot for %s", candIdentifier.String())
	}

	// 100% commission OR no voters: everything (including any voterPool)
	// flows to the delegate. Emit a single COMMISSION_REWARD log.
	if cand.CommissionRate == action.MaxCommissionRate || len(voters) == 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, totalReward); err != nil {
			return nil, err
		}
		log, err := p.encodeRewardLogEntry(rewardingpb.RewardLog_COMMISSION_REWARD, rewardAddr.String(), totalReward, blkHeight, actionHash)
		if err != nil {
			return nil, err
		}
		return []*action.Log{log}, nil
	}

	// First, credit the delegate's commission.
	if commission.Sign() > 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, commission); err != nil {
			return nil, err
		}
	}

	// Sum total weighted votes (one pass).
	totalWeight := new(big.Int)
	for _, v := range voters {
		totalWeight.Add(totalWeight, v.Weight)
	}
	if totalWeight.Sign() == 0 {
		// View said there are voters but every weight is zero — treat as
		// no voters: delegate gets the whole pool.
		if err := p.grantToAccount(ctx, sm, rewardAddr, voterPool); err != nil {
			return nil, err
		}
		total := new(big.Int).Add(commission, voterPool)
		log, err := p.encodeRewardLogEntry(rewardingpb.RewardLog_COMMISSION_REWARD, rewardAddr.String(), total, blkHeight, actionHash)
		if err != nil {
			return nil, err
		}
		return []*action.Log{log}, nil
	}

	// Distribute voterPool proportionally. Walk voters in the sorted order
	// the view gave us — never a map.
	logs := make([]*action.Log, 0, len(voters)+1)
	distributed := new(big.Int)
	for _, v := range voters {
		share := new(big.Int).Mul(voterPool, v.Weight)
		share.Div(share, totalWeight)
		if share.Sign() <= 0 {
			continue
		}
		if err := p.grantToAccount(ctx, sm, v.Voter, share); err != nil {
			return nil, errors.Wrapf(err, "failed to grant voter reward to %s", v.Voter.String())
		}
		distributed.Add(distributed, share)
		voterLog, err := p.encodeRewardLogEntry(rewardingpb.RewardLog_VOTER_REWARD, v.Voter.String(), share, blkHeight, actionHash)
		if err != nil {
			return nil, err
		}
		logs = append(logs, voterLog)
	}

	// Rounding dust = voterPool - sum(shares); always non-negative (integer
	// division rounds down). Grant it to the delegate so the sum of all
	// credits equals totalReward exactly.
	dust := new(big.Int).Sub(voterPool, distributed)
	commissionPlusDust := new(big.Int).Add(commission, dust)
	if dust.Sign() > 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, dust); err != nil {
			return nil, err
		}
	}

	commissionLog, err := p.encodeRewardLogEntry(rewardingpb.RewardLog_COMMISSION_REWARD, rewardAddr.String(), commissionPlusDust, blkHeight, actionHash)
	if err != nil {
		return nil, err
	}
	logs = append(logs, commissionLog)
	return logs, nil
}

// computeCommission returns floor(totalReward * rateBps / 10000) using
// arbitrary-precision arithmetic so high-volume mainnet amounts don't
// overflow a uint64.
func computeCommission(totalReward *big.Int, rateBps uint64) *big.Int {
	if totalReward.Sign() <= 0 || rateBps == 0 {
		return new(big.Int)
	}
	if rateBps >= action.MaxCommissionRate {
		return new(big.Int).Set(totalReward)
	}
	commission := new(big.Int).Mul(totalReward, new(big.Int).SetUint64(rateBps))
	commission.Div(commission, big.NewInt(int64(action.MaxCommissionRate)))
	return commission
}

// encodeRewardLogEntry wraps the existing encodeRewardLog helper into the
// action.Log shape that GrantEpochReward already collects, so callers can
// just append the returned pointer. Kept package-private; voter_reward is
// the only consumer.
func (p *Protocol) encodeRewardLogEntry(typ rewardingpb.RewardLog_RewardType, addr string, amount *big.Int, blkHeight uint64, actionHash hash.Hash256) (*action.Log, error) {
	data, err := p.encodeRewardLog(typ, addr, amount)
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
