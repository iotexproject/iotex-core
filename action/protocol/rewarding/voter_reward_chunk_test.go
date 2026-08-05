// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// enableIIP59 flips ToBeEnabledBlockHeight so cursorEnabled=true at the
// current block height and re-derives the feature contexts. Callers use
// this to turn on the fork gate for a testProtocol-scaffolded ctx that
// starts with the fork off.
//
// EpochsPerRewardEra is forced to 1 so every epoch counts as an era
// boundary; unit tests here exercise the Phase A cursor lifecycle at
// a single epoch's last block, and the default (24) would gate that
// off. Tests that need multi-era cadence override this after calling.
func enableIIP59(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	// Post-activation steady state includes a finished LSD owner-index
	// backfill; GrantEpochReward declines era boundaries until it is. See
	// assumeOwnerIndexBackfillComplete for why the fixture cannot reach that
	// state on its own. Tests that want the backfill still running use
	// enableIIP59Ctx and skip this.
	assumeOwnerIndexBackfillComplete(t)
	return enableIIP59Ctx(t, ctx)
}

// enableIIP59Ctx is enableIIP59 without the backfill assumption: fork gate and
// genesis only.
func enableIIP59Ctx(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	g := genesis.MustExtractGenesisContext(ctx)
	g.ToBeEnabledBlockHeight = 1
	g.Rewarding.EpochsPerRewardEra = 1
	g.Rewarding.HermesRewardVaultAddresses = make([]string, 0, 35)
	for i := 0; i < 35; i++ {
		g.Rewarding.HermesRewardVaultAddresses = append(
			g.Rewarding.HermesRewardVaultAddresses, identityset.Address(i).String(),
		)
	}
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithFeatureCtx(ctx)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	return ctx
}

// TestGrantVoterRewardChunk_DirectPayoutNeedsNoClaim pins that a voter paid by
// the drain gets money in their account balance immediately, with no
// ClaimFromRewardingFund action of their own. The outflow is booked against the
// fund at payout time, which is why totalBalance drops in the same block.
//
// The fixture plants a real bucket rather than a poll-snapshot entry: the drain
// finds voters by walking the key space, so a snapshot-only fixture would
// present it with zero voters and every assertion below would hold vacuously.
func TestGrantVoterRewardChunk_DirectPayoutNeedsNoClaim(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
	voter := identityset.Address(8)
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, []iip59NativeSeed{
		{delegate: candAddr, voter: voter, amount: 1_000_000_000_000_000_000},
	}, nil)

	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance: big.NewInt(500), unclaimedBalance: big.NewInt(500),
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candAddr.Bytes(), big.NewInt(100)))
	r.NoError(p.updateAvailableBalance(ctx, sm, big.NewInt(100)))
	totalWeight, snapshotHash := distributionMetadata(t, sm, candAddr)
	rewardAddr, err := address.FromString(cand.RewardAddress)
	r.NoError(err)
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra: 1,
		Delegates: []epochDrainDelegateWork{{
			CandidateIdentifier: candAddr.Bytes(),
			VoterAmountFrozen:   big.NewInt(100),
			RewardAddress:       rewardAddr.Bytes(),
			TotalWeight:         f.totalWeightOf(candAddr),
			SnapshotHash:        snapshotHash[:],
			FreezeHeight:        iip59FixtureFreezeHeight,
			SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
		}},
	}))
	// The snapshot's own total must agree with the recomputed one, otherwise
	// this fixture would be silently testing the clamp instead.
	r.Zero(totalWeight.Cmp(f.totalWeightOf(candAddr)))

	txLogs, _, err := p.GrantVoterRewardChunk(ctx, sm)
	r.NoError(err)
	r.Len(txLogs, 1)
	r.Equal(iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND, txLogs[0].Type)
	r.Equal(address.RewardingPoolAddr, txLogs[0].Sender)
	r.Equal(voter.String(), txLogs[0].Recipient)
	r.Zero(txLogs[0].Amount.Cmp(big.NewInt(100)))

	account, err := accountutil.LoadAccount(sm, voter)
	r.NoError(err)
	r.Zero(account.Balance.Cmp(big.NewInt(100)))
	unclaimed, _, err := p.UnclaimedBalance(ctx, sm, voter)
	r.NoError(err)
	r.Zero(unclaimed.Sign())
	total, _, err := p.TotalBalance(ctx, sm)
	r.NoError(err)
	r.Zero(total.Cmp(big.NewInt(400)))
	available, _, err := p.AvailableBalance(ctx, sm)
	r.NoError(err)
	r.Zero(available.Cmp(big.NewInt(400)))
}

