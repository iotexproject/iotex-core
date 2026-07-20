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
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// enableIIP59 flips ToBeEnabledBlockHeight so cursorEnabled=true at the
// current block height and re-derives the feature contexts. Callers use
// this to turn on the fork gate for a testProtocol-scaffolded ctx that
// starts with the fork off.
func enableIIP59(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	g := genesis.MustExtractGenesisContext(ctx)
	g.ToBeEnabledBlockHeight = 1
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithFeatureCtx(ctx)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	return ctx
}

// seedChunkCursor loads the same epoch-scoped rewarded-candidate list
// GrantVoterRewardChunk will re-derive and builds a cursor with one
// entry per candidate (frozen voter amount = 1 rau to exercise the
// drain loop without seeding real snapshots). Sets DelegateIndex to
// startIdx and persists it. Returns the cursor for the test to assert
// against.
//
// Post-C3, real cursor entries are compacted (opted-in delegates
// only) and their frozen amount is the sum of pool accrual + epoch
// voter share. These tests care about chunking flow control, not
// distribution correctness, so the entry list is synthesised
// directly rather than driven through a snapshot fixture.
func seedChunkCursor(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	epochNum uint64,
	startIdx uint32,
) *epochDrainCursor {
	t.Helper()
	r := require.New(t)
	_, _, _, _, rewardedCandidates, _, _, err := p.loadEpochDistributionInputs(ctx, sm, epochNum)
	r.NoError(err)
	entries := make([]epochDrainDelegateWork, 0, len(rewardedCandidates))
	for _, cand := range rewardedCandidates {
		if cand == nil {
			continue
		}
		candID, cErr := candidateIdentifierBytes(cand.Address)
		r.NoError(cErr)
		entries = append(entries, epochDrainDelegateWork{
			CandidateIdentifier: candID,
			VoterAmountFrozen:   big.NewInt(1),
		})
	}
	cursor := &epochDrainCursor{
		TargetEra:     epochNum,
		DelegateIndex: startIdx,
		Delegates:     entries,
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))
	return cursor
}

// TestGrantVoterRewardChunk_HappyPath verifies a mid-drain chunk:
// - chunkSize < remaining delegates,
// - DelegateIndex advances by chunkSize,
// - cursor persists (Phase C coda skipped),
// - no sentinel is written yet.
func TestGrantVoterRewardChunk_HappyPath(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		// Fund the rewarding pool so grantToAccount / updateAvailableBalance
		// have headroom for the chunk's payouts.
		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Force chunking: CompoundBatchSize=2 with 4 rewarded delegates
		// (default NumDelegatesForEpochReward=4) yields two chunks.
		p.cfg.CompoundBatchSize = 2

		cursor := seedChunkCursor(t, ctx, sm, p, 1, 0)
		require.Greater(t, len(cursor.Delegates), 2,
			"test precondition: need >2 delegates to exercise mid-drain chunk")
		total := uint32(len(cursor.Delegates))

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		// Cursor must still be present with index advanced by exactly one
		// chunk. Sentinel must NOT be set — this isn't the last chunk.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got, "mid-drain cursor must survive")
		r.Equal(uint64(1), got.TargetEra)
		r.Equal(uint32(2), got.DelegateIndex, "DelegateIndex advances by chunkSize=2")
		r.Equal(int(total), len(got.Delegates), "cursor length unchanged mid-drain")

		// assertNoRewardYet returns nil when sentinel does NOT exist.
		r.NoError(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"mid-drain must not write epoch reward sentinel")
	}, nil, false, 0)
}

// TestGrantVoterRewardChunk_LastChunkRunsCoda verifies the terminal
// chunk runs the post-C3 coda: orphan sweep + cursor delete. The
// epoch sentinel is Phase A's responsibility (written by
// GrantEpochReward) and is NOT touched by the chunk anymore. Seeded
// state: cursor with DelegateIndex advanced so this run consumes only
// the tail slice.
func TestGrantVoterRewardChunk_LastChunkRunsCoda(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		p.cfg.CompoundBatchSize = 2

		// Seed with DelegateIndex pointing at the last chunk. For 4
		// delegates + chunkSize=2, startIdx=2 makes this the terminal
		// chunk that runs the coda.
		cursor := seedChunkCursor(t, ctx, sm, p, 1, 0)
		total := uint32(len(cursor.Delegates))
		r.GreaterOrEqual(int(total), 2)
		cursor.DelegateIndex = total - 2
		r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		// Cursor must be deleted after the coda.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got, "terminal chunk must delete the cursor")
	}, nil, false, 0)
}

