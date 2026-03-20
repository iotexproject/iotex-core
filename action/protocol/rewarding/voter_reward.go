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

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// distributeVoterReward splits a delegate's epoch reward between commission and voters.
// Returns the list of reward logs and nil error on success.
// If the delegate has CommissionRate == 0, this function returns (nil, nil, errNoAutoDistribution)
// and the caller should fall back to the legacy full-reward-to-delegate behavior.
//
// IIP-59: Protocol-Native Voter Reward Distribution
func (p *Protocol) distributeVoterReward(
	ctx context.Context,
	sm protocol.StateManager,
	candidateAddr string,
	rewardAddr address.Address,
	totalReward *big.Int,
	blkHeight uint64,
	actionHash [32]byte,
) ([]*action.Log, error) {
	// Find staking protocol to access candidate and bucket data
	stakingProtocol := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	if stakingProtocol == nil {
		// Staking protocol not registered — fall back to legacy
		return nil, nil
	}

	// Look up the staking.Candidate to get CommissionRate
	candIdentifier, err := address.FromString(candidateAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid candidate address %s", candidateAddr)
	}

	cand, err := stakingProtocol.CandidateByIdentifier(sm, candIdentifier)
	if err != nil {
		// Candidate not found or staking view not available — use legacy behavior
		return nil, nil
	}

	if cand.CommissionRate == 0 {
		// Legacy mode: no auto-distribution
		return nil, nil
	}

	// Calculate commission
	commission := new(big.Int).Mul(totalReward, big.NewInt(int64(cand.CommissionRate)))
	commission.Div(commission, big.NewInt(10000))

	// Credit commission to delegate
	if err := p.grantToAccount(ctx, sm, rewardAddr, commission); err != nil {
		return nil, errors.Wrap(err, "failed to grant commission")
	}

	voterPool := new(big.Int).Sub(totalReward, commission)
	if voterPool.Sign() <= 0 {
		// 100% commission, nothing for voters
		data, _ := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr.String(), commission)
		return []*action.Log{{
			Address:     p.addr.String(),
			Data:        data,
			BlockHeight: blkHeight,
			ActionHash:  actionHash,
		}}, nil
	}

	// Get all active buckets for this delegate
	buckets, err := stakingProtocol.ActiveBucketsByCandidate(sm, cand.GetIdentifier())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get active buckets")
	}

	if len(buckets) == 0 {
		// No voters — full reward to delegate as commission
		if err := p.grantToAccount(ctx, sm, rewardAddr, voterPool); err != nil {
			return nil, err
		}
		data, _ := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr.String(), totalReward)
		return []*action.Log{{
			Address:     p.addr.String(),
			Data:        data,
			BlockHeight: blkHeight,
			ActionHash:  actionHash,
		}}, nil
	}

	// Calculate weighted votes for each voter using staking protocol's config
	voteWeightConsts := stakingProtocol.VoteWeightCalConsts()

	// Aggregate by voter address to avoid per-bucket rounding loss
	voterMap := make(map[string]*big.Int)
	voterAddrs := make(map[string]address.Address)
	totalWeight := big.NewInt(0)

	for _, b := range buckets {
		isSelfStake := b.Index == cand.SelfStakeBucketIdx
		w := staking.CalculateVoteWeight(voteWeightConsts, b, isSelfStake)
		totalWeight.Add(totalWeight, w)

		ownerStr := b.Owner.String()
		voterAddrs[ownerStr] = b.Owner
		if existing, ok := voterMap[ownerStr]; ok {
			voterMap[ownerStr] = new(big.Int).Add(existing, w)
		} else {
			voterMap[ownerStr] = new(big.Int).Set(w)
		}
	}

	if totalWeight.Sign() == 0 {
		// Zero total weight — give everything to delegate
		if err := p.grantToAccount(ctx, sm, rewardAddr, voterPool); err != nil {
			return nil, err
		}
		data, _ := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr.String(), totalReward)
		return []*action.Log{{
			Address:     p.addr.String(),
			Data:        data,
			BlockHeight: blkHeight,
			ActionHash:  actionHash,
		}}, nil
	}

	// Distribute voterPool proportionally
	var rewardLogs []*action.Log
	distributed := big.NewInt(0)

	for addrStr, weight := range voterMap {
		share := new(big.Int).Mul(voterPool, weight)
		share.Div(share, totalWeight)

		if share.Sign() <= 0 {
			continue
		}

		addr := voterAddrs[addrStr]
		if err := p.grantToAccount(ctx, sm, addr, share); err != nil {
			return nil, errors.Wrapf(err, "failed to grant voter reward to %s", addrStr)
		}
		distributed.Add(distributed, share)

		data, err := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, addrStr, share)
		if err != nil {
			return nil, err
		}
		rewardLogs = append(rewardLogs, &action.Log{
			Address:     p.addr.String(),
			Data:        data,
			BlockHeight: blkHeight,
			ActionHash:  actionHash,
		})
	}

	// Rounding dust goes to delegate
	dust := new(big.Int).Sub(voterPool, distributed)
	if dust.Sign() > 0 {
		if err := p.grantToAccount(ctx, sm, rewardAddr, dust); err != nil {
			return nil, err
		}
	}

	// Add commission log
	data, err := p.encodeRewardLog(rewardingpb.RewardLog_EPOCH_REWARD, rewardAddr.String(), new(big.Int).Add(commission, dust))
	if err != nil {
		return nil, err
	}
	rewardLogs = append(rewardLogs, &action.Log{
		Address:     p.addr.String(),
		Data:        data,
		BlockHeight: blkHeight,
		ActionHash:  actionHash,
	})

	return rewardLogs, nil
}
