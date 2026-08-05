// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"sort"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// These tests cover the IIP-59 owner-index activation backfill: the record's
// encoding, the fork gate, the seed, the per-block bound, and the completion
// predicate the rewarding protocol has to consult before freezing an era.

func backfillTestContracts(t *testing.T) (v1, v2, v3 address.Address) {
	t.Helper()
	g := genesis.TestDefault()
	var err error
	v1, err = address.FromString(g.SystemStakingContractAddress)
	require.NoError(t, err)
	v2, err = address.FromString(g.SystemStakingContractV2Address)
	require.NoError(t, err)
	v3, err = address.FromString(g.SystemStakingContractV3Address)
	require.NoError(t, err)
	return
}

// seedPreForkBuckets writes buckets with the gate shut, which is the situation
// the backfill exists to repair: buckets in state, no owner index.
func seedPreForkBuckets(t *testing.T, sm protocol.StateManager, contract address.Address, owners map[uint64]address.Address) {
	t.Helper()
	cs := contractstaking.NewContractStakingStateManager(sm)
	ids := make([]uint64, 0, len(owners))
	for id := range owners {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		bkt := &contractstaking.Bucket{
			Candidate:      identityset.Address(30),
			Owner:          owners[id],
			StakedAmount:   big.NewInt(100),
			StakedDuration: 86400,
			CreatedAt:      1,
		}
		// context.Background() carries no feature context, so OwnerIndexEnabled
		// is false and nothing but the bucket itself is written.
		require.NoError(t, cs.UpsertBucket(context.Background(), contract, id, bkt))
	}
}

func backfillRefIDs(t *testing.T, sm protocol.StateManager, owner address.Address) []uint64 {
	t.Helper()
	refs, _, err := contractstaking.NewStateReader(sm).BucketRefsByOwner(owner)
	require.NoError(t, err)
	out := make([]uint64, 0, len(refs))
	for _, r := range refs {
		out = append(out, r.BucketID)
	}
	return out
}

func TestOwnerIndexBackfillJobCodec(t *testing.T) {
	r := require.New(t)
	v1, v2, _ := backfillTestContracts(t)

	for _, tc := range []*ownerIndexBackfillJob{
		{},
		{Contracts: []contractstaking.BackfillContract{{Address: v1, MaxBucketID: 0}}},
		{
			Contracts: []contractstaking.BackfillContract{
				{Address: v1, MaxBucketID: 12345},
				{Address: v2, MaxBucketID: 1<<63 + 7},
			},
			Cursor: contractstaking.OwnerIndexBackfillCursor{ContractIndex: 1, NextBucketID: 99},
		},
	} {
		buf, err := tc.Serialize()
		r.NoError(err)
		var got ownerIndexBackfillJob
		r.NoError(got.Deserialize(buf))
		r.Equal(tc.Cursor, got.Cursor)
		r.Len(got.Contracts, len(tc.Contracts))
		for i := range tc.Contracts {
			r.Equal(tc.Contracts[i].Address.String(), got.Contracts[i].Address.String())
			r.Equal(tc.Contracts[i].MaxBucketID, got.Contracts[i].MaxBucketID)
		}
	}

	// A truncated or over-long body must not be read as a shorter list.
	full := &ownerIndexBackfillJob{Contracts: []contractstaking.BackfillContract{{Address: v1, MaxBucketID: 1}}}
	buf, err := full.Serialize()
	r.NoError(err)
	var bad ownerIndexBackfillJob
	r.Error(bad.Deserialize(buf[:len(buf)-1]))
	r.Error(bad.Deserialize(append(buf, 0)))
	r.Error(bad.Deserialize(buf[:3]))
	// Unknown version is rejected rather than misparsed.
	wrongVersion := append([]byte{}, buf...)
	wrongVersion[0] = 0xff
	r.Error(bad.Deserialize(wrongVersion))
}

func TestRunOwnerIndexBackfillGate(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, _, _ := backfillTestContracts(t)
	alice := identityset.Address(1)
	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{1: alice, 2: alice})

	// Pre-activation the backfill must not read or write anything at all: the
	// record it would create is consensus state.
	n, err := runOwnerIndexBackfill(forkGateCtx(100, false), sm)
	r.NoError(err)
	r.Zero(n)
	job, err := readOwnerIndexBackfillJob(sm)
	r.NoError(err)
	r.Nil(job)

	// And the completion predicate reads as "not complete", which is what stops
	// the rewarding protocol freezing an era against an index that is not there.
	done, err := OwnerIndexBackfillComplete(sm)
	r.NoError(err)
	r.False(done)
}

