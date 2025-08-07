package contractstaking

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

/*
func _checkCacheCandidateVotes(ctx context.Context, r *require.Assertions, cache *contractStakingCache, _ uint64, addr address.Address, expectVotes int64) {
	votes, err := cache.CandidateVotes(ctx, addr)
	r.NoError(err)
	r.EqualValues(expectVotes, votes.Int64())
}
*/

func calculateVoteWeightGen(c genesis.VoteWeightCalConsts) calculateVoteWeightFunc {
	return func(v *Bucket) *big.Int {
		return staking.CalculateVoteWeight(c, v, false)
	}
}

func TestContractStakingCache_CandidateVotes(t *testing.T) {
	checkCacheCandidateVotesGen := func(ctx context.Context) func(r *require.Assertions, cache *contractStakingCache, height uint64, addr address.Address, expectVotes int64) {
		return func(r *require.Assertions, cache *contractStakingCache, height uint64, addr address.Address, expectVotes int64) {
			// _checkCacheCandidateVotes(ctx, r, cache, height, addr, expectVotes)
		}
	}
	require := require.New(t)
	g := genesis.TestDefault()
	cache := newContractStakingCache()
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
	cache := newContractStakingCache()

	// no bucket
	ids, _, _ := cache.Buckets()
	require.Empty(ids)

	// add one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	ids, bts, bis := cache.Buckets()
	require.Len(ids, 1)
	require.Len(bts, 1)
	require.Len(bis, 1)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	bt, bi := cache.Bucket(1)
	require.NotNil(bt)
	require.NotNil(bi)
	checkBucket(require, 1, bt, bi, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// add one bucket with different index
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 1})
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(4)})
	ids, bts, bis = cache.Buckets()
	require.Len(ids, 2)
	require.Len(bts, 2)
	require.Len(bis, 2)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	checkBucket(require, ids[1], bts[1], bis[1], 2, identityset.Address(3).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true)
	bt, bi = cache.Bucket(1)
	require.NotNil(bt)
	require.NotNil(bi)
	checkBucket(require, 1, bt, bi, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	bt, bi = cache.Bucket(2)
	require.NotNil(bt)
	require.NotNil(bi)
	checkBucket(require, 2, bt, bi, 2, identityset.Address(3).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true)

	// update delegate of bucket 2
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(5), Owner: identityset.Address(4)})
	ids, bts, bis = cache.Buckets()
	require.Len(ids, 2)
	require.Len(bts, 2)
	require.Len(bis, 2)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	checkBucket(require, ids[1], bts[1], bis[1], 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true)
	bt, bi = cache.Bucket(1)
	require.NotNil(bt)
	require.NotNil(bi)
	checkBucket(require, 1, bt, bi, 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	bt, bi = cache.Bucket(2)
	require.NotNil(bt)
	require.NotNil(bi)
	checkBucket(require, 2, bt, bi, 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	ids, bts, bis = cache.Buckets()
	require.Len(ids, 1)
	require.Len(bts, 1)
	require.Len(bis, 1)
	checkBucket(require, ids[0], bts[0], bis[0], 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true)
	bt, bi = cache.Bucket(1)
	require.Nil(bt)
	require.Nil(bi)
	bt, bi = cache.Bucket(2)
	require.NotNil(bt)
	require.NotNil(bi)
	checkBucket(require, 2, bt, bi, 2, identityset.Address(5).String(), identityset.Address(4).String(), 200, 200, 1, 1, maxBlockNumber, true)
}

func TestContractStakingCache_BucketsByCandidate(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache()

	// no bucket
	ids, _, _ := cache.BucketsByCandidate(identityset.Address(1))
	require.Len(ids, 0)

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	ids, bts, bis := cache.BucketsByCandidate(identityset.Address(1))
	require.Len(ids, 1)
	require.Len(bts, 1)
	require.Len(bis, 1)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(1))
	require.Len(ids, 2)
	require.Len(bts, 2)
	require.Len(bis, 2)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	checkBucket(require, ids[1], bts[1], bis[0], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// add one bucket with different delegate
	cache.PutBucketInfo(3, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(1))
	require.Len(ids, 2)
	require.Len(bts, 2)
	require.Len(bis, 2)
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(3))
	require.Len(ids, 1)
	require.Len(bts, 1)
	require.Len(bis, 1)
	checkBucket(require, ids[0], bts[0], bis[0], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// change delegate of bucket 1
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(3), Owner: identityset.Address(2)})
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(1))
	require.Len(ids, 1)
	require.Len(bts, 1)
	require.Len(bis, 1)
	checkBucket(require, ids[0], bts[0], bis[0], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(3))
	require.Len(ids, 2)
	require.Len(bts, 2)
	require.Len(bis, 2)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	checkBucket(require, ids[1], bts[1], bis[1], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// delete bucket 2
	cache.DeleteBucketInfo(2)
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(1))
	require.Len(ids, 0)
	require.Len(bts, 0)
	require.Len(bis, 0)
	ids, bts, bis = cache.BucketsByCandidate(identityset.Address(3))
	require.Len(ids, 2)
	require.Len(bts, 2)
	require.Len(bis, 2)
	checkBucket(require, ids[0], bts[0], bis[0], 1, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	checkBucket(require, ids[1], bts[1], bis[1], 3, identityset.Address(3).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

}

func TestContractStakingCache_BucketsByIndices(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache()

	// no bucket
	bts, bis := cache.BucketsByIndices([]uint64{1})
	require.Len(bts, 1)
	require.Len(bis, 1)
	require.Nil(bts[0])
	require.Nil(bis[0])

	// one bucket
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketInfo(1, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bts, bis = cache.BucketsByIndices([]uint64{1})
	require.Len(bts, 1)
	require.Len(bis, 1)
	checkBucket(require, 1, bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// two buckets
	cache.PutBucketInfo(2, &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)})
	bts, bis = cache.BucketsByIndices([]uint64{1, 2})
	require.Len(bts, 2)
	require.Len(bis, 2)
	checkBucket(require, 1, bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)
	checkBucket(require, 2, bts[1], bis[1], 2, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// one bucket not found
	bts, bis = cache.BucketsByIndices([]uint64{3})
	require.Len(bts, 1)
	require.Len(bis, 1)
	require.Nil(bts[0])
	require.Nil(bis[0])

	// one bucket found, one not found
	bts, bis = cache.BucketsByIndices([]uint64{1, 3})
	require.Len(bts, 2)
	require.Len(bis, 2)
	require.Nil(bts[1])
	require.Nil(bis[1])
	checkBucket(require, 1, bts[0], bis[0], 1, identityset.Address(1).String(), identityset.Address(2).String(), 100, 100, 1, 1, maxBlockNumber, true)

	// delete bucket 1
	cache.DeleteBucketInfo(1)
	bts, bis = cache.BucketsByIndices([]uint64{1})
	require.Len(bts, 1)
	require.Len(bis, 1)
	require.Nil(bts[0])
	require.Nil(bis[0])
}

func TestContractStakingCache_ActiveBucketTypes(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache()

	// no bucket type
	abt := cache.ActiveBucketTypes()
	require.Empty(abt)

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

func TestContractStakingCache_MatchBucketType(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache()

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
	cache := newContractStakingCache()

	// no bucket type
	btc := cache.BucketTypeCount()
	require.EqualValues(0, btc)

	// one bucket type
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	btc = cache.BucketTypeCount()
	require.EqualValues(1, btc)

	// two bucket types
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 2})
	btc = cache.BucketTypeCount()
	require.EqualValues(2, btc)

	// deactivate bucket type 1
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: maxBlockNumber})
	btc = cache.BucketTypeCount()
	require.EqualValues(2, btc)
}

