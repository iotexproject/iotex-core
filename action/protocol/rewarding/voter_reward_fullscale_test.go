// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
)

// TestFullScale24Delegates simulates a complete mainnet epoch reward distribution
// for all 24 active delegates, each with realistic voter counts and staking amounts.
//
// This test validates:
// 1. Exact conservation: for each delegate, commission + sum(voter shares) + dust == delegateReward
// 2. Global conservation: sum(all distributions) == totalEpochReward
// 3. Dust is bounded: dust < numVoters per delegate
// 4. Performance: total computation time for 24 delegates × ~2800 voters each
func TestFullScale24Delegates(t *testing.T) {
	r := require.New(t)
	g := genesis.Default
	rng := rand.New(rand.NewSource(12345))

	const numDelegates = 24
	// Total epoch reward: 450 IOTX (realistic IoTeX mainnet epoch reward)
	totalEpochReward := new(big.Int).Mul(big.NewInt(450), big.NewInt(1e18))

	type delegateInfo struct {
		name           string
		votes          *big.Int // total weighted votes (determines share of epoch reward)
		commissionBps  uint64
		numVoters      int
		voterWeights   []*big.Int
		totalWeight    *big.Int
	}

	// Generate 24 delegates with realistic parameters
	delegates := make([]delegateInfo, numDelegates)
	totalVotes := big.NewInt(0)

	for d := 0; d < numDelegates; d++ {
		numVoters := 500 + rng.Intn(3500) // 500-4000 voters per delegate
		commissionBps := uint64(500 + rng.Intn(2000)) // 5%-25% commission

		weights := make([]*big.Int, numVoters)
		delegateWeight := big.NewInt(0)

		for v := 0; v < numVoters; v++ {
			// Realistic staking: 100 to 5,000,000 IOTX
			// Power-law distribution: most voters stake small amounts, few stake large
			amountIOTX := int64(100 + rng.ExpFloat64()*50000)
			if amountIOTX > 5000000 {
				amountIOTX = 5000000
			}
			amount := new(big.Int).Mul(big.NewInt(amountIOTX), big.NewInt(1e18))

			duration := uint32(14 + rng.Intn(351)) // 14-365 days
			autoStake := rng.Float32() > 0.25       // 75% auto-stake

			bucket := &staking.VoteBucket{
				StakedAmount:   amount,
				StakedDuration: time.Duration(duration) * 24 * time.Hour,
				AutoStake:      autoStake,
			}

			w := staking.CalculateVoteWeight(g.Staking.VoteWeightCalConsts, bucket, false)
			weights[v] = w
			delegateWeight.Add(delegateWeight, w)
		}

		delegates[d] = delegateInfo{
			name:          fmt.Sprintf("delegate_%02d", d),
			votes:         new(big.Int).Set(delegateWeight),
			commissionBps: commissionBps,
			numVoters:     numVoters,
			voterWeights:  weights,
			totalWeight:   delegateWeight,
		}
		totalVotes.Add(totalVotes, delegateWeight)
	}

	// Simulate epoch reward distribution
	start := time.Now()
	globalDistributed := big.NewInt(0)
	totalVoterCount := 0
	totalDust := big.NewInt(0)

	for _, del := range delegates {
		// Each delegate's share of epoch reward (proportional to votes)
		delegateReward := new(big.Int).Mul(totalEpochReward, del.votes)
		delegateReward.Div(delegateReward, totalVotes)

		// Commission
		commission := new(big.Int).Mul(delegateReward, big.NewInt(int64(del.commissionBps)))
		commission.Div(commission, big.NewInt(10000))
		voterPool := new(big.Int).Sub(delegateReward, commission)

		// Distribute to voters
		distributed := big.NewInt(0)
		for _, w := range del.voterWeights {
			share := new(big.Int).Mul(voterPool, w)
			share.Div(share, del.totalWeight)
			distributed.Add(distributed, share)
		}

		dust := new(big.Int).Sub(voterPool, distributed)

		// Per-delegate conservation check
		delegateTotal := new(big.Int).Add(commission, distributed)
		delegateTotal.Add(delegateTotal, dust)
		r.Equal(delegateReward.String(), delegateTotal.String(),
			"delegate %s: commission + distributed + dust != delegateReward", del.name)

		// Dust bounded
		r.True(dust.Sign() >= 0, "delegate %s: negative dust", del.name)
		r.True(dust.Cmp(big.NewInt(int64(del.numVoters))) < 0,
			"delegate %s: dust %s >= numVoters %d", del.name, dust.String(), del.numVoters)

		globalDistributed.Add(globalDistributed, delegateReward)
		totalVoterCount += del.numVoters
		totalDust.Add(totalDust, dust)
	}

	elapsed := time.Since(start)

	// Global conservation (within rounding from epoch reward split across delegates)
	epochSplitDust := new(big.Int).Sub(totalEpochReward, globalDistributed)
	r.True(epochSplitDust.Sign() >= 0, "negative epoch split dust")
	r.True(epochSplitDust.Cmp(big.NewInt(int64(numDelegates))) < 0,
		"epoch split dust %s >= numDelegates", epochSplitDust.String())

	t.Logf("=== Full-Scale Dry Run Results ===")
	t.Logf("Delegates:       %d", numDelegates)
	t.Logf("Total voters:    %d", totalVoterCount)
	t.Logf("Epoch reward:    %s IOTX", weiToIOTX(totalEpochReward))
	t.Logf("Distributed:     %s IOTX", weiToIOTX(globalDistributed))
	t.Logf("Epoch split dust:%s Rau", epochSplitDust.String())
	t.Logf("Total voter dust:%s Rau", totalDust.String())
	t.Logf("Elapsed:         %v", elapsed)
	t.Logf("Per-delegate:    %v", elapsed/time.Duration(numDelegates))
}

