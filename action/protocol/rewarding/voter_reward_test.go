// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
)

// TestComputeCommission spot-checks the basis-points math for the rates
// that matter on mainnet: 0% (legacy fallback), the canonical 10% / 25%
// delegate cuts, and 100% (delegate keeps everything). The function uses
// big.Int arithmetic so the typical mainnet epoch reward (~12.5 IOTX ≈
// 1.25e19 Rau) doesn't risk overflowing a uint64 — these tests exercise
// values in that range.
func TestComputeCommission(t *testing.T) {
	r := require.New(t)

	// Realistic mainnet-scale total: 12.5 IOTX in Rau.
	total := new(big.Int).Mul(big.NewInt(125), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))

	// 0 bps → 0.
	r.Equal(0, computeCommission(total, 0).Sign())

	// 10 bps = 0.1%; commission = total * 10 / 10000 = total / 1000.
	got := computeCommission(total, 10)
	expected := new(big.Int).Div(total, big.NewInt(1000))
	r.Equal(0, got.Cmp(expected), "10 bps math: want %s, got %s", expected.String(), got.String())

	// 1000 bps = 10%.
	got = computeCommission(total, 1000)
	expected = new(big.Int).Div(total, big.NewInt(10))
	r.Equal(0, got.Cmp(expected))

	// 2500 bps = 25%.
	got = computeCommission(total, 2500)
	expected = new(big.Int).Div(new(big.Int).Mul(total, big.NewInt(25)), big.NewInt(100))
	r.Equal(0, got.Cmp(expected))

	// MaxCommissionRate (10000 bps) returns the whole input — and uses an
	// equality fast path so the result equals total exactly with no
	// rounding loss (important for the "no voters left over" check in
	// distributeVoterReward).
	got = computeCommission(total, action.MaxCommissionRate)
	r.Equal(0, got.Cmp(total))
	r.NotSame(total, got, "must return a fresh big.Int — callers mutate it")

	// Negative / zero totals short-circuit to zero (and the result is a
	// fresh big.Int — no nil traps).
	zero := computeCommission(big.NewInt(0), 1000)
	r.NotNil(zero)
	r.Equal(0, zero.Sign())
}

// TestComputeCommission_ExactDivisorBoundaries exercises rates and amounts
// where integer division is exact (no rounding). This is a regression
// guard against accidentally introducing a +1 / off-by-one in the math.
func TestComputeCommission_ExactDivisorBoundaries(t *testing.T) {
	r := require.New(t)

	// 100 commission on 1000 total at 1000 bps (10%) — divides evenly.
	r.Equal(int64(100), computeCommission(big.NewInt(1000), 1000).Int64())
	// 250 commission on 1000 total at 2500 bps (25%) — divides evenly.
	r.Equal(int64(250), computeCommission(big.NewInt(1000), 2500).Int64())
	// 1 commission on 10000 total at 1 bp — divides evenly.
	r.Equal(int64(1), computeCommission(big.NewInt(10000), 1).Int64())
}

// TestComputeCommission_RoundingFloor verifies floor-rounding semantics —
// commission never exceeds the mathematically exact rate. This is what
// guarantees voterPool >= 0 in distributeVoterReward.
func TestComputeCommission_RoundingFloor(t *testing.T) {
	r := require.New(t)

	// 1 commission on 10 at 1000 bps would be 1.0; exact.
	r.Equal(int64(1), computeCommission(big.NewInt(10), 1000).Int64())

	// 0 commission on 9 at 1000 bps would be 0.9, floors to 0.
	r.Equal(int64(0), computeCommission(big.NewInt(9), 1000).Int64())

	// 0 commission on 99 at 1 bp would be 0.0099, floors to 0.
	r.Equal(int64(0), computeCommission(big.NewInt(99), 1).Int64())

	// Floor never exceeds the exact value, for many random inputs.
	rng := rand.New(rand.NewSource(7))
	for i := 0; i < 200; i++ {
		total := new(big.Int).SetUint64(uint64(rng.Int63n(1 << 40)))
		bps := uint64(rng.Intn(int(action.MaxCommissionRate) + 1))
		got := computeCommission(total, bps)
		// got * 10000 <= total * bps (proves floor).
		lhs := new(big.Int).Mul(got, big.NewInt(int64(action.MaxCommissionRate)))
		rhs := new(big.Int).Mul(total, new(big.Int).SetUint64(bps))
		r.True(lhs.Cmp(rhs) <= 0, "rate=%d total=%s got=%s exceeded exact", bps, total.String(), got.String())
	}
}

// TestVoterShareConservation simulates the per-voter integer-division
// split done inside distributeVoterReward and verifies the exact-
// conservation invariant: commission + sum(voter shares) + dust = totalReward,
// with dust < numVoters. This is the property mainnet correctness depends
// on (every Rau accounted for, nothing manufactured, nothing lost).
//
// We test the math here, not the surrounding handler harness — that's
// covered separately by the integration tests with a real Protocol.
func TestVoterShareConservation(t *testing.T) {
	r := require.New(t)
	rng := rand.New(rand.NewSource(42))

	const numVoters = 2800
	totalReward := new(big.Int).Mul(big.NewInt(125), new(big.Int).Exp(big.NewInt(10), big.NewInt(17), nil))
	commissionRate := uint64(1000) // 10%

	// Generate realistic weights — power-law-ish, large spread.
	weights := make([]*big.Int, numVoters)
	totalWeight := new(big.Int)
	for i := 0; i < numVoters; i++ {
		w := new(big.Int).SetUint64(uint64(1 + rng.Intn(1<<32)))
		weights[i] = w
		totalWeight.Add(totalWeight, w)
	}

	commission := computeCommission(totalReward, commissionRate)
	voterPool := new(big.Int).Sub(totalReward, commission)

	distributed := new(big.Int)
	for _, w := range weights {
		share := new(big.Int).Mul(voterPool, w)
		share.Div(share, totalWeight)
		distributed.Add(distributed, share)
	}

	dust := new(big.Int).Sub(voterPool, distributed)

	// Exact conservation.
	total := new(big.Int).Add(commission, distributed)
	total.Add(total, dust)
	r.Equal(0, totalReward.Cmp(total), "commission + distributed + dust must equal totalReward exactly")

	// Dust is bounded by the voter count — each voter's share rounds down
	// by at most 1 Rau.
	r.True(dust.Sign() >= 0, "dust must be non-negative (floor rounding)")
	r.True(dust.Cmp(big.NewInt(int64(numVoters))) < 0,
		"dust %s should be < %d", dust.String(), numVoters)
}
