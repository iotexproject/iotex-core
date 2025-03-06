package contractstaking

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestContractStakingDirty_getBucketType(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache(Config{CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts)})
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
	clean := newContractStakingCache(Config{CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts)})
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
	require.NoError(dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2}))
	require.NoError(dirty.addBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 2, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(2), Owner: identityset.Address(3)}))
	bi, ok = dirty.getBucketInfo(2)
	require.True(ok)
	require.EqualValues(2, bi.TypeIndex)
	require.EqualValues(2, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.Equal(identityset.Address(2), bi.Delegate)
	require.Equal(identityset.Address(3), bi.Owner)

	// modified bucket info
	require.NoError(dirty.updateBucketInfo(1, &bucketInfo{TypeIndex: 2, CreatedAt: 3, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)}))
	bi, ok = dirty.getBucketInfo(1)
	require.True(ok)
	require.EqualValues(2, bi.TypeIndex)
	require.EqualValues(3, bi.CreatedAt)
	require.EqualValues(maxBlockNumber, bi.UnlockedAt)
	require.EqualValues(maxBlockNumber, bi.UnstakedAt)
	require.Equal(identityset.Address(3), bi.Delegate)
	require.Equal(identityset.Address(4), bi.Owner)

	// removed bucket info
	require.NoError(dirty.deleteBucketInfo(1))
	bi, ok = dirty.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)
}

func TestContractStakingDirty_matchBucketType(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache(Config{CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts)})
	dirty := newContractStakingDirty(clean)

	// no bucket type
	id, bt, ok := dirty.matchBucketType(big.NewInt(100), 100)
	require.False(ok)
	require.Nil(bt)
	require.EqualValues(0, id)

	// bucket type in clean cache
	clean.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	id, bt, ok = dirty.matchBucketType(big.NewInt(100), 100)
	require.True(ok)
	require.EqualValues(100, bt.Amount.Int64())
	require.EqualValues(100, bt.Duration)
	require.EqualValues(1, bt.ActivatedAt)
	require.EqualValues(1, id)

	// added bucket type
	require.NoError(dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2}))
	id, bt, ok = dirty.matchBucketType(big.NewInt(200), 200)
	require.True(ok)
	require.EqualValues(200, bt.Amount.Int64())
	require.EqualValues(200, bt.Duration)
	require.EqualValues(2, bt.ActivatedAt)
	require.EqualValues(2, id)
}

func TestContractStakingDirty_getBucketTypeCount(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache(Config{CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts)})
	dirty := newContractStakingDirty(clean)

	// no bucket type
	count := dirty.getBucketTypeCount()
	require.EqualValues(0, count)

	// bucket type in clean cache
	clean.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	count = dirty.getBucketTypeCount()
	require.EqualValues(1, count)

	// added bucket type
	require.NoError(dirty.addBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2}))
	count = dirty.getBucketTypeCount()
	require.EqualValues(2, count)
}

func TestContractStakingDirty_finalize(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache(Config{CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts)})
	dirty := newContractStakingDirty(clean)

	// no dirty data
	batcher, delta := dirty.finalize()
	require.EqualValues(1, batcher.Size())
	info, err := batcher.Entry(0)
	require.NoError(err)
	require.EqualValues(_StakingNS, info.Namespace())
	require.EqualValues(batch.Put, info.WriteType())
	require.EqualValues(_stakingTotalBucketCountKey, info.Key())
	require.EqualValues(byteutil.Uint64ToBytesBigEndian(0), info.Value())
	for _, d := range delta.BucketTypeDelta() {
		require.Len(d, 0)
	}
	for _, d := range delta.BucketTypeDelta() {
		require.Len(d, 0)
	}

	// added bucket type
	bt := &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}
	require.NoError(dirty.addBucketType(1, bt))
	batcher, delta = dirty.finalize()
	require.EqualValues(2, batcher.Size())
	info, err = batcher.Entry(1)
	require.NoError(err)
	require.EqualValues(_StakingBucketTypeNS, info.Namespace())
	require.EqualValues(batch.Put, info.WriteType())
	require.EqualValues(byteutil.Uint64ToBytesBigEndian(1), info.Key())
	require.EqualValues(bt.Serialize(), info.Value())
	btDelta := delta.BucketTypeDelta()
	require.NotNil(btDelta[deltaStateAdded])
	require.Len(btDelta[deltaStateAdded], 1)
	require.EqualValues(100, btDelta[deltaStateAdded][1].Amount.Int64())
	require.EqualValues(100, btDelta[deltaStateAdded][1].Duration)
	require.EqualValues(1, btDelta[deltaStateAdded][1].ActivatedAt)

	// add bucket info
	bi := &bucketInfo{TypeIndex: 1, CreatedAt: 2, UnlockedAt: 3, UnstakedAt: 4, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	require.NoError(dirty.addBucketInfo(1, bi))
	batcher, delta = dirty.finalize()
	require.EqualValues(3, batcher.Size())
	info, err = batcher.Entry(2)
	require.NoError(err)
	require.EqualValues(_StakingBucketInfoNS, info.Namespace())
	require.EqualValues(batch.Put, info.WriteType())
	require.EqualValues(byteutil.Uint64ToBytesBigEndian(1), info.Key())
	require.EqualValues(bi.Serialize(), info.Value())
	biDelta := delta.BucketInfoDelta()
	require.NotNil(biDelta[deltaStateAdded])
	require.Len(biDelta[deltaStateAdded], 1)
	require.EqualValues(1, biDelta[deltaStateAdded][1].TypeIndex)
	require.EqualValues(2, biDelta[deltaStateAdded][1].CreatedAt)
	require.EqualValues(3, biDelta[deltaStateAdded][1].UnlockedAt)
	require.EqualValues(4, biDelta[deltaStateAdded][1].UnstakedAt)
	require.EqualValues(identityset.Address(1).String(), biDelta[deltaStateAdded][1].Delegate.String())
	require.EqualValues(identityset.Address(2).String(), biDelta[deltaStateAdded][1].Owner.String())

}

func TestContractStakingDirty_noSideEffectOnClean(t *testing.T) {
	require := require.New(t)
	clean := newContractStakingCache(Config{CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts)})
	dirty := newContractStakingDirty(clean)

	// add bucket type to dirty cache
	require.NoError(dirty.addBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}))
	// check that clean cache is not affected
	bt, ok := clean.getBucketType(1)
	require.False(ok)
	require.Nil(bt)

	// add bucket info to dirty cache
	require.NoError(dirty.addBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}))
	// check that clean cache is not affected
	bi, ok := clean.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)

	// update bucket type in dirty cache
	require.NoError(dirty.updateBucketType(1, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2}))
	// check that clean cache is not affected
	bt, ok = clean.getBucketType(1)
	require.False(ok)
	require.Nil(bt)

	// update bucket info in dirty cache
	require.NoError(dirty.updateBucketInfo(1, &bucketInfo{TypeIndex: 2, CreatedAt: 3, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)}))
	// check that clean cache is not affected
	bi, ok = clean.getBucketInfo(1)
	require.False(ok)
	require.Nil(bi)

	// update bucket info existed in clean cache
	clean.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	// update bucket info in dirty cache
	require.NoError(dirty.updateBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 3, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)}))
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
	require.NoError(dirty.deleteBucketInfo(3))
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