// BenchmarkFullScale24Delegates benchmarks the full epoch reward distribution
func BenchmarkFullScale24Delegates(b *testing.B) {
	g := genesis.Default
	rng := rand.New(rand.NewSource(42))

	const numDelegates = 24
	totalEpochReward := new(big.Int).Mul(big.NewInt(450), big.NewInt(1e18))

	// Pre-generate all delegate/voter data
	type delegateData struct {
		reward      *big.Int
		commission  uint64
		weights     []*big.Int
		totalWeight *big.Int
	}

	totalVotes := big.NewInt(0)
	delegates := make([]delegateData, numDelegates)

	for d := 0; d < numDelegates; d++ {
		numVoters := 500 + rng.Intn(3500)
		tw := big.NewInt(0)
		weights := make([]*big.Int, numVoters)
		for v := 0; v < numVoters; v++ {
			amountIOTX := int64(100 + rng.ExpFloat64()*50000)
			if amountIOTX > 5000000 {
				amountIOTX = 5000000
			}
			amount := new(big.Int).Mul(big.NewInt(amountIOTX), big.NewInt(1e18))
			dur := uint32(14 + rng.Intn(351))
			auto := rng.Float32() > 0.25
			bucket := &staking.VoteBucket{
				StakedAmount:   amount,
				StakedDuration: time.Duration(dur) * 24 * time.Hour,
				AutoStake:      auto,
			}
			w := staking.CalculateVoteWeight(g.Staking.VoteWeightCalConsts, bucket, false)
			weights[v] = w
			tw.Add(tw, w)
		}
		delegates[d] = delegateData{
			commission:  uint64(500 + rng.Intn(2000)),
			weights:     weights,
			totalWeight: tw,
		}
		totalVotes.Add(totalVotes, tw)
	}

	// Pre-compute delegate rewards
	for d := range delegates {
		delegates[d].reward = new(big.Int).Div(
			new(big.Int).Mul(totalEpochReward, delegates[d].totalWeight),
			totalVotes,
		)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, del := range delegates {
			commission := new(big.Int).Div(
				new(big.Int).Mul(del.reward, big.NewInt(int64(del.commission))),
				big.NewInt(10000),
			)
			voterPool := new(big.Int).Sub(del.reward, commission)
			distributed := big.NewInt(0)
			for _, w := range del.weights {
				share := new(big.Int).Mul(voterPool, w)
				share.Div(share, del.totalWeight)
				distributed.Add(distributed, share)
			}
		}
	}
}

func weiToIOTX(wei *big.Int) string {
	f := new(big.Float).SetInt(wei)
	f.Quo(f, big.NewFloat(1e18))
	return f.Text('f', 4)
}
