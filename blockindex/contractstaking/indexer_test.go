// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_testStakingContractAddress = "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd"
)

func TestNewContractStakingIndexer(t *testing.T) {
	r := require.New(t)

	t.Run("kvStore is nil", func(t *testing.T) {
		_, err := NewContractStakingIndexer(nil, "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd", 0)
		r.Error(err)
		r.Contains(err.Error(), "kv store is nil")
	})

	t.Run("invalid contract address", func(t *testing.T) {
		kvStore := db.NewMemKVStore()
		_, err := NewContractStakingIndexer(kvStore, "invalid address", 0)
		r.Error(err)
		r.Contains(err.Error(), "invalid contract address")
	})

	t.Run("valid input", func(t *testing.T) {
		contractAddr, err := address.FromString("io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd")
		r.NoError(err)
		indexer, err := NewContractStakingIndexer(db.NewMemKVStore(), contractAddr.String(), 0)
		r.NoError(err)
		r.NotNil(indexer)
	})
}

func TestContractStakingIndexerLoadCache(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// create a stake
	height := uint64(1)
	startHeight := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	activateBucketType(r, handler, 10, 100, height)
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	stake(r, handler, owner, delegate, 1, 10, 100, height)
	err = indexer.commit(handler)
	r.NoError(err)
	buckets, err := indexer.Buckets()
	r.NoError(err)
	r.NoError(indexer.Stop(context.Background()))

	// load cache from db
	newIndexer, err := NewContractStakingIndexer(db.NewBoltDB(cfg), _testStakingContractAddress, startHeight)
	r.NoError(err)
	r.NoError(newIndexer.Start(context.Background()))

	// check cache
	newBuckets, err := newIndexer.Buckets()
	r.NoError(err)
	r.Equal(len(buckets), len(newBuckets))
	for i := range buckets {
		r.EqualValues(buckets[i], newBuckets[i])
	}
	newHeight, err := newIndexer.Height()
	r.NoError(err)
	r.Equal(height, newHeight)
	r.Equal(startHeight, newIndexer.StartHeight())
	r.EqualValues(1, newIndexer.TotalBucketCount())
	r.NoError(newIndexer.Stop(context.Background()))
}

func TestContractStakingIndexerDirty(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// before commit dirty, the cache should be empty
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	gotHeight, err := indexer.Height()
	r.NoError(err)
	r.EqualValues(0, gotHeight)
	// after commit dirty, the cache should be updated
	err = indexer.commit(handler)
	r.NoError(err)
	gotHeight, err = indexer.Height()
	r.NoError(err)
	r.EqualValues(height, gotHeight)

	r.NoError(indexer.Stop(context.Background()))
}

func TestContractStakingIndexerThreadSafe(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	wait := sync.WaitGroup{}
	wait.Add(6)
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	// read concurrently
	for i := 0; i < 5; i++ {
		go func() {
			defer wait.Done()
			for i := 0; i < 1000; i++ {
				_, err := indexer.Buckets()
				r.NoError(err)
				_, err = indexer.BucketTypes()
				r.NoError(err)
				_, err = indexer.BucketsByCandidate(delegate)
				r.NoError(err)
				indexer.CandidateVotes(delegate)
				_, err = indexer.Height()
				r.NoError(err)
				indexer.TotalBucketCount()
			}
		}()
	}
	// write
	go func() {
		defer wait.Done()
		// activate bucket type
		handler := newContractStakingEventHandler(indexer.cache, 1)
		activateBucketType(r, handler, 10, 100, 1)
		r.NoError(indexer.commit(handler))
		for i := 2; i < 1000; i++ {
			height := uint64(i)
			handler := newContractStakingEventHandler(indexer.cache, height)
			stake(r, handler, owner, delegate, int64(i), 10, 100, height)
			err := indexer.commit(handler)
			r.NoError(err)
		}
	}()
	wait.Wait()
	r.NoError(indexer.Stop(context.Background()))
	// no panic means thread safe
}

