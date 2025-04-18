package contractstaking

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func _checkCacheCandidateVotes(ctx context.Context, r *require.Assertions, cache *contractStakingCache, height uint64, addr address.Address, expectVotes int64) {
	votes, err := cache.CandidateVotes(ctx, addr, height)
	r.NoError(err)
	r.EqualValues(expectVotes, votes.Int64())
}

func calculateVoteWeightGen(c genesis.VoteWeightCalConsts) calculateVoteWeightFunc {
	return func(v *Bucket) *big.Int {
		return staking.CalculateVoteWeight(c, v, false)
	}
}

func TestContractStakingCache_CandidateVotes(t *testing.T) {
	checkCacheCandidateVotesGen := func(ctx context.Context) func(r *require.Assertions, cache *contractStakingCache, height uint64, addr address.Address, expectVotes int64) {
		return func(r *require.Assertions, cache *contractStakingCache, height uint64, addr address.Address, expectVotes int64) {
			_checkCacheCandidateVotes(ctx, r, cache, height, addr, expectVotes)
		}
	}
	require := require.New(t)
	g := genesis.TestDefault()
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(1).String(), CalculateVoteWeight: calculateVoteWeightGen(g.VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})
	checkCacheCandidateVotes := checkCacheCandidateVotesGen(protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), g), protocol.BlockCtx{BlockHeight: 1})))
	checkCacheCandidateVotesAfterRedsea := checkCacheCandidateVotesGen(protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), g), protocol.BlockCtx{BlockHeight: g.RedseaBlockHeight})))
	// no bucket
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 0)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 0)

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 100)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 103)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 200)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 206)

	// add one bucket with different delegate
	cache.PutBucketInfo(3, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 200)
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(3), 100)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 206)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(3), 103)

	// add one bucket with different owner
	cache.PutBucketInfo(4, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(4)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 300)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 309)

	// add one bucket with different amount
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(5, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 500)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 516)

	// add one bucket with different duration
	cache.PutBucketType(3, &BucketType{Amount: big.NewInt(300), Duration: 200, ActivatedAt: 1})
	cache.PutBucketInfo(6, &bucketInfo{TypeIndex: 3, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 800)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 827)

	// add one bucket that is unstaked
	cache.PutBucketInfo(7, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: 1, UnstakedAt: 1, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 800)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 827)

	// add one bucket that is unlocked and staked
	cache.PutBucketInfo(8, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: 100, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 900)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 927)

	// change delegate of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 800)
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(3), 200)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 824)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(3), 206)

	// change owner of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 800)
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(3), 200)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 824)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(3), 206)

	// change amount of bucket 1
	cache.putBucketInfo(1, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(1), 800)
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(3), 300)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(1), 824)
	checkCacheCandidateVotesAfterRedsea(require, cache, 0, identityset.Address(3), 310)

	// put bucket info and make it disabled
	bi := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)}
	cache.PutBucketInfo(9, bi)
	cache.candidateBucketMap[bi.Delegate.String()][9] = false
	checkCacheCandidateVotes(require, cache, 0, identityset.Address(3), 300)
}

