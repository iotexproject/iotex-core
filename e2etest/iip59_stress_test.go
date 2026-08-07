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
// Voter budget is picked so the drain always spans multiple blocks,
// otherwise the chunk-continuation code path is never exercised.
var iip59StressTiers = map[string]perfTier{
	// 200 voters / budget 80 → 3 continuation chunks per drain,
	// plus the Phase-A cursor-freeze block. Runs in ~2s.
	"small": {numDelegates: 5, numVoters: 200, epochsPerEra: 2, voterBudgetPerBlock: 80},
	// 2,000 voters / budget 500 → 4 continuation chunks. Env-gated
	// (~30s wall-clock).
	"medium": {numDelegates: 20, numVoters: 2_000, epochsPerEra: 2, voterBudgetPerBlock: 500},
}

// iip59SingleDelegateLargeVoterTier drives the per-block voter-cap check:
// one delegate with a long voter list, paid in windows of
// voterBudgetPerBlock.
//
// 1 delegate × 500 voters with voterBudgetPerBlock=50 → about 10
// continuation chunks, each resuming where the last one left the key-space
// shard walk.
//
// epochsPerEra=11 sizes the era wide enough (numDelegates=1,
// NumSubEpochs=2 → blocks_per_era = 22, chunks_per_era = 11) that the whole
// drain completes inside a single era. The IIP-59 §10.2 overrun handler
// resets any cursor still live at the next era boundary, so a fixture that
// leaked the drain across two eras would restart the walk and never show
// monotonic progress.
var iip59SingleDelegateLargeVoterTier = perfTier{
	numDelegates:        1,
	numVoters:           500,
	epochsPerEra:        11,
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

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

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

	watcher := newIIP59DrainWatch()
	for minted := 0; minted < maxBlocks; minted++ {
		blkTime = blkTime.Add(step)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())

		height := bc.TipHeight()
		assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, addrs, height)

		_, _, _, _, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at height %d", height)
		plan, completed, planPresent, err := drainPlan(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainPlan at height %d", height)
		if planPresent && !completed {
			watcher.observe(t, height, plan)
		}

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
		"drain must span ≥ 2 blocks to exercise chunking (voterBudget=%d, voters=%d)",
		tier.voterBudgetPerBlock, tier.numVoters)

	// Final gate: cursor absent at tip height. drainSnapshot already
	// returned !present above; this is a defence-in-depth check.
	tipHeight := bc.TipHeight()
	_, _, _, _, stillDraining, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.Falsef(stillDraining, "drain still in progress at tip height %d", tipHeight)

	// The fund invariant above is a conservation claim: it holds just as well
	// when every voter is paid zero and the whole pool is swept as residual.
	// This is the claim about who got what.
	plan, completed, planPresent, err := drainPlan(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.True(planPresent, "the completed settlement's plan must be readable at tip")
	r.True(completed, "the settlement must be marked completed once the cursor goes absent")
	assertIIP59PerVoterPayouts(t, iip59DrainRun{
		tier:     tier,
		g:        cfg.Genesis,
		plan:     plan,
		balances: iip59VoterBalances(t, test.cs, cfg.Genesis, addrs),
	})
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

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

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

// TestIIP59ChunkedDrainStress_SingleDelegateLargeVoter drives the per-block
// voter cap. Fixture is one delegate x 500 voters with voterBudgetPerBlock=50,
// so the drain must span multiple continuation chunks.
//
// Asserts across chunks:
//   - the drain takes more than one observed block (the cap actually bites);
//   - ShardsDone never goes backwards and never exceeds the shard count;
//   - the shard walk advances across blocks;
//   - fund invariant holds at every block boundary;
//   - the drain terminates cleanly (cursor absent) once every voter is paid.
//
// The exact resume positions are deliberately not pinned. The walk is over the
// voter key space, so where a 50-voter budget lands inside 256 shards is a
// function of the voters' addresses rather than of a delegate's list position;
// asserting a precise sequence would assert something about identityset, not
// about the drain.
func TestIIP59ChunkedDrainStress_SingleDelegateLargeVoter(t *testing.T) {
	r := require.New(t)

	tier := iip59SingleDelegateLargeVoterTier
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

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
		drainStartHeight  uint64
		lastShardsDone    uint32
		shardsObserved    []uint32
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

		shardsDone, resumeLen, _, _, present, err := drainSnapshot(
			test.cs, cfg.Genesis, rewardProto, height,
		)
		r.NoErrorf(err, "drainSnapshot at height %d", height)

		if drainStartHeight == 0 {
			if !present {
				continue
			}
			drainStartHeight = height
			t.Logf("drain begins at height=%d epoch=%d shardsDone=%d",
				height, rp.GetEpochNum(height), shardsDone)
		}

		if present {
			presentSeen++
			r.LessOrEqualf(shardsDone, uint32(256),
				"height %d: shardsDone %d exceeds the shard count", height, shardsDone)
			r.GreaterOrEqualf(shardsDone, lastShardsDone,
				"height %d: shardsDone went backwards (%d -> %d)", height, lastShardsDone, shardsDone)
			lastShardsDone = shardsDone
			if len(shardsObserved) == 0 || shardsObserved[len(shardsObserved)-1] != shardsDone {
				shardsObserved = append(shardsObserved, shardsDone)
			}
			t.Logf("h=%d present=true shardsDone=%d resumeLen=%d", height, shardsDone, resumeLen)
			continue
		}
		absentAfterActive = true
		t.Logf("drain ends at height=%d observations=%d", height, presentSeen)
		break
	}
	r.NotZerof(drainStartHeight, "drain never began within %d blocks", maxBlocks)
	r.Truef(absentAfterActive, "drain never terminated within %d blocks", maxBlocks)
	r.Greaterf(presentSeen, 1,
		"the per-block voter cap never bit: drain finished in %d observed block(s)", presentSeen)
	r.Greaterf(len(shardsObserved), 1,
		"the shard walk never advanced across blocks: observed %v", shardsObserved)

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
		out = append(out, perfVoterAddress(tier, j))
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
// drain_span ≈ ceil(numVoters / voterBudgetPerBlock) + 1 Phase-A block.
func drainMintCeiling(tier perfTier, eras int) int {
	const numSubEpochs = 2 // matches newIIP59PerfCfg
	eraLen := tier.numDelegates * numSubEpochs * int(tier.epochsPerEra)
	// A zero budget means unbounded: the whole era drains in one chunk.
	drainSpan := 1
	if tier.voterBudgetPerBlock > 0 {
		drainSpan = (tier.numVoters + int(tier.voterBudgetPerBlock) - 1) / int(tier.voterBudgetPerBlock)
	}
	blocks := (eraLen + drainSpan + 4) * eras
	if blocks < 100 {
		blocks = 100
	}
	return blocks
}
