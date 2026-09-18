// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/chainservice"
)

// iip59NothingToSettleTier is an era with no voter entitlement at all: 100%
// commission leaves the voter share at zero, so no pending pool is ever
// credited and every era boundary arrives with nothing to distribute.
var iip59NothingToSettleTier = perfTier{
	numDelegates:        3,
	numVoters:           50,
	epochsPerEra:        2,
	voterBudgetPerBlock: 20,
	epochCommissionBPs:  10_000,
}

// eraObservation is one block's worth of the state these two scenarios turn on.
type eraObservation struct {
	height     uint64
	epoch      uint64
	cursor     bool
	scanPhase  uint32
	targetEra  uint64
	poolTotal  *big.Int
	poolCount  int
	abandoned  []string
	chunkCount int
}

// observeEra mints n blocks and records per-block drain and pool state, failing
// the test on any fund-conservation violation along the way.
func observeEra(
	t *testing.T,
	r *require.Assertions,
	test *e2etest,
	cfg genesisCarrier,
	tier perfTier,
	blocks int,
) []eraObservation {
	t.Helper()
	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rp := rolldpos.FindProtocol(test.cs.Registry())
	r.NotNil(rp)
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto)

	addrs := seededStressAddrs(tier)
	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	out := make([]eraObservation, 0, blocks)

	for i := 0; i < blocks; i++ {
		blkTime = blkTime.Add(time.Second)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())
		height := bc.TipHeight()

		// Conservation is the claim that matters most in both scenarios:
		// totalBalance == unclaimed + per-address + pools, at every block.
		assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, addrs, height)

		phase, _, _, era, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at %d", height)
		total, count, err := poolTotals(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "poolTotals at %d", height)
		drains, chunks, err := drainAbandonedLogs(test, height)
		r.NoErrorf(err, "receipts at %d", height)

		out = append(out, eraObservation{
			height: height, epoch: rp.GetEpochNum(height),
			cursor: present, scanPhase: phase, targetEra: era,
			poolTotal: total, poolCount: count,
			abandoned: drains, chunkCount: chunks,
		})
	}
	return out
}

// poolTotals sums every pending voter pool at height.
func poolTotals(
	cs *chainservice.ChainService,
	g genesis.Genesis,
	p *rewarding.Protocol,
	height uint64,
) (*big.Int, int, error) {
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	ctx = protocol.WithFeatureCtx(ctx)
	entries, err := p.TestOnlyAllPoolEntries(ctx, cs.StateFactory())
	if err != nil {
		return nil, 0, err
	}
	total := new(big.Int)
	for _, e := range entries {
		total.Add(total, e.Amount)
	}
	return total, len(entries), nil
}

// TestIIP59EraWithNothingToSettle pins what an era boundary does when no voter
// is owed anything.
//
// Observed: nothing at all. With commission at 100% the voter share is zero, so
// no pending pool is ever credited, and an era boundary that finds no pool
// builds no cursor. No settlement action is emitted, no log, no state. The
// chain crosses boundary after boundary silently and the fund identity holds at
// every block.
//
// This is the case worth pinning precisely because it is invisible: the failure
// mode it guards against is a boundary that materialises an empty cursor and
// then has the dispatcher emit a chunk per block against it forever.
func TestIIP59EraWithNothingToSettle(t *testing.T) {
	r := require.New(t)
	tier := iip59NothingToSettleTier
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

	obs := observeEra(t, r, test, genesisCarrier{cfg.Genesis}, tier, 60)

	erasSeen := map[uint64]bool{}
	for _, o := range obs {
		erasSeen[o.epoch/tier.epochsPerEra] = true
		r.Zerof(o.poolTotal.Sign(),
			"h=%d: a 100%% commission leaves no voter share, so no pool may be credited (%s across %d entries)",
			o.height, o.poolTotal, o.poolCount)
		r.Falsef(o.cursor, "h=%d: nothing to settle must not materialise a cursor", o.height)
		r.Zerof(o.chunkCount, "h=%d: no cursor means no settlement action", o.height)
		r.Emptyf(o.abandoned, "h=%d: nothing was started, so nothing can be abandoned", o.height)
	}
	r.GreaterOrEqualf(len(erasSeen), 3,
		"fixture must cross several era boundaries to show they stay silent, saw %d", len(erasSeen))
}

