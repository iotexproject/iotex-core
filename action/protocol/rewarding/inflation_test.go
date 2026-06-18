// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// IIP-62 §1.1 / §1.3 reference values. The spec only pins a handful of years; the
// rest are derived from the formula and asserted exactly so future refactors of the
// big.Int math cannot drift.
func TestComputeInflationBps_SpecValues(t *testing.T) {
	const (
		y1Bps    uint64 = 500
		num      uint64 = 8000
		den      uint64 = 10000
		floorBps uint64 = 50
	)
	cases := []struct {
		year uint64
		want uint64
	}{
		{1, 500},  // §1.3: Y1 5.00%
		{2, 400},  // 500 * 0.8 = 400
		{3, 320},  // 500 * 0.64 = 320
		{4, 256},  // 500 * 0.512 = 256
		{5, 205},  // §1.1: 204.80 → 205 (round-half-up)
		{6, 164},  // 500 * 0.32768 = 163.84 → 164
		{7, 131},  // 130.97152 → 131 (round-half-up of .97)
		{8, 105},  // 104.78 → 105
		{9, 84},   // 83.886 → 84
		{10, 67},  // §1.1: 67.11 → 67
		{11, 54},  // §1.1: 53.69 → 54
		{12, 50},  // §1.1: 42.95 → floor clamps to 50
		{13, 50},  // formula keeps going below 50; floor holds
		{17, 50},  // floor still holds
		{50, 50},  // arbitrary far-future year — still at floor
		{100, 50}, // ensure no overflow at large exponents
	}
	for _, c := range cases {
		got := ComputeInflationBps(c.year, y1Bps, num, den, floorBps)
		require.Equalf(t, c.want, got, "year %d: want %d, got %d", c.year, c.want, got)
	}
}

func TestComputeInflationBps_EdgeCases(t *testing.T) {
	// Year 0 is a sentinel for "pre-activation"; result must be 0.
	require.Equal(t, uint64(0), ComputeInflationBps(0, 500, 8000, 10000, 50))
	// Y1Bps below floor is clamped up.
	require.Equal(t, uint64(50), ComputeInflationBps(1, 10, 8000, 10000, 50))
}

func TestYearIndex(t *testing.T) {
	const activation, bpy uint64 = 1000, 100
	require.Equal(t, uint64(0), YearIndex(activation, bpy, 999))   // pre-activation
	require.Equal(t, uint64(1), YearIndex(activation, bpy, 1000))  // activation block = Y1 start
	require.Equal(t, uint64(1), YearIndex(activation, bpy, 1099))  // last block of Y1
	require.Equal(t, uint64(2), YearIndex(activation, bpy, 1100))  // first block of Y2
	require.Equal(t, uint64(12), YearIndex(activation, bpy, 2199)) // mid Y12
	require.Equal(t, uint64(0), YearIndex(activation, 0, 9999))    // blocksPerYear=0 is degenerate
}

func TestIsYearBoundary(t *testing.T) {
	const activation, bpy uint64 = 1000, 100
	require.False(t, IsYearBoundary(activation, bpy, 1000)) // activation is Y1 start, not a transition
	require.False(t, IsYearBoundary(activation, bpy, 1050))
	require.True(t, IsYearBoundary(activation, bpy, 1100))
	require.True(t, IsYearBoundary(activation, bpy, 1200))
	require.False(t, IsYearBoundary(activation, bpy, 1101))
}

func TestIsYearFinalBlock(t *testing.T) {
	const activation, bpy uint64 = 1000, 100
	require.False(t, IsYearFinalBlock(activation, bpy, 1000))
	require.True(t, IsYearFinalBlock(activation, bpy, 1099)) // last block of Y1
	require.False(t, IsYearFinalBlock(activation, bpy, 1100))
	require.True(t, IsYearFinalBlock(activation, bpy, 1199)) // last block of Y2
}

// 20-year IIP §1.3 reproduction, with annual mint and end-of-year supply checked
// against the table. The table's headline numbers are independently rounded (the
// spec calls that out), so we assert against the §1.2 formula output exactly and
// only sanity-check the §1.3 rounded values to within ±1M IOTX.
func TestIIP62_SpecTable_20Year(t *testing.T) {
	const (
		y1Bps    uint64 = 500
		num      uint64 = 8000
		den      uint64 = 10000
		floorBps uint64 = 50
	)
	// 9.44B IOTX baseline (§1.3 assumption).
	supply := new(big.Int).Mul(big.NewInt(9_440_000_000), iotxRau())

	// (year, inflationBps, approxAnnualMintMIOTX) — last column is the §1.3 rounded
	// annual mint in millions of IOTX, used as a tolerance check only.
	cases := []struct {
		year                uint64
		wantBps             uint64
		approxAnnualMintMIO int64
	}{
		{1, 500, 472},
		{2, 400, 396},
		{3, 320, 330},
		{4, 256, 272},
		{5, 205, 224},
		{6, 164, 183},
		{7, 131, 148},
		{8, 105, 120},
		{9, 84, 97},
		{10, 67, 78},
		{11, 54, 64},
		{12, 50, 59},
		{13, 50, 59},
		{14, 50, 60},
		{15, 50, 60},
		{16, 50, 60},
		{17, 50, 61},
		{18, 50, 61},
		{19, 50, 61},
		{20, 50, 62},
	}
	cumulative := new(big.Int)
	for _, c := range cases {
		bps := ComputeInflationBps(c.year, y1Bps, num, den, floorBps)
		require.Equalf(t, c.wantBps, bps, "year %d bps", c.year)

		annual := AnnualMint(supply, bps)

		// §1.3 rounded check: annual mint is within 1M IOTX of the spec's headline.
		approxRau := new(big.Int).Mul(big.NewInt(c.approxAnnualMintMIO*1_000_000), iotxRau())
		diff := new(big.Int).Sub(annual, approxRau)
		diff.Abs(diff)
		oneMIotx := new(big.Int).Mul(big.NewInt(1_000_000), iotxRau())
		require.Truef(t, diff.Cmp(oneMIotx) < 0,
			"year %d: annual mint %s diverges from §1.3 by ≥1M IOTX (table approx %s)",
			c.year, annual.String(), approxRau.String())

		cumulative.Add(cumulative, annual)
		supply.Add(supply, annual)
	}

	// §1.3 cumulative: ~2.93B IOTX over 20 years.
	cumulativeMIOTX := new(big.Int).Quo(cumulative, iotxRau())
	require.Truef(t,
		cumulativeMIOTX.Cmp(big.NewInt(2_900_000_000)) > 0 &&
			cumulativeMIOTX.Cmp(big.NewInt(2_950_000_000)) < 0,
		"20-year cumulative mint %s IOTX is outside §1.3 expected ~2.93B band",
		cumulativeMIOTX.String())
}

