package contractstaking

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestContractStakingDelta_BucketInfoDelta(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// add bucket info
	bi := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	require.NoError(cache.AddBucketInfo(1, bi))

	// modify bucket info
	bi = &bucketInfo{TypeIndex: 2, CreatedAt: 2, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)}
	require.NoError(cache.UpdateBucketInfo(2, bi))

	// remove bucket info
	require.NoError(cache.DeleteBucketInfo(3))

	// get bucket info delta
	delta := cache.BucketInfoDelta()

	// check added bucket info
	require.Len(delta[deltaStateAdded], 1)
	added, ok := delta[deltaStateAdded][1]
	require.True(ok)
	require.NotNil(added)
	require.EqualValues(1, added.TypeIndex)
	require.EqualValues(1, added.CreatedAt)
	require.EqualValues(maxBlockNumber, added.UnlockedAt)
	require.EqualValues(maxBlockNumber, added.UnstakedAt)
	require.EqualValues(identityset.Address(1), added.Delegate)
	require.EqualValues(identityset.Address(2), added.Owner)

	// check modified bucket info
	require.Len(delta[deltaStateModified], 1)
	modified, ok := delta[deltaStateModified][2]
	require.True(ok)
	require.NotNil(modified)
	require.EqualValues(2, modified.TypeIndex)
	require.EqualValues(2, modified.CreatedAt)
	require.EqualValues(maxBlockNumber, modified.UnlockedAt)
	require.EqualValues(maxBlockNumber, modified.UnstakedAt)
	require.EqualValues(identityset.Address(3), modified.Delegate)
	require.EqualValues(identityset.Address(4), modified.Owner)

	// check removed bucket info
	require.Len(delta[deltaStateRemoved], 1)
	removed, ok := delta[deltaStateRemoved][3]
	require.True(ok)
	require.Nil(removed)

}

func TestContractStakingDelta_BucketTypeDelta(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// add bucket type
	require.NoError(cache.AddBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}))
	require.NoError(cache.AddBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 100, ActivatedAt: 1}))

	// modify bucket type 1 & 3
	require.NoError(cache.UpdateBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 3}))
	require.NoError(cache.UpdateBucketType(3, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 4}))

	delta := cache.BucketTypeDelta()
	// check added bucket type
	require.Len(delta[deltaStateAdded], 2)
	added, ok := delta[deltaStateAdded][1]
	require.True(ok)
	require.NotNil(added)
	require.EqualValues(100, added.Amount.Int64())
	require.EqualValues(100, added.Duration)
	require.EqualValues(3, added.ActivatedAt)
	// check modified bucket type
	modified, ok := delta[deltaStateModified][3]
	require.True(ok)
	require.NotNil(modified)
	require.EqualValues(100, modified.Amount.Int64())
	require.EqualValues(100, modified.Duration)
	require.EqualValues(4, modified.ActivatedAt)

}

func TestContractStakingDelta_MatchBucketType(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// test with empty bucket type
	index, bucketType, ok := cache.MatchBucketType(big.NewInt(100), 100)
	require.False(ok)
	require.EqualValues(0, index)
	require.Nil(bucketType)

	// add bucket types
	require.NoError(cache.AddBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}))
	require.NoError(cache.AddBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 100, ActivatedAt: 1}))

	// test with amount and duration that match bucket type 1
	amount := big.NewInt(100)
	duration := uint64(100)
	index, bucketType, ok = cache.MatchBucketType(amount, duration)
	require.True(ok)
	require.EqualValues(1, index)
	require.NotNil(bucketType)
	require.EqualValues(big.NewInt(100), bucketType.Amount)
	require.EqualValues(uint64(100), bucketType.Duration)
	require.EqualValues(uint64(1), bucketType.ActivatedAt)

	// test with amount and duration that match bucket type 2
	amount = big.NewInt(200)
	duration = uint64(100)
	index, bucketType, ok = cache.MatchBucketType(amount, duration)
	require.True(ok)
	require.EqualValues(2, index)
	require.NotNil(bucketType)
	require.EqualValues(big.NewInt(200), bucketType.Amount)
	require.EqualValues(uint64(100), bucketType.Duration)
	require.EqualValues(uint64(1), bucketType.ActivatedAt)

	// test with amount and duration that do not match any bucket type
	amount = big.NewInt(300)
	duration = uint64(100)
	index, bucketType, ok = cache.MatchBucketType(amount, duration)
	require.False(ok)
	require.EqualValues(0, index)
	require.Nil(bucketType)
}

