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
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_poll"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// setupChunkedProtocol mirrors testProtocol's fixture but exposes the
// two knobs the chunked-drain suite needs: EpochDrainChunkSize baked
// into the protocol config, and ToBeEnabledBlockHeight controlling
// whether NoVoterRewardDistribution is off (fork gate open) so the
// chunk quota actually takes effect.
func setupChunkedProtocol(t *testing.T, chunkSize uint64, forkOn bool) (context.Context, protocol.StateManager, *Protocol, *rolldpos.Protocol, uint64) {
	ctrl := gomock.NewController(t)
	registry := protocol.NewRegistry()
	sm := testdb.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	g.InitBalanceMap[identityset.Address(28).String()] = "1000"
	g.Rewarding.InitBalanceStr = "0"
	g.Rewarding.ExemptAddrStrsFromEpochReward = []string{}
	g.Rewarding.BlockRewardStr = "10"
	g.Rewarding.EpochRewardStr = "100"
	g.Rewarding.NumDelegatesForEpochReward = 4
	g.Rewarding.FoundationBonusStr = "5"
	g.Rewarding.NumDelegatesForFoundationBonus = 5
	g.Rewarding.FoundationBonusLastEpoch = 365
	g.Rewarding.ProductivityThreshold = 50
	g.Rewarding.EpochDrainChunkSize = chunkSize
	g.XinguBlockHeight = 1
	if forkOn {
		g.ToBeEnabledBlockHeight = 0
	}

	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates,
		g.NumDelegates,
		g.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(g.DardanellesBlockHeight, g.DardanellesNumSubEpochs),
	)
	p := NewProtocol(g.Rewarding)

	candidates := []*state.Candidate{
		{Address: identityset.Address(27).String(), Votes: unit.ConvertIotxToRau(4000000), RewardAddress: identityset.Address(0).String()},
		{Address: identityset.Address(28).String(), Votes: unit.ConvertIotxToRau(3000000), RewardAddress: identityset.Address(28).String()},
		{Address: identityset.Address(29).String(), Votes: unit.ConvertIotxToRau(2000000), RewardAddress: identityset.Address(29).String()},
		{Address: identityset.Address(30).String(), Votes: unit.ConvertIotxToRau(1000000), RewardAddress: identityset.Address(30).String()},
		{Address: identityset.Address(31).String(), Votes: unit.ConvertIotxToRau(500000), RewardAddress: identityset.Address(31).String()},
		{Address: identityset.Address(32).String(), Votes: unit.ConvertIotxToRau(500000), RewardAddress: identityset.Address(32).String()},
	}
	view := protocol.NewMockView(ctrl)
	view.EXPECT().Snapshot().AnyTimes()
	view.EXPECT().Revert(gomock.Any()).AnyTimes()
	require.NoError(t, sm.WriteView("staking", view))

	pp := mock_poll.NewMockProtocol(ctrl)
	pp.EXPECT().Candidates(gomock.Any(), gomock.Any()).Return(candidates, nil).AnyTimes()
	pp.EXPECT().Delegates(gomock.Any(), gomock.Any()).Return(candidates[:5], nil).AnyTimes()
	pp.EXPECT().Register(gomock.Any()).DoAndReturn(func(reg *protocol.Registry) error {
		return reg.Register("poll", pp)
	}).AnyTimes()
	pp.EXPECT().CalculateUnproductiveDelegates(gomock.Any(), gomock.Any()).Return(
		map[string]uint64{
			identityset.Address(29).String(): 1,
			identityset.Address(31).String(): 6,
		}, nil,
	).AnyTimes()
	require.NoError(t, rp.Register(registry))
	require.NoError(t, pp.Register(registry))
	require.NoError(t, p.Register(registry))

	epochLast := g.NumDelegates * g.NumSubEpochs
	ctx := protocol.WithBlockCtx(
		context.Background(),
		protocol.BlockCtx{BlockHeight: 0},
	)
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithFeatureCtx(ctx)
	ap := account.NewProtocol(DepositGas)
	require.NoError(t, ap.Register(registry))
	require.NoError(t, ap.CreateGenesisStates(ctx, sm))
	require.NoError(t, p.CreateGenesisStates(ctx, sm))

	ctx = protocol.WithBlockCtx(
		ctx, protocol.BlockCtx{
			Producer:    identityset.Address(27),
			BlockHeight: epochLast,
		},
	)
	ctx = protocol.WithActionCtx(
		ctx, protocol.ActionCtx{Caller: identityset.Address(28)},
	)
	ctx = protocol.WithBlockchainCtx(
		protocol.WithRegistry(ctx, registry),
		protocol.BlockchainCtx{Tip: protocol.TipInfo{Height: epochLast - 1}},
	)
	// Fund the reward pool so grants have balance to move.
	_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
	require.NoError(t, err)

	return ctx, sm, p, rp, epochLast
}

