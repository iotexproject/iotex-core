// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/chainservice"
)

// iip59StressTiers parameterizes the IIP-59 chunked-drain stress harness.
// Correctness-focused sibling to iip59PerfTiers — asserts fund conservation
// invariants at every block boundary rather than measuring latency.
//
// Compound batch size is picked so the drain always spans multiple blocks,
// otherwise the chunk-continuation code path is never exercised.
var iip59StressTiers = map[string]perfTier{
	// 5 delegates / batch 2 → ceil(5/2) = 3 continuation chunks per drain,
	// plus the Phase-A cursor-freeze block. Runs in ~2s.
	"small": {numDelegates: 5, numVoters: 200, epochsPerEra: 2, compoundBatchSize: 2},
	// 20 delegates / batch 5 → 4 continuation chunks. Env-gated
	// (~30s wall-clock).
	"medium": {numDelegates: 20, numVoters: 2_000, epochsPerEra: 2, compoundBatchSize: 5},
}

// iip59SingleDelegateLargeVoterTier drives the PR 5.5b voter-cap check:
// one delegate with a long voter list, paid in windows of
// voterBudgetPerBlock. compoundBatchSize is deliberately larger than
// numDelegates so the delegate-count cap is never the terminating
// budget — the mid-delegate voter-cap is.
//
// 1 delegate × 500 voters with voterBudgetPerBlock=50 → 10 continuation
// chunks all pointing at DelegateIndex=0, VoterIndex advancing
// 50, 100, ..., 450 across chunks 1-9 and returning to 0 when the
// delegate finishes.
//
// epochsPerEra=11 sizes the era wide enough (numDelegates=1,
// NumSubEpochs=2 → blocks_per_era = 22, chunks_per_era = 11) that all
// 10 chunks complete inside a single era. Since the IIP-59 §10.2
// overrun handler resets any cursor still live at the next era
// boundary, a fixture that leaks the drain across two eras would
// bounce VoterIndex back to 0 and never satisfy the strict
// [0, 50, ..., 450] progression this test asserts.
var iip59SingleDelegateLargeVoterTier = perfTier{
	numDelegates:        1,
	numVoters:           500,
	epochsPerEra:        11,
	compoundBatchSize:   4,
	voterBudgetPerBlock: 50,
}

const iip59StressTierEnv = "IIP59_STRESS_TIER"

// TestIIP59ChunkedDrainStress_SmallTier mints blocks until one full drain
// lifecycle completes (Phase A cursor freeze → cursor-continuation chunks →
// cursor absent). After every committed block it asserts the rewarding-fund
// conservation identity
//
//	totalBalance == unclaimedBalance + Σ(perAddress) + Σ(pool)
//
// across the full seeded address set — a stronger claim than any unit test,
// which is limited to Phase A (the unit mock view does not support
// ConstructBaseView, so Phase B's grantToAccount side-effects never fire).
//
// Also asserts the drain spans ≥ 2 continuation blocks so the chunking
// mechanism is actually exercised, not short-circuited to a single-shot
// drain.
func TestIIP59ChunkedDrainStress_SmallTier(t *testing.T) {
	r := require.New(t)

	tier := selectStressTier(t)
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	installIIP59PerfHooks(t, tier, cfg.Genesis)

	test := newE2ETest(t, cfg)
	defer test.teardown()
	registerEpochProtocols(r, test)

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rp := rolldpos.FindProtocol(test.cs.Registry())
	r.NotNil(rp, "rolldpos protocol must be registered")
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto, "rewarding protocol must be registered")

	addrs := seededStressAddrs(tier)

	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	step := 1 * time.Second

	var (
		drainStartHeight uint64
		drainBlocks      int
		maxBlocks        = drainMintCeiling(tier, 1)
	)

	for minted := 0; minted < maxBlocks; minted++ {
		blkTime = blkTime.Add(step)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())

		height := bc.TipHeight()
		assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, addrs, height)

		_, _, _, _, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at height %d", height)

		if drainStartHeight == 0 {
			if present {
				drainStartHeight = height
				drainBlocks = 1
				t.Logf("drain begins at height=%d epoch=%d", height, rp.GetEpochNum(height))
			}
			continue
		}
		drainBlocks++
		if !present {
			t.Logf("drain ends at height=%d epoch=%d span=%d blocks",
				height, rp.GetEpochNum(height), drainBlocks)
			break
		}
	}
	r.NotZerof(drainStartHeight, "drain never began within %d blocks", maxBlocks)
	r.GreaterOrEqualf(drainBlocks, 2,
		"drain must span ≥ 2 blocks to exercise chunking (batch=%d, delegates=%d)",
		tier.compoundBatchSize, tier.numDelegates)

	// Final gate: cursor absent at tip height. drainSnapshot already
	// returned !present above; this is a defence-in-depth check.
	tipHeight := bc.TipHeight()
	_, _, _, _, stillDraining, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.Falsef(stillDraining, "drain still in progress at tip height %d", tipHeight)
}

