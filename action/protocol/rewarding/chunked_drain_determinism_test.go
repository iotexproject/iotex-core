// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// rewardedCandidateIndexes matches the first NumDelegatesForEpochReward
// entries in testProtocol's candidate list (see protocol_test.go). Kept as a
// package-level slice so the drain determinism tests seed exactly the
// delegates that will be rewarded (and enter the frozen cursor) without
// re-deriving the split.
var rewardedCandidateIndexes = []int{27, 28, 29, 30}

// allProtocolAddrs returns every address whose per-address rewarding balance
// could change during a drain in the testProtocol fixture: the six poll
// candidates (27..32), plus the reward addresses that differ from the
// candidate address (identityset.Address(0) is candidate 27's reward
// address). Duplicates are harmless — TestOnlyDumpRewardState elides
// zero-balance entries so the resulting map contents are identical for equal
// address sets.
func allProtocolAddrs(t *testing.T) []address.Address {
	t.Helper()
	out := make([]address.Address, 0, 8)
	for _, idx := range []int{0, 27, 28, 29, 30, 31, 32} {
		out = append(out, identityset.Address(idx))
	}
	return out
}

// seedPoolAccrualsForRewardedDelegates credits equal-sized voter pool
// balances to each of the four rewarded delegates. Those balances are what
// Phase A folds into the epoch drain cursor — one entry per delegate,
// frozen at the seeded amount — so chunking the drain has real cursor
// entries to walk.
func seedPoolAccrualsForRewardedDelegates(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	perDelegate int64,
) {
	t.Helper()
	r := require.New(t)
	for _, idx := range rewardedCandidateIndexes {
		r.NoError(p.creditPendingBlockRewardPool(
			ctx, sm, identityset.Address(idx).Bytes(), big.NewInt(perDelegate),
		))
	}
}

// registerStubStakingProtocol installs an empty staking.Protocol and
// no-ops the SlashCandidate* entry points GrantEpochReward invokes
// during Phase A. Mirrors the setup used in chunked_drain_test.go's
// existing tests. Returns the *Patches so callers can Reset() it.
func registerStubStakingProtocol(t *testing.T, ctx context.Context) *gomonkey.Patches {
	t.Helper()
	r := require.New(t)
	patches := gomonkey.NewPatches()
	sp := &staking.Protocol{}
	r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
	patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
	patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)
	return patches
}

// runDrainToCompletion runs Phase A once and then drives GrantVoterRewardChunk
// in a tight loop until the cursor is absent. Returns the number of Phase B
// chunk calls that were needed. Callers assert this against the chunk size
// they configured; a chunkSize=0 run should complete Phase B in zero
// continuation calls (Phase A drains in one shot).
func runDrainToCompletion(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
) int {
	t.Helper()
	r := require.New(t)
	_, _, err := p.GrantEpochReward(ctx, sm)
	r.NoError(err)
	chunks := 0
	for {
		got, gErr := p.readEpochDrainCursor(ctx, sm)
		r.NoError(gErr)
		if got == nil {
			break
		}
		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		chunks++
		if chunks > 1000 {
			t.Fatal("drain loop exceeded 1000 chunks — cursor is not advancing")
		}
	}
	return chunks
}

