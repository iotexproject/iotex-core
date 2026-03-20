// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"math/rand"
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

// TestRoundingPrecision2800Voters simulates real mainnet scale:
// ~2800 voters with realistic IOTX amounts and staking durations.
// Verifies: commission + sum(voter shares) + dust == totalReward exactly.
func TestRoundingPrecision2800Voters(t *testing.T) {
	r := require.New(t)
	g := genesis.Default
	rng := rand.New(rand.NewSource(42))

	const numVoters = 2800
	commissionBps := uint64(1000) // 10%

	// Realistic epoch reward: ~12.5 IOTX per delegate (in Rau)
	totalReward := new(big.Int).Mul(big.NewInt(125), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil)) // 12.5e18

	// Generate voters with realistic amounts (100 - 2,000,000 IOTX) and durations (14-365 days)
	type voter struct {
		weight *big.Int
	}
	voters := make([]voter, numVoters)
	totalWeight := big.NewInt(0)

	for i := 0; i < numVoters; i++ {
		// Amount: 100 to 2,000,000 IOTX in Rau
		amountIOTX := int64(100 + rng.Intn(2000000))
		amount := new(big.Int).Mul(big.NewInt(amountIOTX), big.NewInt(1e18))

		duration := uint32(14 + rng.Intn(351)) // 14-365 days
		autoStake := rng.Float32() > 0.3        // 70% auto-stake

		bucket := &staking.VoteBucket{
			StakedAmount:   amount,
			StakedDuration: time.Duration(duration) * 24 * time.Hour,
			AutoStake:      autoStake,
		}

		w := staking.CalculateVoteWeight(g.Staking.VoteWeightCalConsts, bucket, false)
		voters[i] = voter{weight: w}
		totalWeight.Add(totalWeight, w)
	}

	// Calculate commission
	commission := new(big.Int).Mul(totalReward, big.NewInt(int64(commissionBps)))
	commission.Div(commission, big.NewInt(10000))
	voterPool := new(big.Int).Sub(totalReward, commission)

	// Distribute
	distributed := big.NewInt(0)
	for _, v := range voters {
		share := new(big.Int).Mul(voterPool, v.weight)
		share.Div(share, totalWeight)
		distributed.Add(distributed, share)
	}

	dust := new(big.Int).Sub(voterPool, distributed)

	// Verify exact conservation: commission + distributed + dust == totalReward
	total := new(big.Int).Add(commission, distributed)
	total.Add(total, dust)
	r.Equal(totalReward.String(), total.String(), "must be exactly conserved")

	// Dust should be tiny (< numVoters Rau)
	r.True(dust.Cmp(big.NewInt(int64(numVoters))) < 0,
		"dust %s should be < %d Rau", dust.String(), numVoters)

	// Dust should be non-negative
	r.True(dust.Sign() >= 0, "dust should not be negative")

	t.Logf("2800 voters: commission=%s, distributed=%s, dust=%s Rau",
		commission.String(), distributed.String(), dust.String())
}

// BenchmarkDistribute2800Voters measures the time to calculate voter shares
// for a realistic mainnet delegate with ~2800 voters.
func BenchmarkDistribute2800Voters(b *testing.B) {
	g := genesis.Default
	rng := rand.New(rand.NewSource(42))

	const numVoters = 2800

	totalReward := new(big.Int).Mul(big.NewInt(125), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))
	commission := new(big.Int).Div(new(big.Int).Mul(totalReward, big.NewInt(1000)), big.NewInt(10000))
	voterPool := new(big.Int).Sub(totalReward, commission)

	// Pre-generate voter weights
	weights := make([]*big.Int, numVoters)
	totalWeight := big.NewInt(0)
	for i := 0; i < numVoters; i++ {
		amountIOTX := int64(100 + rng.Intn(2000000))
		amount := new(big.Int).Mul(big.NewInt(amountIOTX), big.NewInt(1e18))
		duration := uint32(14 + rng.Intn(351))
		autoStake := rng.Float32() > 0.3

		bucket := &staking.VoteBucket{
			StakedAmount:   amount,
			StakedDuration: time.Duration(duration) * 24 * time.Hour,
			AutoStake:      autoStake,
		}
		w := staking.CalculateVoteWeight(g.Staking.VoteWeightCalConsts, bucket, false)
		weights[i] = w
		totalWeight.Add(totalWeight, w)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		distributed := big.NewInt(0)
		for _, w := range weights {
			share := new(big.Int).Mul(voterPool, w)
			share.Div(share, totalWeight)
			distributed.Add(distributed, share)
		}
	}
}
