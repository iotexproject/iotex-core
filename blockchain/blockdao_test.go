// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestBlockDAO(t *testing.T) {

	getBlocks := func() []*block.Block {
		amount := uint64(50 << 22)
		tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(t, err)

		tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(t, err)

		tsf3, err := testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(t, err)

		tsf4, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(28), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(t, err)

		tsf5, err := testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(29), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(t, err)

		tsf6, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), 4, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
		require.NoError(t, err)

		// create testing executions
		execution1, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
		require.NoError(t, err)
		execution2, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(29), 2, big.NewInt(0), 0, big.NewInt(0), nil)
		require.NoError(t, err)
		execution3, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 3, big.NewInt(2), 0, big.NewInt(0), nil)
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

	blks := getBlocks()
	assert.Equal(t, 3, len(blks))

	testBlockDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		dao := newBlockDAO(kvstore, false, false, 0, config.Default.DB)
		err := dao.Start(ctx)
		assert.Nil(t, err)
		defer func() {
			err = dao.Stop(ctx)
			assert.Nil(t, err)
		}()

		height, err := dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), height)

		// block put order is 0 2 1
		err = dao.putBlock(blks[0])
		assert.Nil(t, err)
		blk, err := dao.getBlock(blks[0].HashBlock())
		assert.Nil(t, err)
		require.NotNil(t, blk)
		assert.Equal(t, blks[0].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), height)

		err = dao.putBlock(blks[2])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[2].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[1].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		// test getting hash by height
		hash, err := dao.getBlockHash(1)
		assert.Nil(t, err)
		assert.Equal(t, blks[0].HashBlock(), hash)

		hash, err = dao.getBlockHash(2)
		assert.Nil(t, err)
		assert.Equal(t, blks[1].HashBlock(), hash)

		hash, err = dao.getBlockHash(3)
		assert.Nil(t, err)
		assert.Equal(t, blks[2].HashBlock(), hash)

		// test getting height by hash
		height, err = dao.getBlockHeight(blks[0].HashBlock())
		assert.Nil(t, err)
		assert.Equal(t, blks[0].Height(), height)

		height, err = dao.getBlockHeight(blks[1].HashBlock())
		assert.Nil(t, err)
		assert.Equal(t, blks[1].Height(), height)

		height, err = dao.getBlockHeight(blks[2].HashBlock())
		assert.Nil(t, err)
		assert.Equal(t, blks[2].Height(), height)

		// test getTipHash
		hash, err = dao.getTipHash()
		assert.Nil(t, err)
		assert.Equal(t, blks[2].HashBlock(), hash)
	}

	testActionsDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		dao := newBlockDAO(kvstore, true, false, 0, config.Default.DB)
		err := dao.Start(ctx)
		assert.Nil(t, err)
		defer func() {
			err = dao.Stop(ctx)
			assert.Nil(t, err)
		}()

		err = dao.putBlock(blks[0])
		assert.Nil(t, err)
		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		err = dao.putBlock(blks[2])

		blkActions1 := blks[0].Actions
		blkActions2 := blks[1].Actions
		blkActions3 := blks[2].Actions

		blkNumActions1 := uint64(len(blkActions1))
		blkNumActions2 := uint64(len(blkActions2))
		blkNumActions3 := uint64(len(blkActions3))

		blkTransferAmount1, blkTransferAmount2, blkTransferAmount3 := big.NewInt(0), big.NewInt(0), big.NewInt(0)

		tsfs, _ := action.ClassifyActions(blkActions1)
		for _, tsf := range tsfs {
			blkTransferAmount1.Add(blkTransferAmount1, tsf.Amount())
		}
		tsfs, _ = action.ClassifyActions(blkActions2)
		for _, tsf := range tsfs {
			blkTransferAmount2.Add(blkTransferAmount2, tsf.Amount())
		}
		tsfs, _ = action.ClassifyActions(blkActions3)
		for _, tsf := range tsfs {
			blkTransferAmount3.Add(blkTransferAmount3, tsf.Amount())
		}

		// Test get actions
		senderActionCount, err := getActionCountBySenderAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(28).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(3), senderActionCount)
		senderActions, err := getActionsBySenderAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(28).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 3, len(senderActions))
		recipientActionCount, err := getActionCountByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(28).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientActionCount)
		recipientActions, err := getActionsByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(28).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientActions))

		senderActionCount, err = getActionCountBySenderAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(29).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(3), senderActionCount)
		senderActions, err = getActionsBySenderAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(29).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 3, len(senderActions))
		recipientActionCount, err = getActionCountByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(29).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientActionCount)
		recipientActions, err = getActionsByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(29).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientActions))

		senderActionCount, err = getActionCountBySenderAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(30).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(3), senderActionCount)
		senderActions, err = getActionsBySenderAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(30).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 3, len(senderActions))
		recipientActionCount, err = getActionCountByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(30).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(1), recipientActionCount)
		recipientActions, err = getActionsByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(30).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 1, len(recipientActions))

		recipientActionCount, err = getActionCountByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(31).Bytes()))
		require.NoError(t, err)
		require.Equal(t, uint64(3), recipientActionCount)
		recipientActions, err = getActionsByRecipientAddress(dao.kvstore, hash.BytesToHash160(identityset.Address(31).Bytes()))
		require.NoError(t, err)
		require.Equal(t, 3, len(recipientActions))

		// test getNumActions
		numActions, err := dao.getNumActions(blks[0].Height())
		require.NoError(t, err)
		require.Equal(t, blkNumActions1, numActions)
		numActions, err = dao.getNumActions(blks[1].Height())
		require.NoError(t, err)
		require.Equal(t, blkNumActions2, numActions)
		numActions, err = dao.getNumActions(blks[2].Height())
		require.NoError(t, err)
		require.Equal(t, blkNumActions3, numActions)

		// test getTranferAmount
		transferAmount, err := dao.getTranferAmount(blks[0].Height())
		require.NoError(t, err)
		require.Equal(t, blkTransferAmount1, transferAmount)
		transferAmount, err = dao.getTranferAmount(blks[1].Height())
		require.NoError(t, err)
		require.Equal(t, blkTransferAmount2, transferAmount)
		transferAmount, err = dao.getTranferAmount(blks[2].Height())
		require.NoError(t, err)
		require.Equal(t, blkTransferAmount3, transferAmount)
	}

	testDeleteDao := func(kvstore db.KVStore, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		dao := newBlockDAO(kvstore, true, false, 0, config.Default.DB)
		err := dao.Start(ctx)
		require.NoError(err)
		defer func() {
			err = dao.Stop(ctx)
			assert.Nil(t, err)
		}()

		// Put blocks first
		err = dao.putBlock(blks[0])
		require.NoError(err)
		err = dao.putBlock(blks[1])
		require.NoError(err)
		err = dao.putBlock(blks[2])
		require.NoError(err)

		tipHeight, err := dao.getBlockchainHeight()
		require.NoError(err)
		require.Equal(uint64(3), tipHeight)
		blk, err := dao.getBlock(blks[2].HashBlock())
		require.NoError(err)
		require.NotNil(blk)

		// Delete tip block
		err = dao.deleteTipBlock()
		require.NoError(err)
		tipHeight, err = dao.getBlockchainHeight()
		require.NoError(err)
		require.Equal(uint64(2), tipHeight)
		blk, err = dao.getBlock(blks[2].HashBlock())
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Nil(blk)
	}

	t.Run("In-memory KV Store for blocks", func(t *testing.T) {
		testBlockDao(db.NewMemKVStore(), t)
	})

	path := "test-kv-store"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg := config.Default.DB
	cfg.DbPath = testPath
	t.Run("Bolt DB for blocks", func(t *testing.T) {
		testBlockDao(db.NewBoltDB(cfg), t)
	})
	t.Run("In-memory KV Store for actions", func(t *testing.T) {
		testActionsDao(db.NewMemKVStore(), t)
	})
	t.Run("Bolt DB for actions", func(t *testing.T) {
		testActionsDao(db.NewBoltDB(cfg), t)
	})
	t.Run("In-memory KV Store deletions", func(t *testing.T) {
		testDeleteDao(db.NewMemKVStore(), t)
	})

	t.Run("Bolt DB deletions", func(t *testing.T) {
		testDeleteDao(db.NewBoltDB(cfg), t)
	})
}