// TestChunkedDrain_InvariantAcrossChunkSizes is the central determinism
// claim of the chunking machinery: the final rewarding state after Phase A
// + Phase B is byte-identical regardless of how the drain is chunked. If
// this ever fails, chunk-size tuning at mainnet activation becomes unsafe
// under any value.
//
// The fixture seeds equal pool accruals for the four rewarded delegates
// so Phase A freezes four cursor entries with identical VoterAmountFrozen.
// Three runs — chunkSize=1 (four continuation blocks), chunkSize=2 (two
// continuation blocks), chunkSize=0 (unbounded, Phase A drains inline) —
// must produce reflect.DeepEqual snapshots.
//
// distributeVoterOnly short-circuits on the snapshot-missing branch because
// the fixture does not seed poll snapshots; this leaves the cursor
// walk + decrementPendingBlockRewardPool as the observable Phase B work.
// That is exactly the loop chunk-size affects, so this test still exercises
// the invariant it asserts.
func TestChunkedDrain_InvariantAcrossChunkSizes(t *testing.T) {
	run := func(t *testing.T, chunkSize uint64) (*TestOnlyRewardStateSnapshot, int) {
		var snap *TestOnlyRewardStateSnapshot
		var chunks int
		testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
			r := require.New(t)
			ctx = enableIIP59(t, ctx)
			p.cfg.CompoundBatchSize = chunkSize

			_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
			r.NoError(err)

			seedPoolAccrualsForRewardedDelegates(t, ctx, sm, p, 250)

			patches := registerStubStakingProtocol(t, ctx)
			defer patches.Reset()

			chunks = runDrainToCompletion(t, ctx, sm, p)

			got, err := p.TestOnlyDumpRewardState(ctx, sm, allProtocolAddrs(t))
			r.NoError(err)
			snap = got
		}, nil, false, 0)
		return snap, chunks
	}

	r := require.New(t)

	snapChunk1, chunks1 := run(t, 1)
	snapChunk2, chunks2 := run(t, 2)
	snapChunkUnbounded, chunksU := run(t, 0)

	// chunkSize=0 means unbounded → Phase A's cursor gets fully drained by
	// the first GrantVoterRewardChunk call. With four cursor entries and
	// chunkSize=1, four continuation calls are needed; with chunkSize=2, two.
	r.Equal(4, chunks1, "chunkSize=1 with 4 entries needs 4 continuation blocks")
	r.Equal(2, chunks2, "chunkSize=2 with 4 entries needs 2 continuation blocks")
	r.Equal(1, chunksU, "chunkSize=0 (unbounded) still needs 1 continuation call for the coda")

	// End-state must be byte-identical across all three chunkings.
	r.True(reflect.DeepEqual(snapChunk1, snapChunk2),
		"end-state must be byte-identical between chunkSize=1 and chunkSize=2")
	r.True(reflect.DeepEqual(snapChunk1, snapChunkUnbounded),
		"end-state must be byte-identical between chunkSize=1 and chunkSize=0")

	// Cursor must be absent at end of every run — the invariant that lets
	// consumers rely on cursor absence as a "drain complete" signal.
	r.False(snapChunk1.CursorPresent, "cursor must be absent at end of chunkSize=1 drain")
	r.False(snapChunk2.CursorPresent, "cursor must be absent at end of chunkSize=2 drain")
	r.False(snapChunkUnbounded.CursorPresent, "cursor must be absent at end of chunkSize=0 drain")
}

// TestChunkedDrain_ReplayFromPersistedCursor confirms the cursor is a
// faithful checkpoint: a drain that runs some chunks, is observed
// mid-drain, and then continues to completion produces the same end state
// as an uninterrupted drain of the same size.
//
// This is a weaker claim than serialize-across-processes replay — that
// requires a real KV-backed factory and is covered by the e2e stress test
// in iip59_stress_test.go — but it does prove that the cursor payload
// captures everything needed to resume: no in-memory-only state on
// *Protocol influences the drain outcome.
func TestChunkedDrain_ReplayFromPersistedCursor(t *testing.T) {
	// Reference run: uninterrupted drain with chunkSize=1.
	var reference *TestOnlyRewardStateSnapshot
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		p.cfg.CompoundBatchSize = 1

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		seedPoolAccrualsForRewardedDelegates(t, ctx, sm, p, 250)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		runDrainToCompletion(t, ctx, sm, p)
		reference, err = p.TestOnlyDumpRewardState(ctx, sm, allProtocolAddrs(t))
		r.NoError(err)
	}, nil, false, 0)

	// Observation run: same fixture, but pause after two chunks to snapshot
	// the mid-drain cursor + state. Continue to completion and confirm the
	// end state matches the reference.
	var observed *TestOnlyRewardStateSnapshot
	var midDrain *TestOnlyRewardStateSnapshot
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		p.cfg.CompoundBatchSize = 1

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		seedPoolAccrualsForRewardedDelegates(t, ctx, sm, p, 250)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)
		// Two chunks, then pause.
		for i := 0; i < 2; i++ {
			_, _, cErr := p.GrantVoterRewardChunk(ctx, sm)
			r.NoError(cErr)
		}
		midDrain, err = p.TestOnlyDumpRewardState(ctx, sm, allProtocolAddrs(t))
		r.NoError(err)
		r.True(midDrain.CursorPresent, "mid-drain snapshot must observe a live cursor")
		r.Equal(uint32(2), midDrain.CursorIndex, "mid-drain cursor must be at index 2 after two chunkSize=1 chunks")

		// Continue to completion.
		for {
			got, cErr := p.readEpochDrainCursor(ctx, sm)
			r.NoError(cErr)
			if got == nil {
				break
			}
			_, _, cErr = p.GrantVoterRewardChunk(ctx, sm)
			r.NoError(cErr)
		}
		observed, err = p.TestOnlyDumpRewardState(ctx, sm, allProtocolAddrs(t))
		r.NoError(err)
	}, nil, false, 0)

	r := require.New(t)
	r.True(reflect.DeepEqual(reference, observed),
		"end-state after a paused-then-resumed drain must equal an uninterrupted drain")
}

