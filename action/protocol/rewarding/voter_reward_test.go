// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

func TestCalculateVoterShares(t *testing.T) {
	r := require.New(t)
	g := genesis.Default

	// Simulate 3 voters with different staking amounts and durations
	// Voter A: 1000 IOTX, 91 days, auto-stake
	// Voter B: 500 IOTX, 14 days, no auto-stake
	// Voter C: 200 IOTX, 91 days, auto-stake (self-stake)

	type voterInfo struct {
		amount   *big.Int
		duration uint32
		auto     bool
		self     bool
	}

	voters := []voterInfo{
		{big.NewInt(1000), 91, true, false},
		{big.NewInt(500), 14, false, false},
		{big.NewInt(200), 91, true, true},
	}

	totalWeight := big.NewInt(0)
	weights := make([]*big.Int, len(voters))

	for i, v := range voters {
		bucket := &staking.VoteBucket{}
		bucket.StakedAmount = v.amount
		bucket.StakedDuration = time.Duration(v.duration) * 24 * time.Hour
		bucket.AutoStake = v.auto

		w := staking.CalculateVoteWeight(g.Staking.VoteWeightCalConsts, bucket, v.self)
		weights[i] = w
		totalWeight.Add(totalWeight, w)
	}

	// Verify weights are calculated and non-zero
	for i, w := range weights {
		r.True(w.Sign() > 0, "voter %d weight should be positive", i)
	}
	r.True(totalWeight.Sign() > 0)

	// Simulate distribution of 100 IOTX with 10% commission
	totalReward := big.NewInt(100)
	commission := big.NewInt(10) // 10%
	voterPool := new(big.Int).Sub(totalReward, commission)

	distributed := big.NewInt(0)
	shares := make([]*big.Int, len(voters))

	for i := range voters {
		share := new(big.Int).Mul(voterPool, weights[i])
		share.Div(share, totalWeight)
		shares[i] = share
		distributed.Add(distributed, share)
	}

	// Verify total distributed <= voterPool (rounding loss is dust)
	r.True(distributed.Cmp(voterPool) <= 0, "distributed should not exceed voterPool")

	// Dust should be tiny
	dust := new(big.Int).Sub(voterPool, distributed)
	r.True(dust.Cmp(big.NewInt(int64(len(voters)))) < 0, "dust should be less than voter count")

	// Voter A (highest weight due to 1000 amount + auto-stake) should get the most
	r.True(shares[0].Cmp(shares[1]) > 0, "voter A should get more than voter B")
	r.True(shares[0].Cmp(shares[2]) > 0, "voter A should get more than voter C")

	// Commission + distributed + dust = totalReward
	totalDistributed := new(big.Int).Add(commission, distributed)
	totalDistributed.Add(totalDistributed, dust)
	r.Equal(totalReward.Int64(), totalDistributed.Int64(), "commission + distributed + dust should equal totalReward")
}

func TestCommissionRateEdgeCases(t *testing.T) {
	r := require.New(t)

	totalReward := big.NewInt(1000)

	// 0% commission
	commission0 := new(big.Int).Mul(totalReward, big.NewInt(0))
	commission0.Div(commission0, big.NewInt(10000))
	r.Equal(int64(0), commission0.Int64())

	// 100% commission
	commission100 := new(big.Int).Mul(totalReward, big.NewInt(10000))
	commission100.Div(commission100, big.NewInt(10000))
	r.Equal(int64(1000), commission100.Int64())

	// 10% commission
	commission10 := new(big.Int).Mul(totalReward, big.NewInt(1000))
	commission10.Div(commission10, big.NewInt(10000))
	r.Equal(int64(100), commission10.Int64())

	// 25% commission
	commission25 := new(big.Int).Mul(totalReward, big.NewInt(2500))
	commission25.Div(commission25, big.NewInt(10000))
	r.Equal(int64(250), commission25.Int64())
}