// TestIIP59EraThatDoesNotFinish pins what happens to the money when a
// settlement cannot complete inside its era.
//
// Observed, per era: the boundary builds a cursor and the drain pays voters
// chunk by chunk, drawing each delegate's pending pool down as it goes. The
// next freeze then supersedes the window mid-walk, the cursor is retired, and
// whatever was still in the pools stays there. It is not swept, not burned, and
// not paid twice -- the following era's boundary builds a fresh cursor over the
// same pools and works the residue off along with its own accrual.
//
// So an unfinished settlement is a deferral, not a loss. What makes that safe
// is that the pools are the ledger: the drain decrements them as it pays, so
// the remainder is exactly what is still owed.
func TestIIP59EraThatDoesNotFinish(t *testing.T) {
	r := require.New(t)
	tier := iip59OverrunTier
	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

	// Long enough that the last settlement to run out of era still has a
	// successor whose drain gets going before the run ends. An era here is 20
	// blocks and its drain starts the block after the boundary.
	obs := observeEra(t, r, test, genesisCarrier{cfg.Genesis}, tier, 130)

	var (
		abandonedAt  []int
		firstPeak    *big.Int
		lastPeak     *big.Int
		resumedCount int
	)
	for i, o := range obs {
		if len(o.abandoned) > 0 {
			abandonedAt = append(abandonedAt, i)
		}
		if firstPeak == nil || o.poolTotal.Cmp(firstPeak) > 0 {
			if len(abandonedAt) <= 1 {
				firstPeak = new(big.Int).Set(o.poolTotal)
			}
		}
		lastPeak = o.poolTotal
	}

	r.GreaterOrEqualf(len(abandonedAt), 2,
		"fixture must let at least two settlements run out of era, saw %d", len(abandonedAt))

	for _, i := range abandonedAt {
		o := obs[i]
		// The residue is still owed, so it has to still be there.
		r.Positivef(o.poolTotal.Sign(),
			"h=%d: a settlement abandoned with the pools emptied would mean the remainder was paid "+
				"or swept, and neither happened", o.height)

		// And the dispatcher has to go quiet: this is what the retirement is for.
		for j := i + 1; j < len(obs) && obs[j].targetEra == o.targetEra; j++ {
			r.Zerof(obs[j].chunkCount,
				"h=%d: era %d was retired at h=%d, so no further settlement action may be emitted",
				obs[j].height, o.targetEra, o.height)
			r.Emptyf(obs[j].abandoned,
				"h=%d: era %d already announced its retirement at h=%d", obs[j].height, o.targetEra, o.height)
		}

		// The next era must pick the residue up rather than strand it: a fresh
		// cursor appears and the pools fall again.
		//
		// Skipped for an abandonment with no successor era inside the observed
		// run -- that says the window ended, not that the residue was lost.
		var sawLaterEra, resumed bool
		for j := i + 1; j < len(obs); j++ {
			if obs[j].targetEra <= o.targetEra {
				continue
			}
			sawLaterEra = true
			if obs[j].cursor && obs[j].poolTotal.Cmp(o.poolTotal) < 0 {
				resumed = true
				break
			}
		}
		if !sawLaterEra {
			continue
		}
		resumedCount++
		r.Truef(resumed,
			"h=%d: era %d left %s in the pools and the era after it never drew that down; "+
				"the residue is stranded rather than deferred",
			o.height, o.targetEra, o.poolTotal)
	}
	r.Positivef(resumedCount,
		"no abandonment in this run had a successor era, so nothing was proved about the residue")

	// Deferral must not become accumulation. If every era shed its residue
	// forward without ever working it off, the pools would climb run over run.
	r.NotNil(firstPeak)
	bound := new(big.Int).Mul(firstPeak, big.NewInt(2))
	r.Truef(lastPeak.Cmp(bound) <= 0,
		"pending pools grew from a first-era peak of %s to %s; residue is accumulating rather than settling",
		firstPeak, lastPeak)
}

// genesisCarrier keeps observeEra's signature short.
type genesisCarrier struct{ Genesis genesis.Genesis }
