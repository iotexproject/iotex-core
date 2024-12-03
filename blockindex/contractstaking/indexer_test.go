// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"cmp"
	"context"
	"math/big"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/slices"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

const (
	_testStakingContractAddress = "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd"
)

var (
	_blockInterval = consensusfsm.DefaultDardanellesUpgradeConfig.BlockInterval
)

func TestNewContractStakingIndexer(t *testing.T) {
	r := require.New(t)

	t.Run("kvStore is nil", func(t *testing.T) {
		_, err := NewContractStakingIndexer(nil, Config{
			ContractAddress:      "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd",
			ContractDeployHeight: 0,
			CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
			BlockInterval:        _blockInterval,
		})
		r.Error(err)
		r.Contains(err.Error(), "kv store is nil")
	})

	t.Run("invalid contract address", func(t *testing.T) {
		kvStore := db.NewMemKVStore()
		_, err := NewContractStakingIndexer(kvStore, Config{
			ContractAddress:      "invalid address",
			ContractDeployHeight: 0,
			CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
			BlockInterval:        _blockInterval,
		})
		r.Error(err)
		r.Contains(err.Error(), "invalid contract address")
	})

	t.Run("valid input", func(t *testing.T) {
		contractAddr, err := address.FromString("io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd")
		r.NoError(err)
		indexer, err := NewContractStakingIndexer(db.NewMemKVStore(), Config{
			ContractAddress:      contractAddr.String(),
			ContractDeployHeight: 0,
			CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
			BlockInterval:        _blockInterval,
		})
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// create a stake
	height := uint64(1)
	startHeight := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache)
	activateBucketType(r, handler, 10, 100, height)
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	stake(r, handler, owner, delegate, 1, 10, 100, height)
	err = indexer.commit(handler, height)
	r.NoError(err)
	buckets, err := indexer.Buckets(height)
	r.NoError(err)
	r.EqualValues(1, len(buckets))
	tbc, err := indexer.TotalBucketCount(height)
	r.EqualValues(1, tbc)
	r.NoError(err)
	h, err := indexer.Height()
	r.NoError(err)
	r.EqualValues(height, h)

	r.NoError(indexer.Stop(context.Background()))

	// load cache from db
	newIndexer, err := NewContractStakingIndexer(db.NewBoltDB(cfg), Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: startHeight,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
	r.NoError(err)
	r.NoError(newIndexer.Start(context.Background()))

	// check cache
	newBuckets, err := newIndexer.Buckets(height)
	r.NoError(err)
	r.Equal(len(buckets), len(newBuckets))
	for i := range buckets {
		r.EqualValues(buckets[i], newBuckets[i])
	}
	newHeight, err := newIndexer.Height()
	r.NoError(err)
	r.Equal(height, newHeight)
	tbc, err = newIndexer.TotalBucketCount(height)
	r.EqualValues(1, tbc)
	r.NoError(err)
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// before commit dirty, the cache should be empty
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache)
	gotHeight, err := indexer.Height()
	r.NoError(err)
	r.EqualValues(0, gotHeight)
	// after commit dirty, the cache should be updated
	err = indexer.commit(handler, height)
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	wait := sync.WaitGroup{}
	wait.Add(6)
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), genesis.Default), protocol.BlockCtx{BlockHeight: 1}))
	// read concurrently
	for i := 0; i < 5; i++ {
		go func() {
			defer wait.Done()
			for i := 0; i < 1000; i++ {
				_, err := indexer.Buckets(0)
				r.NoError(err)
				_, err = indexer.BucketTypes(0)
				r.NoError(err)
				_, err = indexer.BucketsByCandidate(delegate, 0)
				r.NoError(err)
				indexer.CandidateVotes(ctx, delegate, 0)
				_, err = indexer.Height()
				r.NoError(err)
				indexer.TotalBucketCount(0)
			}
		}()
	}
	// write
	go func() {
		defer wait.Done()
		// activate bucket type
		handler := newContractStakingEventHandler(indexer.cache)
		activateBucketType(r, handler, 10, 100, 1)
		r.NoError(indexer.commit(handler, 1))
		for i := 2; i < 1000; i++ {
			height := uint64(i)
			handler := newContractStakingEventHandler(indexer.cache)
			stake(r, handler, owner, delegate, int64(i), 10, 100, height)
			err := indexer.commit(handler, height)
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
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
	handler := newContractStakingEventHandler(indexer.cache)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler, height)
	r.NoError(err)
	bucketTypes, err := indexer.BucketTypes(height)
	r.NoError(err)
	r.Equal(len(bucketTypeData), len(bucketTypes))
	for _, bt := range bucketTypes {
		r.True(existInBucketTypeData(bt) >= 0)
		r.EqualValues(height, bt.ActivatedAt)
	}
	// deactivate
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	for i := 0; i < 2; i++ {
		data := bucketTypeData[i]
		deactivateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler, height)
	r.NoError(err)
	bucketTypes, err = indexer.BucketTypes(height)
	r.NoError(err)
	r.Equal(len(bucketTypeData)-2, len(bucketTypes))
	for _, bt := range bucketTypes {
		r.True(existInBucketTypeData(bt) >= 0)
		r.EqualValues(1, bt.ActivatedAt)
	}
	// reactivate
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	for i := 0; i < 2; i++ {
		data := bucketTypeData[i]
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler, height)
	r.NoError(err)
	bucketTypes, err = indexer.BucketTypes(height)
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
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
	handler := newContractStakingEventHandler(indexer.cache)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler, height)
	r.NoError(err)
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), genesis.Default), protocol.BlockCtx{BlockHeight: 1}))

	// stake
	owner := identityset.Address(0)
	delegate := identityset.Address(1)
	height++
	createHeight := height
	handler = newContractStakingEventHandler(indexer.cache)
	stake(r, handler, owner, delegate, 1, 10, 100, height)
	r.NoError(err)
	r.NoError(indexer.commit(handler, height))
	bucket, ok, err := indexer.Bucket(1, height)
	r.NoError(err)
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
	votes, err := indexer.CandidateVotes(ctx, delegate, height)
	r.NoError(err)
	r.EqualValues(10, votes.Uint64())
	tbc, err := indexer.TotalBucketCount(height)
	r.NoError(err)
	r.EqualValues(1, tbc)

	// transfer
	newOwner := identityset.Address(2)
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	transfer(r, handler, newOwner, int64(bucket.Index))
	r.NoError(indexer.commit(handler, height))
	bucket, ok, err = indexer.Bucket(bucket.Index, height)
	r.NoError(err)
	r.True(ok)
	r.EqualValues(newOwner, bucket.Owner)

	// unlock
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	unlock(r, handler, int64(bucket.Index), height)
	r.NoError(indexer.commit(handler, height))
	bucket, ok, err = indexer.Bucket(bucket.Index, height)
	r.NoError(err)
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
	votes, err = indexer.CandidateVotes(ctx, delegate, height)
	r.NoError(err)
	r.EqualValues(10, votes.Uint64())
	tbc, err = indexer.TotalBucketCount(height)
	r.NoError(err)
	r.EqualValues(1, tbc)

	// lock again
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	lock(r, handler, int64(bucket.Index), int64(10))
	r.NoError(indexer.commit(handler, height))
	bucket, ok, err = indexer.Bucket(bucket.Index, height)
	r.NoError(err)
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
	votes, err = indexer.CandidateVotes(ctx, delegate, height)
	r.NoError(err)
	r.EqualValues(10, votes.Uint64())
	tbc, err = indexer.TotalBucketCount(height)
	r.NoError(err)
	r.EqualValues(1, tbc)

	// unstake
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	unlock(r, handler, int64(bucket.Index), height)
	unstake(r, handler, int64(bucket.Index), height)
	r.NoError(indexer.commit(handler, height))
	bucket, ok, err = indexer.Bucket(bucket.Index, height)
	r.NoError(err)
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
	votes, err = indexer.CandidateVotes(ctx, delegate, height)
	r.NoError(err)
	r.EqualValues(0, votes.Uint64())
	tbc, err = indexer.TotalBucketCount(height)
	r.NoError(err)
	r.EqualValues(1, tbc)

	// withdraw
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	withdraw(r, handler, int64(bucket.Index))
	r.NoError(indexer.commit(handler, height))
	bucket, ok, err = indexer.Bucket(bucket.Index, height)
	r.NoError(err)
	r.False(ok)
	votes, err = indexer.CandidateVotes(ctx, delegate, height)
	r.NoError(err)
	r.EqualValues(0, votes.Uint64())
	tbc, err = indexer.TotalBucketCount(height)
	r.NoError(err)
	r.EqualValues(1, tbc)
}