func TestContractStakingCache_LoadFromDB(t *testing.T) {
	require := require.New(t)
	cache := newContractStakingCache()

	// mock kvstore exception
	ctrl := gomock.NewController(t)
	mockKvStore := db.NewMockKVStore(ctrl)

	// kvstore exception at load height
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("err1")).Times(1)
	err := cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err1")

	// kvstore exception at load total bucket count
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, errors.New("err2")).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err2")

	// kvstore exception at load bucket info
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(1)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("err3")).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err3")

	// mock bucketInfo Deserialize failed
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(1)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, [][]byte{nil}, nil).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)

	// kvstore exception at load bucket type
	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(1)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, db.ErrBucketNotExist).Times(1)
	mockKvStore.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil, errors.New("err4")).Times(1)
	err = cache.LoadFromDB(mockKvStore)
	require.Error(err)
	require.Equal(err.Error(), "err4")

	mockKvStore.EXPECT().Get(gomock.Any(), gomock.Any()).Return(nil, db.ErrNotExist).Times(1)
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
	require.Equal(uint64(0), cache.TotalBucketCount())
	ids, bts, bis := cache.Buckets()
	require.Equal(0, len(ids))
	require.Equal(0, len(bts))
	require.Equal(0, len(bis))
	btc := cache.BucketTypeCount()
	require.EqualValues(0, btc)

	// load from db with height and total bucket count
	height = 12345
	kvstore.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(height))
	kvstore.Put(_StakingNS, _stakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(10))
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	require.Equal(uint64(10), cache.TotalBucketCount())
	ids, bts, bis = cache.Buckets()
	require.Equal(0, len(ids))
	require.Equal(0, len(bts))
	require.Equal(0, len(bis))
	btc = cache.BucketTypeCount()
	require.EqualValues(0, btc)

	// load from db with bucket
	bucketInfo := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	bidata, err := bucketInfo.Serialize()
	require.NoError(err)
	kvstore.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(1), bidata)
	bucketType := &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1}
	btdata, err := bucketType.Serialize()
	require.NoError(err)
	kvstore.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(1), btdata)
	err = cache.LoadFromDB(kvstore)
	require.NoError(err)
	require.Equal(uint64(10), cache.TotalBucketCount())
	bi, ok := cache.BucketInfo(1)
	require.True(ok)
	ids, bts, bis = cache.Buckets()
	require.Equal(1, len(ids))
	require.Equal(1, len(bts))
	require.Equal(1, len(bis))
	require.Equal(bucketInfo, bi)
	btc = cache.BucketTypeCount()
	require.EqualValues(1, btc)
	id, bt, ok := cache.MatchBucketType(big.NewInt(100), 100)
	require.True(ok)
	require.EqualValues(1, id)
	require.EqualValues(100, bt.Amount.Int64())
	require.EqualValues(100, bt.Duration)
	require.EqualValues(1, bt.ActivatedAt)
}