// PerBlockMint: integer-div consistency — perBlock * blocksPerYear + remainder = annual.
func TestPerBlockMint_RemainderClosesYear(t *testing.T) {
	supply := new(big.Int).Mul(big.NewInt(9_440_000_000), iotxRau())
	const (
		bps uint64 = 500
		bpy uint64 = 12_614_400
	)
	perBlock, rem := PerBlockMint(supply, bps, bpy)
	annual := AnnualMint(supply, bps)

	got := new(big.Int).Mul(perBlock, new(big.Int).SetUint64(bpy))
	got.Add(got, rem)
	require.Equalf(t, 0, annual.Cmp(got),
		"perBlock·blocksPerYear + remainder must equal annual mint; annual=%s got=%s",
		annual.String(), got.String())
	require.Truef(t, rem.Cmp(new(big.Int).SetUint64(bpy)) < 0,
		"year-end remainder %s must be < blocksPerYear %d", rem.String(), bpy)
}

// SplitMint: staker is the truncated bps share, Machina is the complement, and the
// per-block truncation bias toward Machina stays strictly below 1 Rau.
func TestSplitMint_StakerTruncatedMachinaComplement(t *testing.T) {
	const (
		stakerBps  uint64 = 8000
		machinaBps uint64 = 2000
		nBlocks           = 10000
	)
	// Pick an mTotal that is intentionally indivisible by bpsDenom so truncation bites.
	mTotal := big.NewInt(123_456_789)

	totalStaker, totalMachina := new(big.Int), new(big.Int)
	for i := 0; i < nBlocks; i++ {
		s, m := SplitMint(mTotal, stakerBps, machinaBps)
		// Staker is floor(mTotal*stakerBps/bpsDenom) every block (no carry).
		wantS := new(big.Int).Quo(new(big.Int).Mul(mTotal, big.NewInt(int64(stakerBps))), big.NewInt(bpsDenom))
		require.Equal(t, 0, s.Cmp(wantS), "staker per-block mismatch: got %s want %s", s, wantS)
		// Machina is exactly the complement, so the split conserves mTotal.
		require.Equal(t, 0, new(big.Int).Add(s, m).Cmp(mTotal), "conservation broken: %s+%s != %s", s, m, mTotal)
		totalStaker.Add(totalStaker, s)
		totalMachina.Add(totalMachina, m)
	}
	// Bias: staker is shorted vs the exact fair share by < 1 Rau/block, so over
	// nBlocks the shortfall (which all accrues to Machina) is strictly < nBlocks Rau.
	fairStaker := new(big.Int).Quo(
		new(big.Int).Mul(mTotal, big.NewInt(int64(nBlocks)*int64(stakerBps))),
		big.NewInt(bpsDenom),
	)
	shortfall := new(big.Int).Sub(fairStaker, totalStaker)
	require.Equal(t, -1, shortfall.Cmp(big.NewInt(nBlocks)), "staker shortfall %s must be < %d Rau", shortfall, nBlocks)
	require.True(t, shortfall.Sign() >= 0, "staker should never be over-paid, got shortfall %s", shortfall)
}

// Conservation: with the Machina share derived as mTotal − mStaker, the split is
// exact every block — mStaker + mMachina == mTotal — for arbitrary inputs.
func TestSplitMint_PerBlockConservation(t *testing.T) {
	mTotal := big.NewInt(1_000_000_007) // a prime, to force a nonzero truncation
	mStaker, mMachina := SplitMint(mTotal, 8000, 2000)

	require.Equal(t, 0, new(big.Int).Add(mStaker, mMachina).Cmp(mTotal),
		"per-block conservation: mStaker=%s mMachina=%s mTotal=%s", mStaker, mMachina, mTotal)

	// Staker is exactly the truncated bps share.
	wantStaker := new(big.Int).Quo(new(big.Int).Mul(mTotal, big.NewInt(8000)), big.NewInt(bpsDenom))
	require.Equal(t, 0, mStaker.Cmp(wantStaker), "staker share mismatch: got %s want %s", mStaker, wantStaker)
}