// TestIIP59ChunkedDrainStress_MultiEra mints blocks across three era
// boundaries, tracking cursor-present transitions. Asserts:
//   - exactly three distinct drain lifecycles fire (one per era);
//   - the invariant holds at every block boundary throughout;
//   - no residual cursor between eras.
//
// The fixture is deliberately the same as the small tier so any regression
// in cursor teardown between eras (e.g. an era-N cursor leaking into era
// N+1's Phase A build) surfaces here rather than only under production
// mainnet scale.
func TestIIP59ChunkedDrainStress_MultiEra(t *testing.T) {
	r := require.New(t)

	tier := iip59StressTiers["small"]
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	installIIP59PerfHooks(t, tier, cfg.Genesis)

	test := newE2ETest(t, cfg)
	defer test.teardown()
	registerEpochProtocols(r, test)

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rp := rolldpos.FindProtocol(test.cs.Registry())
	r.NotNil(rp, "rolldpos protocol must be registered")
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto, "rewarding protocol must be registered")

	addrs := seededStressAddrs(tier)

	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	step := 1 * time.Second

	const targetEras = 3
	var (
		drainsCompleted int
		inDrain         bool
		lastEraSeen     uint64
		maxBlocks       = drainMintCeiling(tier, targetEras)
	)

	for minted := 0; minted < maxBlocks && drainsCompleted < targetEras; minted++ {
		blkTime = blkTime.Add(step)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())

		height := bc.TipHeight()
		assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, addrs, height)

		_, _, _, era, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at height %d", height)

		switch {
		case present && !inDrain:
			inDrain = true
			r.Greaterf(era, lastEraSeen,
				"cursor era %d must be strictly larger than the previously drained era %d",
				era, lastEraSeen)
			lastEraSeen = era
			t.Logf("drain #%d begins at height=%d era=%d",
				drainsCompleted+1, height, era)
		case !present && inDrain:
			inDrain = false
			drainsCompleted++
			t.Logf("drain #%d ends at height=%d", drainsCompleted, height)
		}
	}
	r.Equalf(targetEras, drainsCompleted,
		"expected %d drain lifecycles across %d eras; observed %d",
		targetEras, targetEras, drainsCompleted)
	r.Falsef(inDrain, "final tip must not have a live cursor")
}

// TestIIP59ChunkedDrainStress_SingleDelegateLargeVoter drives the PR 5.5b
// per-block voter cap. Fixture is one delegate × 500 voters with
// voterBudgetPerBlock=50, so the drain must span multiple mid-delegate
// resume chunks that all point at DelegateIndex=0.
//
// Asserts across chunks:
//   - VoterIndex progresses through the sequence 0, 50, 100, ..., 450
//     across the drain (each value may persist across multiple blocks
//     while the harness is between VoterRewardChunk emissions);
//   - DelegateIndex stays 0 throughout (only one delegate in the cursor);
//   - fund invariant holds at every block boundary;
//   - the drain terminates cleanly (cursor absent) once all 500 voters
//     have been paid.
//
// This is the correctness sibling to the delegate-cap tests above: it
// exercises the *inner* loop (per-voter windowing) rather than the outer
// (per-delegate cap).
func TestIIP59ChunkedDrainStress_SingleDelegateLargeVoter(t *testing.T) {
	r := require.New(t)

	tier := iip59SingleDelegateLargeVoterTier
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	installIIP59PerfHooks(t, tier, cfg.Genesis)

	test := newE2ETest(t, cfg)
	defer test.teardown()
	registerEpochProtocols(r, test)

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rp := rolldpos.FindProtocol(test.cs.Registry())
	r.NotNil(rp, "rolldpos protocol must be registered")
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto, "rewarding protocol must be registered")

	addrs := seededStressAddrs(tier)

	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	step := 1 * time.Second

	// With 500 voters and voterBudget=50, expect exactly 10 mid-delegate
	// chunks after the Phase-A freeze (VoterIndex: 50, 100, ..., 500),
	// plus the Phase-A freeze block itself (VoterIndex=0). VoterIndex
	// value 500 is never observed because reaching it clears the cursor.
	expectedChunks := (tier.numVoters + int(tier.voterBudgetPerBlock) - 1) / int(tier.voterBudgetPerBlock)
	r.Equalf(10, expectedChunks, "unexpected fixture: %d/%d != 10 chunks",
		tier.numVoters, tier.voterBudgetPerBlock)

	var (
		drainStartHeight  uint64
		uniqueVoterIdxs   []uint32
		maxBlocks         = drainMintCeiling(tier, 1)
		presentSeen       int
		absentAfterActive bool
	)

	for minted := 0; minted < maxBlocks; minted++ {
		blkTime = blkTime.Add(step)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())

		height := bc.TipHeight()
		assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, addrs, height)

		delegateIdx, voterIdx, _, _, present, err := drainSnapshot(
			test.cs, cfg.Genesis, rewardProto, height,
		)
		r.NoErrorf(err, "drainSnapshot at height %d", height)

		if drainStartHeight == 0 {
			if !present {
				continue
			}
			drainStartHeight = height
			t.Logf("drain begins at height=%d epoch=%d DI=%d VI=%d",
				height, rp.GetEpochNum(height), delegateIdx, voterIdx)
		}

		if present {
			presentSeen++
			// DelegateIndex must stay 0 the entire drain: fixture has one
			// delegate, so the cursor can never advance past index 0.
			r.Equalf(uint32(0), delegateIdx,
				"height %d: DelegateIndex must stay 0 (single-delegate fixture); got %d",
				height, delegateIdx)
			// Track the unique progression of VoterIndex values.
			if len(uniqueVoterIdxs) == 0 || uniqueVoterIdxs[len(uniqueVoterIdxs)-1] != voterIdx {
				uniqueVoterIdxs = append(uniqueVoterIdxs, voterIdx)
			}
			t.Logf("h=%d present=true DI=%d VI=%d", height, delegateIdx, voterIdx)
			continue
		}
		// present=false after drain was active: drain done.
		absentAfterActive = true
		t.Logf("drain ends at height=%d observations=%d", height, presentSeen)
		break
	}
	r.NotZerof(drainStartHeight, "drain never began within %d blocks", maxBlocks)
	r.Truef(absentAfterActive, "drain never terminated within %d blocks", maxBlocks)

	// The unique VoterIndex progression should be exactly:
	//   0 (Phase A freeze),
	//   50, 100, 150, ..., 450 (chunks 1..9).
	// Chunk 10 pays voters 450..499, clears the cursor on the same
	// block, so VoterIndex=500 is never observed by drainSnapshot.
	wantUnique := make([]uint32, 0, expectedChunks)
	wantUnique = append(wantUnique, 0) // Phase A
	for chunk := 1; chunk < expectedChunks; chunk++ {
		wantUnique = append(wantUnique, uint32(chunk)*uint32(tier.voterBudgetPerBlock))
	}
	r.Equalf(wantUnique, uniqueVoterIdxs,
		"unique VoterIndex progression mismatch:\n  got  %v\n  want %v",
		uniqueVoterIdxs, wantUnique)

	// Final gate: cursor absent at tip height.
	tipHeight := bc.TipHeight()
	_, _, _, _, stillDraining, err := drainSnapshot(
		test.cs, cfg.Genesis, rewardProto, tipHeight,
	)
	r.NoError(err)
	r.Falsef(stillDraining, "drain still in progress at tip height %d", tipHeight)
}