func TestContractStakingCache_Buckets(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(27).String()
	cache := newContractStakingCache(Config{ContractAddress: contractAddr, CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	height := uint64(0)
	// no bucket
	bts, err := cache.Buckets(height)
	require.NoError(err)
	require.Empty(bts)

	// add one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	buckets, err := cache.Buckets(height)
	require.NoError(err)
	require.Len(buckets, 1)
	checkVoteBucket(require, buckets[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok, err := cache.Bucket(1, height)
	require.NoError(err)
	require.True(ok)
	checkVoteBucket(require, bucket, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// add one bucket with different index
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 1})
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	bts, err = cache.Buckets(height)
	require.NoError(err)
	bucketMaps := bucketsToMap(bts)
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(3).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok, err = cache.Bucket(1, height)
	require.NoError(err)
	require.True(ok)
	checkVoteBucket(require, bucket, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok, err = cache.Bucket(2, height)
	require.NoError(err)
	require.True(ok)
	checkVoteBucket(require, bucket, 2, identityset.Address(3).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)

	// update delegate of bucket 2
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(5), Owner: identityset.Address(4)})
	bts, err = cache.Buckets(height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok, err = cache.Bucket(1, height)
	require.NoError(err)
	require.True(ok)
	checkVoteBucket(require, bucket, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bucket, ok, err = cache.Bucket(2, height)
	require.NoError(err)
	require.True(ok)
	checkVoteBucket(require, bucket, 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	bts, err = cache.Buckets(height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 1)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
	_, ok, err = cache.Bucket(1, height)
	require.NoError(err)
	require.False(ok)
	bucket, ok, err = cache.Bucket(2, height)
	require.NoError(err)
	require.True(ok)
	checkVoteBucket(require, bucket, 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true, contractAddr)
}

func TestContractStakingCache_BucketsByCandidate(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(27).String()
	cache := newContractStakingCache(Config{ContractAddress: contractAddr, CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	height := uint64(0)
	// no bucket
	buckets, err := cache.BucketsByCandidate(identityset.Address(1), height)
	require.NoError(err)
	require.Len(buckets, 0)

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bts, err := cache.BucketsByCandidate(identityset.Address(1), height)
	require.NoError(err)
	bucketMaps := bucketsToMap(bts)
	require.Len(bucketMaps, 1)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bts, err = cache.BucketsByCandidate(identityset.Address(1), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// add one bucket with different delegate
	cache.PutBucketInfo(3, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	bts, err = cache.BucketsByCandidate(identityset.Address(1), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 2)
	require.Nil(bucketMaps[3])
	bts, err = cache.BucketsByCandidate(identityset.Address(3), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 1)
	checkVoteBucket(require, bucketMaps[3], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// change delegate of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	bts, err = cache.BucketsByCandidate(identityset.Address(1), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 1)
	require.Nil(bucketMaps[1])
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	bts, err = cache.BucketsByCandidate(identityset.Address(3), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[3], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// delete bucket 2
	cache.DeleteBucketInfo(2)
	bts, err = cache.BucketsByCandidate(identityset.Address(1), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 0)
	bts, err = cache.BucketsByCandidate(identityset.Address(3), height)
	require.NoError(err)
	bucketMaps = bucketsToMap(bts)
	require.Len(bucketMaps, 2)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[3], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

}

func TestContractStakingCache_BucketsByIndices(t *testing.T) {
	require := require.New(t)
	contractAddr := identityset.Address(27).String()
	cache := newContractStakingCache(Config{ContractAddress: contractAddr, CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	height := uint64(0)
	// no bucket
	buckets, err := cache.BucketsByIndices([]uint64{1}, height)
	require.NoError(err)
	require.Len(buckets, 0)

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	buckets, err = cache.BucketsByIndices([]uint64{1}, height)
	require.NoError(err)
	require.Len(buckets, 1)
	bucketMaps := bucketsToMap(buckets)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	buckets, err = cache.BucketsByIndices([]uint64{1, 2}, height)
	require.NoError(err)
	require.Len(buckets, 2)
	bucketMaps = bucketsToMap(buckets)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)
	checkVoteBucket(require, bucketMaps[2], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// one bucket not found
	buckets, err = cache.BucketsByIndices([]uint64{3}, height)
	require.NoError(err)
	require.Len(buckets, 0)

	// one bucket found, one not found
	buckets, err = cache.BucketsByIndices([]uint64{1, 3}, height)
	require.NoError(err)
	require.Len(buckets, 1)
	bucketMaps = bucketsToMap(buckets)
	checkVoteBucket(require, bucketMaps[1], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true, contractAddr)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	buckets, err = cache.BucketsByIndices([]uint64{1}, height)
	require.NoError(err)
	require.Len(buckets, 0)
}

func TestContractStakingCache_TotalBucketCount(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(27).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	height := uint64(0)
	// no bucket
	tbc, err := cache.TotalBucketCount(height)
	require.NoError(err)
	require.EqualValues(0, tbc)

	// one bucket
	cache.putTotalBucketCount(1)
	tbc, err = cache.TotalBucketCount(height)
	require.NoError(err)
	require.EqualValues(1, tbc)

	// two buckets
	cache.putTotalBucketCount(2)
	tbc, err = cache.TotalBucketCount(height)
	require.NoError(err)
	require.EqualValues(2, tbc)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	tbc, err = cache.TotalBucketCount(height)
	require.NoError(err)
	require.EqualValues(2, tbc)
}

func TestContractStakingCache_ActiveBucketTypes(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(27).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	height := uint64(0)
	// no bucket type
	abt, err := cache.ActiveBucketTypes(height)
	require.NoError(err)
	require.Empty(abt)

	// one bucket type
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	activeBucketTypes, err := cache.ActiveBucketTypes(height)
	require.NoError(err)
	require.Len(activeBucketTypes, 1)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)

	// two bucket types
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	activeBucketTypes, err = cache.ActiveBucketTypes(height)
	require.NoError(err)
	require.Len(activeBucketTypes, 2)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)

	// add one inactive bucket type
	cache.PutBucketType(3, &BucketType{Amount: big.NewInt(300), Duration: 300, ActivatedAt: maxBlockNumber})
	activeBucketTypes, err = cache.ActiveBucketTypes(height)
	require.NoError(err)
	require.Len(activeBucketTypes, 2)
	require.EqualValues(100, activeBucketTypes[1].Amount.Int64())
	require.EqualValues(100, activeBucketTypes[1].Duration)
	require.EqualValues(1, activeBucketTypes[1].ActivatedAt)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)

	// deactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: maxBlockNumber})
	activeBucketTypes, err = cache.ActiveBucketTypes(height)
	require.NoError(err)
	require.Len(activeBucketTypes, 1)
	require.EqualValues(200, activeBucketTypes[2].Amount.Int64())
	require.EqualValues(200, activeBucketTypes[2].Duration)
	require.EqualValues(2, activeBucketTypes[2].ActivatedAt)

	// reactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	activeBucketTypes, err = cache.ActiveBucketTypes(height)
	require.NoError(err)
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
	g := genesis.TestDefault()
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(27).String(), CalculateVoteWeight: calculateVoteWeightGen(g.VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})
	height := uint64(1)
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), g), protocol.BlockCtx{BlockHeight: height}))

	// create delta with one bucket type
	delta := newContractStakingDelta()
	delta.AddBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	// merge delta into cache
	err := cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket type was added to cache
	activeBucketTypes, err := cache.ActiveBucketTypes(height)
	require.NoError(err)
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
	votes, err := cache.CandidateVotes(ctx, identityset.Address(1), height)
	require.NoError(err)
	require.EqualValues(100, votes.Int64())

	// create delta with updated bucket delegate
	delta = newContractStakingDelta()
	delta.UpdateBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	// merge delta into cache
	err = cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket delegate was updated and vote count is correct
	votes, err = cache.CandidateVotes(ctx, identityset.Address(1), height)
	require.NoError(err)
	require.EqualValues(0, votes.Int64())
	votes, err = cache.CandidateVotes(ctx, identityset.Address(3), height)
	require.NoError(err)
	require.EqualValues(100, votes.Int64())

	// create delta with deleted bucket
	delta = newContractStakingDelta()
	delta.DeleteBucketInfo(1)
	// merge delta into cache
	err = cache.Merge(delta, height)
	require.NoError(err)
	// check that bucket was deleted from cache and vote count is 0
	votes, err = cache.CandidateVotes(ctx, identityset.Address(3), height)
	require.NoError(err)
	require.EqualValues(0, votes.Int64())

	// invalid delta
	err = cache.Merge(nil, height)
	require.Error(err)
	require.Equal(err.Error(), "invalid contract staking delta")
}

func TestContractStakingCache_MatchBucketType(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(27).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

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
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(27).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	height := uint64(0)
	// no bucket type
	btc, err := cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(0, btc)

	// one bucket type
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	btc, err = cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(1, btc)

	// two bucket types
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	btc, err = cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(2, btc)

	// deactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: maxBlockNumber})
	btc, err = cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(2, btc)

	btc, err = cache.BucketTypeCount(1)
	require.Error(err)
	require.Contains(err.Error(), "invalid height")
}

func TestContractStakingCache_LoadFromDB(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(27).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})

	// mock kvstore exception
	ctrl := gomock.NewController(t)
	mockKvStore := db.NewMockKVStore(ctrl)

	// kvstore exception at load height
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("err1")).Times(1)
	err := cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err1")

	// kvstore exception at load total bucket count
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(1)
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("err2")).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err2")

	// kvstore exception at load bucket info
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(2)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("err3")).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err3")

	// mock bucketInfo Deserialize failed
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(2)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, [][]byte{nil}, nil).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)

	// kvstore exception at load bucket type
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(2)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, db.ErrBucketNotExist).Times(1)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("err4")).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err4")

	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(2)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, db.ErrBucketNotExist).Times(1)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, [][]byte{nil}, nil).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)

	// load from empty db
	path, err := testutil.PathOfTempFile("staking.db")
	require.NoError(err)
	defer testutil.CleanupPath(path)
	cfg := config.Default.DB
	cfg.DbPath = path
	kvstore := db.NewBoltDB(cfg)
	require.NoError(kvstore.Start(context.Background()))
	defer kvstore.Stop(context.Background())

	height := uint64(0)
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	tbc, err := cache.TotalBucketCount(height)
	require.NoError(err)
	require.Equal(uint64(0), tbc)
	bts, err := cache.Buckets(height)
	require.NoError(err)
	require.Equal(0, len(bts))
	btc, err := cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(0, btc)

	// load from db with height and total bucket count
	height = 12345
	kvstore.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(height))
	kvstore.Put(_StakingNS, _stakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(10))
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	tbc, err = cache.TotalBucketCount(height)
	require.NoError(err)
	require.Equal(uint64(10), tbc)
	bts, err = cache.Buckets(height)
	require.NoError(err)
	require.Equal(0, len(bts))
	btc, err = cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(0, btc)

	// load from db with bucket
	bucketInfo := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	kvstore.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(1), bucketInfo.Serialize())
	bucketType := &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}
	kvstore.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(1), bucketType.Serialize())
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	tbc, err = cache.TotalBucketCount(height)
	require.NoError(err)
	require.Equal(uint64(10), tbc)
	bi, ok := cache.BucketInfo(1)
	require.True(ok)
	bts, err = cache.Buckets(height)
	require.NoError(err)
	require.Equal(1, len(bts))
	require.Equal(bucketInfo, bi)
	btc, err = cache.BucketTypeCount(height)
	require.NoError(err)
	require.EqualValues(1, btc)
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

