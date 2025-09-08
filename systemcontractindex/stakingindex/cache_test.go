package stakingindex

import (
	"context"
	"math/big"
	"reflect"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/stretchr/testify/require"
)

func TestBase(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cache := newCache()
	t.Run("empty", func(t *testing.T) {
		require.Equal(0, len(cache.BucketIdxs()))
		require.False(cache.IsDirty())
	})
	t.Run("clone", func(t *testing.T) {
		clone := cache.Clone()
		if clone == cache {
			t.Error("Expected clone to be a different instance")
		}
		if !reflect.DeepEqual(clone, cache) {
			t.Error("Expected clone to be equal to original")
		}
	})
	t.Run("put bucket", func(t *testing.T) {
		require.Nil(cache.Bucket(1))
		bkt1 := &Bucket{
			Candidate:      identityset.Address(1),
			Owner:          identityset.Address(2),
			StakedAmount:   big.NewInt(1000),
			StakedDuration: 3600,
			CreatedAt:      123456,
		}
		bkt2 := &Bucket{
			Candidate:      identityset.Address(3),
			Owner:          identityset.Address(4),
			StakedAmount:   big.NewInt(2000),
			StakedDuration: 7200,
			CreatedAt:      654321,
		}
		cache.PutBucket(1, bkt1)
		cache.PutBucket(2, bkt2)
		require.Equal(2, len(cache.BucketIdxs()))
		require.Equal(bkt1, cache.Bucket(1))
		require.Equal(bkt2, cache.Bucket(2))
		bkts := cache.Buckets([]uint64{1, 2, 3})
		require.Equal(2, len(bkts))
		require.Equal(bkt1, bkts[0])
		require.Equal(bkt2, bkts[1])
		require.Equal([]uint64{1}, cache.BucketIdsByCandidate(identityset.Address(1)))
		require.Equal([]uint64{2}, cache.BucketIdsByCandidate(identityset.Address(3)))
		t.Run("delete bucket", func(t *testing.T) {
			cache.DeleteBucket(1)
			require.Nil(cache.Bucket(1))
			require.Equal(1, len(cache.BucketIdxs()))
			t.Run("commit", func(t *testing.T) {
				sm := mock_chainmanager.NewMockStateManager(ctrl)
				sm.EXPECT().DelState(gomock.Any(), gomock.Any()).Return(uint64(0), nil).Times(1)
				sm.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).Return(uint64(0), nil).Times(1)
				_, err := cache.Commit(context.Background(), identityset.Address(10), false, sm)
				require.NoError(err)
			})
		})
	})
}