// CumulativeMinted must match a brute-force per-block sum of mTotal across years,
// including the year-end remainder flush on each year's final block. Cross-checks the
// closed-form derivation that replaces the old stored postActivationMinted counter.
func TestCumulativeMinted_MatchesBruteForce(t *testing.T) {
	const (
		activation    = uint64(360)
		blocksPerYear = uint64(50)
		y1Bps         = uint64(10000) // 100%/yr so per-block mint is chunky
		num           = uint64(8000)
		denom         = uint64(10000)
		floorBps      = uint64(50)
	)
	activationSupply, _ := new(big.Int).SetString("100000000000000000000", 10) // 100 IOTX

	// Brute-force running sum of mTotal, recomputing the year-start snapshot via the
	// same recurrence mintAndAllocate uses at each boundary.
	want := new(big.Int)
	curYear := uint64(0)
	var yearStart *big.Int
	var bps uint64
	// Walk 3 years + a few blocks so multiple boundaries + final-block flushes are hit.
	for h := activation; h <= activation+3*blocksPerYear+7; h++ {
		year := YearIndex(activation, blocksPerYear, h)
		if year != curYear {
			yearStart = ComputeYearStartSupply(activationSupply, year, y1Bps, num, denom, floorBps, blocksPerYear)
			bps = ComputeInflationBps(year, y1Bps, num, denom, floorBps)
			curYear = year
		}
		perBlock, rem := PerBlockMint(yearStart, bps, blocksPerYear)
		mTotal := new(big.Int).Set(perBlock)
		if IsYearFinalBlock(activation, blocksPerYear, h) {
			mTotal.Add(mTotal, rem)
		}
		want.Add(want, mTotal)

		got := CumulativeMinted(activationSupply, yearStart, bps, activation, blocksPerYear, h)
		require.Equalf(t, 0, got.Cmp(want), "CumulativeMinted(%d) = %s, brute-force = %s", h, got, want)
	}
	// Before activation it is zero.
	require.Equal(t, 0, CumulativeMinted(activationSupply, activationSupply, y1Bps, activation, blocksPerYear, activation-1).Sign())
}

// EpochInflationSurplus must equal a brute-force per-block Σ max(0, mStaker − blockReward),
// across single-year, year-final-block, and year-straddling epoch ranges, and must ignore
// pre-activation blocks. Cross-checks the closed-form that replaces the old stored
// epochRemainderAccumulator.
func TestEpochInflationSurplus_MatchesBruteForce(t *testing.T) {
	const (
		activation    = uint64(360)
		blocksPerYear = uint64(50)
		y1Bps         = uint64(10000)
		num           = uint64(8000)
		denom         = uint64(10000)
		floorBps      = uint64(50)
		stakerBps     = uint64(8000)
	)
	activationSupply, _ := new(big.Int).SetString("100000000000000000000", 10)
	blockReward := big.NewInt(1_000_000_000) // small vs per-block mint so excess is usually >0

	bruteForce := func(epochStart, epochEnd uint64) *big.Int {
		sum := new(big.Int)
		for h := epochStart; h <= epochEnd; h++ {
			year := YearIndex(activation, blocksPerYear, h)
			if year == 0 {
				continue
			}
			ys := ComputeYearStartSupply(activationSupply, year, y1Bps, num, denom, floorBps, blocksPerYear)
			bps := ComputeInflationBps(year, y1Bps, num, denom, floorBps)
			perBlock, rem := PerBlockMint(ys, bps, blocksPerYear)
			mTotal := new(big.Int).Set(perBlock)
			if IsYearFinalBlock(activation, blocksPerYear, h) {
				mTotal.Add(mTotal, rem)
			}
			mStaker := new(big.Int).Quo(new(big.Int).Mul(mTotal, big.NewInt(int64(stakerBps))), big.NewInt(bpsDenom))
			if mStaker.Cmp(blockReward) > 0 {
				sum.Add(sum, new(big.Int).Sub(mStaker, blockReward))
			}
		}
		return sum
	}

	cases := []struct {
		name               string
		epochStart, epochE uint64
	}{
		{"pre-activation only", activation - 20, activation - 1},
		{"straddles activation", activation - 5, activation + 4},
		{"single mid-year window", activation + 10, activation + 19},
		{"includes year-final block", activation + blocksPerYear - 3, activation + blocksPerYear + 2},
		{"straddles year boundary deep", activation + 2*blocksPerYear - 4, activation + 2*blocksPerYear + 5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EpochInflationSurplus(
				activationSupply, activation, blocksPerYear, tc.epochStart, tc.epochE,
				y1Bps, num, denom, floorBps, stakerBps, blockReward,
			)
			require.Equal(t, 0, got.Cmp(bruteForce(tc.epochStart, tc.epochE)),
				"surplus mismatch: got %s want %s", got, bruteForce(tc.epochStart, tc.epochE))
		})
	}
}

// fixedHeightSR reports a chosen height from State() while delegating the actual read
// to the wrapped StateManager. testProtocol's mock StateManager always reports height 0,
// which would defeat the height-dependent derivation in the OutstandingSupply /
// PostActivationMinted getters; production StateReaders return the query height (verified:
// workingSet.State returns ws.height).
type fixedHeightSR struct {
	protocol.StateManager
	height uint64
}

func (s fixedHeightSR) State(v interface{}, opts ...protocol.StateOption) (uint64, error) {
	_, err := s.StateManager.State(v, opts...)
	return s.height, err
}