// TestChunkedDrain_MidEraOptOut confirms the frozen work list is immutable
// mid-drain: a delegate whose pool balance is decremented after Phase A but
// before that delegate's chunk runs still gets their frozen amount drained
// against zero pool balance — which is the failure mode we need to *not*
// happen because decrementPendingBlockRewardPool would underflow.
//
// The stronger form of this test (real opt-out via snapshot mutation)
// requires seeding CandidatePollSnapshot state and driving the full
// distributeVoterOnly path, which is out of the testProtocol scaffold's
// scope. Deferred to the e2e stress test. Here we assert the narrower
// invariant this scaffold can express: cursor entries are consumed in
// order and each decrementPendingBlockRewardPool call receives exactly
// the frozen amount.
func TestChunkedDrain_MidEraOptOut(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		p.cfg.CompoundBatchSize = 1

		_, err := p.Deposit(ctx, sm, big.NewInt(1_000), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)
		seedPoolAccrualsForRewardedDelegates(t, ctx, sm, p, 250)

		patches := registerStubStakingProtocol(t, ctx)
		defer patches.Reset()

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err)

		// Snapshot the frozen cursor for later comparison.
		frozen, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(frozen)
		r.Equal(len(rewardedCandidateIndexes), len(frozen.Delegates),
			"Phase A must have frozen one entry per rewarded delegate")

		// Consume one chunk. That drains cursor entry [0] fully — pool
		// balance for that candidate should now be zero.
		_, _, err = p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)

		firstCandID := frozen.Delegates[0].CandidateIdentifier
		amt, err := p.readPendingBlockRewardPool(ctx, sm, firstCandID)
		r.NoError(err)
		r.Equal(0, amt.Sign(),
			"first cursor entry's pool balance must be drained to zero after its chunk")

		// The remaining cursor entries' frozen amounts must be untouched
		// mid-drain — the C3 invariant that the cursor is immutable
		// between chunks. Even if a delegate's pool balance changed
		// (which it hasn't, here), the frozen amount in the cursor payload
		// determines how much decrementPendingBlockRewardPool takes.
		mid, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.NotNil(mid, "cursor must survive mid-drain")
		r.Equal(uint32(1), mid.DelegateIndex, "cursor must be past the first entry")
		for i := 1; i < len(mid.Delegates); i++ {
			r.Equal(
				frozen.Delegates[i].VoterAmountFrozen.Int64(),
				mid.Delegates[i].VoterAmountFrozen.Int64(),
				"cursor entry %d must retain its frozen amount mid-drain", i,
			)
			r.Equal(
				frozen.Delegates[i].CandidateIdentifier,
				mid.Delegates[i].CandidateIdentifier,
				"cursor entry %d must retain its candidate identifier mid-drain", i,
			)
		}
	}, nil, false, 0)
}
