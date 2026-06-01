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
// Used to flush yearMintRemainder per IIP-62 §4.1.
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
// basis-point shares, carrying sub-bpsDenom dust between blocks so that over time
// the realized split is exact. dustStakerIn / dustMachinaIn are the per-share dust
// accumulators from the previous block (in "Rau·bps" units); the returned dust
// values must be persisted for the next call.
//
// stakerBps + machinaBps must equal bpsDenom; the caller is expected to enforce this
// at genesis-validation time so the assertion does not run hot per block.
func SplitMint(
	mTotal *big.Int,
	stakerBps, machinaBps uint64,
	dustStakerIn, dustMachinaIn *big.Int,
) (mStaker, mMachina, dustStakerOut, dustMachinaOut *big.Int) {
	bpsDenomBig := big.NewInt(bpsDenom)

	stakerNum := new(big.Int).Mul(mTotal, new(big.Int).SetUint64(stakerBps))
	if dustStakerIn != nil {
		stakerNum.Add(stakerNum, dustStakerIn)
	}
	mStaker = new(big.Int).Quo(stakerNum, bpsDenomBig)
	dustStakerOut = new(big.Int).Mod(stakerNum, bpsDenomBig)

	machinaNum := new(big.Int).Mul(mTotal, new(big.Int).SetUint64(machinaBps))
	if dustMachinaIn != nil {
		machinaNum.Add(machinaNum, dustMachinaIn)
	}
	mMachina = new(big.Int).Quo(machinaNum, bpsDenomBig)
	dustMachinaOut = new(big.Int).Mod(machinaNum, bpsDenomBig)

	return mStaker, mMachina, dustStakerOut, dustMachinaOut
}