// End-to-end coverage of the derived OutstandingSupply / PostActivationMinted getters:
// they must reconstruct supply from the boundary-only snapshot plus the query height
// returned by p.state, for partial-year, activation-block, pre-activation, and post-year-
// boundary reads.
func TestOutstandingSupplyGetters_Derived(t *testing.T) {
	machinaAddr := identityset.Address(33).String()
	const blocksPerYear = uint64(100)
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		req := require.New(t)
		g := genesis.MustExtractGenesisContext(ctx)
		blkCtx := protocol.MustGetBlockCtx(ctx)
		g.Rewarding.InflationRateY1Bps = 500
		g.Rewarding.InflationDecayNumerator = 8000
		g.Rewarding.InflationDecayDenominator = 10000
		g.Rewarding.InflationFloorBps = 50
		g.Rewarding.BlocksPerYear = blocksPerYear
		g.Rewarding.StakerShareBps = 8000
		g.Rewarding.MachinaShareBps = 2000
		g.Rewarding.MachinaDaoAddress = machinaAddr
		g.Rewarding.OutstandingSupplyAtActivation = "100000000000000000000" // 100 IOTX
		activation := blkCtx.BlockHeight
		g.ToBeEnabledBlockHeight = activation
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		req.NoError(p.CreatePreStates(ctx, sm))

		supply := g.Rewarding.OutstandingSupplyAtActivationBig()
		denomB := big.NewInt(10000)
		bpyB := new(big.Int).SetUint64(blocksPerYear)
		// Year 1 per-block mint, computed with plain arithmetic (independent of the
		// production helpers): annual = supply·500/10000, perBlock = annual/blocksPerYear.
		annual1 := new(big.Int).Quo(new(big.Int).Mul(supply, big.NewInt(500)), denomB)
		perBlock1 := new(big.Int).Quo(annual1, bpyB)
		// Year 2: ComputeInflationBps(2)=400 (see TestComputeInflationBps_SpecValues).
		yearStart2 := new(big.Int).Add(supply, annual1)
		annual2 := new(big.Int).Quo(new(big.Int).Mul(yearStart2, big.NewInt(400)), denomB)
		perBlock2 := new(big.Int).Quo(annual2, bpyB)

		// assertAt queries the getters at height h (via the fixed-height wrapper) and
		// checks both against wantMinted (= PostActivationMinted; OutstandingSupply is
		// supply + that). Also confirms the getter echoes the query height.
		assertAt := func(h uint64, wantMinted *big.Int) {
			sr := fixedHeightSR{StateManager: sm, height: h}
			os, hgt, err := p.OutstandingSupply(ctx, sr)
			req.NoError(err)
			req.Equalf(h, hgt, "OutstandingSupply height echo at %d", h)
			req.Equalf(0, os.Cmp(new(big.Int).Add(supply, wantMinted)),
				"OutstandingSupply at %d: got %s want %s", h, os, new(big.Int).Add(supply, wantMinted))
			pam, _, err := p.PostActivationMinted(ctx, sr)
			req.NoError(err)
			req.Equalf(0, pam.Cmp(wantMinted), "PostActivationMinted at %d: got %s want %s", h, pam, wantMinted)
		}

		mulN := func(a *big.Int, n int64) *big.Int { return new(big.Int).Mul(a, big.NewInt(n)) }

		// At/before activation: nothing minted yet (state exists, but height precedes Y1).
		assertAt(activation-1, new(big.Int))
		// Activation block itself mints exactly one perBlock.
		assertAt(activation, perBlock1)
		// Mid-year-1 partial: blocks [activation, activation+50] = 51 blocks.
		assertAt(activation+50, mulN(perBlock1, 51))

		// Cross into year 2: run mintAndAllocate at the boundary block so the stored
		// snapshot advances to the Y2 base (ComputeYearStartSupply(2)).
		blkCtx.BlockHeight = activation + blocksPerYear
		ctx = protocol.WithBlockCtx(ctx, blkCtx)
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		_, _, err := p.mintAndAllocate(ctx, sm)
		req.NoError(err)

		// Year-2 read: completed year 1 (annual1) + 11 blocks of year 2.
		wantY2 := new(big.Int).Add(annual1, mulN(perBlock2, 11))
		assertAt(activation+blocksPerYear+10, wantY2)
	}, nil, false, 0)
}

func TestInflationState_RoundTrip(t *testing.T) {
	s := newInflationState()
	s.outstandingSupplyAtYearStart.SetString("9440000000000000000000000000", 10)
	s.currentInflationBps = 500
	s.currentYearIndex = 1

	data, err := s.Serialize()
	require.NoError(t, err)

	out := newInflationState()
	require.NoError(t, out.Deserialize(data))

	require.Equal(t, 0, s.outstandingSupplyAtYearStart.Cmp(out.outstandingSupplyAtYearStart))
	require.Equal(t, s.currentInflationBps, out.currentInflationBps)
	require.Equal(t, s.currentYearIndex, out.currentYearIndex)
}

func TestValidateInflationConfig(t *testing.T) {
	valid := &genesis.Rewarding{
		InflationRateY1Bps:            500,
		InflationDecayNumerator:       8000,
		InflationDecayDenominator:     10000,
		InflationFloorBps:             50,
		BlocksPerYear:                 12_614_400,
		StakerShareBps:                8000,
		MachinaShareBps:               2000,
		MachinaDaoAddress:             "io1ar5l5s268rtgzshltnqv88mua06ucm58dx678y",
		OutstandingSupplyAtActivation: "9440000000000000000000000000",
	}
	require.NoError(t, validateInflationConfig(valid))

	cases := []struct {
		name    string
		mutate  func(*genesis.Rewarding)
		wantSub string
	}{
		{"shares don't sum", func(c *genesis.Rewarding) { c.StakerShareBps = 7000 }, "share splits"},
		{"floor above Y1", func(c *genesis.Rewarding) { c.InflationFloorBps = 600 }, "InflationFloorBps"},
		{"zero decay denom", func(c *genesis.Rewarding) { c.InflationDecayDenominator = 0 }, "Denominator"},
		{"decay > 1", func(c *genesis.Rewarding) { c.InflationDecayNumerator = 11000 }, "decay must be ≤ 1"},
		{"zero blocksPerYear", func(c *genesis.Rewarding) { c.BlocksPerYear = 0 }, "BlocksPerYear"},
		{"empty machina addr", func(c *genesis.Rewarding) { c.MachinaDaoAddress = "" }, "MachinaDaoAddress must"},
		{"malformed machina addr", func(c *genesis.Rewarding) { c.MachinaDaoAddress = "not-an-address" }, "does not parse"},
		{"empty supply", func(c *genesis.Rewarding) { c.OutstandingSupplyAtActivation = "" }, "OutstandingSupplyAtActivation"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := *valid
			c.mutate(&cfg)
			err := validateInflationConfig(&cfg)
			require.Error(t, err)
			require.Contains(t, err.Error(), c.wantSub)
		})
	}
}

