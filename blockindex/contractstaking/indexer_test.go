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

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

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
	newIndexer, err := NewContractStakingIndexer(db.NewBoltDB(cfg), _testStakingContractAddress, 0)
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
			stake(r, handler, owner, delegate, 1, 10, 100, height)
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

	// unlock
	height++
	handler = newContractStakingEventHandler(indexer.cache, height)
	unlock(r, handler, int64(bucket.Index), height)
	r.NoError(indexer.commit(handler))
	bucket, ok = indexer.Bucket(bucket.Index)
	r.True(ok)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(100, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.False(bucket.AutoStake)
	r.EqualValues(height-1, bucket.CreateBlockHeight)
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
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(10, bucket.StakedDurationBlockNumber)
	r.EqualValues(height-2, bucket.StakeStartBlockHeight)
	r.True(bucket.AutoStake)
	r.EqualValues(height-2, bucket.CreateBlockHeight)
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
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(10, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.False(bucket.AutoStake)
	r.EqualValues(height-3, bucket.CreateBlockHeight)
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