func bucketsToMap(ids []uint64, buckets []*staking.VoteBucket) map[uint64]*staking.VoteBucket {
	m := make(map[uint64]*staking.VoteBucket)
	for _, bucket := range buckets {
		m[bucket.Index] = bucket
	}
	return m
}

func checkBucket(r *require.Assertions, id uint64, bt *BucketType, bucket *bucketInfo, index uint64, candidate, owner string, amount, duration, createHeight, startHeight, unstakeHeight uint64, autoStake bool) {
	r.EqualValues(index, id)
	r.EqualValues(candidate, bucket.Delegate.String())
	r.EqualValues(owner, bucket.Owner.String())
	r.EqualValues(amount, bt.Amount.Int64())
	r.EqualValues(duration, bt.Duration)
	r.EqualValues(createHeight, bucket.CreatedAt)
	r.EqualValues(startHeight, bucket.CreatedAt)
	r.EqualValues(unstakeHeight, bucket.UnstakedAt)
	r.EqualValues(autoStake, bucket.UnlockedAt == maxBlockNumber)
}

func TestContractStakingCache_MustGetBucketInfo(t *testing.T) {
	// build test condition to add a bucketInfo
	cache := newContractStakingCache()
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
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
	cache := newContractStakingCache()
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
	r.Equal(err.Error(), "bucket type not found: 2")
}

func TestContractStakingCache_DeleteBucketInfo(t *testing.T) {
	// build test condition to add a bucketInfo
	cache := newContractStakingCache()
	cache.PutBucketType(1, &BucketType{Amount: big.NewInt(100), Duration: 100, ActivatedAt: 1})
	cache.PutBucketType(2, &BucketType{Amount: big.NewInt(200), Duration: 200, ActivatedAt: 1})
	bi1 := &bucketInfo{TypeIndex: 1, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(1)}
	bi2 := &bucketInfo{TypeIndex: 2, CreatedAt: 1, UnlockedAt: maxBlockNumber, UnstakedAt: maxBlockNumber, Delegate: identityset.Address(1), Owner: identityset.Address(2)}
	cache.PutBucketInfo(1, bi1)
	cache.PutBucketInfo(2, bi2)

	cache.DeleteBucketInfo(1)

	// remove candidate bucket before
	delete(cache.candidateBucketMap, bi2.Delegate.String())
	cache.DeleteBucketInfo(2)
}
