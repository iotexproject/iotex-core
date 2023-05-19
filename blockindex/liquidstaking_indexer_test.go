// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestContractStakingIndexerLoadCache(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer := &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
	r.NoError(indexer.Start(context.Background()))

	// create a stake
	dirty := newContractStakingDirty(indexer.cache)
	height := uint64(1)
	dirty.putHeight(height)
	activateBucketType(r, dirty, 10, 100, height)
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	stake(r, dirty, owner, delegate, 1, 10, 100, height)
	err = indexer.commit(dirty)
	r.NoError(err)
	buckets, err := indexer.Buckets()
	r.NoError(err)
	r.NoError(indexer.Stop(context.Background()))

	// load cache from db
	newIndexer := &contractStakingIndexer{
		kvstore: db.NewBoltDB(cfg),
		cache:   newContractStakingCache(),
	}
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
	indexer := &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
	r.NoError(indexer.Start(context.Background()))

	// before commit dirty, the cache should be empty
	dirty := newContractStakingDirty(indexer.cache)
	height := uint64(1)
	dirty.putHeight(height)
	gotHeight, err := indexer.Height()
	r.NoError(err)
	r.EqualValues(0, gotHeight)
	// after commit dirty, the cache should be updated
	err = indexer.commit(dirty)
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
	indexer := &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
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
		dirty := newContractStakingDirty(indexer.cache)
		activateBucketType(r, dirty, 10, 100, 1)
		r.NoError(indexer.commit(dirty))
		for i := 2; i < 1000; i++ {
			dirty := newContractStakingDirty(indexer.cache)
			height := uint64(i)
			dirty.putHeight(height)
			stake(r, dirty, owner, delegate, 1, 10, 100, height)
			err := indexer.commit(dirty)
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
	indexer := &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
	r.NoError(indexer.Start(context.Background()))

	// activate
	bucketTypeData := [][2]int64{
		{10, 10},
		{20, 10},
		{10, 100},
		{20, 100},
	}
	existInBucketTypeData := func(bt *ContractStakingBucketType) int {
		idx := slices.IndexFunc(bucketTypeData, func(data [2]int64) bool {
			if bt.Amount.Int64() == data[0] && int64(bt.Duration) == data[1] {
				return true
			}
			return false
		})
		return idx
	}

	height := uint64(1)
	dirty := newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	for _, data := range bucketTypeData {
		activateBucketType(r, dirty, data[0], data[1], height)
	}
	err = indexer.commit(dirty)
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
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	for i := 0; i < 2; i++ {
		data := bucketTypeData[i]
		deactivateBucketType(r, dirty, data[0], data[1], height)
	}
	err = indexer.commit(dirty)
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
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	for i := 0; i < 2; i++ {
		data := bucketTypeData[i]
		activateBucketType(r, dirty, data[0], data[1], height)
	}
	err = indexer.commit(dirty)
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
	indexer := &contractStakingIndexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	bucketTypeData := [][2]int64{
		{10, 10},
		{20, 10},
		{10, 100},
		{20, 100},
	}
	height := uint64(1)
	dirty := newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	for _, data := range bucketTypeData {
		activateBucketType(r, dirty, data[0], data[1], height)
	}
	err = indexer.commit(dirty)
	r.NoError(err)

	// stake
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	height++
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	stake(r, dirty, owner, delegate, 1, 10, 100, height)
	r.NoError(err)
	r.NoError(indexer.commit(dirty))
	bucket, err := indexer.Bucket(1)
	r.NoError(err)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(100, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.True(bucket.AutoStake)
	r.EqualValues(height, bucket.CreateBlockHeight)
	r.EqualValues(maxBlockNumber, bucket.UnstakeStartBlockHeight)
	r.EqualValues(StakingContractAddress, bucket.ContractAddress)
	r.EqualValues(10, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// unlock
	height++
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	unlock(r, dirty, int64(bucket.Index), height)
	r.NoError(indexer.commit(dirty))
	bucket, err = indexer.Bucket(bucket.Index)
	r.NoError(err)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(100, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.False(bucket.AutoStake)
	r.EqualValues(height-1, bucket.CreateBlockHeight)
	r.EqualValues(maxBlockNumber, bucket.UnstakeStartBlockHeight)
	r.EqualValues(StakingContractAddress, bucket.ContractAddress)
	r.EqualValues(10, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// lock again
	height++
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	lock(r, dirty, int64(bucket.Index), int64(10))
	r.NoError(indexer.commit(dirty))
	bucket, err = indexer.Bucket(bucket.Index)
	r.NoError(err)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(10, bucket.StakedDurationBlockNumber)
	r.EqualValues(height-2, bucket.StakeStartBlockHeight)
	r.True(bucket.AutoStake)
	r.EqualValues(height-2, bucket.CreateBlockHeight)
	r.EqualValues(maxBlockNumber, bucket.UnstakeStartBlockHeight)
	r.EqualValues(StakingContractAddress, bucket.ContractAddress)
	r.EqualValues(10, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// unstake
	height++
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	unlock(r, dirty, int64(bucket.Index), height)
	unstake(r, dirty, int64(bucket.Index), height)
	r.NoError(indexer.commit(dirty))
	bucket, err = indexer.Bucket(bucket.Index)
	r.NoError(err)
	r.EqualValues(1, bucket.Index)
	r.EqualValues(owner, bucket.Owner)
	r.EqualValues(delegate, bucket.Candidate)
	r.EqualValues(10, bucket.StakedAmount.Int64())
	r.EqualValues(10, bucket.StakedDurationBlockNumber)
	r.EqualValues(height, bucket.StakeStartBlockHeight)
	r.False(bucket.AutoStake)
	r.EqualValues(height-3, bucket.CreateBlockHeight)
	r.EqualValues(height, bucket.UnstakeStartBlockHeight)
	r.EqualValues(StakingContractAddress, bucket.ContractAddress)
	r.EqualValues(0, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())

	// withdraw
	height++
	dirty = newContractStakingDirty(indexer.cache)
	dirty.putHeight(height)
	withdraw(r, dirty, int64(bucket.Index))
	r.NoError(indexer.commit(dirty))
	bucket, err = indexer.Bucket(bucket.Index)
	r.ErrorIs(err, ErrBucketInfoNotExist)
	r.EqualValues(0, indexer.CandidateVotes(delegate).Uint64())
	r.EqualValues(1, indexer.TotalBucketCount())
}

func activateBucketType(r *require.Assertions, dirty *contractStakingDirty, amount, duration int64, height uint64) {
	err := dirty.handleBucketTypeActivatedEvent(eventParam{
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	}, height)
	r.NoError(err)
}

func deactivateBucketType(r *require.Assertions, dirty *contractStakingDirty, amount, duration int64, height uint64) {
	err := dirty.handleBucketTypeDeactivatedEvent(eventParam{
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	}, height)
	r.NoError(err)
}

func stake(r *require.Assertions, dirty *contractStakingDirty, owner, candidate address.Address, token, amount, duration int64, height uint64) {
	err := dirty.handleTransferEvent(eventParam{
		"to":      common.BytesToAddress(owner.Bytes()),
		"tokenId": big.NewInt(token),
	})
	r.NoError(err)
	err = dirty.handleStakedEvent(eventParam{
		"tokenId":  big.NewInt(token),
		"delegate": common.BytesToAddress(candidate.Bytes()),
		"amount":   big.NewInt(amount),
		"duration": big.NewInt(duration),
	}, height)
	r.NoError(err)
}

func unlock(r *require.Assertions, dirty *contractStakingDirty, token int64, height uint64) {
	err := dirty.handleUnlockedEvent(eventParam{
		"tokenId": big.NewInt(token),
	}, height)
	r.NoError(err)
}

func lock(r *require.Assertions, dirty *contractStakingDirty, token, duration int64) {
	err := dirty.handleLockedEvent(eventParam{
		"tokenId":  big.NewInt(token),
		"duration": big.NewInt(duration),
	})
	r.NoError(err)
}

func unstake(r *require.Assertions, dirty *contractStakingDirty, token int64, height uint64) {
	err := dirty.handleUnstakedEvent(eventParam{
		"tokenId": big.NewInt(token),
	}, height)
	r.NoError(err)
}

func withdraw(r *require.Assertions, dirty *contractStakingDirty, token int64) {
	err := dirty.handleWithdrawalEvent(eventParam{
		"tokenId": big.NewInt(token),
	})
	r.NoError(err)
}
