package contractstaking

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestContractStakingDirty_getBucketType(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache()
	dirty := newContractStakingDirty(clean)

	// no bucket type
	bt, ok := dirty.getBucketType(1)
	require.False(ok)
	require.Nil(bt)

	// bucket type in clean cache
	clean.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	bt, ok = dirty.getBucketType(1)
	require.True(ok)
	require.EqualValues(100, bt.Amount.Int64())
	require.EqualValues(100, bt.Duration)
	require.EqualValues(1, bt.ActivatedAt)

	// added bucket type
	dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	bt, ok = dirty.getBucketType(2)
	require.True(ok)
	require.EqualValues(200, bt.Amount.Int64())
	require.EqualValues(200, bt.Duration)
	require.EqualValues(2, bt.ActivatedAt)
}

func TestContractStakingDirty_getBucketInfo(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache()
	dirty := newContractStakingDirty(clean)

	// no bucket info
	bi, ok := dirty.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)

	// bucket info in clean cache
	clean.putBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	clean.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bi, ok = dirty.getBucketInfo(1)
	require.True(ok)
	require.EqualValues(1, bi.TypeIndex)
	require.EqualValues(1, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.Equal(identityset.Address(1), bi.Delegate)
	require.Equal(identityset.Address(2), bi.Owner)

	// added bucket info
	dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	dirty.addBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 2, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(2), Owner: identityset.Address(3)})
	bi, ok = dirty.getBucketInfo(2)
	require.True(ok)
	require.EqualValues(2, bi.TypeIndex)
	require.EqualValues(2, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.Equal(identityset.Address(2), bi.Delegate)
	require.Equal(identityset.Address(3), bi.Owner)

	// modified bucket info
	dirty.updateBucketInfo(1, &bucketInfo{TypeIndex: 2, CreatedAt: 3, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	bi, ok = dirty.getBucketInfo(1)
	require.True(ok)
	require.EqualValues(2, bi.TypeIndex)
	require.EqualValues(3, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.Equal(identityset.Address(3), bi.Delegate)
	require.Equal(identityset.Address(4), bi.Owner)

	// removed bucket info
	dirty.deleteBucketInfo(1)
	bi, ok = dirty.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)
}

func TestContractStakingDirty_matchBucketType(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache()
	dirty := newContractStakingDirty(clean)

	// no bucket type
	id, bt := dirty.matchBucketType(big.NewInt(100), 100)
	require.Nil(bt)
	require.EqualValues(0, id)

	// bucket type in clean cache
	clean.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	id, bt = dirty.matchBucketType(big.NewInt(100), 100)
	require.NotNil(bt)
	require.EqualValues(100, bt.Amount.Int64())
	require.EqualValues(100, bt.Duration)
	require.EqualValues(1, bt.ActivatedAt)
	require.EqualValues(1, id)

	// added bucket type
	dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	id, bt = dirty.matchBucketType(big.NewInt(200), 200)
	require.NotNil(bt)
	require.EqualValues(200, bt.Amount.Int64())
	require.EqualValues(200, bt.Duration)
	require.EqualValues(2, bt.ActivatedAt)
	require.EqualValues(2, id)
}

func TestContractStakingDirty_getBucketTypeCount(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache()
	dirty := newContractStakingDirty(clean)

	// no bucket type
	count := dirty.getBucketTypeCount()
	require.EqualValues(0, count)

	// bucket type in clean cache
	clean.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	count = dirty.getBucketTypeCount()
	require.EqualValues(1, count)

	// added bucket type
	dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	count = dirty.getBucketTypeCount()
	require.EqualValues(2, count)
}

func TestContractStakingDirty_finalize(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache()
	dirty := newContractStakingDirty(newWrappedCache(clean))
	totalCnt := clean.TotalBucketCount()
	require.EqualValues(0, totalCnt)

	// no dirty data
	batcher, cache := dirty.Finalize()
	require.EqualValues(1, batcher.Size())
	info, err := batcher.Entry(0)
	require.NoError(err)
	require.EqualValues(_StakingNS, info.Namespace())
	require.EqualValues(batch.Put, info.WriteType())
	require.EqualValues(_stakingTotalBucketCountKey, info.Key())
	require.EqualValues(byteutil.Uint64ToBytesBigEndian(0), info.Value())
	require.Equal(0, cache.BucketTypeCount())

	// added bucket type
	bt := &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}
	dirty.addBucketType(1, bt)
	batcher, cache = dirty.Finalize()
	require.EqualValues(2, batcher.Size())
	info, err = batcher.Entry(1)
	require.NoError(err)
	require.EqualValues(_StakingBucketTypeNS, info.Namespace())
	require.EqualValues(batch.Put, info.WriteType())
	require.EqualValues(byteutil.Uint64ToBytesBigEndian(1), info.Key())
	btdata, err := bt.Serialize()
	require.NoError(err)
	require.EqualValues(btdata, info.Value())
	require.Equal(1, cache.BucketTypeCount())

	// add bucket info
	bi := &bucketInfo{TypeIndex: 1, CreatedAt: 2, UnlockedAt: 3, UnstakedAt: 4, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	dirty.addBucketInfo(1, bi)
	batcher, cache = dirty.Finalize()
	require.EqualValues(3, batcher.Size())
	info, err = batcher.Entry(2)
	require.NoError(err)
	require.EqualValues(_StakingBucketInfoNS, info.Namespace())
	require.EqualValues(batch.Put, info.WriteType())
	require.EqualValues(byteutil.Uint64ToBytesBigEndian(1), info.Key())
	bidata, err := bi.Serialize()
	require.NoError(err)
	require.EqualValues(bidata, info.Value())
	totalCnt = cache.TotalBucketCount()
	require.EqualValues(1, totalCnt)
}

func TestContractStakingDirty_noSideEffectOnClean(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache()
	dirty := newContractStakingDirty(newWrappedCache(clean))

	// add bucket type to dirty cache
	dirty.addBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	// check that clean cache is not affected
	bt, ok := clean.getBucketType(1)
	require.False(ok)
	require.Nil(bt)

	// add bucket info to dirty cache
	dirty.addBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	// check that clean cache is not affected
	bi, ok := clean.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)

	// update bucket type in dirty cache
	dirty.updateBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 3})
	// check that clean cache is not affected
	bt, ok = clean.getBucketType(1)
	require.False(ok)
	require.Nil(bt)

	// update bucket info in dirty cache
	dirty.updateBucketInfo(1, &bucketInfo{TypeIndex: 2, CreatedAt: 3, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	// check that clean cache is not affected
	bi, ok = clean.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)

	clean.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	// update bucket info existed in clean cache
	clean.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	// update bucket info in dirty cache
	dirty.updateBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 3, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	// check that clean cache is not affected
	bi, ok = clean.getBucketInfo(2)
	require.True(ok)
	require.EqualValues(1, bi.TypeIndex)
	require.EqualValues(1, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.EqualValues(identityset.Address(1).String(), bi.Delegate.String())
	require.EqualValues(identityset.Address(2).String(), bi.Owner.String())

	// remove bucket info existed in clean cache
	clean.PutBucketInfo(3, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	// remove bucket info from dirty cache
	dirty.deleteBucketInfo(3)
	// check that clean cache is not affected
	bi, ok = clean.getBucketInfo(3)
	require.True(ok)
	require.EqualValues(1, bi.TypeIndex)
	require.EqualValues(1, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.EqualValues(identityset.Address(1).String(), bi.Delegate.String())
	require.EqualValues(identityset.Address(2).String(), bi.Owner.String())

}
