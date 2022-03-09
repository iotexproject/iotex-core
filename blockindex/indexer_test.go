// Copyright (c) 2019 IoTeX Foundation
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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func getTestBlocks(t *testing.T) []*block.Block {
	amount := uint64(50 << 22)
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf3, err := action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf4, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(28), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf5, err := action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(29), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf6, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), 4, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	// create testing executions
	execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution2, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(29), 2, big.NewInt(0), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution3, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 3, big.NewInt(2), 0, big.NewInt(0), nil)
	require.NoError(t, err)

	hash1 := hash.Hash256{}
	fnv.New32().Sum(hash1[:])
	blk1, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash1).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf4, execution1).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash2 := hash.Hash256{}
	fnv.New32().Sum(hash2[:])
	blk2, err := block.NewTestingBuilder().
		SetHeight(2).
		SetPrevBlockHash(hash2).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf2, tsf5, execution2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash3 := hash.Hash256{}
	fnv.New32().Sum(hash3[:])
	blk3, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash3).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf3, tsf6, execution3).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	return []*block.Block{&blk1, &blk2, &blk3}
}

func TestIndexer(t *testing.T) {
	require := require.New(t)

	blks := getTestBlocks(t)
	t1Hash, _ := blks[0].Actions[0].Hash()
	t4Hash, _ := blks[0].Actions[1].Hash()
	e1Hash, _ := blks[0].Actions[2].Hash()
	t2Hash, _ := blks[1].Actions[0].Hash()
	t5Hash, _ := blks[1].Actions[1].Hash()
	e2Hash, _ := blks[1].Actions[2].Hash()
	t3Hash, _ := blks[2].Actions[0].Hash()
	t6Hash, _ := blks[2].Actions[1].Hash()
	e3Hash, _ := blks[2].Actions[2].Hash()

	addr28 := hash.BytesToHash160(identityset.Address(28).Bytes())
	addr29 := hash.BytesToHash160(identityset.Address(29).Bytes())
	addr30 := hash.BytesToHash160(identityset.Address(30).Bytes())
	addr31 := hash.BytesToHash160(identityset.Address(31).Bytes())

	type index struct {
		addr   hash.Hash160
		hashes [][]byte
	}

	indexTests := []struct {
		total     uint64
		hashTotal [][]byte
		actions   [4]index
	}{
		{
			9,
			[][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t2Hash[:], t5Hash[:], e2Hash[:], t3Hash[:], t6Hash[:], e3Hash[:]},
			[4]index{
				{addr28, [][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t6Hash[:]}},
				{addr29, [][]byte{t4Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]}},
				{addr30, [][]byte{t5Hash[:], t3Hash[:], t6Hash[:], e3Hash[:]}},
				{addr31, [][]byte{e1Hash[:], e2Hash[:], e3Hash[:]}},
			},
		},
		{
			6,
			[][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]},
			[4]index{
				{addr28, [][]byte{t1Hash[:], t4Hash[:], e1Hash[:]}},
				{addr29, [][]byte{t4Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]}},
				{addr30, [][]byte{t5Hash[:]}},
				{addr31, [][]byte{e1Hash[:], e2Hash[:]}},
			},
		},
		{
			3,
			[][]byte{t1Hash[:], t4Hash[:], e1Hash[:]},
			[4]index{
				{addr28, [][]byte{t1Hash[:], t4Hash[:], e1Hash[:]}},
				{addr29, [][]byte{t4Hash[:]}},
				{addr30, nil},
				{addr31, [][]byte{e1Hash[:]}},
			},
		},
		{
			0,
			nil,
			[4]index{
				{addr28, nil},
				{addr29, nil},
				{addr30, nil},
				{addr31, nil},
			},
		},
	}

	testIndexer := func(kvStore db.KVStore, t *testing.T) {
		ctx := context.Background()
		indexer, err := NewIndexer(kvStore, hash.ZeroHash256)
		require.NoError(err)
		require.NoError(indexer.Start(ctx))
		defer func() {
			require.NoError(indexer.Stop(ctx))
		}()

		height, err := indexer.Height()
		require.NoError(err)
		require.EqualValues(0, height)

		require.NoError(indexer.PutBlock(context.Background(), blks[0]))
		// cannot skip block when indexing
		err = indexer.PutBlock(context.Background(), blks[2])
		require.Equal(db.ErrInvalid, errors.Cause(err))
		require.NoError(indexer.PutBlock(context.Background(), blks[1]))
		height, err = indexer.Height()
		require.NoError(err)
		require.EqualValues(2, height)
		total, err := indexer.GetTotalActions()
		require.NoError(err)
		require.EqualValues(6, total)

		require.NoError(indexer.PutBlock(context.Background(), blks[2]))
		height, err = indexer.Height()
		require.NoError(err)
		require.EqualValues(3, height)

		// test block index
		for i := 0; i < 3; i++ {
			h, err := indexer.GetBlockHash(blks[i].Height())
			require.NoError(err)
			require.Equal(blks[i].HashBlock(), h)
			height, err := indexer.GetBlockHeight(h)
			require.NoError(err)
			require.Equal(blks[i].Height(), height)
			bd, err := indexer.GetBlockIndex(blks[i].Height())
			require.NoError(err)
			require.Equal(h[:], bd.Hash())
			require.EqualValues(len(blks[i].Actions), bd.NumAction())

			// test amount
			amount := big.NewInt(0)
			tsfs, _ := action.ClassifyActions(blks[i].Actions)
			for _, tsf := range tsfs {
				amount.Add(amount, tsf.Amount())
			}
			require.Equal(amount, bd.TsfAmount())

			// Test GetActionIndex
			for j := 0; j < 3; j++ {
				actIndex, err := indexer.GetActionIndex(indexTests[0].hashTotal[i*3+j])
				require.NoError(err)
				require.Equal(blks[i].Height(), actIndex.blkHeight)
			}
		}

		// non-existing address has 0 actions
		actionCount, err := indexer.GetActionCountByAddress(hash.BytesToHash160(identityset.Address(13).Bytes()))
		require.NoError(err)
		require.EqualValues(0, actionCount)

		// Test get actions
		total, err = indexer.GetTotalActions()
		require.NoError(err)
		require.EqualValues(indexTests[0].total, total)
		_, err = indexer.GetActionHashFromIndex(1, total)
		require.Equal(db.ErrInvalid, errors.Cause(err))
		actions, err := indexer.GetActionHashFromIndex(0, total)
		require.NoError(err)
		require.Equal(actions, indexTests[0].hashTotal)
		for i := range indexTests[0].actions {
			actionCount, err := indexer.GetActionCountByAddress(indexTests[0].actions[i].addr)
			require.NoError(err)
			require.EqualValues(len(indexTests[0].actions[i].hashes), actionCount)
			if actionCount > 0 {
				actions, err := indexer.GetActionsByAddress(indexTests[0].actions[i].addr, 0, actionCount)
				require.NoError(err)
				require.Equal(actions, indexTests[0].actions[i].hashes)
			}
		}
	}

	testDelete := func(kvStore db.KVStore, t *testing.T) {
		ctx := context.Background()
		indexer, err := NewIndexer(kvStore, hash.ZeroHash256)
		require.NoError(err)
		require.NoError(indexer.Start(ctx))
		defer func() {
			require.NoError(indexer.Stop(ctx))
		}()

		for i := 0; i < 3; i++ {
			require.NoError(indexer.PutBlock(context.Background(), blks[i]))
		}

		for i := range indexTests[0].actions {
			actionCount, err := indexer.GetActionCountByAddress(indexTests[0].actions[i].addr)
			require.NoError(err)
			require.EqualValues(len(indexTests[0].actions[i].hashes), actionCount)
		}

		// delete tip block one by one, verify address/action after each deletion
		for i := range indexTests {
			if i == 0 {
				// tests[0] is the whole address/action data at block height 3
				continue
			}

			require.NoError(indexer.DeleteTipBlock(blks[3-i]))
			tipHeight, err := indexer.Height()
			require.NoError(err)
			require.EqualValues(uint64(3-i), tipHeight)
			h, err := indexer.GetBlockHash(tipHeight)
			require.NoError(err)
			if i <= 2 {
				require.Equal(blks[2-i].HashBlock(), h)
			} else {
				require.Equal(hash.ZeroHash256, h)
			}

			total, err := indexer.GetTotalActions()
			require.NoError(err)
			require.EqualValues(indexTests[i].total, total)
			if total > 0 {
				_, err = indexer.GetActionHashFromIndex(1, total)
				require.Equal(db.ErrInvalid, errors.Cause(err))
				actions, err := indexer.GetActionHashFromIndex(0, total)
				require.NoError(err)
				require.Equal(actions, indexTests[i].hashTotal)
			}
			for j := range indexTests[i].actions {
				actionCount, err := indexer.GetActionCountByAddress(indexTests[i].actions[j].addr)
				require.NoError(err)
				require.EqualValues(len(indexTests[i].actions[j].hashes), actionCount)
				if actionCount > 0 {
					actions, err := indexer.GetActionsByAddress(indexTests[i].actions[j].addr, 0, actionCount)
					require.NoError(err)
					require.Equal(actions, indexTests[i].actions[j].hashes)
				}
			}
		}

		tipHeight, err := indexer.Height()
		require.NoError(err)
		require.EqualValues(0, tipHeight)
		total, err := indexer.GetTotalActions()
		require.NoError(err)
		require.EqualValues(0, total)
	}

	t.Run("In-memory KV indexer", func(t *testing.T) {
		testIndexer(db.NewMemKVStore(), t)
	})
	path := "test-indexer"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)
	defer testutil.CleanupPath(testPath)
	cfg := db.DefaultConfig
	cfg.DbPath = testPath

	t.Run("Bolt DB indexer", func(t *testing.T) {
		testutil.CleanupPath(testPath)
		defer testutil.CleanupPath(testPath)
		testIndexer(db.NewBoltDB(cfg), t)
	})

	t.Run("In-memory KV delete", func(t *testing.T) {
		testDelete(db.NewMemKVStore(), t)
	})
	t.Run("Bolt DB delete", func(t *testing.T) {
		testutil.CleanupPath(testPath)
		defer testutil.CleanupPath(testPath)
		testDelete(db.NewBoltDB(cfg), t)
	})
}