// TestGrantVoterRewardChunk_CrossEraContinuation verifies that a cursor
// whose TargetEra != current block's epoch still drives the epoch-scoped
// state load from cursor.TargetEra, so the drain completes cleanly on a
// block whose GetEpochNum(blockHeight) differs from the era being
// drained. This is the scenario the C2 `>` guard patched, now handled
// naturally because GrantVoterRewardChunk never reads the block's epoch.
func TestGrantVoterRewardChunk_CrossEraContinuation(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		p.cfg.CompoundBatchSize = 2

		// Cursor pins the era that started the drain (era 1); this
		// continuation block is well into a later era.
		cursor := seedChunkCursor(t, ctx, sm, p, 1, 0)
		total := uint32(len(cursor.Delegates))
		cursor.DelegateIndex = total - 2
		r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))

		// Bump BlockCtx height into a much later block so a bug that
		// used rp.GetEpochNum(blockHeight) instead of cursor.TargetEra
		// would produce a wildly different epochNum and misload state.
		g := genesis.MustExtractGenesisContext(ctx)
		later := 5 * g.NumDelegates * g.NumSubEpochs
		blk := protocol.MustGetBlockCtx(ctx)
		blk.BlockHeight = later
		ctx = protocol.WithBlockCtx(ctx, blk)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		// Post-C3 the chunk's coda no longer writes a sentinel — that
		// moved back to GrantEpochReward. The invariant this test
		// still guards is that a cross-era continuation completes
		// without corrupting cursor state.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got, "terminal chunk must delete the cursor")
	}, nil, false, 0)
}

// TestGrantVoterRewardChunk_MissingCursorErrors verifies the dispatcher
// invariant: GrantVoterRewardChunk without a live cursor is unambiguous
// programmer error (CreatePostSystemActions should never emit the action
// when the cursor is absent). Handler must reject loud rather than
// silently no-op.
func TestGrantVoterRewardChunk_MissingCursorErrors(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		// Sanity-check: no cursor written.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got, "test precondition: no cursor present")

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.Error(err)
		r.Contains(err.Error(), "voter reward chunk dispatched without a live cursor")
	}, nil, false, 0)
}

// TestGrantVoterRewardChunk_PreForkRejects verifies the fork gate blocks
// VoterRewardChunk both at Validate (external, defense-in-depth) and at
// the handler (internal, defense-in-depth). With NoVoterRewardDistribution
// true, neither path may allow the action through.
func TestGrantVoterRewardChunk_PreForkRejects(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		// testProtocol leaves the fork gate closed by default; sanity check.
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		r.True(protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution,
			"test precondition: fork gate must be closed")

		// Validate path: producer must equal caller for the check to reach
		// the fork gate. Rewire ActionCtx so caller == producer.
		blk := protocol.MustGetBlockCtx(ctx)
		validateCtx := protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:       blk.Producer,
			GasPrice:     big.NewInt(0),
			IntrinsicGas: 0,
		})
		validateCtx = protocol.WithFeatureCtx(validateCtx)
		elp := createGrantRewardAction(action.VoterRewardChunk, blk.BlockHeight)
		err := p.Validate(validateCtx, elp, nil)
		r.Error(err)
		r.Contains(err.Error(), "voter reward chunk action not enabled yet")

		// Handler path: defense-in-depth if the action ever reaches Handle
		// pre-fork (should be unreachable via normal execution).
		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.Error(err)
		r.Contains(err.Error(), "voter reward chunk action requires IIP-59 fork")

		// Also verify the fork gate is symmetric to the "feature off
		// ignores cursor" invariant on GrantEpochReward: with a cursor
		// injected, the pre-fork VoterRewardChunk handler still refuses
		// rather than reading it.
		injected := &epochDrainCursor{
			TargetEra:     1,
			DelegateIndex: 0,
			Delegates: []epochDrainDelegateWork{
				{CandidateIdentifier: identityset.Address(27).Bytes(), VoterAmountFrozen: big.NewInt(1)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, injected))
		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.Error(err)
		r.Contains(err.Error(), "voter reward chunk action requires IIP-59 fork")
	}, nil, false, 0)
}

