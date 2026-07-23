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
	g := genesis.MustExtractGenesisContext(ctx)
	g.ToBeEnabledBlockHeight = 1
	g.Rewarding.EpochsPerRewardEra = 1
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithFeatureCtx(ctx)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	return ctx
}

func TestGrantVoterRewardChunk_DirectPayoutNeedsNoClaim(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
	voter := identityset.Address(8)

	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance: big.NewInt(500), unclaimedBalance: big.NewInt(500),
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candAddr.Bytes(), big.NewInt(100)))
	r.NoError(p.updateAvailableBalance(ctx, sm, big.NewInt(100)))
	r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, candAddr, &staking.CandidatePollSnapshot{
		Entries: []staking.VoterWeight{{Voter: voter, Weight: big.NewInt(1)}},
	}))
	totalWeight, snapshotHash, lastWeightedIndex, hasWeightedEntries := distributionMetadata(t, sm, candAddr)
	rewardAddr, err := address.FromString(cand.RewardAddress)
	r.NoError(err)
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra: 1,
		Delegates: []epochDrainDelegateWork{{
			CandidateIdentifier: candAddr.Bytes(),
			VoterAmountFrozen:   big.NewInt(100),
			RewardAddress:       rewardAddr.Bytes(),
			TotalWeight:         totalWeight,
			SnapshotHash:        snapshotHash[:],
			LastWeightedIndex:   lastWeightedIndex,
			HasWeightedEntries:  hasWeightedEntries,
		}},
	}))

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

// seedChunkCursor loads the same epoch-scoped rewarded-candidate list
// GrantVoterRewardChunk will re-derive and builds a cursor with one
// entry per candidate (frozen voter amount = 1 rau to exercise the
// drain loop without seeding real snapshots). Sets DelegateIndex to
// startIdx and persists it. Returns the cursor for the test to assert
// against.
//
// Post-C3, real cursor entries are compacted to delegates with a non-zero
// pool, and their frozen amount is the sum of pool accrual + epoch
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
		candID, cErr := candidateIdentifierBytes(cand.Identity)
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

		cursor := seedChunkCursor(t, ctx, sm, p, 1, 0)
		require.Greater(t, len(cursor.Delegates), 2,
			"test precondition: need >2 delegates to exercise mid-drain chunk")
		total := uint32(len(cursor.Delegates))
		deferredID := cursor.Delegates[0].CandidateIdentifier
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, deferredID, big.NewInt(77)))

		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		// Missing snapshots route no voters, so all entries are resolved in
		// one call and the cursor is deleted by the coda.
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got)
		r.Greater(int(total), 0)
		deferred, err := p.readPendingBlockRewardPool(ctx, sm, deferredID)
		r.NoError(err)
		r.Zero(deferred.Cmp(big.NewInt(77)), "missing snapshot must remain pending for a later era")

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

		// Seed with DelegateIndex pointing at the last two entries so this
		// call runs the terminal coda.
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
		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Seed only candidate 27's pool. Phase A will freeze exactly one
		// cursor entry — makes the "same delegate accrues more mid-drain"
		// invariant unambiguous to assert.
		candID := identityset.Address(27).Bytes()
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(250)))
		r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, identityset.Address(27), &staking.CandidatePollSnapshot{
			BlockCommissionBasisPoints: _basisPointsDenom,
			EpochCommissionBasisPoints: _basisPointsDenom,
			Registered:                 true,
			Entries: []staking.VoterWeight{{
				Voter: identityset.Address(27), Weight: big.NewInt(1),
			}},
		}))
		for _, idx := range []int{28, 29, 30} {
			r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, identityset.Address(idx), &staking.CandidatePollSnapshot{
				BlockCommissionBasisPoints: _basisPointsDenom,
				EpochCommissionBasisPoints: _basisPointsDenom,
				Registered:                 true,
				Entries: []staking.VoterWeight{{
					Voter: identityset.Address(idx), Weight: big.NewInt(1),
				}},
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

		// Drive Phase B to completion. The frozen 250 is paid while the later
		// 100 remains in the pending pool for the next era.
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
// "<target_era>:<delegate_index>:<voter_index>:<delegates_remaining>"
// and the amount field is the sentinel "0" (this log carries no
// monetary value).
//
// Fixture: build a cursor with synthetic entries, advance the cursor's
// DelegateIndex to 1 so we can assert the exact snapshot fields the
// log encodes, and run one chunk.
func TestGrantVoterRewardChunk_EmitsCursorProgress(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Seed a cursor already advanced to DelegateIndex=1 so the log
		// encoding has non-zero delegate_index and remaining fields to
		// assert against.
		cursor := seedChunkCursor(t, ctx, sm, p, 1, 1)
		total := uint32(len(cursor.Delegates))
		r.GreaterOrEqual(int(total), 2, "test precondition: need at least 2 entries so remaining>0")
		expectedRemaining := total - cursor.DelegateIndex

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
		// delegate_index=1 (unchanged by seedChunkCursor's post-write
		// mutation because we set startIdx=1), voter_index=0 (no
		// mid-delegate stop injected), delegates_remaining = total-1.
		want := fmt.Sprintf("%d:%d:%d:%d", uint64(1), uint32(1), uint32(0), expectedRemaining)
		r.Equal(want, progress.Addr, "addr encodes pre-drain cursor tuple")
		r.Equal("0", progress.Amount, "CURSOR_PROGRESS amount is the fixed sentinel '0'")
	}, nil, false, 0)
}
