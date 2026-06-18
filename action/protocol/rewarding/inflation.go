// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import "math/big"

// Pure inflation math for IIP-62 "Productive Inflation". All functions in this file
// are side-effect-free and operate on math/big.Int — the high-year compounded ratios
// (e.g. 8000^11 ≈ 8.6e42) overflow 64-bit. The state-mutating wrapper lives in
// inflation_state.go.

const bpsDenom = 10000

// YearIndex returns the 1-indexed Year that contains height under the IIP-62 curve.
// Year 1 covers [activation, activation+blocksPerYear); Year 2 covers
// [activation+blocksPerYear, activation+2*blocksPerYear); and so on. Returns 0 for
// heights strictly before activation (i.e. pre-activation; mint should not run).
func YearIndex(activation, blocksPerYear, height uint64) uint64 {
	if height < activation || blocksPerYear == 0 {
		return 0
	}
	return (height-activation)/blocksPerYear + 1
}

// IsYearBoundary reports whether height is the first block of a new Year ≥ 2.
// The activation block itself is the start of Year 1 and returns false here — its
// initialization is the activation hook, not a year transition.
func IsYearBoundary(activation, blocksPerYear, height uint64) bool {
	if height <= activation || blocksPerYear == 0 {
		return false
	}
	return (height-activation)%blocksPerYear == 0
}

// IsYearFinalBlock reports whether height is the last block of the current Year.
// On this block the per-year remainder (annualMint mod blocksPerYear) is added to the
// mint so the realized annual mint exactly matches the §1.3 table (IIP-62 §4.1).
func IsYearFinalBlock(activation, blocksPerYear, height uint64) bool {
	if height < activation || blocksPerYear == 0 {
		return false
	}
	return (height-activation+1)%blocksPerYear == 0
}

// ComputeInflationBps returns the per-year inflation rate in basis points under the
// IIP-62 curve, with round-half-up bps rounding and a permanent lower-bound clamp.
//
//	rate(year) = max(floorBps, round_half_up(y1Bps · num^(year-1) / denom^(year-1)))
//
// Year is 1-indexed (year=1 returns y1Bps without exponentiation).
func ComputeInflationBps(year, y1Bps, num, denom, floorBps uint64) uint64 {
	if year == 0 {
		return 0
	}
	if year == 1 {
		if y1Bps < floorBps {
			return floorBps
		}
		return y1Bps
	}
	exp := year - 1
	numPow := new(big.Int).Exp(new(big.Int).SetUint64(num), new(big.Int).SetUint64(exp), nil)
	denPow := new(big.Int).Exp(new(big.Int).SetUint64(denom), new(big.Int).SetUint64(exp), nil)
	numerator := new(big.Int).Mul(new(big.Int).SetUint64(y1Bps), numPow)
	// round half up: (numerator + denominator/2) / denominator
	halfDen := new(big.Int).Rsh(denPow, 1)
	numerator.Add(numerator, halfDen)
	res := new(big.Int).Quo(numerator, denPow).Uint64()
	if res < floorBps {
		return floorBps
	}
	return res
}

// AnnualMint returns supplyAtYearStart · inflationBps / bpsDenom in Rau (integer).
// This is the §1.3 column "Annual Mint" — the exact total minted in the Year.
func AnnualMint(supplyAtYearStart *big.Int, inflationBps uint64) *big.Int {
	annual := new(big.Int).Mul(supplyAtYearStart, new(big.Int).SetUint64(inflationBps))
	annual.Quo(annual, big.NewInt(bpsDenom))
	return annual
}

// PerBlockMint returns the constant per-block mint amount and the year-end remainder
// (in Rau) under the §1.2 rule: per_block = annualMint / blocksPerYear; remainder =
// annualMint − per_block · blocksPerYear. The remainder is flushed on the Year's
// final block so the realized annual mint exactly equals AnnualMint(...).
func PerBlockMint(supplyAtYearStart *big.Int, inflationBps, blocksPerYear uint64) (perBlock, yearEndRemainder *big.Int) {
	annual := AnnualMint(supplyAtYearStart, inflationBps)
	if blocksPerYear == 0 {
		return new(big.Int), new(big.Int).Set(annual)
	}
	bpy := new(big.Int).SetUint64(blocksPerYear)
	perBlock = new(big.Int).Quo(annual, bpy)
	consumed := new(big.Int).Mul(perBlock, bpy)
	yearEndRemainder = new(big.Int).Sub(annual, consumed)
	return perBlock, yearEndRemainder
}

// SplitMint distributes mTotal between the staker pool and the Machina DAO using
// basis-point shares. It is a pure, stateless function: the staker share is the
// truncated mTotal·stakerBps/bpsDenom and the Machina share is the complement,
// mMachina = mTotal − mStaker. This conserves mTotal exactly every block
// (mStaker + mMachina == mTotal).
//
// No sub-bpsDenom dust is carried across blocks. The per-block truncation biases
// at most (bpsDenom−1)/bpsDenom < 1 Rau toward Machina; with constant per-block
// mint that totals well under a micro-IOTX over the chain's lifetime, so exact
// per-share carry is not worth the persisted-state and reorg surface it costs.
//
// stakerBps + machinaBps must equal bpsDenom; the caller is expected to enforce this
// at genesis-validation time so the assertion does not run hot per block. machinaBps
// is therefore implied by stakerBps and is not read here.
func SplitMint(
	mTotal *big.Int,
	stakerBps, machinaBps uint64,
) (mStaker, mMachina *big.Int) {
	bpsDenomBig := big.NewInt(bpsDenom)

	stakerNum := new(big.Int).Mul(mTotal, new(big.Int).SetUint64(stakerBps))
	mStaker = new(big.Int).Quo(stakerNum, bpsDenomBig)
	mMachina = new(big.Int).Sub(mTotal, mStaker)

	return mStaker, mMachina
}