// selectStressTier picks the tier for the SmallTier test — default small,
// overridable to medium via env var so CI can opt into 2000-voter runs
// without spawning a new test binary.
func selectStressTier(t *testing.T) perfTier {
	name := os.Getenv(iip59StressTierEnv)
	if name == "" {
		name = "small"
	}
	tier, ok := iip59StressTiers[name]
	if !ok {
		t.Fatalf("unknown %s=%q; want small|medium", iip59StressTierEnv, name)
	}
	if name == "medium" && testing.Short() {
		t.Skipf("skipping medium stress tier under -short")
	}
	return tier
}

// seededStressAddrs enumerates every address that the perf-bench seeder
// could credit a reward to: each planted delegate's reward address, plus
// each planted voter. TestOnlyAssertFundInvariant cannot enumerate on its
// own (no namespace scan), so the caller supplies the exhaustive set.
func seededStressAddrs(tier perfTier) []address.Address {
	out := make([]address.Address, 0, tier.numDelegates+tier.numVoters)
	for i := 0; i < tier.numDelegates; i++ {
		out = append(out, staking.TestOnlyPerfBenchDelegateAddress(i))
	}
	for j := 0; j < tier.numVoters; j++ {
		out = append(out, staking.TestOnlyPerfBenchVoterAddress(j))
	}
	return out
}

// assertStressInvariant reads fund state at height and confirms
//
//	totalBalance == unclaimedBalance + Σ(perAddress) + Σ(pool).
//
// Fails the test with a formatted delta on mismatch so a broken invariant
// points directly at the offending block boundary.
func assertStressInvariant(
	t *testing.T,
	cs *chainservice.ChainService,
	g genesis.Genesis,
	p *rewarding.Protocol,
	addrs []address.Address,
	height uint64,
) {
	t.Helper()
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	ctx = protocol.WithFeatureCtx(ctx)
	if err := p.TestOnlyAssertFundInvariant(ctx, cs.StateFactory(), addrs); err != nil {
		t.Fatalf("fund invariant violated at height %d: %v", height, err)
	}
}

// drainMintCeiling bounds the mint loop so a broken test cannot spin
// forever. Sized for eras × (era_length + drain_span) with a safety
// multiplier. era_length ≈ numDelegates × numSubEpochs × epochsPerEra;
// drain_span ≈ ceil(numDelegates / compoundBatchSize) + 1 Phase-A block.
func drainMintCeiling(tier perfTier, eras int) int {
	const numSubEpochs = 2 // matches newIIP59PerfCfg
	eraLen := tier.numDelegates * numSubEpochs * int(tier.epochsPerEra)
	drainSpan := (tier.numDelegates + int(tier.compoundBatchSize) - 1) / int(tier.compoundBatchSize)
	blocks := (eraLen + drainSpan + 4) * eras
	if blocks < 100 {
		blocks = 100
	}
	return blocks
}
