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
// materializes. VoterBudgetPerBlock bounds voters processed; the independent
// key budget prevents enumeration work from following the total index size.
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

// denseVoterRangeScenario plants count voters in one compact address range.
func denseVoterRangeScenario(
	t *testing.T,
	prefix byte,
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
		voters[i] = voterWithPrefix(prefix, i)
		// Stakes vary but stay small: int64 rau amounts overflow past ~9
		// tokens' worth of multiples, and an overflowed (negative) amount is
		// rejected by the bucket writer long before the drain sees it.
		seeds = append(seeds, iip59NativeSeed{
			delegate: delegate, voter: voters[i], amount: int64(i%5+1) * rau,
		})
	}
	// Start in the populated range so the first chunk measures real scan work.
	newDrainScenario(t, ctx, sm, p, []byte{prefix}, 1_000_000_000, seeds, nil)
	return ctx, &scanCountingStateManager{StateManager: sm}, p, voters
}

// TestVoterDrainScanCostDoesNotFollowVoterSetSize is the R4 regression.
//
// It is deliberately a *relative* assertion. An absolute key count would encode
// how many index streams the fixture happens to populate and would have to be
// retuned whenever that changed. The property that matters cannot be tuned
// away: quadrupling the number of voters in one compact range must not
// increase what a single block reads. Before the fix the first block scanned
// the whole range, so the two counts differed by the same factor as the range
// sizes.
func TestVoterDrainScanCostDoesNotFollowVoterSetSize(t *testing.T) {
	r := require.New(t)
	const prefix = byte(0x7a)
	const budget = uint64(5)

	firstChunkScanKeys := func(count int) int {
		ctx, sm, p, _ := denseVoterRangeScenario(t, prefix, count, budget)
		before := sm.rangeScanKeys
		_, _, err := p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		return sm.rangeScanKeys - before
	}

	small := firstChunkScanKeys(50)
	large := firstChunkScanKeys(200)

	r.Greater(small, 0, "the fixture must actually issue ranged scans")
	r.LessOrEqual(large, small,
		"one block's scan cost must be set by the per-block budget, not by voter population "+
			"(50 voters cost %d keys, 200 voters cost %d)", small, large)
	r.Less(large, 200,
		"a single block must not materialize the whole voter range (read %d keys for 200 voters)", large)

	r.LessOrEqual(large, int(budget)*_voterScanKeyBudgetPerVoter,
		"scan keys must stay inside the per-block key budget")
}

// liveAndCOWStreams names the four key streams staking.ScanFrozenVoters
// merges: the native and liquid-staking live voter indexes, plus the
// copy-on-write entry range for each. Its length is what a single round's key
// cost is a multiple of.
var liveAndCOWStreams = [4]string{"native", "lsd", "cowNative", "cowLSD"}

// TestVoterDrainDenseRangePaysEveryVoterExactlyOnce is the correctness half
// of R4, and the reason the bound is a coverage bound rather than a result
// count.
//
// Truncating each of the four merged streams to N and taking the first N
// results is wrong: the copy-on-write scan drops tombstones *after* scanning,
// so a truncated stream returns fewer addresses than keys, and a naive
// count-based resume can step over a voter that another stream would have
// produced below the cut. Because ResumeVoter then advances past them, such a
// voter is never revisited and never paid in this era -- a silent underpayment
// that no fund invariant would catch, since the money would remain in the
// pending pool.
func TestVoterDrainDenseRangePaysEveryVoterExactlyOnce(t *testing.T) {
	r := require.New(t)
	const prefix = byte(0x7a)
	const count = 120
	ctx, sm, p, voters := denseVoterRangeScenario(t, prefix, count, 5)

	paid := map[string]int{}
	for i := 0; ; i++ {
		cursor, err := p.readVoterRewardDistributionState(ctx, sm)
		r.NoError(err)
		if cursor == nil || cursor.completed() {
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

func TestVoterDrainProcessesExactlyTwoThousandVotersPerBlock(t *testing.T) {
	r := require.New(t)
	const (
		prefix = byte(0x7a)
		count  = 2001
		budget = uint64(2000)
	)
	ctx, sm, p, _ := denseVoterRangeScenario(t, prefix, count, budget)

	txLogs, _, err := p.GrantVoterRewardChunk(ctx, sm)
	r.NoError(err)
	paid := 0
	for _, l := range txLogs {
		if l.Type == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND {
			paid++
		}
	}
	r.Equal(int(budget), paid)
	cursor, err := p.readVoterRewardDistributionState(ctx, sm)
	r.NoError(err)
	r.False(cursor.completed())

	txLogs, _, err = p.GrantVoterRewardChunk(ctx, sm)
	r.NoError(err)
	paid = 0
	for _, l := range txLogs {
		if l.Type == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND {
			paid++
		}
	}
	r.Equal(1, paid)
	cursor, err = p.readVoterRewardDistributionState(ctx, sm)
	r.NoError(err)
	r.True(cursor.completed())
}
