// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// seedBuckets writes buckets straight to state with the fork gate OFF, which
// is exactly the pre-activation situation the backfill exists to repair: the
// buckets are in state and the owner index is not.
func seedBuckets(t *testing.T, cs *ContractStakingStateManager, contract address.Address, owners map[uint64]address.Address) {
	t.Helper()
	for id, owner := range owners {
		require.NoError(t, cs.UpsertBucket(context.Background(), contract, id, testBucket(owner)))
	}
}

// snapshotIndex collects the whole owner index so two runs can be compared.
func snapshotIndex(t *testing.T, sm protocol.StateManager, owners []address.Address) map[string][]ContractBucketRef {
	t.Helper()
	out := make(map[string][]ContractBucketRef, len(owners))
	for _, o := range owners {
		if refs, ok := rawIndex(t, sm, o); ok {
			out[o.String()] = refs
		}
	}
	return out
}

func backfillFixture(t *testing.T) (*ContractStakingStateManager, protocol.StateManager, []BackfillContract, []address.Address) {
	t.Helper()
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	c1, c2 := identityset.Address(20), identityset.Address(21)
	alice, bob, carol := identityset.Address(1), identityset.Address(2), identityset.Address(3)

	// Ids 3 and 6 of c1 are burnt: the backfill must tolerate gaps.
	seedBuckets(t, cs, c1, map[uint64]address.Address{
		0: alice, 1: bob, 2: alice, 4: carol, 5: bob, 7: alice,
	})
	seedBuckets(t, cs, c2, map[uint64]address.Address{
		0: carol, 1: alice, 2: carol,
	})

	contracts := []BackfillContract{
		{Address: c1, NumOfBucket: 8},
		{Address: c2, NumOfBucket: 3},
	}
	return cs, sm, contracts, []address.Address{alice, bob, carol}
}

// TestBackfillOwnerIndexOneShot builds the whole index in a single call.
func TestBackfillOwnerIndexOneShot(t *testing.T) {
	r := require.New(t)
	cs, sm, contracts, owners := backfillFixture(t)

	cursor, err := BackfillOwnerIndex(context.Background(), cs, contracts, OwnerIndexBackfillCursor{}, 1000)
	r.NoError(err)
	r.True(cursor.Done(contracts))

	idx := snapshotIndex(t, sm, owners)
	// alice owns c1{0,2,7} and c2{1}; identityset.Address(21) sorts before
	// Address(20) by raw address bytes, so the c2 ref comes first.
	r.Equal([]uint64{1, 0, 2, 7}, refIDs(idx[owners[0].String()]))
	r.Len(idx[owners[1].String()], 2) // bob: c1{1,5}
	r.Len(idx[owners[2].String()], 3) // carol: c1{4} + c2{0,2}
}

// TestBackfillOwnerIndexResumable is the point of the batching: draining the
// backfill a few buckets at a time across many calls must land on exactly the
// state a single call would have produced. Otherwise an activation path that
// spreads the work over blocks would not be replayable.
func TestBackfillOwnerIndexResumable(t *testing.T) {
	r := require.New(t)

	_, oneShotSM, contracts, owners := backfillFixture(t)
	oneShotCS := NewContractStakingStateManager(oneShotSM)
	_, err := BackfillOwnerIndex(context.Background(), oneShotCS, contracts, OwnerIndexBackfillCursor{}, 1000)
	r.NoError(err)
	want := snapshotIndex(t, oneShotSM, owners)

	for _, limit := range []int{1, 2, 3, 5, 11} {
		batchedCS, batchedSM, batchedContracts, batchedOwners := backfillFixture(t)
		cursor := OwnerIndexBackfillCursor{}
		calls := 0
		for !cursor.Done(batchedContracts) {
			cursor, err = BackfillOwnerIndex(context.Background(), batchedCS, batchedContracts, cursor, limit)
			r.NoError(err)
			calls++
			r.Less(calls, 100, "backfill with limit %d is not making progress", limit)
		}
		// 11 ids total across both contracts, so the call count is bounded by
		// ceil(11/limit) plus the one that observes the end.
		r.LessOrEqual(calls, (11+limit-1)/limit+1)
		r.Equal(want, snapshotIndex(t, batchedSM, batchedOwners),
			"limit %d produced a different index than the one-shot build", limit)
	}
}