func TestContractStakingCache_MustGetBucketInfo(t *testing.T) {
	// build test condition to add a bucketInfo
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(1).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})

	tryCatchMustGetBucketInfo := func(i uint64) (v *bucketInfo, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errors.New(fmt.Sprint(r))
			}
		}()
		v = cache.MustGetBucketInfo(i)
		return
	}

	r := require.New(t)
	v, err := tryCatchMustGetBucketInfo(1)
	r.NotNil(v)
	r.NoError(err)

	v, err = tryCatchMustGetBucketInfo(2)
	r.Nil(v)
	r.Error(err)
	r.Equal(err.Error(), "bucket info not found")
}

func TestContractStakingCache_MustGetBucketType(t *testing.T) {
	// build test condition to add a bucketType
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(1).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})

	tryCatchMustGetBucketType := func(i uint64) (v *BucketType, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = errors.New(fmt.Sprint(r))
			}
		}()
		v = cache.MustGetBucketType(i)
		return
	}

	r := require.New(t)
	v, err := tryCatchMustGetBucketType(1)
	r.NotNil(v)
	r.NoError(err)

	v, err = tryCatchMustGetBucketType(2)
	r.Nil(v)
	r.Error(err)
	r.Equal(err.Error(), "bucket type not found")
}

func TestContractStakingCache_DeleteBucketInfo(t *testing.T) {
	// build test condition to add a bucketInfo
	cache := newContractStakingCache(Config{ContractAddress: identityset.Address(1).String(), CalculateVoteWeight: calculateVoteWeightGen(genesis.TestDefault().VoteWeightCalConsts), BlocksToDuration: _blockDurationFn})
	bi1 := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(1)}
	bi2 := &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	cache.PutBucketInfo(1, bi1)
	cache.PutBucketInfo(2, bi2)

	cache.DeleteBucketInfo(1)

	// remove candidate bucket before
	delete(cache.candidateBucketMap, bi2.Delegate.String())
	cache.DeleteBucketInfo(2)
}