// bindEpochGrantMocks patches the staking-slash surface used by
// GrantEpochReward → slashUqd so unit tests without a real staking
// protocol don't crash on the slash callback.
func bindEpochGrantMocks(t *testing.T, ctx context.Context) *gomonkey.Patches {
	sp := &staking.Protocol{}
	require.NoError(t, sp.Register(protocol.MustGetRegistry(ctx)))
	patches := gomonkey.NewPatches()
	patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
	patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)
	return patches
}

// unclaimedBalanceOf reads the unclaimed reward balance for the given
// identity-set index; small helper to keep assertion tables compact.
func unclaimedBalanceOf(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol, idx int) *big.Int {
	amt, _, err := p.UnclaimedBalance(ctx, sm, identityset.Address(idx))
	require.NoError(t, err)
	return amt
}

// TestGrantEpochReward_ChunkedMatchesSingle asserts the chunked drain
// produces the same per-address unclaimed balances as the single-block
// drain when everything else is held constant. This is the correctness
// gate for the mitigation-3 refactor.
func TestGrantEpochReward_ChunkedMatchesSingle(t *testing.T) {
	runOnce := func(chunkSize uint64, forkOn bool) map[int]*big.Int {
		ctx, sm, p, _, _ := setupChunkedProtocol(t, chunkSize, forkOn)
		patches := bindEpochGrantMocks(t, ctx)
		defer patches.Reset()

		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		// Drive GrantEpochReward until the cursor clears. Baseline
		// (chunkSize=0) completes in a single call; chunked mode may
		// require multiple. Cap the loop generously to fail loud on a
		// runaway.
		for i := 0; i < 32; i++ {
			_, _, err := p.GrantEpochReward(ctx, sm)
			require.NoError(t, err)
			cursor, err := p.readEpochDrainCursor(ctx, sm)
			require.NoError(t, err)
			if cursor == nil {
				break
			}
			// Continuation blocks execute on non-boundary heights.
			blkCtx := protocol.MustGetBlockCtx(ctx)
			blkCtx.BlockHeight++
			ctx = protocol.WithBlockCtx(ctx, blkCtx)
			ctx = protocol.WithFeatureCtx(ctx)
			ctx = protocol.WithFeatureWithHeightCtx(ctx)
		}
		result := map[int]*big.Int{}
		for _, idx := range []int{0, 27, 28, 29, 30, 31, 32} {
			result[idx] = unclaimedBalanceOf(t, ctx, sm, p, idx)
		}
		return result
	}

	// Baseline: fork off, no chunking. This is the legacy single-block
	// path preserved by the refactor.
	baseline := runOnce(0, false)
	// Chunked: fork on, chunkSize=2. Should produce identical balances.
	chunked := runOnce(2, true)
	for idx, wantBal := range baseline {
		require.Equalf(t, wantBal, chunked[idx],
			"chunked drain balance mismatch for identity %d: baseline=%s chunked=%s",
			idx, wantBal, chunked[idx])
	}
}