func TestContractStakingIndexerBucketType(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// activate
	bucketTypeData := [][2]int64{
		{10, 10},
		{20, 10},
		{10, 100},
		{20, 100},
	}
	existInBucketTypeData := func(bt *BucketType) int {
		idx := slices.IndexFunc(bucketTypeData, func(data [2]int64) bool {
			if bt.Amount.Int64() == data[0] && int64(bt.Duration) == data[1] {
				return true
			}
			return false
		})
		return idx
	}

	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler)
	r.NoError(err)
	bucketTypes, err := indexer.BucketTypes()
	r.NoError(err)
	r.Equal(len(bucketTypeData), len(bucketTypes))
	for _, bt := range bucketTypes {
		r.True(existInBucketTypeData(bt) >= 0)
		r.EqualValues(height, bt.ActivatedAt)
	}
	// deactivate
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	for i := 0; i < 2; i++ {
		data := bucketTypeData[i]
		deactivateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler)
	r.NoError(err)
	bucketTypes, err = indexer.BucketTypes()
	r.NoError(err)
	r.Equal(len(bucketTypeData)-2, len(bucketTypes))
	for _, bt := range bucketTypes {
		r.True(existInBucketTypeData(bt) >= 0)
		r.EqualValues(1, bt.ActivatedAt)
	}
	// reactivate
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	for i := 0; i < 2; i++ {
		data := bucketTypeData[i]
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler)
	r.NoError(err)
	bucketTypes, err = indexer.BucketTypes()
	r.NoError(err)
	r.Equal(len(bucketTypeData), len(bucketTypes))
	for _, bt := range bucketTypes {
		idx := existInBucketTypeData(bt)
		r.True(idx >= 0)
		if idx < 2 {
			r.EqualValues(height, bt.ActivatedAt)
		} else {
			r.EqualValues(1, bt.ActivatedAt)
		}
	}
	r.NoError(indexer.Stop(context.Background()))
}