// seedChunkCursor loads the same epoch-scoped rewarded-candidate list Phase A
// would derive and builds a cursor with one entry per candidate (frozen voter
// amount = 1 rau). It also opens an era copy-on-write window, because the drain
// refuses to run against a closed one.
//
// No buckets are planted, so the shard walk finds no voters and every entry's
// rau stays undistributed. That is deliberate: these tests are about chunk flow
// control -- cursor lifecycle, the terminal coda, the fork gate -- and a fixture
// that also moved money would make their assertions depend on payout maths they
// are not trying to pin. Distribution correctness lives in the shard-drain and
// clamp tests, which plant real buckets.
//
// shardsDone pre-advances the shard walk so a caller can put the cursor one
// shard away from completion and exercise the terminal coda directly.
func seedChunkCursor(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	epochNum uint64,
	shardsDone uint16,
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
		candID, cErr := candidateIdentifierBytes(cand.Identity)
		r.NoError(cErr)
		entries = append(entries, epochDrainDelegateWork{
			CandidateIdentifier: candID,
			VoterAmountFrozen:   big.NewInt(1),
			FreezeHeight:        iip59FixtureFreezeHeight,
			SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
		})
	}
	openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)
	cursor := &epochDrainCursor{
		TargetEra:  epochNum,
		ShardsDone: shardsDone,
		Delegates:  entries,
	}
	r.NoError(p.writeEpochDrainCursor(ctx, sm, cursor))
	return cursor
}

// TestGrantVoterRewardChunk_UnroutableDelegatesFinish verifies delegates whose
// snapshots are unavailable do not consume voter budget or leave a cursor
// stuck behind a separate delegate-count cap.
func TestGrantVoterRewardChunk_UnroutableDelegatesFinish(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		// Fund the rewarding pool so grantToAccount / updateAvailableBalance
		// have headroom for the chunk's payouts.
		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		cursor := seedChunkCursor(t, ctx, sm, p, 1, 0)
		require.Greater(t, len(cursor.Delegates), 2,
			"test precondition: need >2 delegates to exercise mid-drain chunk")
		deferredID := cursor.Delegates[0].CandidateIdentifier
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, deferredID, big.NewInt(77)))

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		// An empty voter key space costs no budget, so the whole shard
		// rotation is walked in one call and the coda marks the cursor done.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got)
		r.True(got.Completed)
		r.Equal(totalShards, got.ShardsDone)
		deferred, err := p.readPendingBlockRewardPool(ctx, sm, deferredID)
		r.NoError(err)
		r.Zero(deferred.Cmp(big.NewInt(77)), "missing snapshot must remain pending for a later era")

		// assertNoRewardYet returns nil when sentinel does NOT exist.
		r.NoError(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"mid-drain must not write epoch reward sentinel")
	}, nil, false, 0)
}