// TestGrantEpochReward_CursorLifecycle verifies the cursor is created on
// the first chunk, advances on interim chunks, and is deleted once the
// drain finishes. Also confirms the epoch-reward sentinel is written
// only on the final call.
func TestGrantEpochReward_CursorLifecycle(t *testing.T) {
	ctx, sm, p, rp, epochLast := setupChunkedProtocol(t, 1, true)
	patches := bindEpochGrantMocks(t, ctx)
	defer patches.Reset()

	epochNum := rp.GetEpochNum(epochLast)
	ctx = protocol.WithFeatureCtx(ctx)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)

	// Before the first call: cursor absent, sentinel absent.
	// assertNoRewardYet returns nil error iff the sentinel is NOT set.
	c0, err := p.readEpochDrainCursor(ctx, sm)
	require.NoError(t, err)
	require.Nil(t, c0)
	require.NoError(t,
		p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum),
		"epoch-reward sentinel unexpectedly present before Phase A",
	)

	// Drive chunks one at a time, capturing cursor state after each.
	var chunkCount int
	for i := 0; i < 32; i++ {
		_, _, err := p.GrantEpochReward(ctx, sm)
		require.NoError(t, err)
		chunkCount++
		cursor, err := p.readEpochDrainCursor(ctx, sm)
		require.NoError(t, err)
		if cursor == nil {
			break
		}
		// Sentinel must NOT exist while drain is in flight — its
		// existence is the "epoch fully rewarded" signal.
		require.NoError(t,
			p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum),
			"epoch-reward sentinel written while cursor is still live",
		)
		require.Equal(t, epochNum, cursor.TargetEpoch)
		require.Greater(t, cursor.DelegateIndex, uint32(0))

		blkCtx := protocol.MustGetBlockCtx(ctx)
		blkCtx.BlockHeight++
		ctx = protocol.WithBlockCtx(ctx, blkCtx)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
	}
	require.Greater(t, chunkCount, 1, "chunkSize=1 should require multiple GrantEpochReward calls")

	// Post-drain: cursor gone, sentinel written (assertNoRewardYet errs).
	cFinal, err := p.readEpochDrainCursor(ctx, sm)
	require.NoError(t, err)
	require.Nil(t, cFinal)
	require.Error(t,
		p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, epochNum),
		"epoch-reward sentinel missing after drain completed",
	)
}

// TestGrantEpochReward_RejectsStaleCursor covers the overrun guard: if
// a cursor from a prior epoch is still on disk when a new epoch's
// GrantEpochReward runs, the call must fail loudly rather than
// interleave two drains.
func TestGrantEpochReward_RejectsStaleCursor(t *testing.T) {
	ctx, sm, p, rp, epochLast := setupChunkedProtocol(t, 1, true)
	patches := bindEpochGrantMocks(t, ctx)
	defer patches.Reset()

	epochNum := rp.GetEpochNum(epochLast)
	require.Greater(t, epochNum, uint64(0))
	// Seed a stale cursor pretending epoch N-1's drain never finished.
	stale := &epochDrainCursor{
		TargetEpoch:   epochNum - 1,
		DelegateIndex: 3,
	}
	require.NoError(t, p.writeEpochDrainCursor(ctx, sm, stale))

	ctx = protocol.WithFeatureCtx(ctx)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	_, _, err := p.GrantEpochReward(ctx, sm)
	require.Error(t, err)
	require.Contains(t, err.Error(), "prior epoch")
}

// TestCreatePostSystemActions_EmitsContinuation covers the scheduler
// hook: when the block is non-epoch-boundary AND a live cursor exists,
// an extra EpochReward grant action must be appended so the drain can
// keep progressing.
func TestCreatePostSystemActions_EmitsContinuation(t *testing.T) {
	ctx, sm, p, _, epochLast := setupChunkedProtocol(t, 1, true)

	// Move to a mid-epoch block (not the last block in an epoch).
	blkCtx := protocol.MustGetBlockCtx(ctx)
	blkCtx.BlockHeight = epochLast + 1
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	ctx = protocol.WithFeatureCtx(ctx)

	// No cursor: continuation should NOT emit an extra grant.
	baseline, err := p.CreatePostSystemActions(ctx, sm)
	require.NoError(t, err)
	require.Len(t, baseline, 1, "expected only BlockReward grant on a mid-epoch block with no cursor")

	// Seed a live cursor for the current epoch.
	live := &epochDrainCursor{
		TargetEpoch:   1,
		DelegateIndex: 2,
	}
	require.NoError(t, p.writeEpochDrainCursor(ctx, sm, live))

	extended, err := p.CreatePostSystemActions(ctx, sm)
	require.NoError(t, err)
	require.Len(t, extended, 2, "expected BlockReward + EpochReward continuation grant")
}
