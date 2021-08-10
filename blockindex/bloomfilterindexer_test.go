// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"hash/fnv"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
)

var (
	data1 = hash.Hash256b([]byte("Deposit"))
	data2 = hash.Hash256b([]byte("Withdraw"))
)

func newTestLog(addr string, topics []hash.Hash256) *action.Log {
	return &action.Log{
		Address:     addr,
		Topics:      topics,
		Data:        []byte("cd07d8a74179e032f030d9244"),
		BlockHeight: 1,
		ActionHash:  hash.ZeroHash256,
		Index:       1,
	}
}

func getTestLogBlocks(t *testing.T) []*block.Block {
	amount := uint64(50 << 22)
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	// create testing executions
	execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution2, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(29), 2, big.NewInt(0), 0, big.NewInt(0), nil)
	require.NoError(t, err)

	testLog1 := newTestLog(identityset.Address(28).String(), []hash.Hash256{data1})
	receipt1 := &action.Receipt{}
	receipt1.AddLogs(testLog1)

	testLog2 := newTestLog(identityset.Address(28).String(), []hash.Hash256{data2})
	receipt2 := &action.Receipt{}
	receipt2.AddLogs(testLog1, testLog2)

	testLog3 := newTestLog(identityset.Address(18).String(), []hash.Hash256{data1})
	receipt3 := &action.Receipt{}
	receipt3.AddLogs(testLog3)

	testLog4 := newTestLog(identityset.Address(18).String(), []hash.Hash256{data2})
	receipt4 := &action.Receipt{}
	receipt4.AddLogs(testLog4)

	testLog5 := newTestLog(identityset.Address(28).String(), []hash.Hash256{data1, data2})
	receipt5 := &action.Receipt{}
	receipt5.AddLogs(testLog5)

	hash1 := hash.Hash256{}
	fnv.New32().Sum(hash1[:])
	blk1, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash1).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2, execution1).
		SetReceipts([]*action.Receipt{receipt1}).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash2 := hash.Hash256{}
	fnv.New32().Sum(hash2[:])
	blk2, err := block.NewTestingBuilder().
		SetHeight(2).
		SetPrevBlockHash(hash2).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, execution2).
		SetReceipts([]*action.Receipt{receipt2}).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash3 := hash.Hash256{}
	fnv.New32().Sum(hash3[:])
	blk3, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash3).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf2, execution2).
		SetReceipts([]*action.Receipt{receipt3}).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash4 := hash.Hash256{}
	fnv.New32().Sum(hash4[:])
	blk4, err := block.NewTestingBuilder().
		SetHeight(4).
		SetPrevBlockHash(hash4).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf2, execution1, execution2).
		SetReceipts([]*action.Receipt{receipt4}).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash5 := hash.Hash256{}
	fnv.New32().Sum(hash5[:])
	blk5, err := block.NewTestingBuilder().
		SetHeight(5).
		SetPrevBlockHash(hash5).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, execution1).
		SetReceipts([]*action.Receipt{receipt5}).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	return []*block.Block{&blk1, &blk2, &blk3, &blk4, &blk5}
}

func TestBloomfilterIndexer(t *testing.T) {
	require := require.New(t)

	blks := getTestLogBlocks(t)

	testFilter := []*iotexapi.LogsFilter{
		{
			Address: []string{},
			Topics:  []*iotexapi.Topics{},
		},
		{
			Address: []string{identityset.Address(28).String()},
			Topics: []*iotexapi.Topics{
				{
					Topic: [][]byte{
						data1[:],
						data2[:],
					},
				},
				nil,
			},
		},
		{
			Address: []string{identityset.Address(18).String()},
			Topics: []*iotexapi.Topics{
				{
					Topic: [][]byte{
						data1[:],
					},
				},
				nil,
			},
		},
		{
			Address: []string{identityset.Address(28).String()},
			Topics: []*iotexapi.Topics{
				nil,
				{
					Topic: [][]byte{
						data2[:],
					},
				},
			},
		},
	}

	expectedRes := []bool{
		false,
		false,
		true,
		false,
		false,
	}

	expectedCount := []uint64{
		4,
		12,
		16,
		4,
		10,
	}

	expectedRes2 := [][]uint64{
		[]uint64{1, 2, 3, 4, 5},
		[]uint64{1, 2, 5},
		[]uint64{3},
		[]uint64{5},
	}

	expectedRes3 := [][]uint64{
		[]uint64{4, 5},
		[]uint64{5},
		[]uint64{},
		[]uint64{5},
	}

	expectedRes4 := [][]uint64{
		[]uint64{1, 2, 3},
		[]uint64{1, 2},
		[]uint64{3},
		[]uint64{},
	}

	testIndexer := func(kvStore db.KVStore, t *testing.T) {
		ctx := context.Background()
		cfg := config.Default.Indexer
		cfg.RangeBloomFilterNumElements = 16
		cfg.RangeBloomFilterSize = 4096
		cfg.RangeBloomFilterNumHash = 4

		indexer, err := NewBloomfilterIndexer(kvStore, cfg)
		require.NoError(err)
		require.NoError(indexer.Start(ctx))
		defer func() {
			require.NoError(indexer.Stop(ctx))
		}()

		require.Equal(cfg.RangeBloomFilterNumElements, indexer.RangeBloomFilterNumElements())

		height, err := indexer.Height()
		require.NoError(err)
		require.EqualValues(0, height)

		testinglf := logfilter.NewLogFilter(testFilter[2], nil, nil)

		for i := 0; i < len(blks); i++ {
			require.NoError(indexer.PutBlock(context.Background(), blks[i]))
			height, err := indexer.Height()
			require.NoError(err)
			require.Equal(blks[i].Height(), height)

			blockLevelbf, err := indexer.BlockFilterByHeight(blks[i].Height())
			require.NoError(err)
			require.Equal(expectedRes[i], testinglf.ExistInBloomFilterv2(blockLevelbf))

			rangeLevelBf, err := indexer.RangeFilterByHeight(blks[i].Height())
			require.NoError(err)
			require.Equal(cfg.RangeBloomFilterSize, rangeLevelBf.Size())
			require.Equal(cfg.RangeBloomFilterNumHash, rangeLevelBf.NumHash())
			require.Equal(expectedCount[i], rangeLevelBf.NumElements())
		}

		for i, l := range testFilter {
			lf := logfilter.NewLogFilter(l, nil, nil)

			res, err := indexer.FilterBlocksInRange(lf, 1, 5)
			require.NoError(err)
			require.Equal(expectedRes2[i], res)

			res, err = indexer.FilterBlocksInRange(lf, 4, 5)
			require.NoError(err)
			require.Equal(expectedRes3[i], res)

			res, err = indexer.FilterBlocksInRange(lf, 1, 3)
			require.NoError(err)
			require.Equal(expectedRes4[i], res)
		}

		bfs, err := indexer.(*bloomfilterIndexer).getRangeFilters(1, 5)
		require.NoError(err)
		require.Equal(2, len(bfs))
		require.EqualValues(1, bfs[0].Start())
		require.EqualValues(3, bfs[0].End())
		require.EqualValues(4, bfs[1].Start())
		require.EqualValues(5, bfs[1].End())
	}

	path := "test-indexer"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)
	cfg := db.DefaultConfig
	cfg.DbPath = testPath

	t.Run("Bolt DB indexer", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer testutil.CleanupPath(t, testPath)
		testIndexer(db.NewBoltDB(cfg), t)
	})
}
