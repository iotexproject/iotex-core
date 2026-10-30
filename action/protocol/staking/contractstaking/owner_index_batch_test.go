// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"sort"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// AddOwnerRefs is the batch form of addOwnerRef, added so the IIP-59 activation
// backfill can write an owner's whole list in one read-modify-write instead of
// one per bucket. These tests hold it to the property that makes the swap safe:
// it must be indistinguishable from calling addOwnerRef once per ref.

// seedBuckets writes buckets straight to state with the fork gate OFF, which
// is exactly the pre-activation situation the backfill exists to repair: the
// buckets are in state and the owner index is not.
func seedBuckets(t *testing.T, cs *ContractStakingStateManager, contract address.Address, owners map[uint64]address.Address) {
	t.Helper()
	ids := make([]uint64, 0, len(owners))
	for id := range owners {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		require.NoError(t, cs.UpsertBucket(context.Background(), contract, id, testBucket(owners[id])))
	}
}

func TestAddOwnerRefsMatchesAddOwnerRefOneByOne(t *testing.T) {
	ctx := forkCtx(true)
	c1, c2 := identityset.Address(20), identityset.Address(21)
	alice := identityset.Address(1)

	// Deliberately out of sorted order and with a duplicate, because the
	// backfill's scan order is state order, not ref order.
	refs := []ContractBucketRef{
		{Contract: c2, BucketID: 5},
		{Contract: c1, BucketID: 9},
		{Contract: c1, BucketID: 2},
		{Contract: c2, BucketID: 1},
		{Contract: c1, BucketID: 9},
	}

	batched := func() ContractBucketRefs {
		sm := newTestSM(t)
		require.NoError(t, NewContractStakingStateManager(sm).AddOwnerRefs(ctx, alice, refs))
		got, ok := rawIndex(t, sm, alice)
		require.True(t, ok)
		return got
	}()

	oneByOne := func() ContractBucketRefs {
		sm := newTestSM(t)
		cs := NewContractStakingStateManager(sm)
		for _, ref := range refs {
			require.NoError(t, cs.addOwnerRef(ctx, alice, ref))
		}
		got, ok := rawIndex(t, sm, alice)
		require.True(t, ok)
		return got
	}()

	require.Equal(t, oneByOne, batched)
	// Spelled out as well, so this does not pass by both sides being wrong.
	// c2 leads because compareRef orders by raw contract bytes first and
	// identityset.Address(21) sorts below Address(20) -- the list is ordered by
	// address, not by the order the contracts were scanned in.
	require.Less(t, string(c2.Bytes()), string(c1.Bytes()))
	require.Equal(t, ContractBucketRefs{
		{Contract: c2, BucketID: 1},
		{Contract: c2, BucketID: 5},
		{Contract: c1, BucketID: 2},
		{Contract: c1, BucketID: 9},
	}, batched)
}

func TestAddOwnerRefsMergesIntoAnExistingList(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(true)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	c1 := identityset.Address(20)
	alice := identityset.Address(1)

	r.NoError(cs.addOwnerRef(ctx, alice, ContractBucketRef{Contract: c1, BucketID: 4}))
	r.NoError(cs.AddOwnerRefs(ctx, alice, []ContractBucketRef{
		{Contract: c1, BucketID: 1},
		{Contract: c1, BucketID: 4}, // already there
		{Contract: c1, BucketID: 7},
	}))

	refs, ok := rawIndex(t, sm, alice)
	r.True(ok)
	r.Equal([]uint64{1, 4, 7}, refIDs(refs))
}

func TestAddOwnerRefsIsIdempotentAndWritesNothingTwice(t *testing.T) {
	r := require.New(t)
	ctx := forkCtx(true)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	c1 := identityset.Address(20)
	alice := identityset.Address(1)
	refs := []ContractBucketRef{{Contract: c1, BucketID: 1}, {Contract: c1, BucketID: 2}}

	r.NoError(cs.AddOwnerRefs(ctx, alice, refs))
	first, ok := rawIndex(t, sm, alice)
	r.True(ok)

	// A second pass must be a no-op all the way down: same list, and -- because
	// every ref is already present -- no era copy either, which is the property
	// addOwnerRef guarantees by checking membership before snapshotting.
	r.NoError(cs.AddOwnerRefs(ctx, alice, refs))
	second, ok := rawIndex(t, sm, alice)
	r.True(ok)
	r.Equal(first, second)
}

func TestAddOwnerRefsEmptyDoesNotCreateAKey(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	alice := identityset.Address(1)

	r.NoError(NewContractStakingStateManager(sm).AddOwnerRefs(forkCtx(true), alice, nil))
	_, ok := rawIndex(t, sm, alice)
	r.False(ok, "an owner with no buckets must have no key, not an empty list")
}
