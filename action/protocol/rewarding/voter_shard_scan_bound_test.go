// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// scanCountingStateManager counts the keys every ranged States() scan
// materializes. That is the quantity R4 is about: the per-block voter budget
// used to bound only the voters a block *paid*, while the shard read that fed
// the loop was issued unbounded and therefore sized by the shard, which an
// attacker chooses.
type scanCountingStateManager struct {
	protocol.StateManager
	rangeScans    int
	rangeScanKeys int
}

func (c *scanCountingStateManager) States(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
	height, iter, err := c.StateManager.States(opts...)
	if err != nil {
		return height, iter, err
	}
	cfg, cfgErr := protocol.CreateStateConfig(opts...)
	if cfgErr == nil && cfg.Keys == nil && (cfg.RangeMin != nil || cfg.RangeMax != nil) {
		c.rangeScans++
		c.rangeScanKeys += iter.Size()
	}
	return height, iter, err
}

// stuffedShardScenario plants `count` voters that all share one shard byte,
// which is the adversarial shape: the first byte of an address is grindable, so
// an attacker can pile an unbounded number of voters into a single contiguous
// key range and make whichever block lands on that shard read all of it.
func stuffedShardScenario(
	t *testing.T,
	shard byte,
	count int,
	budget uint64,
) (context.Context, *scanCountingStateManager, *Protocol, []address.Address) {
	t.Helper()
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	p.cfg.VoterBudgetPerBlock = budget

	const rau = int64(1_000_000_000_000_000_000)
	delegate := identityset.Address(4)
	voters := make([]address.Address, count)
	seeds := make([]iip59NativeSeed, 0, count)
	for i := 0; i < count; i++ {
		voters[i] = sameShardVoter(shard, i)
		// Stakes vary but stay small: int64 rau amounts overflow past ~9
		// tokens' worth of multiples, and an overflowed (negative) amount is
		// rejected by the bucket writer long before the drain sees it.
		seeds = append(seeds, iip59NativeSeed{
			delegate: delegate, voter: voters[i], amount: int64(i%5+1) * rau,
		})
	}
	// A seed whose start shard is the stuffed one, so the very first chunk
	// lands on it rather than sweeping empty shards first.
	newDrainScenario(t, ctx, sm, p, []byte{shard}, 1_000_000_000, seeds, nil)
	return ctx, &scanCountingStateManager{StateManager: sm}, p, voters
}

// TestVoterDrainScanCostDoesNotFollowShardSize is the R4 regression.
//
// It is deliberately a *relative* assertion. An absolute key count would encode
// how many index streams the fixture happens to populate and would have to be
// retuned whenever that changed. The property that matters cannot be tuned
// away: quadrupling the number of voters crammed into one shard must not
// increase what a single block reads. Before the fix the first block scanned
// the whole shard, so the two counts differed by the same factor as the shard
// sizes.
func TestVoterDrainScanCostDoesNotFollowShardSize(t *testing.T) {
	r := require.New(t)
	const shard = byte(0x7a)
	const budget = uint64(5)

	firstChunkScanKeys := func(count int) int {
		ctx, sm, p, _ := stuffedShardScenario(t, shard, count, budget)
		before := sm.rangeScanKeys
		_, _, err := p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		return sm.rangeScanKeys - before
	}

	small := firstChunkScanKeys(50)
	large := firstChunkScanKeys(200)

	r.Greater(small, 0, "the fixture must actually issue ranged scans")
	r.LessOrEqual(large, small,
		"one block's scan cost must be set by the per-block budget, not by shard population "+
			"(50 voters cost %d keys, 200 voters cost %d)", small, large)
	r.Less(large, 200,
		"a single block must not materialize a whole stuffed shard (read %d keys for 200 voters)", large)

	// And the bound is the one the budget implies, not an accident: at most the
	// per-block key budget plus the one round that may cross it.
	maxRoundKeys := len(liveAndCOWStreams) * (int(budget) + 1)
	r.LessOrEqual(large, int(budget)*_voterScanKeyBudgetPerVoter+maxRoundKeys,
		"scan keys must stay inside the per-block key budget")
}