// End-to-end IIP-62 mintAndAllocate: configure the Rewarding genesis with a small
// supply, fire CreatePreStates at the activation block to seed InflationState, then
// run GrantBlockReward at the next height. Asserts the staker → Fund credit, the
// Machina → account credit, and the two TransactionLogs emitted.
func TestMintAndAllocate_EmitsTransactionLogs(t *testing.T) {
	machinaAddr := identityset.Address(33).String()
	// Tiny supply so the per-block mint is human-readable in failure messages.
	const supplyRau = "100000000000000000000" // 100 IOTX
	const blocksPerYear = 100                 // short year for fast tests

	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		req := require.New(t)
		g := genesis.MustExtractGenesisContext(ctx)
		// Splice in IIP-62 params. testProtocol used the package default Y1=500, etc.
		g.Rewarding.InflationRateY1Bps = 10000 // 100% — pick a chunky rate so per-block mint > 0
		g.Rewarding.InflationDecayNumerator = 8000
		g.Rewarding.InflationDecayDenominator = 10000
		g.Rewarding.InflationFloorBps = 50
		g.Rewarding.BlocksPerYear = blocksPerYear
		g.Rewarding.StakerShareBps = 8000
		g.Rewarding.MachinaShareBps = 2000
		g.Rewarding.MachinaDaoAddress = machinaAddr
		g.Rewarding.OutstandingSupplyAtActivation = supplyRau

		// Activate IIP-62 at the current test block height.
		blkCtx := protocol.MustGetBlockCtx(ctx)
		activation := blkCtx.BlockHeight
		g.ToBeEnabledBlockHeight = activation
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)

		// Seed InflationState via CreatePreStates at the activation block.
		req.NoError(p.CreatePreStates(ctx, sm))

		// Step to the next block: still in Y1, but past activation so mintAndAllocate fires.
		blkCtx.BlockHeight = activation + 1
		ctx = protocol.WithBlockCtx(ctx, blkCtx)
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)

		// Snapshot pre-state.
		fundBefore, _, err := p.AvailableBalance(ctx, sm)
		req.NoError(err)
		machinaAccBefore, err := accountutil.LoadAccount(sm, mustAddr(machinaAddr))
		req.NoError(err)
		machinaBalBefore := new(big.Int).Set(machinaAccBefore.Balance)

		// Compute expected per-block mint via the same pure math the protocol uses.
		supply, _ := new(big.Int).SetString(supplyRau, 10)
		annual := AnnualMint(supply, 10000) // 100% of 100 IOTX = 100 IOTX/year
		expectPerBlock := new(big.Int).Quo(annual, big.NewInt(blocksPerYear))
		expectStaker := new(big.Int).Quo(new(big.Int).Mul(expectPerBlock, big.NewInt(8000)), big.NewInt(10000))
		expectMachina := new(big.Int).Quo(new(big.Int).Mul(expectPerBlock, big.NewInt(2000)), big.NewInt(10000))

		_, mintTLogs, err := p.GrantBlockReward(ctx, sm)
		req.NoError(err)
		req.Len(mintTLogs, 2, "expected staker + machina mint tLogs")

		// Staker log: INFLATION_MINT_STAKER, Sender="" (protocol mint), Recipient=RewardingPoolAddr.
		req.Equal(iotextypes.TransactionLogType_INFLATION_MINT_STAKER, mintTLogs[0].Type)
		req.Equal("", mintTLogs[0].Sender, "Sender must be empty to mark protocol mint")
		req.Equal(address.RewardingPoolAddr, mintTLogs[0].Recipient)
		req.Equalf(0, mintTLogs[0].Amount.Cmp(expectStaker),
			"staker mint amount mismatch: got %s want %s", mintTLogs[0].Amount, expectStaker)

		// Machina log: INFLATION_MINT_MACHINA, Sender="", Recipient=Machina.
		req.Equal(iotextypes.TransactionLogType_INFLATION_MINT_MACHINA, mintTLogs[1].Type)
		req.Equal("", mintTLogs[1].Sender, "Sender must be empty to mark protocol mint")
		req.Equal(machinaAddr, mintTLogs[1].Recipient)
		req.Equalf(0, mintTLogs[1].Amount.Cmp(expectMachina),
			"machina mint amount mismatch: got %s want %s", mintTLogs[1].Amount, expectMachina)

		// State side-effects: the fund is credited mStaker by the mint, then debited
		// effective_block_reward = min(a.blockReward, mStaker) by the producer grant.
		// testProtocol seeds admin blockReward = 10 and mStaker ≫ 10, so the clamp is 10
		// and the net fund gain is mStaker − 10.
		effectiveBlock := big.NewInt(10)
		expectFundGain := new(big.Int).Sub(expectStaker, effectiveBlock)
		fundAfter, _, err := p.AvailableBalance(ctx, sm)
		req.NoError(err)
		gain := new(big.Int).Sub(fundAfter, fundBefore)
		req.Equalf(0, gain.Cmp(expectFundGain),
			"fund delta mismatch: got %s want %s", gain, expectFundGain)

		machinaAccAfter, err := accountutil.LoadAccount(sm, mustAddr(machinaAddr))
		req.NoError(err)
		machinaGain := new(big.Int).Sub(machinaAccAfter.Balance, machinaBalBefore)
		req.Equalf(0, machinaGain.Cmp(expectMachina),
			"machina balance delta mismatch: got %s want %s", machinaGain, expectMachina)
	}, nil, false, 0)
}