func TestBlockDao_putReceipts(t *testing.T) {
	blkDao := newBlockDAO(db.NewMemKVStore(), true, false, 0, config.Default.DB)
	receipts := []*action.Receipt{
		{
			BlockHeight:     1,
			ActionHash:      hash.Hash256b([]byte("1")),
			Status:          1,
			GasConsumed:     1,
			ContractAddress: "1",
			Logs:            []*action.Log{},
		},
		{
			BlockHeight:     1,
			ActionHash:      hash.Hash256b([]byte("1")),
			Status:          2,
			GasConsumed:     2,
			ContractAddress: "2",
			Logs:            []*action.Log{},
		},
	}
	require.NoError(t, blkDao.putReceipts(1, receipts))
	for _, receipt := range receipts {
		r, err := blkDao.getReceiptByActionHash(receipt.ActionHash)
		require.NoError(t, err)
		assert.Equal(t, receipt.ActionHash, r.ActionHash)
	}
}

func BenchmarkBlockCache(b *testing.B) {
	test := func(cacheSize int, b *testing.B) {
		b.StopTimer()
		path := filepath.Join(os.TempDir(), fmt.Sprintf("test-%d.db", rand.Int()))
		cfg := config.DB{
			DbPath:     path,
			NumRetries: 1,
		}
		defer func() {
			if !fileutil.FileExists(path) {
				return
			}
			require.NoError(b, os.RemoveAll(path))
		}()
		store := db.NewBoltDB(cfg)

		blkDao := newBlockDAO(store, false, false, cacheSize, config.Default.DB)
		require.NoError(b, blkDao.Start(context.Background()))
		defer func() {
			require.NoError(b, blkDao.Stop(context.Background()))
		}()
		prevHash := hash.ZeroHash256
		var err error
		numBlks := 8640
		for i := 1; i <= numBlks; i++ {
			actions := make([]action.SealedEnvelope, 10)
			for j := 0; j < 10; j++ {
				actions[j], err = testutil.SignedTransfer(
					identityset.Address(j).String(),
					identityset.PrivateKey(j+1),
					1,
					unit.ConvertIotxToRau(1),
					nil,
					testutil.TestGasLimit,
					testutil.TestGasPrice,
				)
				require.NoError(b, err)
			}
			tb := block.TestingBuilder{}
			blk, err := tb.SetPrevBlockHash(prevHash).
				SetVersion(1).
				SetTimeStamp(time.Now()).
				SetHeight(uint64(i)).
				AddActions(actions...).
				SignAndBuild(identityset.PrivateKey(0))
			require.NoError(b, err)
			err = blkDao.putBlock(&blk)
			require.NoError(b, err)
			prevHash = blk.HashBlock()
		}
		b.ResetTimer()
		b.StartTimer()
		for n := 0; n < b.N; n++ {
			hash, _ := blkDao.getBlockHash(uint64(rand.Intn(numBlks) + 1))
			_, _ = blkDao.getBlock(hash)
		}
		b.StopTimer()
	}
	b.Run("cache", func(b *testing.B) {
		test(8640, b)
	})
	b.Run("no-cache", func(b *testing.B) {
		test(0, b)
	})
}