func TestRunOwnerIndexBackfillSeedsEveryContract(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, v2, v3 := backfillTestContracts(t)
	alice, bob := identityset.Address(1), identityset.Address(2)

	// V1 has a meta record because the V1 indexer wrote one; V2 and V3 never
	// had one, which is the defect. Note the V1 mark is deliberately the top id
	// (5), not a count of 6 -- see contractstaking.BackfillContract.
	seedPreForkBuckets(t, sm, v1, map[uint64]address.Address{1: alice, 3: bob, 5: alice})
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(v1, 5))
	seedPreForkBuckets(t, sm, v2, map[uint64]address.Address{1: bob, 2: bob})
	seedPreForkBuckets(t, sm, v3, map[uint64]address.Address{7: alice})

	ctx := forkGateCtx(100, true)
	n, err := runOwnerIndexBackfill(ctx, sm)
	r.NoError(err)
	r.Positive(n)

	job, err := readOwnerIndexBackfillJob(sm)
	r.NoError(err)
	r.NotNil(job)
	r.Len(job.Contracts, 3)
	// Sorted by raw address bytes so the cursor means the same thing on every
	// node.
	for i := 1; i < len(job.Contracts); i++ {
		r.Less(
			string(job.Contracts[i-1].Address.Bytes()),
			string(job.Contracts[i].Address.Bytes()),
		)
	}
	bounds := map[string]uint64{}
	for _, c := range job.Contracts {
		bounds[c.Address.String()] = c.MaxBucketID
	}
	r.EqualValues(5, bounds[v1.String()])
	r.EqualValues(2, bounds[v2.String()])
	r.EqualValues(7, bounds[v3.String()])

	// Defect C: seeding must also leave every contract with a frozen bound, or
	// the era window rejects all of their buckets.
	marks, err := contractstaking.BucketHighWaterMarks(sm)
	r.NoError(err)
	got := map[string]uint64{}
	for _, m := range marks {
		addr, err := address.FromBytes(m.Contract)
		r.NoError(err)
		got[addr.String()] = m.NumOfBuckets
	}
	r.EqualValues(5, got[v1.String()])
	r.EqualValues(2, got[v2.String()])
	r.EqualValues(7, got[v3.String()])

	// One batch is enough here; run until the cursor stops moving.
	for i := 0; ; i++ {
		done, err := OwnerIndexBackfillComplete(sm)
		r.NoError(err)
		if done {
			break
		}
		_, err = runOwnerIndexBackfill(ctx, sm)
		r.NoError(err)
		r.Less(i, 50, "backfill is not making progress")
	}

	// Every pre-fork bucket is now reachable from its owner, including the top
	// id of each contract -- the ids an exclusive bound would have dropped
	// (v1{5}, v2{2}, v3{7}). The list is keyed by (contract, id), so the ids
	// alone are not in numeric order.
	r.ElementsMatch([]uint64{1, 5, 7}, backfillRefIDs(t, sm, alice))
	r.ElementsMatch([]uint64{3, 1, 2}, backfillRefIDs(t, sm, bob))
}

func TestRunOwnerIndexBackfillIsBoundedAndResumable(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	v1, _, _ := backfillTestContracts(t)
	alice := identityset.Address(1)

	// More ids than one batch can hold, so completion has to take several
	// blocks. The bound is what makes this safe to run inside a block.
	owners := map[uint64]address.Address{}
	for id := uint64(1); id <= 600; id++ {
		owners[id] = alice
	}
	seedPreForkBuckets(t, sm, v1, owners)

	ctx := forkGateCtx(100, true)
	blocks := 0
	for {
		done, err := OwnerIndexBackfillComplete(sm)
		r.NoError(err)
		if done {
			break
		}
		n, err := runOwnerIndexBackfill(ctx, sm)
		r.NoError(err)
		r.LessOrEqual(n, _ownerIndexBackfillPerBlock)
		blocks++
		r.Less(blocks, 20, "backfill is not making progress")
	}
	// 601 ids (0..600) at 256 per block.
	r.GreaterOrEqual(blocks, 3)
	r.Len(backfillRefIDs(t, sm, alice), 600)

	// Finished means finished: further blocks neither work nor rewrite.
	n, err := runOwnerIndexBackfill(ctx, sm)
	r.NoError(err)
	r.Zero(n)
}

func TestRunOwnerIndexBackfillNoBuckets(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)

	// A chain where no contract has ever minted: the job seeds empty and is
	// complete immediately, so era boundaries are never blocked.
	_, err := runOwnerIndexBackfill(forkGateCtx(100, true), sm)
	r.NoError(err)
	job, err := readOwnerIndexBackfillJob(sm)
	r.NoError(err)
	r.NotNil(job)
	r.Empty(job.Contracts)
	done, err := OwnerIndexBackfillComplete(sm)
	r.NoError(err)
	r.True(done)

	// No high-water marks are invented for contracts that have no buckets: a
	// mark of 0 would claim bucket 0 existed at the freeze height.
	marks, err := contractstaking.BucketHighWaterMarks(sm)
	r.NoError(err)
	r.Empty(marks)
}

// TestFrozenContractBucketUnknownContract pins the Defect C behaviour at the
// read side: a contract with no frozen mark is denied, not allowed. Allowing
// would admit buckets minted after the freeze into a frozen era, which is worse
// than an under-payment; the deny is made noisy in the log instead.
func TestFrozenContractBucketUnknownContract(t *testing.T) {
	r := require.New(t)
	sm := eraTestSM(t)
	known, unknown := identityset.Address(20), identityset.Address(21)

	ctx := forkGateCtx(eraTestFreezeHeight, true)
	r.NoError(contractstaking.NewContractStakingStateManager(sm).UpdateNumOfBuckets(known, 4))
	r.NoError(TestOnlyBeginEraCOWWindow(ctx, sm, eraTestFreezeHeight))
	window, err := EraCOWWindow(sm)
	r.NoError(err)

	r.True(window.ContractKnown(known.Bytes()))
	r.False(window.ContractKnown(unknown.Bytes()))
	r.True(window.ContractBucketExisted(known.Bytes(), 4))
	r.False(window.ContractBucketExisted(known.Bytes(), 5))
	r.False(window.ContractBucketExisted(unknown.Bytes(), 1))

	_, err = FrozenContractBucket(sm, window, unknown, 1)
	r.ErrorIs(err, ErrBucketPostFreeze)
}
