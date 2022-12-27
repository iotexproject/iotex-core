// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/testutil"
)

const (
	_blockStoreBatchSize = 16
)

func TestNewFileDAOv2(t *testing.T) {
	testNewFd := func(fd *fileDAOv2, t *testing.T) {
		r := require.New(t)

		ctx := context.Background()
		r.NoError(fd.Start(ctx))
		defer fd.Stop(ctx)
		tip := fd.loadTip().Height
		r.NoError(testCommitBlocks(t, fd, tip+1, tip+3, hash.ZeroHash256))

		// new file does not use legacy's namespaces
		for _, v := range []string{
			_blockNS,
			_blockHeaderNS,
			_blockBodyNS,
			_blockFooterNS,
			_receiptsNS,
		} {
			_, err := fd.kvStore.Get(v, []byte{})
			r.Error(err)
			r.True(strings.Contains(err.Error(), " = "+hex.EncodeToString([]byte(v))+" doesn't exist"))
		}

		// test counting index add empty transaction log
		ser := (&block.BlkTransactionLog{}).Serialize()
		r.Equal([]byte{}, ser)
		for _, test := range []struct {
			compress string
			height   uint64
		}{
			{"", 3},
			{compress.Gzip, 4},
			{compress.Snappy, 5},
		} {
			data := ser
			if test.compress != "" {
				var err error
				data, err = compress.Compress(ser, test.compress)
				r.NoError(err)
			}
			r.NoError(addOneEntryToBatch(fd.hashStore, data, fd.batch))
			r.NoError(fd.kvStore.WriteBatch(fd.batch))
			v, err := fd.hashStore.Get(test.height)
			r.NoError(err)
			r.Equal(data, v)
			if test.compress != "" {
				v, err = compress.Decompress(v, test.compress)
			}
			r.NoError(err)
			r.Equal(ser, v)
		}
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-newfd")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	r.Equal(compress.Snappy, cfg.Compressor)
	r.Equal(16, cfg.BlockStoreBatchSize)
	cfg.DbPath = testPath
	deser := block.NewDeserializer(_defaultEVMNetworkID)
	_, err = newFileDAOv2(0, cfg, deser)
	r.Equal(ErrNotSupported, err)

	inMemFd, err := newFileDAOv2InMem(1)
	r.NoError(err)
	fd, err := newFileDAOv2(2, cfg, deser)
	r.NoError(err)

	for _, v2Fd := range []*fileDAOv2{inMemFd, fd} {
		t.Run("test newFileDAOv2", func(t *testing.T) {
			testNewFd(v2Fd, t)
		})
	}
}

func TestNewFdInterface(t *testing.T) {
	testFdInterface := func(cfg db.Config, start uint64, t *testing.T) {
		r := require.New(t)

		testutil.CleanupPath(cfg.DbPath)
		deser := block.NewDeserializer(_defaultEVMNetworkID)
		fd, err := newFileDAOv2(start, cfg, deser)
		r.NoError(err)

		ctx := context.Background()
		r.NoError(fd.Start(ctx))
		defer fd.Stop(ctx)

		height, err := fd.Bottom()
		r.NoError(err)
		r.Equal(start, height)
		height, err = fd.Height()
		r.NoError(err)
		r.Equal(start-1, height)

		// cannot commit height != tip+1
		builder := block.NewTestingBuilder()
		h := hash.ZeroHash256
		blk := createTestingBlock(builder, start-1, h)
		r.Equal(ErrInvalidTipHeight, fd.PutBlock(ctx, blk))
		blk = createTestingBlock(builder, start+1, h)
		r.Equal(ErrInvalidTipHeight, fd.PutBlock(ctx, blk))

		// verify API for genesis block
		h, err = fd.GetBlockHash(0)
		r.NoError(err)
		r.Equal(block.GenesisHash(), h)
		height, err = fd.GetBlockHeight(h)
		r.NoError(err)
		r.Zero(height)
		blk, err = fd.GetBlock(h)
		r.NoError(err)
		r.Equal(block.GenesisBlock(), blk)

		// commit _blockStoreBatchSize blocks
		for i := uint64(0); i < fd.header.BlockStoreSize; i++ {
			blk = createTestingBlock(builder, start+i, h)
			r.NoError(fd.PutBlock(ctx, blk))
			h = blk.HashBlock()
			height, err = fd.Height()
			r.NoError(err)
			r.Equal(start+i, height)
			if i < fd.header.BlockStoreSize-1 {
				r.EqualValues(0, fd.lowestBlockOfStoreTip())
				r.Equal(start-1, fd.highestBlockOfStoreTip())
			} else {
				r.Equal(start, fd.lowestBlockOfStoreTip())
				r.Equal(start+fd.header.BlockStoreSize-1, fd.highestBlockOfStoreTip())
				r.Equal(start+fd.header.BlockStoreSize-1, height)
			}
		}

		// commit 3 more blocks
		for i := uint64(1); i <= 3; i++ {
			blk = createTestingBlock(builder, height+i, h)
			r.NoError(fd.PutBlock(ctx, blk))
			h = blk.HashBlock()
			r.Equal(start, fd.lowestBlockOfStoreTip())
			r.Equal(start+fd.header.BlockStoreSize-1, fd.highestBlockOfStoreTip())
		}
		height, err = fd.Height()
		r.NoError(err)
		r.Equal(start+fd.header.BlockStoreSize+2, height)
		r.False(fd.ContainsHeight(start - 1))
		r.False(fd.ContainsHeight(height + 1))

		// verify API for all blocks
		r.True(fd.ContainsTransactionLog())
		for i := height; i >= start; i-- {
			height, err = fd.Bottom()
			r.NoError(err)
			r.Equal(start, height)
			r.True(fd.ContainsHeight(i))
			height, err = fd.Height()
			r.NoError(err)
			r.Equal(i, height)
			h, err = fd.GetBlockHash(i)
			r.NoError(err)
			height, err = fd.GetBlockHeight(h)
			r.NoError(err)
			r.Equal(height, i)
			blk, err = fd.GetBlockByHeight(i)
			r.NoError(err)
			r.Equal(h, blk.HashBlock())
			receipt, err := fd.GetReceipts(i)
			r.NoError(err)
			r.EqualValues(1, receipt[0].Status)
			r.Equal(height, receipt[0].BlockHeight)
			r.Equal(blk.Header.PrevHash(), receipt[0].ActionHash)
			log, err := fd.TransactionLogs(i)
			r.NoError(err)
			l := log.Logs[0]
			r.Equal(receipt[0].ActionHash[:], l.ActionHash)
			r.EqualValues(1, l.NumTransactions)
			tx := l.Transactions[0]
			r.Equal(big.NewInt(100).String(), tx.Amount)
			r.Equal(hex.EncodeToString(l.ActionHash[:]), tx.Sender)
			r.Equal(hex.EncodeToString(l.ActionHash[:]), tx.Recipient)
			r.Equal(iotextypes.TransactionLogType_NATIVE_TRANSFER, tx.Type)

			// test DeleteTipBlock()
			r.NoError(fd.DeleteTipBlock())
			r.False(fd.ContainsHeight(i))
			_, err = fd.GetBlockHash(i)
			r.Equal(db.ErrNotExist, err)
			_, err = fd.GetBlockHeight(h)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetBlock(h)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetBlockByHeight(i)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetReceipts(i)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.TransactionLogs(i)
			r.Equal(ErrNotSupported, err)
		}

		// after deleting all blocks
		height, err = fd.Height()
		r.NoError(err)
		r.Equal(start-1, height)
		h, err = fd.GetBlockHash(height)
		if height == 0 {
			r.NoError(err)
			r.Equal(block.GenesisHash(), h)
		} else {
			r.Equal(db.ErrNotExist, err)
			r.Equal(hash.ZeroHash256, h)
		}
		r.EqualValues(0, fd.lowestBlockOfStoreTip())
		r.Equal(start-1, fd.highestBlockOfStoreTip())
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-interface")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	deser := block.NewDeserializer(_defaultEVMNetworkID)
	_, err = newFileDAOv2(0, cfg, deser)
	r.Equal(ErrNotSupported, err)
	genesis.SetGenesisTimestamp(genesis.Default.Timestamp)
	block.LoadGenesisHash(&genesis.Default)

	for _, compress := range []string{"", compress.Snappy} {
		for _, start := range []uint64{1, 5, _blockStoreBatchSize + 1, 4 * _blockStoreBatchSize} {
			cfg.Compressor = compress
			t.Run("test fileDAOv2 interface", func(t *testing.T) {
				testFdInterface(cfg, start, t)
			})
		}
	}
}

func TestNewFdStart(t *testing.T) {
	testFdStart := func(cfg db.Config, start uint64, t *testing.T) {
		r := require.New(t)
		deser := block.NewDeserializer(_defaultEVMNetworkID)
		for _, num := range []uint64{3, _blockStoreBatchSize - 1, _blockStoreBatchSize, 2*_blockStoreBatchSize - 1} {
			testutil.CleanupPath(cfg.DbPath)
			fd, err := newFileDAOv2(start, cfg, deser)
			r.NoError(err)
			ctx := context.Background()
			r.NoError(fd.Start(ctx))
			defer fd.Stop(ctx)

			r.NoError(testCommitBlocks(t, fd, start, start+num-1, hash.ZeroHash256))
			height, err := fd.Height()
			r.NoError(err)
			r.Equal(start+num-1, height)
			r.NoError(fd.Stop(ctx))

			// start from existing file
			fd = openFileDAOv2(cfg, deser)
			r.NoError(fd.Start(ctx))
			height, err = fd.Bottom()
			r.NoError(err)
			r.Equal(start, height)
			height, err = fd.Height()
			r.NoError(err)
			r.Equal(start+num-1, height)

			// verify API for all blocks
			for i := start; i < start+num; i++ {
				r.True(fd.ContainsHeight(i))
				h, err := fd.GetBlockHash(i)
				r.NoError(err)
				height, err = fd.GetBlockHeight(h)
				r.NoError(err)
				r.Equal(height, i)
				blk, err := fd.GetBlockByHeight(i)
				r.NoError(err)
				r.Equal(h, blk.HashBlock())
				receipt, err := fd.GetReceipts(i)
				r.NoError(err)
				r.EqualValues(1, receipt[0].Status)
				r.Equal(height, receipt[0].BlockHeight)
				r.Equal(blk.Header.PrevHash(), receipt[0].ActionHash)
				log, err := fd.TransactionLogs(i)
				r.NoError(err)
				r.NotNil(log)
				l := log.Logs[0]
				r.Equal(receipt[0].ActionHash[:], l.ActionHash)
				r.EqualValues(1, l.NumTransactions)
				tx := l.Transactions[0]
				r.Equal(big.NewInt(100).String(), tx.Amount)
				r.Equal(hex.EncodeToString(l.ActionHash[:]), tx.Sender)
				r.Equal(hex.EncodeToString(l.ActionHash[:]), tx.Recipient)
				r.Equal(iotextypes.TransactionLogType_NATIVE_TRANSFER, tx.Type)
			}
		}
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-start")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	for _, compress := range []string{"", compress.Gzip} {
		for _, start := range []uint64{1, 5, _blockStoreBatchSize + 1, 4 * _blockStoreBatchSize} {
			cfg.Compressor = compress
			t.Run("test fileDAOv2 start", func(t *testing.T) {
				testFdStart(cfg, start, t)
			})
		}
	}
}
