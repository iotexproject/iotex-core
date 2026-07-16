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
	"github.com/iotexproject/iotex-core/v2/state"
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

// TestBuildEpochDrainCursor_FreezesPoolBalances verifies Phase A's
// core invariant: the pool balance captured at cursor build time
// stays fixed even if the live pool entry is subsequently mutated.
// Chunk-B consumers read PoolAmountFrozen from the cursor, so
// continued GrantBlockReward credits must not inflate the drain
// payout for a delegate that has already been counted.
func TestBuildEpochDrainCursor_FreezesPoolBalances(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	candA := &state.Candidate{Address: identityset.Address(1).String(), RewardAddress: identityset.Address(2).String(), Votes: big.NewInt(1)}
	candB := &state.Candidate{Address: identityset.Address(3).String(), RewardAddress: identityset.Address(4).String(), Votes: big.NewInt(1)}
	candC := &state.Candidate{Address: identityset.Address(5).String(), RewardAddress: identityset.Address(6).String(), Votes: big.NewInt(1)}

	candBytesA, err := candidateIdentifierBytes(candA.Address)
	r.NoError(err)
	candBytesB, err := candidateIdentifierBytes(candB.Address)
	r.NoError(err)

	// Seed known pool balances for A and B; leave C empty.
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candBytesA, big.NewInt(1_000)))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candBytesB, big.NewInt(2_500)))

	cursor, err := p.buildEpochDrainCursor(ctx, sm, 42, []*state.Candidate{candA, candB, candC})
	r.NoError(err)
	r.NotNil(cursor)
	r.Equal(uint64(42), cursor.TargetEra)
	r.Equal(uint32(0), cursor.DelegateIndex)
	r.Len(cursor.Delegates, 3)
	r.Equal(int64(1_000), cursor.Delegates[0].PoolAmountFrozen.Int64())
	r.Equal(int64(2_500), cursor.Delegates[1].PoolAmountFrozen.Int64())
	r.Equal(int64(0), cursor.Delegates[2].PoolAmountFrozen.Int64(),
		"empty pool entry must freeze as zero (not nil)")

	// Mutate the live pool AFTER the freeze — cursor values must not budge.
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candBytesA, big.NewInt(9_999)))
	r.Equal(int64(1_000), cursor.Delegates[0].PoolAmountFrozen.Int64(),
		"cursor value must decouple from post-freeze pool credits")
}

// TestGrantEpochReward_RejectsStaleCursor confirms the corruption guard.
// A cursor pinned to a FUTURE epoch is corrupt state — Phase A can only
// pin to the current epoch. GrantEpochReward must reject this loud
// instead of silently proceeding. (A cursor pinned to a PRIOR epoch,
// by contrast, is a legitimate multi-block drain still in progress.)
func TestGrantEpochReward_RejectsStaleCursor(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		// Flip the fork gate on so cursorEnabled=true and the stale-cursor
		// check runs. Height is already the last block of epoch 1, which is
		// past ToBeEnabledBlockHeight=1.
		g := genesis.MustExtractGenesisContext(ctx)
		g.ToBeEnabledBlockHeight = 1
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)

		// Inject a cursor pointing at era 9999 — nowhere near the current
		// epoch — so TargetEra != epochNum forces the guard to fire.
		stale := &epochDrainCursor{
			TargetEra:     9_999,
			DelegateIndex: 0,
			Delegates: []epochDrainDelegateWork{
				{CandidateIdentifier: identityset.Address(27).Bytes(), PoolAmountFrozen: big.NewInt(1)},
			},
		}
		r.NoError(p.writeEpochDrainCursor(ctx, sm, stale))

		// Provide enough deposit that any accidental partial run would be
		// visible; the assert is on the specific error string, though.
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
		r.Contains(err.Error(), "future epoch")
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
				{CandidateIdentifier: identityset.Address(27).Bytes(), PoolAmountFrozen: big.NewInt(42)},
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
		r.Equal(int64(42), got.Delegates[0].PoolAmountFrozen.Int64())
	}, nil, false, 0)
}