// TestGrantVoterRewardChunk_LastChunkRunsCoda verifies the terminal
// chunk runs the post-C3 coda: orphan sweep + cursor completion. The
// epoch sentinel is Phase A's responsibility (written by
// GrantEpochReward) and is NOT touched by the chunk anymore. Seeded
// state: cursor with the shard walk one shard from the end, so this run
// consumes only the tail of the rotation.
func TestGrantVoterRewardChunk_LastChunkRunsCoda(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		// One shard left to walk, so this single call runs the terminal coda.
		cursor := seedChunkCursor(t, ctx, sm, p, 1, totalShards-1)
		r.NotEmpty(cursor.Delegates)

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		// Cursor remains queryable after the coda and is marked complete.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got)
		r.True(got.Completed)
		r.Equal(totalShards, got.ShardsDone)
		r.Equal(protocol.MustGetBlockCtx(ctx).BlockHeight, got.CompletedHeight)
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

		// Cursor pins the era that started the drain (era 1); this
		// continuation block is well into a later era.
		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		cursor := seedChunkCursor(t, ctx, sm, p, 1, totalShards-1)
		r.NotEmpty(cursor.Delegates)

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
		r.NotNil(got)
		r.True(got.Completed)
		r.Equal(totalShards, got.ShardsDone)
		r.Equal(later, got.CompletedHeight)
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

func TestCompletedCursorDoesNotDispatchChunk(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
		blk := protocol.MustGetBlockCtx(ctx)
		blk.BlockHeight = rp.GetEpochHeight(1)
		ctx = protocol.WithBlockCtx(ctx, blk)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)

		r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
			TargetEra: 1, StartEpoch: 1, EndEpoch: 1, Completed: true, CompletedHeight: blk.BlockHeight,
		}))
		grants, err := p.CreatePostSystemActions(ctx, sm)
		r.NoError(err)
		r.Len(grants, 1)
		r.Equal(action.BlockReward, grants[0].Action().(*action.GrantReward).RewardType())

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.ErrorContains(err, "completed cursor")
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
			TargetEra: 1,
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
//   - cursor is marked complete (era-N drain completed)
//   - a fresh Phase A at epoch 2's last block folds the 100 residual
//     into a new era-N+1 cursor entry with VoterAmountFrozen=100
func TestGrantVoterRewardChunk_LateAccrualSurvivesToNextEra(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Seed only candidate 27's pool. Phase A will freeze exactly one
		// cursor entry — makes the "same delegate accrues more mid-drain"
		// invariant unambiguous to assert.
		candID := identityset.Address(27).Bytes()
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(250)))
		// FreezeHeight is what makes a work item payable: an item without one
		// has no defensible evaluation height for the weight recompute, so the
		// drain skips it and preserves its pool. These snapshots need it or the
		// residual sweep below never runs.
		for _, idx := range []int{27, 28, 29, 30} {
			r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, identityset.Address(idx), &staking.CandidatePollSnapshot{
				OnchainRewardEnabled:       true,
				BlockCommissionBasisPoints: _basisPointsDenom,
				EpochCommissionBasisPoints: _basisPointsDenom,
				Registered:                 true,
				TotalWeight:                big.NewInt(1),
				FreezeHeight:               iip59FixtureFreezeHeight,
				SelfStakeBucketIdx:         staking.NoSelfStakeBucketIndex,
			}))
		}

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		// Era-N Phase A: freeze the cursor at 250 for candidate 27.
		_, eraNLogs, err := p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		// This is a clean era boundary — no prior cursor was live, so the
		// §10.2 overrun handoff must NOT fire. The observability contract
		// is: EPOCH_DRAIN_OVERRUN appears iff a stale cursor was found and
		// deleted at Phase A entry.
		for _, entry := range eraNLogs {
			rl := &rewardingpb.RewardLog{}
			r.NoError(proto.Unmarshal(entry.Data, rl))
			r.NotEqual(rewardingpb.RewardLog_EPOCH_DRAIN_OVERRUN, rl.Type,
				"no overrun log allowed at a clean Phase A entry")
		}
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

		// Phase A opens the era copy-on-write window inside FreezePollSnapshot,
		// which this fixture bypasses by writing snapshots directly, so open it
		// here -- the drain refuses to read live buckets.
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

		// Drive Phase B to completion. The frozen 250 is paid while the later
		// 100 remains in the pending pool for the next era.
		for {
			got, cErr := p.readEpochDrainCursor(ctx, sm)
			r.NoError(cErr)
			if got == nil || got.Completed {
				break
			}
			_, _, cErr = p.GrantVoterRewardChunk(ctx, sm)
			r.NoError(cErr)
		}

		// Assert 27's pool has the 100 late credit surviving. The completed
		// era-N cursor remains queryable until the next boundary.
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

		_, eraNext1Logs, err := p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		// Era-N drained cleanly to completion before this era-N+1 Phase A,
		// so the overrun handoff still must not fire.
		for _, entry := range eraNext1Logs {
			rl := &rewardingpb.RewardLog{}
			r.NoError(proto.Unmarshal(entry.Data, rl))
			r.NotEqual(rewardingpb.RewardLog_EPOCH_DRAIN_OVERRUN, rl.Type,
				"clean era transition after in-time drain must not emit overrun")
		}

		nextCursor, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(nextCursor, "era-N+1 Phase A must produce a cursor from the surviving pool residual")
		r.Equal(uint64(2), nextCursor.TargetEra)
		r.Equal(uint64(2), nextCursor.StartEpoch, "a completed cursor must not extend the next era range")
		r.Equal(uint64(2), nextCursor.EndEpoch)
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