// TestBackfillOwnerIndexMidwayInspection checks the cursor really is a resume
// point and not just a loop counter: stopping halfway leaves a partial index
// whose contents are the prefix of the full one.
func TestBackfillOwnerIndexMidwayInspection(t *testing.T) {
	r := require.New(t)
	cs, sm, contracts, owners := backfillFixture(t)

	cursor, err := BackfillOwnerIndex(context.Background(), cs, contracts, OwnerIndexBackfillCursor{}, 3)
	r.NoError(err)
	r.False(cursor.Done(contracts))
	r.Equal(0, cursor.ContractIndex)
	r.Equal(uint64(3), cursor.NextBucketID)

	// Only ids 0..2 of the first contract are in so far.
	idx := snapshotIndex(t, sm, owners)
	r.Equal([]uint64{0, 2}, refIDs(idx[owners[0].String()]))
	r.Equal([]uint64{1}, refIDs(idx[owners[1].String()]))
	r.NotContains(idx, owners[2].String())

	// Resuming from the cursor finishes the job.
	cursor, err = BackfillOwnerIndex(context.Background(), cs, contracts, cursor, 1000)
	r.NoError(err)
	r.True(cursor.Done(contracts))
	r.Len(snapshotIndex(t, sm, owners), 3)
}

// TestBackfillOwnerIndexIdempotent: re-running over already indexed buckets
// must not duplicate refs, so a crash between "wrote the refs" and "persisted
// the cursor" is recoverable by replaying the batch.
func TestBackfillOwnerIndexIdempotent(t *testing.T) {
	r := require.New(t)
	cs, sm, contracts, owners := backfillFixture(t)

	for i := 0; i < 3; i++ {
		_, err := BackfillOwnerIndex(context.Background(), cs, contracts, OwnerIndexBackfillCursor{}, 1000)
		r.NoError(err)
	}
	idx := snapshotIndex(t, sm, owners)
	r.Equal([]uint64{1, 0, 2, 7}, refIDs(idx[owners[0].String()]))
	r.Len(idx[owners[1].String()], 2)
	r.Len(idx[owners[2].String()], 3)
}

// TestBackfillOwnerIndexBoundedWork pins that a call never reads more buckets
// than the batch limit, which is what makes it safe to run inside a block.
func TestBackfillOwnerIndexBoundedWork(t *testing.T) {
	r := require.New(t)
	cs, _, contracts, _ := backfillFixture(t)

	cursor := OwnerIndexBackfillCursor{}
	cursor, err := BackfillOwnerIndex(context.Background(), cs, contracts, cursor, 4)
	r.NoError(err)
	r.Equal(OwnerIndexBackfillCursor{ContractIndex: 0, NextBucketID: 4}, cursor)

	cursor, err = BackfillOwnerIndex(context.Background(), cs, contracts, cursor, 4)
	r.NoError(err)
	// 4 more ids: 4,5,6,7 finishes contract 0 and rolls to contract 1.
	r.Equal(OwnerIndexBackfillCursor{ContractIndex: 1, NextBucketID: 0}, cursor)
}

// TestBackfillOwnerIndexValidation covers the argument guards.
func TestBackfillOwnerIndexValidation(t *testing.T) {
	r := require.New(t)
	cs, _, contracts, _ := backfillFixture(t)

	_, err := BackfillOwnerIndex(context.Background(), cs, contracts, OwnerIndexBackfillCursor{}, 0)
	r.ErrorContains(err, "positive")

	_, err = BackfillOwnerIndex(context.Background(), cs, contracts, OwnerIndexBackfillCursor{ContractIndex: -1}, 10)
	r.ErrorContains(err, "invalid backfill cursor")

	_, err = BackfillOwnerIndex(context.Background(), cs, []BackfillContract{{Address: nil, NumOfBucket: 1}}, OwnerIndexBackfillCursor{}, 10)
	r.ErrorContains(err, "nil contract address")

	// An empty contract list is trivially done.
	cursor, err := BackfillOwnerIndex(context.Background(), cs, nil, OwnerIndexBackfillCursor{}, 10)
	r.NoError(err)
	r.True(cursor.Done(nil))
}
