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
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// assumeOwnerIndexBackfillComplete makes staking.OwnerIndexBackfillComplete
// report "done" for the duration of the test.
//
// On a real node the LSD owner-index backfill job record is seeded and advanced
// by the staking protocol's CreatePreStates, which runs at the top of every
// block from the activation height onwards. By the time GrantEpochReward runs
// -- it is a system action, well after pre-states -- the record always exists,
// and a few hundred blocks after activation it is permanently done. That is the
// steady state for essentially the whole life of the chain.
//
// These unit fixtures drive GrantEpochReward directly against a mock state
// manager and never run staking's pre-states, so the record is absent and the
// predicate correctly reads "incomplete". Patching it puts the fixture back on
// the steady-state path it is meant to be testing. The un-patched behaviour is
// covered deliberately by TestEraBoundaryDeclinedWhileOwnerIndexBackfillRuns.
func assumeOwnerIndexBackfillComplete(t *testing.T) {
	t.Helper()
	patches := gomonkey.ApplyFunc(
		staking.OwnerIndexBackfillComplete,
		func(protocol.StateReader) (bool, error) { return true, nil },
	)
	t.Cleanup(patches.Reset)
}

// TestEraBoundaryDeclinedWhileOwnerIndexBackfillRuns pins the backfill gate.
//
// The LSD owner index is built one bounded batch per block starting at
// activation, so for the first few hundred blocks the liquid-staking half of
// the voter set is knowingly incomplete. An era boundary taken against it would
// freeze a voter set that is missing every not-yet-backfilled voter: the era is
// sealed, the drain never visits them, and their share silently becomes
// residual. The loss is permanent, so the boundary has to be declined and the
// pool rolled forward instead.
//
// Three things have to hold at once, and the third is the one that is easy to
// miss: the freeze that opens the era copy-on-write window keys on epoch
// arithmetic alone and has already run ~1.5 epochs earlier. Declining the
// boundary does not un-open that window, so this block still has to seal it --
// otherwise the copy-on-write hooks stay armed for a full era, taxing every
// bucket write for a snapshot no drain will ever read.
func TestEraBoundaryDeclinedWhileOwnerIndexBackfillRuns(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59Ctx(t, ctx)
		// Deliberately NOT assumeOwnerIndexBackfillComplete: no backfill job
		// record exists, which is exactly what an in-progress backfill looks
		// like to this predicate.
		complete, err := staking.OwnerIndexBackfillComplete(sm)
		r.NoError(err)
		r.False(complete, "the fixture must present an incomplete backfill")

		_, err = p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Same shape as TestGrantEpochReward_PoolAccrualBuildsCursor, which
		// asserts that this exact state DOES build a cursor once the backfill
		// is done. The only difference between the two is the gate.
		candID := identityset.Address(27).Bytes()
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, candID, big.NewInt(1_234)))
		r.NoError(staking.TestOnlyPutPollSnapshotFor(sm, identityset.Address(27), &staking.CandidatePollSnapshot{
			OnchainRewardEnabled:       true,
			BlockCommissionBasisPoints: _basisPointsDenom,
			EpochCommissionBasisPoints: _basisPointsDenom,
			Registered:                 true,
			TotalWeight:                big.NewInt(1),
		}))

		// The freeze for this era already ran and left a window open.
		openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err, "a declined boundary must not halt the block")

		// 1. No cursor: nothing was frozen against the incomplete voter set.
		cursor, err := p.readEpochDrainCursor(ctx, sm)
		r.NoError(err)
		r.Nil(cursor, "an era boundary must not freeze a knowingly incomplete voter set")

		// 2. The money is still there. This is what makes declining safe rather
		// than merely delayed-lossy: the pool is only drained on a boundary that
		// fires, so the accrual survives into the next era untouched.
		pool, err := p.readPendingBlockRewardPool(ctx, sm, candID)
		r.NoError(err)
		r.Equal(int64(1_234), pool.Int64(),
			"a declined boundary must roll the pending pool forward, not drop it")

		// 3. The window opened by this era's freeze is sealed anyway.
		window, err := staking.EraCOWWindow(sm)
		r.NoError(err)
		r.False(window.Open(),
			"a declined boundary still has to seal the window its freeze opened, "+
				"or the copy-on-write hooks stay armed for a whole era")
	}, nil, false, 0)
}

// TestZeroWorkSealSkippedWhileADrainIsLive is the guard on the seal added for a
// declined boundary.
//
// On a boundary that fires, resolveStaleDrainCursor has already handed off or
// deleted any overrun cursor before persistDrainCursor runs, so there is never
// a live drain to protect. A declined boundary skips that handoff. Sealing
// underneath an in-flight drain would leave it reading buckets that are no
// longer copy-on-write maintained, i.e. silently rerouted onto live state
// mid-era.
func TestZeroWorkSealSkippedWhileADrainIsLive(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, candAddr := newVoterRewardCtx(t, true)
	window := openEraWindowForTest(t, ctx, sm, iip59FixtureFreezeHeight)
	r.True(window.Open())

	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		TargetEra:      1,
		StartEpoch:     1,
		EndEpoch:       1,
		SettlementSeed: make([]byte, 32),
		Delegates: []epochDrainDelegateWork{{
			CandidateIdentifier: candAddr.Bytes(),
			VoterAmountFrozen:   big.NewInt(10),
		}},
		Distributed: []*big.Int{big.NewInt(0)},
	}))

	r.NoError(p.persistDrainCursor(ctx, sm, 2, 2, hash.Hash256{}, nil, true))

	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	r.True(window.Open(),
		"a zero-work boundary must not seal a window an incomplete drain is still reading through")
}