// TestGrantVoterRewardChunk_EmitsCursorProgress confirms the IIP-59
// §10.3 observability contract: every GrantVoterRewardChunk call emits
// a CURSOR_PROGRESS log carrying a pre-drain snapshot of the live
// cursor. The log must be the FIRST entry in rewardLogs (off-chain
// verifiers pattern-match on the leading log to key their per-block
// cursor-pile-up detector). The addr field encodes the tuple
// "<target_era>:<shards_done>:<resume_voter_hex>:<shards_remaining>"
// and the amount field is the sentinel "0" (this log carries no
// monetary value).
//
// Fixture: build a cursor with synthetic entries, pre-advance the shard walk
// so the snapshot has non-zero shards_done and remaining fields to assert.
func TestGrantVoterRewardChunk_EmitsCursorProgress(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		const done = uint16(1)
		cursor := seedChunkCursor(t, ctx, sm, p, 1, done)
		r.NotEmpty(cursor.Delegates)
		expectedRemaining := uint32(totalShards - done)

		_, rewardLogs, err := p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		r.NotEmpty(rewardLogs, "chunk must emit at least the progress log")

		// The progress log is seeded into rewardLogs before the drain
		// loop runs (see reward.go GrantVoterRewardChunk), so it must be
		// the first entry regardless of what the drain produces after.
		progress := &rewardingpb.RewardLog{}
		r.NoError(proto.Unmarshal(rewardLogs[0].Data, progress))
		r.Equal(rewardingpb.RewardLog_CURSOR_PROGRESS, progress.Type,
			"first log per chunk must be the CURSOR_PROGRESS snapshot")

		// Snapshot encoding matches the PRE-drain cursor: target_era=1,
		// shards_done=1, resume_voter empty (no mid-shard stop injected),
		// remaining = 256-1.
		want := fmt.Sprintf("%d:%d:%x:%d", uint64(1), done, []byte(nil), expectedRemaining)
		r.Equal(want, progress.Addr, "addr encodes pre-drain cursor tuple")
		r.Equal("0", progress.Amount, "CURSOR_PROGRESS amount is the fixed sentinel '0'")
	}, nil, false, 0)
}

// TestVoterRewardChunkFailureIsLoudAndCounted pins the reporting contract for a
// failed drain chunk. Two things must both hold, and they pull in opposite
// directions:
//
//   - the block still commits, with a Failure receipt. Halting block production
//     on a bad chunk would let one delegate's unpayable pool stop the chain.
//   - the failure is not silent. The cursor does not advance, the chain does,
//     and the next era boundary's writeEpochDrainCursor overwrites plan and
//     progress together -- so an unnoticed run of failures discards an era of
//     voter payouts with nothing left in state to show for it.
//
// The counter is the machine-readable half of that; the Error-level log carries
// the era and cursor position for a human. Asserted here as a delta rather than
// an absolute so the test does not depend on what else in the package touched
// the process-global registry first.
func TestVoterRewardChunkFailureIsLoudAndCounted(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		blk := protocol.MustGetBlockCtx(ctx)
		ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
			Caller:   blk.Producer,
			GasPrice: big.NewInt(0),
		})
		// Settling a failed system action rolls the working set back to the
		// snapshot Handle took. Nothing here asserts on that; it only must not
		// abort the mock.
		testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))

		// No cursor is the simplest reachable failure, and the one Handle
		// cannot tell apart from any other: every drain failure arrives here
		// as a non-nil error out of GrantVoterRewardChunk.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got, "test precondition: no cursor present")

		before := counterValue(t, _iip59DrainChunkFailureMtc)
		receipt, err := p.Handle(ctx, createGrantRewardAction(action.VoterRewardChunk, blk.BlockHeight), sm)
		r.NoError(err, "a failed chunk must settle, not abort the block")
		r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
		r.Equal(before+1, counterValue(t, _iip59DrainChunkFailureMtc))

		// The sibling grant types in the same switch keep their quiet Debug
		// handling; only VoterRewardChunk was upgraded. A failed epoch grant
		// must not move the drain counter.
		before = counterValue(t, _iip59DrainChunkFailureMtc)
		_, err = p.Handle(ctx, createGrantRewardAction(action.EpochReward, blk.BlockHeight), sm)
		r.NoError(err)
		r.Equal(before, counterValue(t, _iip59DrainChunkFailureMtc))
	}, nil, false, 0)
}

// counterValue reads a Counter without pulling in
// prometheus/client_golang/prometheus/testutil, which would add a go-cmp
// dependency this module does not otherwise carry.
func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	m := &dto.Metric{}
	require.NoError(t, c.Write(m))
	return m.GetCounter().GetValue()
}
