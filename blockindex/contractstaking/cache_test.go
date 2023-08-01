package contractstaking

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestContractStakingCache_CandidateVotes(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")

	// no bucket
	require.EqualValues(0, cache.CandidateVotes(identityset.Address(1)).Int64())

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	require.EqualValues(100, cache.CandidateVotes(identityset.Address(1)).Int64())

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	require.EqualValues(200, cache.CandidateVotes(identityset.Address(1)).Int64())

	// add one bucket with different delegate
	cache.PutBucketInfo(3, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	require.EqualValues(200, cache.CandidateVotes(identityset.Address(1)).Int64())
	require.EqualValues(100, cache.CandidateVotes(identityset.Address(3)).Int64())

	// add one bucket with different owner
	cache.PutBucketInfo(4, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(4)})
	require.EqualValues(300, cache.CandidateVotes(identityset.Address(1)).Int64())

	// add one bucket with different amount
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(5, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	require.EqualValues(500, cache.CandidateVotes(identityset.Address(1)).Int64())

	// add one bucket with different duration
	cache.PutBucketType(3, &BucketType{Amount: big.NewInt(300), Duration: 200, ActivatedAt: 1})
	cache.PutBucketInfo(6, &bucketInfo{TypeIndex: 3, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	require.EqualValues(800, cache.CandidateVotes(identityset.Address(1)).Int64())

	// add one bucket that is unstaked
	cache.PutBucketInfo(7, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: 1, UnstakedAt: 1, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	require.EqualValues(800, cache.CandidateVotes(identityset.Address(1)).Int64())

	// add one bucket that is unlocked and staked
	cache.PutBucketInfo(8, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: 100, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	require.EqualValues(900, cache.CandidateVotes(identityset.Address(1)).Int64())

	// change delegate of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	require.EqualValues(800, cache.CandidateVotes(identityset.Address(1)).Int64())
	require.EqualValues(200, cache.CandidateVotes(identityset.Address(3)).Int64())

	// change owner of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	require.EqualValues(800, cache.CandidateVotes(identityset.Address(1)).Int64())
	require.EqualValues(200, cache.CandidateVotes(identityset.Address(3)).Int64())

	// change amount of bucket 1
	cache.putBucketInfo(1, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	require.EqualValues(800, cache.CandidateVotes(identityset.Address(1)).Int64())
	require.EqualValues(300, cache.CandidateVotes(identityset.Address(3)).Int64())
}

func TestContractStakingCache_Buckets(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(27).String()
	cache := newContractStakingCache(contractAddr)

	// no bucket
	require.Empty(cache.Buckets())

	// add one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	buckets := cache.Buckets()
	require.Len(buckets, 1)
	checkVoteBucket(require, buckets[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok := cache.Bucket(1)
	require.True(ok)
	checkVoteBucket(require, bucket, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// add one bucket with different index
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 1})
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	bucketMaps := bucketsToMap(cache.Buckets())
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(3).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok = cache.Bucket(1)
	require.True(ok)
	checkVoteBucket(require, bucket, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok = cache.Bucket(2)
	require.True(ok)
	checkVoteBucket(require, bucket, 2, identityset.Address(3).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)

	// update delegate of bucket 2
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(5), Owner: identityset.Address(4)})
	bucketMaps = bucketsToMap(cache.Buckets())
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok = cache.Bucket(1)
	require.True(ok)
	checkVoteBucket(require, bucket, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok = cache.Bucket(2)
	require.True(ok)
	checkVoteBucket(require, bucket, 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	bucketMaps = bucketsToMap(cache.Buckets())
	require.Len(bucketMaps, 1)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
	_, ok = cache.Bucket(1)
	require.False(ok)
	bucket, ok = cache.Bucket(2)
	require.True(ok)
	checkVoteBucket(require, bucket, 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
}

func TestContractStakingCache_BucketsByCandidate(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(27).String()
	cache := newContractStakingCache(contractAddr)

	// no bucket
	buckets := cache.BucketsByCandidate(identityset.Address(1))
	require.Len(buckets, 0)

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bucketMaps := bucketsToMap(cache.BucketsByCandidate(identityset.Address(1)))
	require.Len(bucketMaps, 1)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(1)))
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// add one bucket with different delegate
	cache.PutBucketInfo(3, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(1)))
	require.Len(bucketMaps, 2)
	require.Nil(bucketMaps[3])
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(3)))
	require.Len(bucketMaps, 1)
	checkVoteBucket(require, bucketMaps[3], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// change delegate of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(1)))
	require.Len(bucketMaps, 1)
	require.Nil(bucketMaps[1])
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(3)))
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[3], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// delete bucket 2
	cache.DeleteBucketInfo(2)
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(1)))
	require.Len(bucketMaps, 0)
	bucketMaps = bucketsToMap(cache.BucketsByCandidate(identityset.Address(3)))
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[3], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

}

func TestContractStakingCache_BucketsByIndices(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(27).String()
	cache := newContractStakingCache(contractAddr)

	// no bucket
	buckets, err := cache.BucketsByIndices([]uint64{1})
	require.NoError(err)
	require.Len(buckets, 0)

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	buckets, err = cache.BucketsByIndices([]uint64{1})
	require.NoError(err)
	require.Len(buckets, 1)
	bucketMaps := bucketsToMap(buckets)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	buckets, err = cache.BucketsByIndices([]uint64{1, 2})
	require.NoError(err)
	require.Len(buckets, 2)
	bucketMaps = bucketsToMap(buckets)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// one bucket not found
	buckets, err = cache.BucketsByIndices([]uint64{3})
	require.NoError(err)
	require.Len(buckets, 0)

	// one bucket found, one not found
	buckets, err = cache.BucketsByIndices([]uint64{1, 3})
	require.NoError(err)
	require.Len(buckets, 1)
	bucketMaps = bucketsToMap(buckets)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	buckets, err = cache.BucketsByIndices([]uint64{1})
	require.NoError(err)
	require.Len(buckets, 0)
}

func TestContractStakingCache_TotalBucketCount(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")

	// no bucket
	require.EqualValues(0, cache.TotalBucketCount())

	// one bucket
	cache.putTotalBucketCount(1)
	require.EqualValues(1, cache.TotalBucketCount())

	// two buckets
	cache.putTotalBucketCount(2)
	require.EqualValues(2, cache.TotalBucketCount())

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	require.EqualValues(2, cache.TotalBucketCount())
}

func TestContractStakingCache_ActiveBucketTypes(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")

	// no bucket type
	require.Empty(cache.ActiveBucketTypes())

	// one bucket type
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	activeBucketTypes := cache.ActiveBucketTypes()
	require.Len(activeBucketTypes, 1)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)

	// two bucket types
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	activeBucketTypes = cache.ActiveBucketTypes()
	require.Len(activeBucketTypes, 2)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)

	// add one inactive bucket type
	cache.PutBucketType(3, &BucketType{Amount: big.NewInt(300), Duration: 300, ActivatedAt: maxBlockNumber})
	activeBucketTypes = cache.ActiveBucketTypes()
	require.Len(activeBucketTypes, 2)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)

	// deactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: maxBlockNumber})
	activeBucketTypes = cache.ActiveBucketTypes()
	require.Len(activeBucketTypes, 1)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)

	// reactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	activeBucketTypes = cache.ActiveBucketTypes()
	require.Len(activeBucketTypes, 2)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)
}

