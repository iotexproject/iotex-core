// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// The IIP-59 era window bounds a contract's buckets with the contract's bucket
// high-water mark, and rejects every bucket of a contract that has no mark.
// Before this, exactly one code path maintained the mark -- the V1 indexer's
// own Commit -- so V2 and V3 buckets were dropped from every frozen weight. The
// mark is now raised by UpsertBucket, the single writer of contract bucket
// state that all three indexers funnel through.

func TestRaiseNumOfBucketsIsRaiseOnly(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	contract := identityset.Address(20)

	// No record yet.
	_, err := cs.NumOfBuckets(contract)
	r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)

	r.NoError(cs.RaiseNumOfBuckets(contract, 5))
	mark, err := cs.NumOfBuckets(contract)
	r.NoError(err)
	r.EqualValues(5, mark)

	// Lower ids must not move it: the mark's meaning is "nothing above this
	// existed", so lowering it would let a post-freeze bucket into a frozen era.
	r.NoError(cs.RaiseNumOfBuckets(contract, 3))
	r.NoError(cs.RaiseNumOfBuckets(contract, 0))
	mark, err = cs.NumOfBuckets(contract)
	r.NoError(err)
	r.EqualValues(5, mark)

	r.NoError(cs.RaiseNumOfBuckets(contract, 9))
	mark, err = cs.NumOfBuckets(contract)
	r.NoError(err)
	r.EqualValues(9, mark)

	// Id 0 on a contract with no record at all still creates the record, so
	// "mark absent" never means "mark is zero".
	other := identityset.Address(21)
	r.NoError(cs.RaiseNumOfBuckets(other, 0))
	mark, err = cs.NumOfBuckets(other)
	r.NoError(err)
	r.EqualValues(0, mark)
}

func TestUpsertBucketMaintainsHighWaterMark(t *testing.T) {
	r := require.New(t)
	contract := identityset.Address(20)
	alice := identityset.Address(1)

	t.Run("post-activation the mark tracks the max id", func(t *testing.T) {
		sm := newTestSM(t)
		cs := NewContractStakingStateManager(sm)
		ctx := forkCtx(true)

		// Out of order on purpose: the mark is a max, not a last-write.
		for _, id := range []uint64{1, 7, 3} {
			r.NoError(cs.UpsertBucket(ctx, contract, id, testBucket(alice)))
		}
		mark, err := cs.NumOfBuckets(contract)
		r.NoError(err)
		r.EqualValues(7, mark)

		// And the era window can now bound the contract, which is the whole
		// point: BucketHighWaterMarks is what beginEraCOWWindow freezes.
		marks, err := BucketHighWaterMarks(sm)
		r.NoError(err)
		r.Len(marks, 1)
		r.Equal(contract.Bytes(), marks[0].Contract)
		r.EqualValues(7, marks[0].NumOfBuckets)
	})

	t.Run("pre-activation nothing is written", func(t *testing.T) {
		// The mark is consensus state. Writing it a block early is a hard fork.
		sm := newTestSM(t)
		cs := NewContractStakingStateManager(sm)
		ctx := forkCtx(false)

		r.NoError(cs.UpsertBucket(ctx, contract, 4, testBucket(alice)))
		_, err := cs.NumOfBuckets(contract)
		r.ErrorIs(errors.Cause(err), state.ErrStateNotExist)

		marks, err := BucketHighWaterMarks(sm)
		r.NoError(err)
		r.Empty(marks)
	})

	t.Run("every contract gets one, not just V1", func(t *testing.T) {
		// The defect: only blockindex/contractstaking (V1) ever called
		// UpdateNumOfBuckets. The V2/V3 indexers and nftEventHandler never did,
		// so their contracts had no frozen bound at all.
		sm := newTestSM(t)
		cs := NewContractStakingStateManager(sm)
		ctx := forkCtx(true)
		v1, v2, v3 := identityset.Address(20), identityset.Address(21), identityset.Address(22)

		r.NoError(cs.UpsertBucket(ctx, v1, 11, testBucket(alice)))
		r.NoError(cs.UpsertBucket(ctx, v2, 22, testBucket(alice)))
		r.NoError(cs.UpsertBucket(ctx, v3, 33, testBucket(alice)))

		marks, err := BucketHighWaterMarks(sm)
		r.NoError(err)
		r.Len(marks, 3)
		got := make(map[string]uint64, 3)
		for _, m := range marks {
			got[string(m.Contract)] = m.NumOfBuckets
		}
		r.EqualValues(11, got[string(v1.Bytes())])
		r.EqualValues(22, got[string(v2.Bytes())])
		r.EqualValues(33, got[string(v3.Bytes())])
	})
}

func TestMaxBucketIDInState(t *testing.T) {
	r := require.New(t)
	sm := newTestSM(t)
	cs := NewContractStakingStateManager(sm)
	contract, empty := identityset.Address(20), identityset.Address(21)
	alice := identityset.Address(1)

	_, found, err := cs.MaxBucketIDInState(empty)
	r.NoError(err)
	r.False(found, "a contract with no buckets must be distinguishable from one whose max id is 0")

	// Ids chosen so that little-endian key order is not id order: 1 sorts after
	// 256 by raw bytes, so "take the last key" would answer 256.
	seedBuckets(t, cs, contract, map[uint64]address.Address{1: alice, 256: alice, 300: alice})
	maxID, found, err := cs.MaxBucketIDInState(contract)
	r.NoError(err)
	r.True(found)
	r.EqualValues(300, maxID)

	// Id 0 alone reads as found, not as absent.
	only0 := identityset.Address(22)
	seedBuckets(t, cs, only0, map[uint64]address.Address{0: alice})
	maxID, found, err = cs.MaxBucketIDInState(only0)
	r.NoError(err)
	r.True(found)
	r.EqualValues(0, maxID)
}