// IIP-62 step G: post-activation, GrantEpochReward funds the split from the derived
// per-block inflation surplus (EpochInflationSurplus over the epoch's block range)
// instead of admin.epochReward. No stored accumulator is read or reset.
func TestGrantEpochReward_UsesDerivedSurplus(t *testing.T) {
	machinaAddr := identityset.Address(33).String()
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		req := require.New(t)

		// Activate IIP-62 at the harness's current height (also the last block of
		// epoch 1, so assertLastBlockInEpoch passes inside GrantEpochReward). Only the
		// activation block itself is post-activation in this epoch, so the derived
		// surplus is exactly one block's excess.
		//
		// Params chosen so the single-block surplus is small and fundable from the
		// harness's tiny accounts:
		//   annual   = supply·bps/denom = 4_000_000·500/10000 = 200_000
		//   perBlock = annual/blocksPerYear = 200_000/1000     = 200
		//   mStaker  = perBlock·stakerBps/denom = 200·8000/10000 = 160
		//   excess   = mStaker − a.blockReward (=10)            = 150  → epoch surplus
		// Address(27)'s slice (votes 4M of top-4 total 10M) routes to Address(0):
		//   150·4M/10M = 60  (the legacy admin.epochReward=100 path would give 40).
		g := genesis.MustExtractGenesisContext(ctx)
		blkCtx := protocol.MustGetBlockCtx(ctx)
		g.Rewarding.InflationRateY1Bps = 500
		g.Rewarding.InflationDecayNumerator = 8000
		g.Rewarding.InflationDecayDenominator = 10000
		g.Rewarding.InflationFloorBps = 50
		g.Rewarding.BlocksPerYear = 1000
		g.Rewarding.StakerShareBps = 8000
		g.Rewarding.MachinaShareBps = 2000
		g.Rewarding.MachinaDaoAddress = machinaAddr
		g.Rewarding.OutstandingSupplyAtActivation = "4000000"
		g.ToBeEnabledBlockHeight = blkCtx.BlockHeight
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)

		// Seed InflationState via the activation pre-state hook.
		req.NoError(p.CreatePreStates(ctx, sm))

		// Fund the rewarding pool so the grant + foundation bonus succeed. Caller
		// (Address(28)) is seeded with 1000 by the harness.
		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		req.NoError(err)

		// Staking mock (mirrors TestProtocol_GrantEpochReward).
		registry := protocol.MustGetRegistry(ctx)
		sp := &staking.Protocol{}
		req.NoError(sp.Register(registry))
		patches := gomonkey.NewPatches()
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)
		defer patches.Reset()

		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		_, rewardLogs, err := p.GrantEpochReward(ctx, sm)
		req.NoError(err)

		// Address(27)'s votes (4M of 10M total) → reward routed to Address(0).
		// Derived-surplus slice = 150·4M/10M = 60; legacy a.epochReward=100 slice = 40.
		var address0Reward *big.Int
		for _, l := range rewardLogs {
			var rl rewardingpb.RewardLog
			req.NoError(proto.Unmarshal(l.Data, &rl))
			if rl.Type == rewardingpb.RewardLog_EPOCH_REWARD && rl.Addr == identityset.Address(0).String() {
				amt, ok := new(big.Int).SetString(rl.Amount, 10)
				req.True(ok)
				address0Reward = amt
				break
			}
		}
		req.NotNilf(address0Reward, "no EPOCH_REWARD log for Address(0); got %d logs", len(rewardLogs))
		req.Equalf(0, address0Reward.Cmp(big.NewInt(60)),
			"Address(0) reward = %s; expected 60 (derived-surplus path) — 40 means legacy path fired",
			address0Reward)
	}, nil, false, 0)
}