func TestContractStakingCache_Merge(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")
	height := uint64(1)

	// create delta with one bucket type
	delta := newContractStakingDelta()
	delta.AddBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	// merge delta into cache
	err := cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket type was added to cache
	activeBucketTypes := cache.ActiveBucketTypes()
	require.Len(activeBucketTypes, 1)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)
	require.EqualValues(height, cache.Height())

	// create delta with one bucket
	delta = newContractStakingDelta()
	delta.AddBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	// merge delta into cache
	err = cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket was added to cache and vote count is correct
	require.EqualValues(100, cache.CandidateVotes(identityset.Address(1)).Int64())

	// create delta with updated bucket delegate
	delta = newContractStakingDelta()
	delta.UpdateBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	// merge delta into cache
	err = cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket delegate was updated and vote count is correct
	require.EqualValues(0, cache.CandidateVotes(identityset.Address(1)).Int64())
	require.EqualValues(100, cache.CandidateVotes(identityset.Address(3)).Int64())

	// create delta with deleted bucket
	delta = newContractStakingDelta()
	delta.DeleteBucketInfo(1)
	// merge delta into cache
	err = cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket was deleted from cache and vote count is 0
	require.EqualValues(0, cache.CandidateVotes(identityset.Address(3)).Int64())
}