// liveAndCOWStreams names the four key streams staking.FrozenShardVoters
// merges: the native and liquid-staking live voter indexes, plus the
// copy-on-write entry range for each. Its length is what a single round's key
// cost is a multiple of.
var liveAndCOWStreams = [4]string{"native", "lsd", "cowNative", "cowLSD"}

// TestVoterDrainStuffedShardPaysEveryVoterExactlyOnce is the correctness half
// of R4, and the reason the bound is a coverage bound rather than a result
// count.
//
// Truncating each of the four merged streams to N and taking the first N
// results is wrong: the copy-on-write scan drops tombstones *after* scanning,
// so a truncated stream returns fewer addresses than keys, and a naive
// count-based resume can step over a voter that another stream would have
// produced below the cut. Because ResumeVoter then advances past them, such a
// voter is never revisited and never paid -- a silent, permanent underpayment
// that no fund invariant would catch, since the money would fall through to the
// residual sweep.
func TestVoterDrainStuffedShardPaysEveryVoterExactlyOnce(t *testing.T) {
	r := require.New(t)
	const shard = byte(0x7a)
	const count = 120
	ctx, sm, p, voters := stuffedShardScenario(t, shard, count, 5)

	paid := map[string]int{}
	for i := 0; ; i++ {
		cursor, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		if cursor == nil || cursor.Completed {
			break
		}
		txLogs, _, err := p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		for _, l := range txLogs {
			if l.Type == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND {
				paid[l.Recipient]++
			}
		}
		r.Less(i, 4000, "drain did not complete; the bounded scan is not advancing the cursor")
	}
	for _, v := range voters {
		r.Equal(1, paid[v.String()],
			"voter %s must be paid exactly once by the bounded walk", v)
	}
}

// TestBoundedShardReaderPassesThroughUnrangedCalls guards the decorator's
// blast radius. Only ranged scans may be limited: the copy-on-write tombstone
// check is a point read per key, and a keyed States() call has no ordering to
// truncate.
func TestBoundedShardReaderPassesThroughUnrangedCalls(t *testing.T) {
	r := require.New(t)
	_, sm, _, _, _ := newVoterRewardCtx(t, true)
	reader := newBoundedShardReader(sm, 3)
	r.Equal(4, reader.limit, "the injected limit must leave a slot for the resume key")

	// No range set: not decorated, not recorded.
	_, _, _ = reader.States(protocol.NamespaceOption("test"), protocol.KeysOption(func() ([][]byte, error) {
		return [][]byte{[]byte("a")}, nil
	}))
	r.Empty(reader.scans, "keyed States() must not be recorded as a bounded scan")

	coverage, complete, err := reader.coverage()
	r.NoError(err)
	r.True(complete, "no bounded scans means nothing constrains coverage")
	r.Equal(_completeCoverage, coverage)
}

// TestVoterScanLimitZeroBudgetIsUnbounded pins the pre-fork / unconfigured
// path: a zero voter budget means "no limit", and the decorator must then be a
// pure pass-through so behaviour is byte-identical to before this file existed.
func TestVoterScanLimitZeroBudgetIsUnbounded(t *testing.T) {
	r := require.New(t)
	r.Equal(0, voterScanLimit(0, 0))
	r.Equal(0, voterScanLimit(0, 100))
	r.Equal(5, voterScanLimit(5, 40))
	r.Equal(3, voterScanLimit(5, 3), "the key budget caps the voter budget")
	// A non-positive key budget imposes no extra cap. It is unreachable from
	// the drain (the chunk loop breaks before calling in with an exhausted key
	// budget), so the safe reading is "no constraint supplied", not "limit to
	// zero" -- a zero here would mean unbounded, the opposite of intent.
	r.Equal(5, voterScanLimit(5, 0))
	r.Equal(5, voterScanLimit(5, -1))

	_, sm, _, _, _ := newVoterRewardCtx(t, true)
	reader := newBoundedShardReader(sm, 0)
	r.Equal(0, reader.limit)
}