func TestContractStakingIndexerBucketInfo(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	bucketTypeData := [][2]int64{
		{10, 10},
		{20, 10},
		{10, 100},
		{20, 100},
	}
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler)
	r.NoError(err)

	// stake
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	height++
	createHeight := height
	handler = newContractStakingEventHandler(indexer.cache, height)
	stake(r, handler, owner, delegate, 1, 10, 100, height)
	r.NoError(err)
	r.NoError(indexer.commit(handler))
	bucket, ok := indexer.Bucket(1)
	r.True(ok)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(100, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.True(bucket.AutoStake)
	r.EqualValues(height, bucket.CreateBlockHeight)
	r.EqualValues(maxBlockNumber, bucket.UnstakeStartBlockHeight)
	r.EqualValues(_testStakingContractAddress, bucket.ContractAddress)
	r.EqualValues(10, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// transfer
	newOwner := identityset.Address(2)
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	transfer(r, handler, newOwner, int64(bucket.Index))
	r.NoError(indexer.commit(handler))
	bucket, ok = indexer.Bucket(bucket.Index)
	r.True(ok)
	r.EqualValues(newOwner, bucket.Owner)

	// unlock
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	unlock(r, handler, int64(bucket.Index), height)
	r.NoError(indexer.commit(handler))
	bucket, ok = indexer.Bucket(bucket.Index)
	r.True(ok)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(newOwner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(100, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.False(bucket.AutoStake)
	r.EqualValues(createHeight, bucket.CreateBlockHeight)
	r.EqualValues(maxBlockNumber, bucket.UnstakeStartBlockHeight)
	r.EqualValues(_testStakingContractAddress, bucket.ContractAddress)
	r.EqualValues(10, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// lock again
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	lock(r, handler, int64(bucket.Index), int64(10))
	r.NoError(indexer.commit(handler))
	bucket, ok = indexer.Bucket(bucket.Index)
	r.True(ok)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(newOwner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(10, bucket.StakedDurationBlockNumber)
	r.EqualValues(createHeight, bucket.StakeStartBlockHeight)
	r.True(bucket.AutoStake)
	r.EqualValues(createHeight, bucket.CreateBlockHeight)
	r.EqualValues(maxBlockNumber, bucket.UnstakeStartBlockHeight)
	r.EqualValues(_testStakingContractAddress, bucket.ContractAddress)
	r.EqualValues(10, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// unstake
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	unlock(r, handler, int64(bucket.Index), height)
	unstake(r, handler, int64(bucket.Index), height)
	r.NoError(indexer.commit(handler))
	bucket, ok = indexer.Bucket(bucket.Index)
	r.True(ok)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(newOwner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(10, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.False(bucket.AutoStake)
	r.EqualValues(createHeight, bucket.CreateBlockHeight)
	r.EqualValues(height, bucket.UnstakeStartBlockHeight)
	r.EqualValues(_testStakingContractAddress, bucket.ContractAddress)
	r.EqualValues(0, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// withdraw
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	withdraw(r, handler, int64(bucket.Index))
	r.NoError(indexer.commit(handler))
	bucket, ok = indexer.Bucket(bucket.Index)
	r.False(ok)
	r.EqualValues(0, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())
}

func TestContractStakingIndexerChangeBucketType(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	bucketTypeData := [][2]int64{
		{10, 10},
		{20, 10},
		{10, 100},
		{20, 100},
	}
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler)
	r.NoError(err)

	t.Run("expand bucket type", func(t *testing.T) {
		owner := identityset.Address(0)
		delegate := identityset.Address(1)
		height++
		handler = newContractStakingEventHandler(indexer.cache, height)
		stake(r, handler, owner, delegate, 1, 10, 100, height)
		r.NoError(err)
		r.NoError(indexer.commit(handler))
		bucket, ok := indexer.Bucket(1)
		r.True(ok)

		expandBucketType(r, handler, int64(bucket.Index), 20, 100)
		r.NoError(indexer.commit(handler))
		bucket, ok = indexer.Bucket(bucket.Index)
		r.True(ok)
		r.EqualValues(20, bucket.StakedAmount.Int64())
		r.EqualValues(100, bucket.StakedDurationBlockNumber)
	})
}

func TestContractStakingIndexerReadBuckets(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	bucketTypeData := [][2]int64{
		{10, 10},
		{20, 10},
		{10, 100},
		{20, 100},
	}
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler)
	r.NoError(err)

	// stake
	stakeData := []struct {
		owner, delegate  int
		amount, duration uint64
	}{
		{1, 2, 10, 10},
		{1, 2, 20, 10},
		{1, 2, 10, 100},
		{1, 2, 20, 100},
		{1, 3, 10, 100},
		{1, 3, 20, 100},
	}
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	for i, data := range stakeData {
		stake(r, handler, identityset.Address(data.owner), identityset.Address(data.delegate), int64(i), int64(data.amount), int64(data.duration), height)
	}
	r.NoError(err)
	r.NoError(indexer.commit(handler))

	t.Run("Buckets", func(t *testing.T) {
		buckets, err := indexer.Buckets()
		r.NoError(err)
		r.Len(buckets, len(stakeData))
	})

	t.Run("BucketsByCandidate", func(t *testing.T) {
		candidateMap := make(map[int]int)
		for i := range stakeData {
			candidateMap[stakeData[i].delegate]++
		}
		for cand := range candidateMap {
			buckets, err := indexer.BucketsByCandidate(identityset.Address(cand))
			r.NoError(err)
			r.Len(buckets, candidateMap[cand])
		}
	})

	t.Run("BucketsByIndices", func(t *testing.T) {
		indices := []uint64{0, 1, 2, 3, 4, 5, 6}
		buckets, err := indexer.BucketsByIndices(indices)
		r.NoError(err)
		expectedLen := 0
		for _, idx := range indices {
			if int(idx) < len(stakeData) {
				expectedLen++
			}
		}
		r.Len(buckets, expectedLen)
	})

	t.Run("TotalBucketCount", func(t *testing.T) {
		r.EqualValues(len(stakeData), indexer.TotalBucketCount())
	})

	t.Run("CandidateVotes", func(t *testing.T) {
		candidateMap := make(map[int]int64)
		for i := range stakeData {
			candidateMap[stakeData[i].delegate] += int64(stakeData[i].amount)
		}
		candidates := []int{1, 2, 3}
		for _, cand := range candidates {
			votes := candidateMap[cand]
			r.EqualValues(votes, indexer.CandidateVotes(identityset.Address(cand)).Uint64())
		}
	})
}

func TestContractStakingIndexerCacheClean(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	activateBucketType(r, handler, 10, 10, height)
	activateBucketType(r, handler, 20, 20, height)
	// create bucket
	owner := identityset.Address(10)
	delegate1 := identityset.Address(1)
	delegate2 := identityset.Address(2)
	stake(r, handler, owner, delegate1, 1, 10, 10, height)
	stake(r, handler, owner, delegate1, 2, 20, 20, height)
	stake(r, handler, owner, delegate2, 3, 20, 20, height)
	stake(r, handler, owner, delegate2, 4, 20, 20, height)
	r.Len(indexer.cache.ActiveBucketTypes(), 0)
	r.Len(indexer.cache.Buckets(), 0)
	r.NoError(indexer.commit(handler))
	r.Len(indexer.cache.ActiveBucketTypes(), 2)
	r.Len(indexer.cache.Buckets(), 4)

	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	changeDelegate(r, handler, delegate1, 3)
	transfer(r, handler, delegate1, 1)
	bt, ok := indexer.Bucket(3)
	r.True(ok)
	r.Equal(delegate2.String(), bt.Candidate.String())
	bt, ok = indexer.Bucket(1)
	r.True(ok)
	r.Equal(owner.String(), bt.Owner.String())
	r.NoError(indexer.commit(handler))
	bt, ok = indexer.Bucket(3)
	r.True(ok)
	r.Equal(delegate1.String(), bt.Candidate.String())
	bt, ok = indexer.Bucket(1)
	r.True(ok)
	r.Equal(delegate1.String(), bt.Owner.String())
}

func TestContractStakingIndexerVotes(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, _testStakingContractAddress, 0)
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache, height)
	activateBucketType(r, handler, 10, 10, height)
	activateBucketType(r, handler, 20, 20, height)
	activateBucketType(r, handler, 30, 20, height)
	activateBucketType(r, handler, 60, 20, height)
	// create bucket
	owner := identityset.Address(10)
	delegate1 := identityset.Address(1)
	delegate2 := identityset.Address(2)
	stake(r, handler, owner, delegate1, 1, 10, 10, height)
	stake(r, handler, owner, delegate1, 2, 20, 20, height)
	stake(r, handler, owner, delegate2, 3, 20, 20, height)
	stake(r, handler, owner, delegate2, 4, 20, 20, height)
	r.NoError(indexer.commit(handler))
	r.EqualValues(30, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(40, indexer.CandidateVotes(delegate2).Uint64())
	r.EqualValues(0, indexer.CandidateVotes(owner).Uint64())

	// change delegate bucket 3 to delegate1
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	changeDelegate(r, handler, delegate1, 3)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(20, indexer.CandidateVotes(delegate2).Uint64())

	// unlock bucket 1 & 4
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	unlock(r, handler, 1, height)
	unlock(r, handler, 4, height)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(20, indexer.CandidateVotes(delegate2).Uint64())

	// unstake bucket 1 & lock 4
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	unstake(r, handler, 1, height)
	lock(r, handler, 4, 20)
	r.NoError(indexer.commit(handler))
	r.EqualValues(40, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(20, indexer.CandidateVotes(delegate2).Uint64())

	// expand bucket 2
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	expandBucketType(r, handler, 2, 30, 20)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(20, indexer.CandidateVotes(delegate2).Uint64())

	// transfer bucket 4
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	transfer(r, handler, delegate2, 4)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(20, indexer.CandidateVotes(delegate2).Uint64())

	// create bucket 5, 6, 7
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	stake(r, handler, owner, delegate2, 5, 20, 20, height)
	stake(r, handler, owner, delegate2, 6, 20, 20, height)
	stake(r, handler, owner, delegate2, 7, 20, 20, height)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(80, indexer.CandidateVotes(delegate2).Uint64())

	// merge bucket 5, 6, 7
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	mergeBuckets(r, handler, []int64{5, 6, 7}, 60, 20)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(80, indexer.CandidateVotes(delegate2).Uint64())

	// unlock & unstake 5
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	unlock(r, handler, 5, height)
	unstake(r, handler, 5, height)
	r.NoError(indexer.commit(handler))
	r.EqualValues(50, indexer.CandidateVotes(delegate1).Uint64())
	r.EqualValues(20, indexer.CandidateVotes(delegate2).Uint64())

	t.Run("Height", func(t *testing.T) {
		h, err := indexer.Height()
		r.NoError(err)
		r.EqualValues(height, h)
	})

	t.Run("BucketTypes", func(t *testing.T) {
		bts, err := indexer.BucketTypes()
		r.NoError(err)
		r.Len(bts, 4)
		slices.SortFunc(bts, func(i, j *staking.ContractStakingBucketType) bool {
			return i.Amount.Int64() < j.Amount.Int64()
		})
		r.EqualValues(10, bts[0].Duration)
		r.EqualValues(20, bts[1].Duration)
		r.EqualValues(20, bts[2].Duration)
		r.EqualValues(20, bts[3].Duration)
		r.EqualValues(10, bts[0].Amount.Int64())
		r.EqualValues(20, bts[1].Amount.Int64())
		r.EqualValues(30, bts[2].Amount.Int64())
		r.EqualValues(60, bts[3].Amount.Int64())
	})

	t.Run("Buckets", func(t *testing.T) {
		bts, err := indexer.Buckets()
		r.NoError(err)
		r.Len(bts, 5)
		slices.SortFunc(bts, func(i, j *staking.VoteBucket) bool {
			return i.Index < j.Index
		})
		r.EqualValues(1, bts[0].Index)
		r.EqualValues(2, bts[1].Index)
		r.EqualValues(3, bts[2].Index)
		r.EqualValues(4, bts[3].Index)
		r.EqualValues(5, bts[4].Index)
		r.EqualValues(10, bts[0].StakedDurationBlockNumber)
		r.EqualValues(20, bts[1].StakedDurationBlockNumber)
		r.EqualValues(20, bts[2].StakedDurationBlockNumber)
		r.EqualValues(20, bts[3].StakedDurationBlockNumber)
		r.EqualValues(20, bts[4].StakedDurationBlockNumber)
		r.EqualValues(10, bts[0].StakedAmount.Int64())
		r.EqualValues(30, bts[1].StakedAmount.Int64())
		r.EqualValues(20, bts[2].StakedAmount.Int64())
		r.EqualValues(20, bts[3].StakedAmount.Int64())
		r.EqualValues(60, bts[4].StakedAmount.Int64())
		r.EqualValues(delegate1.String(), bts[0].Candidate.String())
		r.EqualValues(delegate1.String(), bts[1].Candidate.String())
		r.EqualValues(delegate1.String(), bts[2].Candidate.String())
		r.EqualValues(delegate2.String(), bts[3].Candidate.String())
		r.EqualValues(delegate2.String(), bts[4].Candidate.String())
		r.EqualValues(owner.String(), bts[0].Owner.String())
		r.EqualValues(owner.String(), bts[1].Owner.String())
		r.EqualValues(owner.String(), bts[2].Owner.String())
		r.EqualValues(delegate2.String(), bts[3].Owner.String())
		r.EqualValues(owner.String(), bts[4].Owner.String())
		r.False(bts[0].AutoStake)
		r.True(bts[1].AutoStake)
		r.True(bts[2].AutoStake)
		r.True(bts[3].AutoStake)
		r.False(bts[4].AutoStake)
		r.EqualValues(1, bts[0].CreateBlockHeight)
		r.EqualValues(1, bts[1].CreateBlockHeight)
		r.EqualValues(1, bts[2].CreateBlockHeight)
		r.EqualValues(1, bts[3].CreateBlockHeight)
		r.EqualValues(7, bts[4].CreateBlockHeight)
		r.EqualValues(3, bts[0].StakeStartBlockHeight)
		r.EqualValues(1, bts[1].StakeStartBlockHeight)
		r.EqualValues(1, bts[2].StakeStartBlockHeight)
		r.EqualValues(1, bts[3].StakeStartBlockHeight)
		r.EqualValues(9, bts[4].StakeStartBlockHeight)
		r.EqualValues(4, bts[0].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[1].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[2].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[3].UnstakeStartBlockHeight)
		r.EqualValues(9, bts[4].UnstakeStartBlockHeight)
		for _, b := range bts {
			r.EqualValues(0, b.StakedDuration)
			r.EqualValues(time.Time{}, b.CreateTime)
			r.EqualValues(time.Time{}, b.StakeStartTime)
			r.EqualValues(time.Time{}, b.UnstakeStartTime)
			r.EqualValues(_testStakingContractAddress, b.ContractAddress)
		}
	})

	t.Run("BucketsByCandidate", func(t *testing.T) {
		d1Bts, err := indexer.BucketsByCandidate(delegate1)
		r.NoError(err)
		r.Len(d1Bts, 3)
		slices.SortFunc(d1Bts, func(i, j *staking.VoteBucket) bool {
			return i.Index < j.Index
		})
		r.EqualValues(1, d1Bts[0].Index)
		r.EqualValues(2, d1Bts[1].Index)
		r.EqualValues(3, d1Bts[2].Index)
		d2Bts, err := indexer.BucketsByCandidate(delegate2)
		r.NoError(err)
		r.Len(d2Bts, 2)
		slices.SortFunc(d2Bts, func(i, j *staking.VoteBucket) bool {
			return i.Index < j.Index
		})
		r.EqualValues(4, d2Bts[0].Index)
		r.EqualValues(5, d2Bts[1].Index)
	})

	t.Run("BucketsByIndices", func(t *testing.T) {
		bts, err := indexer.BucketsByIndices([]uint64{1, 2, 3, 4, 5})
		r.NoError(err)
		r.Len(bts, 5)
	})
}

func BenchmarkIndexer_PutBlockBeforeContractHeight(b *testing.B) {
	// Create a new Indexer with a contract height of 100
	indexer := &Indexer{contractDeployHeight: 100}

	// Create a mock block with a height of 50
	blk := &block.Block{}

	// Run the benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := indexer.PutBlock(context.Background(), blk)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func activateBucketType(r *require.Assertions, handler *contractStakingEventHandler, amount, duration int64, height uint64) {
	err := handler.handleBucketTypeActivatedEvent(eventParam{
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	}, height)
	r.NoError(err)
}

func deactivateBucketType(r *require.Assertions, handler *contractStakingEventHandler, amount, duration int64, height uint64) {
	err := handler.handleBucketTypeDeactivatedEvent(eventParam{
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	}, height)
	r.NoError(err)
}

func stake(r *require.Assertions, handler *contractStakingEventHandler, owner, candidate address.Address, token, amount, duration int64, height uint64) {
	err := handler.handleTransferEvent(eventParam{
		"to":      common.BytesToAddress(owner.Bytes()),
		"tokenId": big.NewInt(token),
	})
	r.NoError(err)
	err = handler.handleStakedEvent(eventParam{
		"tokenId":  big.NewInt(token),
		"delegate": common.BytesToAddress(candidate.Bytes()),
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	}, height)
	r.NoError(err)
}

func unlock(r *require.Assertions, handler *contractStakingEventHandler, token int64, height uint64) {
	err := handler.handleUnlockedEvent(eventParam{
		"tokenId": big.NewInt(token),
	}, height)
	r.NoError(err)
}

func lock(r *require.Assertions, handler *contractStakingEventHandler, token, duration int64) {
	err := handler.handleLockedEvent(eventParam{
		"tokenId":  big.NewInt(token),
		"duration": big.NewInt(duration),
	})
	r.NoError(err)
}

func unstake(r *require.Assertions, handler *contractStakingEventHandler, token int64, height uint64) {
	err := handler.handleUnstakedEvent(eventParam{
		"tokenId": big.NewInt(token),
	}, height)
	r.NoError(err)
}

func withdraw(r *require.Assertions, handler *contractStakingEventHandler, token int64) {
	err := handler.handleWithdrawalEvent(eventParam{
		"tokenId": big.NewInt(token),
	})
	r.NoError(err)
}

func expandBucketType(r *require.Assertions, handler *contractStakingEventHandler, token, amount, duration int64) {
	err := handler.handleBucketExpandedEvent(eventParam{
		"tokenId":  big.NewInt(token),
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	})
	r.NoError(err)
}

func transfer(r *require.Assertions, handler *contractStakingEventHandler, owner address.Address, token int64) {
	err := handler.handleTransferEvent(eventParam{
		"to":      common.BytesToAddress(owner.Bytes()),
		"tokenId": big.NewInt(token),
	})
	r.NoError(err)
}

func changeDelegate(r *require.Assertions, handler *contractStakingEventHandler, delegate address.Address, token int64) {
	err := handler.handleDelegateChangedEvent(eventParam{
		"newDelegate": common.BytesToAddress(delegate.Bytes()),
		"tokenId":     big.NewInt(token),
	})
	r.NoError(err)
}

func mergeBuckets(r *require.Assertions, handler *contractStakingEventHandler, tokenIds []int64, amount, duration int64) {
	tokens := make([]*big.Int, len(tokenIds))
	for i, token := range tokenIds {
		tokens[i] = big.NewInt(token)
	}
	err := handler.handleMergedEvent(eventParam{
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
		"tokenIds": tokens,
	})
	r.NoError(err)
}