func TestContractStakingCache_MatchBucketType(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")

	// no bucket types
	_, bucketType, ok := cache.MatchBucketType(big.NewInt(100), 100)
	require.False(ok)
	require.Nil(bucketType)

	// one bucket type
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	// match exact bucket type
	id, bucketType, ok := cache.MatchBucketType(big.NewInt(100), 100)
	require.True(ok)
	require.EqualValues(1, id)
	require.EqualValues(100, bucketType.Amount.Int64())
	require.EqualValues(100, bucketType.Duration)
	require.EqualValues(1, bucketType.ActivatedAt)

	// match bucket type with different amount
	_, bucketType, ok = cache.MatchBucketType(big.NewInt(200), 100)
	require.False(ok)
	require.Nil(bucketType)

	// match bucket type with different duration
	_, bucketType, ok = cache.MatchBucketType(big.NewInt(100), 200)
	require.False(ok)
	require.Nil(bucketType)

	// no match
	_, bucketType, ok = cache.MatchBucketType(big.NewInt(200), 200)
	require.False(ok)
	require.Nil(bucketType)
}

func TestContractStakingCache_BucketTypeCount(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")

	// no bucket type
	require.EqualValues(0, cache.BucketTypeCount())

	// one bucket type
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	require.EqualValues(1, cache.BucketTypeCount())

	// two bucket types
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	require.EqualValues(2, cache.BucketTypeCount())

	// deactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: maxBlockNumber})
	require.EqualValues(2, cache.BucketTypeCount())
}

func TestContractStakingCache_LoadFromDB(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache("")

	// load from empty db
	path, err := testutil.PathOfTempFile("staking.db")
	require.NoError(err)
	defer testutil.CleanupPath(path)
	cfg := config.Default.DB
	cfg.DbPath = path
	kvstore := db.NewBoltDB(cfg)
	require.NoError(kvstore.Start(context.Background()))
	defer kvstore.Stop(context.Background())

	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	require.Equal(uint64(0), cache.TotalBucketCount())
	require.Equal(0, len(cache.Buckets()))
	require.EqualValues(0, cache.BucketTypeCount())

	// load from db with height and total bucket count
	kvstore.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(12345))
	kvstore.Put(_StakingNS, _stakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(10))
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	require.Equal(uint64(10), cache.TotalBucketCount())
	require.Equal(0, len(cache.Buckets()))
	require.EqualValues(0, cache.BucketTypeCount())

	// load from db with bucket
	bucketInfo := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	kvstore.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(1), bucketInfo.Serialize())
	bucketType := &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}
	kvstore.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(1), bucketType.Serialize())
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	require.Equal(uint64(10), cache.TotalBucketCount())
	bi, ok := cache.BucketInfo(1)
	require.True(ok)
	require.Equal(1, len(cache.Buckets()))
	require.Equal(bucketInfo, bi)
	require.EqualValues(1, cache.BucketTypeCount())
	id, bt, ok := cache.MatchBucketType(big.NewInt(100), 100)
	require.True(ok)
	require.EqualValues(1, id)
	require.EqualValues(100, bt.Amount.Int64())
	require.EqualValues(100, bt.Duration)
	require.EqualValues(1, bt.ActivatedAt)
}

func bucketsToMap(buckets []*staking.VoteBucket) map[uint64]*staking.VoteBucket {
	m := make(map[uint64]*staking.VoteBucket)
	for _, bucket := range buckets {
		m[bucket.Index] = bucket
	}
	return m
}

func checkVoteBucket(r *require.Assertions, bucket *staking.VoteBucket, index uint64, candidate, owner string, amount, duration, createHeight, startHeight, unstakeHeight uint64, autoStake bool, contractAddr string) {
	r.EqualValues(index, bucket.Index)
	r.EqualValues(candidate, bucket.Candidate.String())
	r.EqualValues(owner, bucket.Owner.String())
	r.EqualValues(amount, bucket.StakedAmount.Int64())
	r.EqualValues(duration, bucket.StakedDurationBlockNumber)
	r.EqualValues(createHeight, bucket.CreateBlockHeight)
	r.EqualValues(startHeight, bucket.StakeStartBlockHeight)
	r.EqualValues(unstakeHeight, bucket.UnstakeStartBlockHeight)
	r.EqualValues(autoStake, bucket.AutoStake)
	r.EqualValues(contractAddr, bucket.ContractAddress)
}
