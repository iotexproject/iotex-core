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

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestEpochDrainChunkSize confirms the fork gate short-circuits the
// chunk size to 0. Pre-fork, no matter how large CompoundBatchSize is
// set, the epoch drain must run as a single-block loop and never touch
// the cursor.
func TestEpochDrainChunkSize(t *testing.T) {
	r := require.New(t)

	forkOff, _, pForkOff, _, _ := newVoterRewardCtx(t, false)
	pForkOff.cfg.CompoundBatchSize = 500
	r.Equal(uint32(0), pForkOff.epochDrainChunkSize(forkOff),
		"pre-fork: chunkSize must be 0 regardless of CompoundBatchSize")

	forkOn, _, pForkOn, _, _ := newVoterRewardCtx(t, true)
	pForkOn.cfg.CompoundBatchSize = 500
	r.Equal(uint32(500), pForkOn.epochDrainChunkSize(forkOn),
		"post-fork with batch=500: chunkSize must be 500")

	pForkOn.cfg.CompoundBatchSize = 0
	r.Equal(uint32(0), pForkOn.epochDrainChunkSize(forkOn),
		"post-fork with batch=0: chunkSize must be 0 (legacy single-block)")
}

// TestGrantEpochReward_SkipsCursorWhenNoVoterShare confirms the C3
// invariant that cursor entries only materialize for delegates whose
// per-delegate epoch split (or block-time pool accrual) yielded a voter
// portion. With no frozen snapshots seeded, every delegate falls to the
// fallback branch of splitDelegateEpochReward and the whole grant runs
// as a pre-fork-style single-block coda — sentinel written, cursor
// absent — even though the fork gate is on.
func TestGrantEpochReward_SkipsCursorWhenNoVoterShare(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(got, "no delegate has voter share → cursor must not be persisted")

		r.Error(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"sentinel must be written by GrantEpochReward even when the cursor is empty")
	}, nil, false, 0)
}

// TestGrantEpochReward_RejectsAnyLiveCursor confirms Phase A's tightened
// corruption guard. After C2.1 split the continuation path into
// GrantVoterRewardChunk, GrantEpochReward runs Phase A only — and Phase A
// can never coexist with a live cursor. Any cursor at Phase A entry is
// unambiguous corrupt state (mid-drain continuation was supposed to run
// GrantVoterRewardChunk, not GrantEpochReward), so the guard must fire
// regardless of which era the cursor pins.
func TestGrantEpochReward_RejectsAnyLiveCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		// Flip the fork gate on so cursorEnabled=true and the corruption
		// check runs. Height is already the last block of epoch 1, which
		// is past ToBeEnabledBlockHeight=1.
		g := genesis.MustExtractGenesisContext(ctx)
		g.ToBeEnabledBlockHeight = 1
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)

		// Inject a cursor pinned to the CURRENT epoch — this used to be
		// silently accepted (mid-drain continuation), but under the C2.1
		// split it's now unambiguous corruption: continuation blocks must
		// run GrantVoterRewardChunk, not GrantEpochReward.
		live := &epochDrainCursor{
			TargetEra:     1,
			DelegateIndex: 0,
			Delegates: []epochDrainDelegateWork{
				{CandidateIdentifier: identityset.Address(27).Bytes(), VoterAmountFrozen: big.NewInt(1)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, live))

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.Error(err)
		r.Contains(err.Error(), "cursor unexpectedly live at Phase A entry")
	}, nil, false, 0)
}

// TestGrantEpochReward_FeatureOffIgnoresCursor confirms the fork-off
// invariant: with NoVoterRewardDistribution=true, GrantEpochReward
// runs the legacy single-block loop and NEVER reads/writes/deletes
// the cursor. A cursor persisted before the fork opened must survive
// a legacy epoch grant untouched.
func TestGrantEpochReward_FeatureOffIgnoresCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		// testProtocol leaves ToBeEnabledBlockHeight at the default
		// (math.MaxUint64), so fork is off. Sanity-check.
		r.True(protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution,
			"testProtocol default must have fork off")

		// Seed a plausible-looking cursor for the CURRENT epoch so that,
		// if the fork gate were mistakenly bypassed, GrantEpochReward
		// would take the continuation branch and skip Phase A. A survivor
		// cursor after grant proves cursorEnabled=false held.
		injected := &epochDrainCursor{
			TargetEra:     1,
			DelegateIndex: 0,
			Delegates: []epochDrainDelegateWork{
				{CandidateIdentifier: identityset.Address(27).Bytes(), VoterAmountFrozen: big.NewInt(42)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, injected))

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := gomonkey.NewPatches()
		defer patches.Reset()

		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		// The cursor must still be present, unmodified. If the fork-off
		// path had read it, we'd have taken the continuation branch and
		// either skipped Phase A's assertNoRewardYet (invalid) or written
		// the sentinel + deleted the cursor (equally invalid).
		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got, "cursor must survive a legacy epoch grant")
		r.Equal(injected.TargetEra, got.TargetEra)
		r.Equal(injected.DelegateIndex, got.DelegateIndex)
		r.Len(got.Delegates, 1)
		r.Equal(injected.Delegates[0].CandidateIdentifier, got.Delegates[0].CandidateIdentifier)
		r.Equal(int64(42), got.Delegates[0].VoterAmountFrozen.Int64())
	}, nil, false, 0)
}

// TestGrantEpochReward_PoolAccrualBuildsCursor confirms that block-time
// voter accruals — pool balance credited by GrantBlockReward for
// opted-in delegates — get folded into the epoch-boundary cursor even
// when the per-delegate epoch split has no fresh voter share (fallback
// branch of splitDelegateEpochReward). This is what preserves late-
// arriving voter accruals across the era boundary: the cursor freezes
// pool + epochShare, and Phase B's decrement removes exactly that
// frozen amount from the pool.
func TestGrantEpochReward_PoolAccrualBuildsCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Seed a pool balance for candidate 27 (the first candidate the
		// default poll list uses). No frozen snapshot exists, so the
		// epoch-side split returns (amount, 0) — the cursor entry comes
		// purely from the pool accrual.
		candID := identityset.Address(27).Bytes()
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(1_234)))

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		got, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(got, "pool accrual must build a cursor entry")
		r.Equal(uint64(1), got.TargetEra)
		r.Equal(uint32(0), got.DelegateIndex)

		var found bool
		for _, work := range got.Delegates {
			if string(work.CandidateIdentifier) == string(candID) {
				found = true
				r.Equal(int64(1_234), work.VoterAmountFrozen.Int64(),
					"cursor must freeze the pool accrual as-is when epoch split yields no voter share")
			}
		}
		r.True(found, "candidate 27 must appear in the cursor")

		// Sentinel is written by GrantEpochReward regardless of cursor state.
		r.Error(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"sentinel must be written in the same call that builds the cursor")
	}, nil, false, 0)
}
