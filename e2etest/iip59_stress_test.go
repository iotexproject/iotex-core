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

		_, _, _, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
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
	_, _, _, stillDraining, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, tipHeight)
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

		_, _, era, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
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