// TestGrantVoterRewardChunk_LateAccrualSurvivesToNextEra guards the C3
// design's cross-era accrual claim: a block-side voter credit that lands
// in a delegate's pending pool after Phase A has frozen the era-N cursor
// but before that delegate's era-N chunk runs is NOT drained by the
// era-N chunk (which drains only the frozen amount), stays in the pool
// through the era-N coda, and is folded into the era-N+1 cursor by the
// next Phase A.
//
// Fixture: seed 250 rau in candidate 27's pool → Phase A at epoch 1's
// last block freezes 250 in the cursor entry. Before the first Phase B
// chunk runs, seed another 100 rau in the same pool entry (mimicking a
// block-time voter credit from GrantBlockReward for a block produced by
// candidate 27 during era N+1). Drive Phase B to completion. Assert:
//
//   - candidate 27's pool balance = 100 (residual, not 0 and not -100)
//   - cursor is absent (era-N drain completed)
//   - a fresh Phase A at epoch 2's last block folds the 100 residual
//     into a new era-N+1 cursor entry with VoterAmountFrozen=100
func TestGrantVoterRewardChunk_LateAccrualSurvivesToNextEra(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		p.cfg.CompoundBatchSize = 1

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Seed only candidate 27's pool. Phase A will freeze exactly one
		// cursor entry — makes the "same delegate accrues more mid-drain"
		// invariant unambiguous to assert.
		candID := identityset.Address(27).Bytes()
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(250)))

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		// Era-N Phase A: freeze the cursor at 250 for candidate 27.
		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		frozen, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(frozen)
		r.Equal(uint64(1), frozen.TargetEra)
		var frozenAmt int64
		for _, w := range frozen.Delegates {
			if string(w.CandidateIdentifier) == string(candID) {
				frozenAmt = w.VoterAmountFrozen.Int64()
			}
		}
		r.Equal(int64(250), frozenAmt, "Phase A must freeze 250 for candidate 27")

		// Mid-drain late accrual: another 100 lands in candidate 27's pool
		// while the era-N cursor is still live.
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(100)))

		// Drive Phase B to completion. distributeVoterOnly returns early
		// (no snapshot seeded), so the observable Phase B effect for this
		// delegate is decrementPendingBlockRewardPool(frozen=250) exactly.
		for {
			got, cErr := p.readEpochDrainCursor(ctx, sm)
			r.NoError(cErr)
			if got == nil {
				break
			}
			_, _, cErr = p.GrantVoterRewardChunk(ctx, sm)
			r.NoError(cErr)
		}

		// Assert 27's pool has the 100 late credit surviving, and that the
		// era-N cursor is gone.
		residual, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		r.NoError(err)
		r.Equal(int64(100), residual.Int64(),
			"late accrual must survive era-N drain — era-N chunk drains only the frozen 250")

		// Advance the block context to epoch 2's last block and re-derive
		// feature ctxs, then run era-N+1 Phase A. It must build a fresh
		// cursor entry for candidate 27 with the residual folded in.
		rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
		g := genesis.MustExtractGenesisContext(ctx)
		epochLast := rp.GetEpochLastBlockHeight(2)
		blk := protocol.MustGetBlockCtx(ctx)
		blk.BlockHeight = epochLast
		blk.Producer = identityset.Address(27)
		ctx = protocol.WithBlockCtx(ctx, blk)
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		nextCursor, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(nextCursor, "era-N+1 Phase A must produce a cursor from the surviving pool residual")
		r.Equal(uint64(2), nextCursor.TargetEra)
		var carriedAmt int64
		for _, w := range nextCursor.Delegates {
			if string(w.CandidateIdentifier) == string(candID) {
				carriedAmt = w.VoterAmountFrozen.Int64()
			}
		}
		r.Equal(int64(100), carriedAmt,
			"era-N+1 cursor must freeze the 100 rau that survived era-N drain")
	}, nil, false, 0)
}