// IIP-62 §4.1 invariant: the rewarding Fund must never underflow across the
// per-block mint + block-reward debit cycle. This test walks many post-activation
// blocks and, after each GrantBlockReward, asserts:
//   - totalBalance >= unclaimedBalance (Claim has not run, so this must always hold)
//   - unclaimedBalance >= 0 (the floor-regime clamp prevents the debit from exceeding
//     the mint credit; without the clamp this would underflow once mStaker < blockReward)
//   - derived PostActivationMinted == sum of per-block mTotal credited (no double-count,
//     no drop) — exercises the CumulativeMinted derivation against an independent sum
//
// Two regimes are covered as sub-tests:
//
//	high-mint: supply=100 IOTX × Y1=100%/year → mStaker ≫ a.blockReward (10 rau);
//	           the producer grant is bounded by blockReward and the rest accrues to fund.
//	floor:     supply=1000 rau × Y1=100%/year → mStaker (≈8 rau) < blockReward (10 rau);
//	           the step-F clamp kicks in and the producer is paid mStaker (not blockReward),
//	           so unclaimedBalance stays at zero block-over-block instead of underflowing.
func TestMintAndAllocate_FundInvariant(t *testing.T) {
	machinaAddr := identityset.Address(33).String()
	const blocksPerYear = 100
	const numBlocks = 25

	cases := []struct {
		name     string
		supply   string // OutstandingSupplyAtActivation, in rau
		regimeIs string // "high" or "floor" — annotates failure messages
	}{
		{"high_mint_regime", "100000000000000000000", "high"}, // 100 IOTX
		{"floor_regime", "1000", "floor"},                     // 1000 rau → mStaker < blockReward(10)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
				req := require.New(t)
				g := genesis.MustExtractGenesisContext(ctx)
				g.Rewarding.InflationRateY1Bps = 10000
				g.Rewarding.InflationDecayNumerator = 8000
				g.Rewarding.InflationDecayDenominator = 10000
				g.Rewarding.InflationFloorBps = 50
				g.Rewarding.BlocksPerYear = blocksPerYear
				g.Rewarding.StakerShareBps = 8000
				g.Rewarding.MachinaShareBps = 2000
				g.Rewarding.MachinaDaoAddress = machinaAddr
				g.Rewarding.OutstandingSupplyAtActivation = tc.supply

				blkCtx := protocol.MustGetBlockCtx(ctx)
				activation := blkCtx.BlockHeight
				g.ToBeEnabledBlockHeight = activation
				ctx = genesis.WithGenesisContext(ctx, g)
				ctx = protocol.WithFeatureCtx(ctx)

				req.NoError(p.CreatePreStates(ctx, sm))

				expectedMinted := new(big.Int)
				// Mint from the activation block itself (production grants there), so the
				// derived CumulativeMinted — which counts every block from activation —
				// matches our independent running sum.
				for i := uint64(0); i < numBlocks; i++ {
					blkCtx.BlockHeight = activation + i
					ctx = protocol.WithBlockCtx(ctx, blkCtx)
					ctx = genesis.WithGenesisContext(ctx, g)
					ctx = protocol.WithFeatureCtx(ctx)

					// Snapshot inflation state pre-grant so we can compute the expected
					// per-block mint credit independently of the protocol's accounting.
					infPre := newInflationState()
					_, err := p.state(ctx, sm, _inflKey, infPre)
					req.NoError(err)
					perBlock, rem := PerBlockMint(infPre.outstandingSupplyAtYearStart, infPre.currentInflationBps, blocksPerYear)
					mTotal := new(big.Int).Set(perBlock)
					if IsYearFinalBlock(activation, blocksPerYear, blkCtx.BlockHeight) {
						mTotal.Add(mTotal, rem)
					}
					expectedMinted.Add(expectedMinted, mTotal)

					_, _, err = p.GrantBlockReward(ctx, sm)
					req.NoErrorf(err, "%s: GrantBlockReward failed at block %d", tc.regimeIs, blkCtx.BlockHeight)

					// Core invariant: total >= unclaimed >= 0 after every block.
					total, _, err := p.TotalBalance(ctx, sm)
					req.NoError(err)
					unclaimed, _, err := p.AvailableBalance(ctx, sm)
					req.NoError(err)
					req.Truef(total.Sign() >= 0, "%s: totalBalance went negative at block %d: %s", tc.regimeIs, blkCtx.BlockHeight, total)
					req.Truef(unclaimed.Sign() >= 0, "%s: unclaimedBalance underflowed at block %d: %s", tc.regimeIs, blkCtx.BlockHeight, unclaimed)
					req.Truef(total.Cmp(unclaimed) >= 0, "%s: totalBalance < unclaimedBalance at block %d (t=%s u=%s)", tc.regimeIs, blkCtx.BlockHeight, total, unclaimed)

					// Derived cumulative mint must equal our independently summed mTotal.
					// (The mock StateManager reports height 0, so exercise the pure
					// CumulativeMinted derivation directly with the real block height; the
					// PostActivationMinted getter wires this same helper to the working-set
					// height in production.)
					infPost := newInflationState()
					_, err = p.state(ctx, sm, _inflKey, infPost)
					req.NoError(err)
					minted := CumulativeMinted(
						g.Rewarding.OutstandingSupplyAtActivationBig(),
						infPost.outstandingSupplyAtYearStart,
						infPost.currentInflationBps,
						activation, blocksPerYear, blkCtx.BlockHeight,
					)
					req.Equalf(0, minted.Cmp(expectedMinted),
						"%s: CumulativeMinted drift at block %d (got %s want %s)",
						tc.regimeIs, blkCtx.BlockHeight, minted, expectedMinted)
				}
			}, nil, false, 0)
		})
	}
}