// stakerShare returns floor(mTotal · stakerBps / bpsDenom), the staker-pool mint for a
// block whose total mint is mTotal. Mirrors SplitMint's staker leg; factored out so the
// derived cumulative / epoch-surplus helpers compute the same value without allocating
// the Machina complement.
func stakerShare(mTotal *big.Int, stakerBps uint64) *big.Int {
	n := new(big.Int).Mul(mTotal, new(big.Int).SetUint64(stakerBps))
	return n.Quo(n, big.NewInt(bpsDenom))
}

// ComputeYearStartSupply replays the IIP-62 disinflation recurrence from genesis to
// return OutstandingSupplyAtYearStart for the given (1-indexed) year:
//
//	S(1) = activationSupply
//	S(k+1) = S(k) + AnnualMint(S(k), ComputeInflationBps(k, …))
//
// This is exact because the per-year remainder flush makes the realized mint of every
// completed year equal AnnualMint(...) exactly. It is self-contained from genesis params
// (no stored state), O(year), and called at most once per year boundary / twice per
// epoch — never per block. Returns activationSupply for year ≤ 1.
func ComputeYearStartSupply(
	activationSupply *big.Int,
	year, y1Bps, num, denom, floorBps, blocksPerYear uint64,
) *big.Int {
	s := new(big.Int).Set(activationSupply)
	for k := uint64(1); k < year; k++ {
		bps := ComputeInflationBps(k, y1Bps, num, denom, floorBps)
		s.Add(s, AnnualMint(s, bps))
	}
	return s
}

// CumulativeMinted returns the total productive inflation minted from activation through
// height (inclusive), i.e. the value the old per-block PostActivationMinted counter held.
// yearStart / bps are OutstandingSupplyAtYearStart and CurrentInflationBps as of height's
// year (the persisted snapshot, valid at the read height). It splits into:
//
//	completed years : yearStart − activationSupply  (= Σ AnnualMint of years < current)
//	current year    : blocksMinted·perBlock (+ remainder on the year's final block)
//
// Returns 0 for heights before activation. OutstandingSupply = activationSupply + this.
func CumulativeMinted(
	activationSupply, yearStart *big.Int,
	bps, activation, blocksPerYear, height uint64,
) *big.Int {
	year := YearIndex(activation, blocksPerYear, height)
	if year == 0 {
		return new(big.Int)
	}
	completed := new(big.Int).Sub(yearStart, activationSupply)
	perBlock, rem := PerBlockMint(yearStart, bps, blocksPerYear)
	yearFirst := activation + (year-1)*blocksPerYear
	blocksMinted := new(big.Int).SetUint64(height - yearFirst + 1)
	partial := new(big.Int).Mul(perBlock, blocksMinted)
	if IsYearFinalBlock(activation, blocksPerYear, height) {
		partial.Add(partial, rem)
	}
	return completed.Add(completed, partial)
}

// EpochInflationSurplus returns Σ over blocks b in [epochStart, epochEnd] of
// max(0, mStaker(b) − blockReward) — the per-block staker-share excess over the (clamped)
// block reward that the old EpochRemainderAccumulator banked and GrantEpochReward paid as
// the epoch reward. It is computed in closed form per year-segment: an epoch lies in a
// single year (1 segment) unless it straddles a year boundary (2 segments, only possible
// if activation is not epoch-aligned). Within a segment mStaker is constant except on the
// year's final block, which carries the remainder-boosted mint. Pre-activation blocks
// (year 0) contribute nothing. blockReward is the unclamped admin block reward — the same
// threshold calculateTotalRewardAndTip uses for the step-F clamp.
func EpochInflationSurplus(
	activationSupply *big.Int,
	activation, blocksPerYear, epochStart, epochEnd uint64,
	y1Bps, num, denom, floorBps, stakerBps uint64,
	blockReward *big.Int,
) *big.Int {
	sum := new(big.Int)
	if blocksPerYear == 0 {
		return sum
	}
	yHi := YearIndex(activation, blocksPerYear, epochEnd)
	for year := uint64(1); year <= yHi; year++ {
		yearFirst := activation + (year-1)*blocksPerYear
		yearFinal := activation + year*blocksPerYear - 1
		lo := epochStart
		if yearFirst > lo {
			lo = yearFirst
		}
		hi := epochEnd
		if yearFinal < hi {
			hi = yearFinal
		}
		if lo > hi {
			continue
		}
		bps := ComputeInflationBps(year, y1Bps, num, denom, floorBps)
		ys := ComputeYearStartSupply(activationSupply, year, y1Bps, num, denom, floorBps, blocksPerYear)
		perBlock, rem := PerBlockMint(ys, bps, blocksPerYear)
		excess := blockExcess(perBlock, stakerBps, blockReward)
		n := new(big.Int).SetUint64(hi - lo + 1)
		sum.Add(sum, n.Mul(n, excess))
		// The year's final block (if inside this segment) mints perBlock+rem, not perBlock.
		if rem.Sign() > 0 && lo <= yearFinal && yearFinal <= hi {
			boosted := new(big.Int).Add(perBlock, rem)
			sum.Add(sum, blockExcess(boosted, stakerBps, blockReward))
			sum.Sub(sum, excess) // swap out the one normal block we over-counted
		}
	}
	return sum
}

// blockExcess returns max(0, stakerShare(mTotal) − blockReward).
func blockExcess(mTotal *big.Int, stakerBps uint64, blockReward *big.Int) *big.Int {
	mStaker := stakerShare(mTotal, stakerBps)
	if mStaker.Cmp(blockReward) <= 0 {
		return new(big.Int)
	}
	return mStaker.Sub(mStaker, blockReward)
}
