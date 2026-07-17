// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
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

// seedChunkCursor loads the same epoch-scoped inputs GrantVoterRewardChunk
// will re-derive, builds a cursor whose length matches the epoch's
// rewardedCandidates slice, sets DelegateIndex to startIdx, and persists
// it. Returns the cursor for the test to assert against.
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
	cursor, err := p.buildEpochDrainCursor(ctx, sm, epochNum, rewardedCandidates)
	r.NoError(err)
	cursor.DelegateIndex = startIdx
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
// chunk runs Phase C: sentinel written for cursor.TargetEra and cursor
// deleted. Seeded state: cursor with DelegateIndex advanced so this run
// consumes only the tail slice.
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

		// Sentinel for the ORIGINAL epoch (cursor.TargetEra=1) must be
		// present. A second grant attempt against epoch 1 must now fail
		// with the idempotency guard.
		r.Error(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"terminal chunk must write epoch reward sentinel")
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

		// Sentinel is keyed by cursor.TargetEra (=1), NOT by the current
		// block's epoch. A cross-era bug would either misplace the
		// sentinel or corrupt state; verify epoch 1's sentinel landed.
		err = p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1)
		r.Error(err, "sentinel must land under cursor.TargetEra (epoch 1)")

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
				{CandidateIdentifier: identityset.Address(27).Bytes(), PoolAmountFrozen: big.NewInt(1)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, injected))
		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.Error(err)
		r.Contains(err.Error(), "voter reward chunk action requires IIP-59 fork")
	}, nil, false, 0)
}
