// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/stretchr/testify/require"
)

// TestNewWrappedCache tests the creation and functionality of the wrapped cache
func TestNewWrappedCache(t *testing.T) {
	require := require.New(t)
	t.Run("nil base cache", func(t *testing.T) {
		require.Panics(func() {
			newWrappedCache(nil)
		}, "base staking cache cannot be nil")
	})
	t.Run("non-nil base cache", func(t *testing.T) {
		base := newContractStakingCache()
		base.PutBucketType(1, &BucketType{
			Amount:   big.NewInt(1000),
			Duration: 100,
		})
		base.PutBucketType(2, &BucketType{
			Amount:   big.NewInt(2000),
			Duration: 200,
		})
		base.PutBucketInfo(1, &bucketInfo{
			TypeIndex:  1,
			Delegate:   identityset.Address(0),
			Owner:      identityset.Address(2),
			CreatedAt:  1,
			UnlockedAt: maxBlockNumber,
			UnstakedAt: maxBlockNumber,
		})
		base.PutBucketInfo(2, &bucketInfo{
			TypeIndex:  2,
			Delegate:   identityset.Address(0),
			Owner:      identityset.Address(3),
			CreatedAt:  1,
			UnlockedAt: maxBlockNumber,
			UnstakedAt: maxBlockNumber,
		})
		base.PutBucketInfo(3, &bucketInfo{
			TypeIndex:  1,
			Delegate:   identityset.Address(1),
			Owner:      identityset.Address(2),
			CreatedAt:  1,
			UnlockedAt: maxBlockNumber,
			UnstakedAt: maxBlockNumber,
		})
		base.PutBucketInfo(4, &bucketInfo{
			TypeIndex:  2,
			Delegate:   identityset.Address(1),
			Owner:      identityset.Address(3),
			CreatedAt:  1,
			UnlockedAt: maxBlockNumber,
			UnstakedAt: maxBlockNumber,
		})
		require.Equal(uint64(4), base.TotalBucketCount())
		require.Equal(2, base.BucketTypeCount())
		ids, types, infos := base.Buckets()
		require.Equal([]uint64{1, 2, 3, 4}, ids)
		require.Equal(4, len(types))
		require.Equal(4, len(infos))
		ids, types, infos = base.BucketsByCandidate(identityset.Address(0))
		require.Equal([]uint64{1, 2}, ids)
		require.Equal(2, len(types))
		require.Equal(2, len(infos))
		ids, types, infos = base.BucketsByCandidate(identityset.Address(1))
		require.Equal([]uint64{3, 4}, ids)
		require.Equal(2, len(types))
		require.Equal(2, len(infos))
		ids, types, infos = base.BucketsByCandidate(identityset.Address(2))
		require.Equal(0, len(ids))
		require.Equal(0, len(types))
		require.Equal(0, len(infos))
		wrapped := newWrappedCache(base)
		t.Run("wrapped cache properties", func(t *testing.T) {
			require.NotNil(wrapped)
			require.Equal(uint64(4), wrapped.TotalBucketCount())
			require.Equal(2, wrapped.BucketTypeCount())
		})
		t.Run("put an existing bucket type", func(t *testing.T) {
			existingType := &BucketType{
				Amount:   big.NewInt(2000),
				Duration: 100,
			}
			require.Panics(func() {
				wrapped.PutBucketType(1, existingType)
			}, "putting an existing bucket type should panic")
			existingType.Amount = big.NewInt(1000)
			require.Panics(func() {
				wrapped.PutBucketType(2, existingType)
			}, "putting an existing bucket type should panic")
			require.Equal(2, wrapped.BucketTypeCount())
		})
		t.Run("put new bucket type", func(t *testing.T) {
			newType := &BucketType{
				Amount:   big.NewInt(3000),
				Duration: 300,
			}
			wrapped.PutBucketType(3, newType)
			require.Equal(3, wrapped.BucketTypeCount())
			require.Equal(newType, wrapped.MustGetBucketType(3))
			require.Equal(2, base.BucketTypeCount())
			require.Panics(func() {
				base.MustGetBucketType(3)
			}, "must get bucket type from wrapped cache")
		})
		t.Run("put new bucket info", func(t *testing.T) {
			newInfo := &bucketInfo{
				TypeIndex:  3,
				Delegate:   identityset.Address(1),
				Owner:      identityset.Address(5),
				CreatedAt:  1,
				UnlockedAt: maxBlockNumber,
				UnstakedAt: maxBlockNumber,
			}
			wrapped.PutBucketInfo(5, newInfo)
			require.Equal(newInfo, wrapped.MustGetBucketInfo(5))
			require.Equal(uint64(5), wrapped.TotalBucketCount())
			require.Equal(uint64(4), base.TotalBucketCount())
			ids, types, infos = wrapped.BucketsByCandidate(identityset.Address(1))
			require.Equal([]uint64{3, 4, 5}, ids)
			require.Equal(3, len(types))
			require.Equal(3, len(infos))
			require.Panics(func() {
				base.MustGetBucketInfo(5)
			}, "must get bucket info from wrapped cache")
		})
		t.Run("update existing bucket info", func(t *testing.T) {
			ids, types, infos = wrapped.BucketsByCandidate(identityset.Address(6))
			require.Equal([]uint64{}, ids)
			require.Equal(0, len(types))
			require.Equal(0, len(infos))
			existingInfo := wrapped.MustGetBucketInfo(1)
			existingInfo.Delegate = identityset.Address(6)
			wrapped.PutBucketInfo(1, existingInfo)
			updatedInfo := wrapped.MustGetBucketInfo(1)
			require.Equal(identityset.Address(6), updatedInfo.Delegate)
			require.Equal(existingInfo, updatedInfo)
			ids, types, infos = wrapped.BucketsByCandidate(identityset.Address(0))
			require.Equal([]uint64{2}, ids)
			require.Equal(1, len(types))
			require.Equal(1, len(infos))
			ids, types, infos = wrapped.BucketsByCandidate(identityset.Address(6))
			require.Equal([]uint64{1}, ids)
			require.Equal(1, len(types))
			require.Equal(1, len(infos))
		})
		t.Run("delete bucket info", func(t *testing.T) {
			ids, types, infos = wrapped.BucketsByCandidate(identityset.Address(0))
			require.Equal([]uint64{2}, ids)
			require.Equal(1, len(types))
			require.Equal(1, len(infos))
			wrapped.DeleteBucketInfo(2)
			_, ok := wrapped.BucketInfo(2)
			require.False(ok, "bucket info should be deleted")
			require.Equal(uint64(5), wrapped.TotalBucketCount())
			require.Equal(uint64(4), base.TotalBucketCount())
			ids, types, infos = wrapped.BucketsByCandidate(identityset.Address(0))
			require.Equal([]uint64{}, ids)
			require.Equal(0, len(types))
			require.Equal(0, len(infos))
			_, ok = base.BucketInfo(2)
			require.True(ok, "base cache should still have the bucket info")
		})
	})
}