// IIP-62 reorg-safety: a validator that re-executes a year-boundary block (e.g.
// after an orphan) must produce byte-identical InflationState. The crossing
// branch in mintAndAllocate fires when YearIndex(height) != currentYearIndex —
// persisting currentYearIndex is what makes the branch re-fire deterministically
// on re-execution.
//
// Strategy: drive state to "last block of Y1," capture the (inflation, fund,
// machinaBalance) tuple, then run mintAndAllocate at the Y2-first block twice
// with a full state restore between runs. Assert the two post-states are equal.
func TestMintAndAllocate_ReorgSafe_YearBoundary(t *testing.T) {
	machinaAddr := identityset.Address(33).String()
	const (
		blocksPerYear = 100
		// Pick a supply chunky enough that the Y2 boundary actually moves the snapshot
		// and the per-block mint is non-trivial (so equality is a meaningful assertion).
		supplyRau = "100000000000000000000" // 100 IOTX
	)

	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		req := require.New(t)
		g := genesis.MustExtractGenesisContext(ctx)
		g.Rewarding.InflationRateY1Bps = 10000
		g.Rewarding.InflationDecayNumerator = 8000
		g.Rewarding.InflationDecayDenominator = 10000
		g.Rewarding.InflationFloorBps = 50
		g.Rewarding.BlocksPerYear = blocksPerYear
		g.Rewarding.StakerShareBps = 8000
		g.Rewarding.MachinaShareBps = 2000
		g.Rewarding.MachinaDaoAddress = machinaAddr
		g.Rewarding.OutstandingSupplyAtActivation = supplyRau

		blkCtx := protocol.MustGetBlockCtx(ctx)
		activation := blkCtx.BlockHeight
		g.ToBeEnabledBlockHeight = activation
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		req.NoError(p.CreatePreStates(ctx, sm))

		// Walk Y1 to its final block via mintAndAllocate. Activation height itself is
		// Y1's first block, so Y1's last block is activation + blocksPerYear - 1; the
		// loop covers activation+1 .. activation+blocksPerYear-1. Skip the producer-
		// grant debit since we only need to compare the inflation/fund/machina shape
		// that drives the boundary branch.
		for i := uint64(1); i < blocksPerYear; i++ {
			blkCtx.BlockHeight = activation + i
			ctx = protocol.WithBlockCtx(ctx, blkCtx)
			ctx = genesis.WithGenesisContext(ctx, g)
			ctx = protocol.WithFeatureCtx(ctx)
			_, _, err := p.mintAndAllocate(ctx, sm)
			req.NoErrorf(err, "Y1 walk at block %d", blkCtx.BlockHeight)
		}

		// Sanity: end of Y1 — currentYearIndex must still be 1 (boundary not yet crossed).
		preInf := newInflationState()
		_, err := p.state(ctx, sm, _inflKey, preInf)
		req.NoError(err)
		req.Equalf(uint64(1), preInf.currentYearIndex, "expected end-of-Y1 currentYearIndex=1, got %d", preInf.currentYearIndex)

		// Snapshot full pre-boundary state.
		preInfBytes, err := preInf.Serialize()
		req.NoError(err)
		preFund := fund{}
		_, err = p.state(ctx, sm, _fundKey, &preFund)
		req.NoError(err)
		preFundTotal := new(big.Int).Set(preFund.totalBalance)
		preFundUnclaimed := new(big.Int).Set(preFund.unclaimedBalance)
		preMachina, err := accountutil.LoadAccount(sm, mustAddr(machinaAddr))
		req.NoError(err)
		preMachinaBal := new(big.Int).Set(preMachina.Balance)

		// Step to Y2-first block.
		boundary := activation + blocksPerYear
		blkCtx.BlockHeight = boundary
		ctx = protocol.WithBlockCtx(ctx, blkCtx)
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)

		// Run A: original execution of the boundary block.
		_, _, err = p.mintAndAllocate(ctx, sm)
		req.NoError(err)
		postA := newInflationState()
		_, err = p.state(ctx, sm, _inflKey, postA)
		req.NoError(err)
		req.Equalf(uint64(2), postA.currentYearIndex, "boundary branch did not fire; currentYearIndex=%d", postA.currentYearIndex)
		postAInfBytes, err := postA.Serialize()
		req.NoError(err)
		postAFund := fund{}
		_, err = p.state(ctx, sm, _fundKey, &postAFund)
		req.NoError(err)
		postAMachina, err := accountutil.LoadAccount(sm, mustAddr(machinaAddr))
		req.NoError(err)

		// Restore pre-boundary state — simulate the orphan being rolled back.
		restored := newInflationState()
		req.NoError(restored.Deserialize(preInfBytes))
		req.NoError(p.putState(ctx, sm, _inflKey, restored))
		preFund.totalBalance = preFundTotal
		preFund.unclaimedBalance = preFundUnclaimed
		req.NoError(p.putState(ctx, sm, _fundKey, &preFund))
		preMachina.Balance = preMachinaBal
		req.NoError(accountutil.StoreAccount(sm, mustAddr(machinaAddr), preMachina))

		// Run B: re-execution of the same boundary block on restored state.
		_, _, err = p.mintAndAllocate(ctx, sm)
		req.NoError(err)
		postB := newInflationState()
		_, err = p.state(ctx, sm, _inflKey, postB)
		req.NoError(err)
		postBInfBytes, err := postB.Serialize()
		req.NoError(err)
		postBFund := fund{}
		_, err = p.state(ctx, sm, _fundKey, &postBFund)
		req.NoError(err)
		postBMachina, err := accountutil.LoadAccount(sm, mustAddr(machinaAddr))
		req.NoError(err)

		// Byte-identical inflation state across reorg.
		req.Equal(postAInfBytes, postBInfBytes, "InflationState diverged across reorg of year-boundary block")
		// Spot-check the derived balances too.
		req.Equalf(0, postAFund.totalBalance.Cmp(postBFund.totalBalance),
			"fund.totalBalance diverged: A=%s B=%s", postAFund.totalBalance, postBFund.totalBalance)
		req.Equalf(0, postAFund.unclaimedBalance.Cmp(postBFund.unclaimedBalance),
			"fund.unclaimedBalance diverged: A=%s B=%s", postAFund.unclaimedBalance, postBFund.unclaimedBalance)
		req.Equalf(0, postAMachina.Balance.Cmp(postBMachina.Balance),
			"machina balance diverged: A=%s B=%s", postAMachina.Balance, postBMachina.Balance)
	}, nil, false, 0)
}

func mustAddr(s string) address.Address {
	a, err := address.FromString(s)
	if err != nil {
		panic(err)
	}
	return a
}

// iotxRau returns 10^18 (1 IOTX in Rau) as a big.Int.
func iotxRau() *big.Int {
	r, _ := new(big.Int).SetString("1000000000000000000", 10)
	return r
}