func TestContractStakingDelta_GetBucketInfo(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// add bucket info
	bi := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	require.NoError(cache.AddBucketInfo(1, bi))

	// get added bucket info
	info, state := cache.GetBucketInfo(1)
	require.NotNil(info)
	require.EqualValues(1, info.TypeIndex)
	require.EqualValues(1, info.CreatedAt)
	require.EqualValues(maxBlockNumber, info.UnlockedAt)
	require.EqualValues(maxBlockNumber, info.UnstakedAt)
	require.EqualValues(identityset.Address(1), info.Delegate)
	require.EqualValues(identityset.Address(2), info.Owner)
	require.EqualValues(deltaStateAdded, state)

	// modify bucket info 2
	bi = &bucketInfo{TypeIndex: 2, CreatedAt: 2, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)}
	require.NoError(cache.UpdateBucketInfo(2, bi))
	// get modified bucket info
	info, state = cache.GetBucketInfo(2)
	require.NotNil(info)
	require.EqualValues(2, info.TypeIndex)
	require.EqualValues(2, info.CreatedAt)
	require.EqualValues(maxBlockNumber, info.UnlockedAt)
	require.EqualValues(maxBlockNumber, info.UnstakedAt)
	require.EqualValues(identityset.Address(3), info.Delegate)
	require.EqualValues(identityset.Address(4), info.Owner)
	require.EqualValues(deltaStateModified, state)

	// remove bucket info 2
	require.NoError(cache.DeleteBucketInfo(2))
	// get removed bucket info
	info, state = cache.GetBucketInfo(2)
	require.Nil(info)
	require.EqualValues(deltaStateRemoved, state)
}

func TestContractStakingDelta_GetBucketType(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// add bucket type
	bt := &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}
	require.NoError(cache.AddBucketType(1, bt))

	// get added bucket type
	bucketType, state := cache.GetBucketType(1)
	require.NotNil(bucketType)
	require.EqualValues(big.NewInt(100), bucketType.Amount)
	require.EqualValues(100, bucketType.Duration)
	require.EqualValues(1, bucketType.ActivatedAt)
	require.EqualValues(deltaStateAdded, state)

	// modify bucket type
	bt = &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2}
	require.NoError(cache.UpdateBucketType(2, bt))
	// get modified bucket type
	bucketType, state = cache.GetBucketType(2)
	require.NotNil(bucketType)
	require.EqualValues(big.NewInt(200), bucketType.Amount)
	require.EqualValues(200, bucketType.Duration)
	require.EqualValues(2, bucketType.ActivatedAt)
	require.EqualValues(deltaStateModified, state)

}

func TestContractStakingDelta_AddedBucketCnt(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// test with no added bucket info
	addedBucketCnt := cache.AddedBucketCnt()
	require.EqualValues(0, addedBucketCnt)

	// add bucket types
	require.NoError(cache.AddBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}))
	require.NoError(cache.AddBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 100, ActivatedAt: 1}))

	// add bucket info
	bi := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	require.NoError(cache.AddBucketInfo(1, bi))
	// add bucket info
	bi = &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	require.NoError(cache.AddBucketInfo(2, bi))

	// test with added bucket info
	addedBucketCnt = cache.AddedBucketCnt()
	require.EqualValues(2, addedBucketCnt)

	// remove bucket info
	require.NoError(cache.DeleteBucketInfo(3))

	// test with removed bucket info
	addedBucketCnt = cache.AddedBucketCnt()
	require.EqualValues(2, addedBucketCnt)
}

func TestContractStakingDelta_AddedBucketTypeCnt(t *testing.T) {
	require := require.New(t)

	// create a new delta cache
	cache := newContractStakingDelta()

	// test with no added bucket types
	addedBucketTypeCnt := cache.AddedBucketTypeCnt()
	require.EqualValues(0, addedBucketTypeCnt)

	// add bucket types
	require.NoError(cache.AddBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}))
	require.NoError(cache.AddBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 100, ActivatedAt: 1}))
	require.NoError(cache.AddBucketType(3, &BucketType{Amount: big.NewInt(300), Duration: 100, ActivatedAt: 1}))

	// test with added bucket type
	addedBucketTypeCnt = cache.AddedBucketTypeCnt()
	require.EqualValues(3, addedBucketTypeCnt)
}