func TestContractStakingIndexerChangeBucketType(t *testing.T) {
	r := require.New(t)
	testDBPath, err := testutil.PathOfTempFile("staking.db")
	r.NoError(err)
	defer testutil.CleanupPath(testDBPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testDBPath
	kvStore := db.NewBoltDB(cfg)
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
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
	handler := newContractStakingEventHandler(indexer.cache)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler, height)
	r.NoError(err)

	t.Run("expand bucket type", func(t *testing.T) {
		owner := identityset.Address(0)
		delegate := identityset.Address(1)
		height++
		handler = newContractStakingEventHandler(indexer.cache)
		stake(r, handler, owner, delegate, 1, 10, 100, height)
		r.NoError(err)
		r.NoError(indexer.commit(handler, height))
		bucket, ok, err := indexer.Bucket(1, height)
		r.NoError(err)
		r.True(ok)

		expandBucketType(r, handler, int64(bucket.Index), 20, 100)
		r.NoError(indexer.commit(handler, height))
		bucket, ok, err = indexer.Bucket(bucket.Index, height)
		r.NoError(err)
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
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
	handler := newContractStakingEventHandler(indexer.cache)
	for _, data := range bucketTypeData {
		activateBucketType(r, handler, data[0], data[1], height)
	}
	err = indexer.commit(handler, height)
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
	handler = newContractStakingEventHandler(indexer.cache)
	for i, data := range stakeData {
		stake(r, handler, identityset.Address(data.owner), identityset.Address(data.delegate), int64(i), int64(data.amount), int64(data.duration), height)
	}
	r.NoError(err)
	r.NoError(indexer.commit(handler, height))

	t.Run("Buckets", func(t *testing.T) {
		buckets, err := indexer.Buckets(height)
		r.NoError(err)
		r.Len(buckets, len(stakeData))
	})

	t.Run("BucketsByCandidate", func(t *testing.T) {
		candidateMap := make(map[int]int)
		for i := range stakeData {
			candidateMap[stakeData[i].delegate]++
		}
		for cand := range candidateMap {
			buckets, err := indexer.BucketsByCandidate(identityset.Address(cand), height)
			r.NoError(err)
			r.Len(buckets, candidateMap[cand])
		}
	})

	t.Run("BucketsByIndices", func(t *testing.T) {
		indices := []uint64{0, 1, 2, 3, 4, 5, 6}
		buckets, err := indexer.BucketsByIndices(indices, height)
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
		tbc, err := indexer.TotalBucketCount(height)
		r.NoError(err)
		r.EqualValues(len(stakeData), tbc)
	})

	t.Run("CandidateVotes", func(t *testing.T) {
		ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), genesis.Default), protocol.BlockCtx{BlockHeight: 1}))
		candidateMap := make(map[int]int64)
		for i := range stakeData {
			candidateMap[stakeData[i].delegate] += int64(stakeData[i].amount)
		}
		candidates := []int{1, 2, 3}
		for _, cand := range candidates {
			votes := candidateMap[cand]
			cvotes, err := indexer.CandidateVotes(ctx, identityset.Address(cand), height)
			r.NoError(err)
			r.EqualValues(votes, cvotes.Uint64())
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))

	// init bucket type
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache)
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
	abt, err := indexer.cache.ActiveBucketTypes(height - 1)
	r.NoError(err)
	r.Len(abt, 0)
	bts, err := indexer.cache.Buckets(height - 1)
	r.NoError(err)
	r.Len(bts, 0)
	r.NoError(indexer.commit(handler, height))
	abt, err = indexer.cache.ActiveBucketTypes(height)
	r.NoError(err)
	r.Len(abt, 2)
	bts, err = indexer.cache.Buckets(height)
	r.NoError(err)
	r.Len(bts, 4)

	height++
	handler = newContractStakingEventHandler(indexer.cache)
	changeDelegate(r, handler, delegate1, 3)
	transfer(r, handler, delegate1, 1)
	bt, ok, err := indexer.Bucket(3, height-1)
	r.NoError(err)
	r.True(ok)
	r.Equal(delegate2.String(), bt.Candidate.String())
	bt, ok, err = indexer.Bucket(1, height-1)
	r.NoError(err)
	r.True(ok)
	r.Equal(owner.String(), bt.Owner.String())
	r.NoError(indexer.commit(handler, height))
	bt, ok, err = indexer.Bucket(3, height)
	r.NoError(err)
	r.True(ok)
	r.Equal(delegate1.String(), bt.Candidate.String())
	bt, ok, err = indexer.Bucket(1, height)
	r.NoError(err)
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
	indexer, err := NewContractStakingIndexer(kvStore, Config{
		ContractAddress:      _testStakingContractAddress,
		ContractDeployHeight: 0,
		CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
		BlockInterval:        _blockInterval,
	})
	r.NoError(err)
	r.NoError(indexer.Start(context.Background()))
	ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), genesis.Default), protocol.BlockCtx{BlockHeight: 1}))

	// init bucket type
	height := uint64(1)
	handler := newContractStakingEventHandler(indexer.cache)
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
	r.NoError(indexer.commit(handler, height))
	votes, err := indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(30, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(40, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, owner, height)
	r.EqualValues(0, votes.Uint64())

	// change delegate bucket 3 to delegate1
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	changeDelegate(r, handler, delegate1, 3)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	// unlock bucket 1 & 4
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	unlock(r, handler, 1, height)
	unlock(r, handler, 4, height)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	// unstake bucket 1 & lock 4
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	unstake(r, handler, 1, height)
	lock(r, handler, 4, 20)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(40, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	// expand bucket 2
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	expandBucketType(r, handler, 2, 30, 20)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	// transfer bucket 4
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	transfer(r, handler, delegate2, 4)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	// create bucket 5, 6, 7
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	stake(r, handler, owner, delegate2, 5, 20, 20, height)
	stake(r, handler, owner, delegate2, 6, 20, 20, height)
	stake(r, handler, owner, delegate2, 7, 20, 20, height)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(80, votes.Uint64())

	// merge bucket 5, 6, 7
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	mergeBuckets(r, handler, []int64{5, 6, 7}, 60, 20)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(80, votes.Uint64())

	// unlock & unstake 5
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	unlock(r, handler, 5, height)
	unstake(r, handler, 5, height)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(50, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	// create & merge bucket 8, 9, 10
	height++
	handler = newContractStakingEventHandler(indexer.cache)
	stake(r, handler, owner, delegate1, 8, 20, 20, height)
	stake(r, handler, owner, delegate2, 9, 20, 20, height)
	stake(r, handler, owner, delegate2, 10, 20, 20, height)
	mergeBuckets(r, handler, []int64{8, 9, 10}, 60, 20)
	r.NoError(indexer.commit(handler, height))
	votes, err = indexer.CandidateVotes(ctx, delegate1, height)
	r.NoError(err)
	r.EqualValues(110, votes.Uint64())
	votes, err = indexer.CandidateVotes(ctx, delegate2, height)
	r.NoError(err)
	r.EqualValues(20, votes.Uint64())

	t.Run("Height", func(t *testing.T) {
		h, err := indexer.Height()
		r.NoError(err)
		r.EqualValues(height, h)
	})

	t.Run("BucketTypes", func(t *testing.T) {
		bts, err := indexer.BucketTypes(height)
		r.NoError(err)
		r.Len(bts, 4)
		slices.SortFunc(bts, func(i, j *staking.ContractStakingBucketType) int {
			return i.Amount.Cmp(j.Amount)
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
		bts, err := indexer.Buckets(height)
		r.NoError(err)
		r.Len(bts, 6)
		slices.SortFunc(bts, func(i, j *staking.VoteBucket) int {
			return cmp.Compare(i.Index, j.Index)
		})
		r.EqualValues(1, bts[0].Index)
		r.EqualValues(2, bts[1].Index)
		r.EqualValues(3, bts[2].Index)
		r.EqualValues(4, bts[3].Index)
		r.EqualValues(5, bts[4].Index)
		r.EqualValues(8, bts[5].Index)
		r.EqualValues(10*_blockInterval, bts[0].StakedDuration)
		r.EqualValues(20*_blockInterval, bts[1].StakedDuration)
		r.EqualValues(20*_blockInterval, bts[2].StakedDuration)
		r.EqualValues(20*_blockInterval, bts[3].StakedDuration)
		r.EqualValues(20*_blockInterval, bts[4].StakedDuration)
		r.EqualValues(20*_blockInterval, bts[5].StakedDuration)
		r.EqualValues(10, bts[0].StakedDurationBlockNumber)
		r.EqualValues(20, bts[1].StakedDurationBlockNumber)
		r.EqualValues(20, bts[2].StakedDurationBlockNumber)
		r.EqualValues(20, bts[3].StakedDurationBlockNumber)
		r.EqualValues(20, bts[4].StakedDurationBlockNumber)
		r.EqualValues(20, bts[5].StakedDurationBlockNumber)
		r.EqualValues(10, bts[0].StakedAmount.Int64())
		r.EqualValues(30, bts[1].StakedAmount.Int64())
		r.EqualValues(20, bts[2].StakedAmount.Int64())
		r.EqualValues(20, bts[3].StakedAmount.Int64())
		r.EqualValues(60, bts[4].StakedAmount.Int64())
		r.EqualValues(60, bts[5].StakedAmount.Int64())
		r.EqualValues(delegate1.String(), bts[0].Candidate.String())
		r.EqualValues(delegate1.String(), bts[1].Candidate.String())
		r.EqualValues(delegate1.String(), bts[2].Candidate.String())
		r.EqualValues(delegate2.String(), bts[3].Candidate.String())
		r.EqualValues(delegate2.String(), bts[4].Candidate.String())
		r.EqualValues(delegate1.String(), bts[5].Candidate.String())
		r.EqualValues(owner.String(), bts[0].Owner.String())
		r.EqualValues(owner.String(), bts[1].Owner.String())
		r.EqualValues(owner.String(), bts[2].Owner.String())
		r.EqualValues(delegate2.String(), bts[3].Owner.String())
		r.EqualValues(owner.String(), bts[4].Owner.String())
		r.EqualValues(owner.String(), bts[5].Owner.String())
		r.False(bts[0].AutoStake)
		r.True(bts[1].AutoStake)
		r.True(bts[2].AutoStake)
		r.True(bts[3].AutoStake)
		r.False(bts[4].AutoStake)
		r.True(bts[5].AutoStake)
		r.EqualValues(1, bts[0].CreateBlockHeight)
		r.EqualValues(1, bts[1].CreateBlockHeight)
		r.EqualValues(1, bts[2].CreateBlockHeight)
		r.EqualValues(1, bts[3].CreateBlockHeight)
		r.EqualValues(7, bts[4].CreateBlockHeight)
		r.EqualValues(10, bts[5].CreateBlockHeight)
		r.EqualValues(3, bts[0].StakeStartBlockHeight)
		r.EqualValues(1, bts[1].StakeStartBlockHeight)
		r.EqualValues(1, bts[2].StakeStartBlockHeight)
		r.EqualValues(1, bts[3].StakeStartBlockHeight)
		r.EqualValues(9, bts[4].StakeStartBlockHeight)
		r.EqualValues(10, bts[5].StakeStartBlockHeight)
		r.EqualValues(4, bts[0].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[1].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[2].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[3].UnstakeStartBlockHeight)
		r.EqualValues(9, bts[4].UnstakeStartBlockHeight)
		r.EqualValues(maxBlockNumber, bts[5].UnstakeStartBlockHeight)
		for _, b := range bts {
			r.EqualValues(time.Time{}, b.CreateTime)
			r.EqualValues(time.Time{}, b.StakeStartTime)
			r.EqualValues(time.Time{}, b.UnstakeStartTime)
			r.EqualValues(_testStakingContractAddress, b.ContractAddress)
		}
	})

	t.Run("BucketsByCandidate", func(t *testing.T) {
		d1Bts, err := indexer.BucketsByCandidate(delegate1, height)
		r.NoError(err)
		r.Len(d1Bts, 4)
		slices.SortFunc(d1Bts, func(i, j *staking.VoteBucket) int {
			return cmp.Compare(i.Index, j.Index)
		})
		r.EqualValues(1, d1Bts[0].Index)
		r.EqualValues(2, d1Bts[1].Index)
		r.EqualValues(3, d1Bts[2].Index)
		r.EqualValues(8, d1Bts[3].Index)
		d2Bts, err := indexer.BucketsByCandidate(delegate2, height)
		r.NoError(err)
		r.Len(d2Bts, 2)
		slices.SortFunc(d2Bts, func(i, j *staking.VoteBucket) int {
			return cmp.Compare(i.Index, j.Index)
		})
		r.EqualValues(4, d2Bts[0].Index)
		r.EqualValues(5, d2Bts[1].Index)
	})

	t.Run("BucketsByIndices", func(t *testing.T) {
		bts, err := indexer.BucketsByIndices([]uint64{1, 2, 3, 4, 5, 8}, height)
		r.NoError(err)
		r.Len(bts, 6)
	})
}

func TestIndexer_ReadHeightRestriction(t *testing.T) {
	r := require.New(t)

	cases := []struct {
		startHeight uint64
		height      uint64
		readHeight  uint64
		valid       bool
	}{
		{0, 0, 0, true},
		{0, 0, 1, false},
		{0, 2, 0, true},
		{0, 2, 1, true},
		{0, 2, 2, true},
		{0, 2, 3, false},
		{10, 0, 0, true},
		{10, 0, 1, true},
		{10, 0, 9, true},
		{10, 0, 10, false},
		{10, 0, 11, false},
		{10, 10, 0, true},
		{10, 10, 1, true},
		{10, 10, 9, true},
		{10, 10, 10, true},
		{10, 10, 11, false},
	}

	for idx, c := range cases {
		name := strconv.FormatInt(int64(idx), 10)
		t.Run(name, func(t *testing.T) {
			// Create a new Indexer
			height := c.height
			startHeight := c.startHeight
			cfg := config.Default.DB
			dbPath, err := testutil.PathOfTempFile("db")
			r.NoError(err)
			cfg.DbPath = dbPath
			indexer, err := NewContractStakingIndexer(db.NewBoltDB(cfg), Config{
				ContractAddress:      identityset.Address(1).String(),
				ContractDeployHeight: startHeight,
				CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
				BlockInterval:        _blockInterval,
			})
			r.NoError(err)
			r.NoError(indexer.Start(context.Background()))
			defer func() {
				r.NoError(indexer.Stop(context.Background()))
				testutil.CleanupPath(dbPath)
			}()
			indexer.cache.putHeight(height)
			// check read api
			ctx := protocol.WithFeatureCtx(protocol.WithBlockCtx(genesis.WithGenesisContext(context.Background(), genesis.Default), protocol.BlockCtx{BlockHeight: 1}))
			h := c.readHeight
			delegate := identityset.Address(1)
			if c.valid {
				_, err = indexer.Buckets(h)
				r.NoError(err)
				_, err = indexer.BucketTypes(h)
				r.NoError(err)
				_, err = indexer.BucketsByCandidate(delegate, h)
				r.NoError(err)
				_, err = indexer.BucketsByIndices([]uint64{1, 2, 3, 4, 5, 8}, h)
				r.NoError(err)
				_, err = indexer.CandidateVotes(ctx, delegate, h)
				r.NoError(err)
				_, _, err = indexer.Bucket(1, h)
				r.NoError(err)
				_, err = indexer.TotalBucketCount(h)
				r.NoError(err)
			} else {
				_, err = indexer.Buckets(h)
				r.ErrorIs(err, ErrInvalidHeight)
				_, err = indexer.BucketTypes(h)
				r.ErrorIs(err, ErrInvalidHeight)
				_, err = indexer.BucketsByCandidate(delegate, h)
				r.ErrorIs(err, ErrInvalidHeight)
				_, err = indexer.BucketsByIndices([]uint64{1, 2, 3, 4, 5, 8}, h)
				r.ErrorIs(err, ErrInvalidHeight)
				_, err = indexer.CandidateVotes(ctx, delegate, h)
				r.ErrorIs(err, ErrInvalidHeight)
				_, _, err = indexer.Bucket(1, h)
				r.ErrorIs(err, ErrInvalidHeight)
				_, err = indexer.TotalBucketCount(h)
				r.ErrorIs(err, ErrInvalidHeight)
			}
		})
	}
}

func TestIndexer_PutBlock(t *testing.T) {
	r := require.New(t)

	cases := []struct {
		name           string
		height         uint64
		startHeight    uint64
		blockHeight    uint64
		expectedHeight uint64
		errMsg         string
	}{
		{"block < height < start", 10, 20, 9, 10, ""},
		{"block = height < start", 10, 20, 10, 10, ""},
		{"height < block < start", 10, 20, 11, 10, ""},
		{"height < block = start", 10, 20, 20, 20, ""},
		{"height < start < block", 10, 20, 21, 10, "invalid block height 21, expect 20"},
		{"block < start < height", 20, 10, 9, 20, ""},
		{"block = start < height", 20, 10, 10, 20, ""},
		{"start < block < height", 20, 10, 11, 20, ""},
		{"start < block = height", 20, 10, 20, 20, ""},
		{"start < height < block", 20, 10, 21, 21, ""},
		{"start < height < block+", 20, 10, 22, 20, "invalid block height 22, expect 21"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Create a new Indexer
			height := c.height
			startHeight := c.startHeight
			cfg := config.Default.DB
			dbPath, err := testutil.PathOfTempFile("db")
			r.NoError(err)
			cfg.DbPath = dbPath
			indexer, err := NewContractStakingIndexer(db.NewBoltDB(cfg), Config{
				ContractAddress:      identityset.Address(1).String(),
				ContractDeployHeight: startHeight,
				CalculateVoteWeight:  calculateVoteWeightGen(genesis.Default.VoteWeightCalConsts),
				BlockInterval:        _blockInterval,
			})
			r.NoError(err)
			r.NoError(indexer.Start(context.Background()))
			defer func() {
				r.NoError(indexer.Stop(context.Background()))
				testutil.CleanupPath(dbPath)
			}()
			indexer.cache.putHeight(height)
			// Create a mock block
			builder := block.NewBuilder(block.NewRunnableActionsBuilder().Build())
			builder.SetHeight(c.blockHeight)
			blk, err := builder.SignAndBuild(identityset.PrivateKey(1))
			r.NoError(err)
			// Put the block
			err = indexer.PutBlock(context.Background(), &blk)
			if c.errMsg != "" {
				r.ErrorContains(err, c.errMsg)
			} else {
				r.NoError(err)
			}
			// Check the block height
			r.EqualValues(c.expectedHeight, indexer.cache.Height())
		})
	}

}

func BenchmarkIndexer_PutBlockBeforeContractHeight(b *testing.B) {
	// Create a new Indexer with a contract height of 100
	indexer := &Indexer{config: Config{ContractDeployHeight: 100}}

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
