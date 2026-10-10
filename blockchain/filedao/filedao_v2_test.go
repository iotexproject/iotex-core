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
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/compress"
	"github.com/iotexproject/iotex-core/v2/testutil"
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
			b := batch.NewBatch()
			apply := fd.hashStore.AddToBatch(b, data)
			r.NoError(fd.kvStore.WriteBatch(b))
			apply()
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
	g := genesis.TestDefault()
	genesis.SetGenesisTimestamp(g.Timestamp)
	block.LoadGenesisHash(&g)

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

func TestBlockWithSidecar(t *testing.T) {
	testBlockWithSidecar := func(cfg db.Config, start uint64, r *require.Assertions) {
		testutil.CleanupPath(cfg.DbPath)
		r.Equal(5, cfg.BlockStoreBatchSize)
		deser := block.NewDeserializer(_defaultEVMNetworkID)
		fd, err := newFileDAOv2(start, cfg, deser)
		r.NoError(err)
		ctx := context.Background()
		r.NoError(fd.Start(ctx))
		defer func() {
			r.NoError(fd.Stop(ctx))
		}()

		blks, err := block.CreateTestBlockWithBlob(int(start), cfg.BlockStoreBatchSize+2)
		r.NoError(err)
		for _, blk := range blks {
			r.True(blk.HasBlob())
			r.NoError(fd.PutBlock(ctx, blk))
		}
		for i := 0; i < cfg.BlockStoreBatchSize+2; i++ {
			blk, err := fd.GetBlockByHeight(start + uint64(i))
			r.NoError(err)
			if i < 2 {
				// blocks written to disk has sidecar removed
				r.False(blk.HasBlob())
				r.Equal(4, len(blk.Actions))
				blk.Actions[0].Hash()
				r.Equal(blks[i].Actions[0], blk.Actions[0])
				r.NotEqual(blks[i].Actions[1], blk.Actions[1])
				blk.Actions[2].Hash()
				r.Equal(blks[i].Actions[2], blk.Actions[2])
				r.NotEqual(blks[i].Actions[3], blk.Actions[3])
			} else {
				// blocks in the staging buffer still has sidecar attached
				r.True(blk.HasBlob())
				r.Equal(blks[i], blk)
			}
			h := blk.HashBlock()
			height, err := fd.GetBlockHeight(h)
			r.NoError(err)
			r.Equal(start+uint64(i), height)
			hash, err := fd.GetBlockHash(height)
			r.NoError(err)
			r.Equal(h, hash)
		}
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-sidecar")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	cfg.BlockStoreBatchSize = 5
	cfg.DbPath = testPath
	for _, compress := range []string{"", compress.Snappy} {
		for _, start := range []uint64{1, 4, uint64(cfg.BlockStoreBatchSize) + 3, 3 * uint64(cfg.BlockStoreBatchSize)} {
			cfg.Compressor = compress
			t.Run("test block with sidecar", func(t *testing.T) {
				testBlockWithSidecar(cfg, start, r)
			})
		}
	}
}

type failingWriteKVStore struct {
	db.KVStore
}

func (failingWriteKVStore) WriteBatch(batch.KVStoreBatch) error {
	return errors.New("write failed")
}

func TestFileDAOv2FailedWrite(t *testing.T) {
	deser := block.NewDeserializer(_defaultEVMNetworkID)
	ctx := context.Background()
	builder := block.NewTestingBuilder()
	h := hash.ZeroHash256
	blks := make([]*block.Block, 0, 3*_blockStoreBatchSize)
	for i := uint64(1); i <= 3*_blockStoreBatchSize; i++ {
		blk := createTestingBlock(builder, i, h)
		blks = append(blks, blk)
		h = blk.HashBlock()
	}
	// checkBlocks checks the tip and every data relation of blks
	checkBlocks := func(r *require.Assertions, fd *fileDAOv2, blks []*block.Block) {
		height, err := fd.Height()
		r.NoError(err)
		r.Equal(uint64(len(blks)), height)
		r.Equal(blks[len(blks)-1].HashBlock(), fd.loadTip().Hash)
		for _, want := range blks {
			got, err := fd.GetBlockByHeight(want.Height())
			r.NoError(err)
			r.Equal(want.HashBlock(), got.HashBlock())
			h, err := fd.GetBlockHash(want.Height())
			r.NoError(err)
			r.Equal(want.HashBlock(), h)
			height, err := fd.GetBlockHeight(want.HashBlock())
			r.NoError(err)
			r.Equal(want.Height(), height)
			receipts, err := fd.GetReceipts(want.Height())
			r.NoError(err)
			r.Equal(want.Height(), receipts[0].BlockHeight)
			wantLog, err := block.DeserializeSystemLogPb(want.TransactionLog().Serialize())
			r.NoError(err)
			r.NotEmpty(wantLog.Logs)
			gotLog, err := fd.TransactionLogs(want.Height())
			r.NoError(err)
			r.True(proto.Equal(wantLog, gotLog))
		}
	}
	// checkNotWritten checks that nothing of blk can be read
	checkNotWritten := func(r *require.Assertions, fd *fileDAOv2, blk *block.Block) {
		_, err := fd.GetBlockByHeight(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.GetReceipts(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.TransactionLogs(blk.Height())
		r.ErrorIs(err, ErrNotSupported)
		_, err = fd.GetBlockHeight(blk.HashBlock())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.GetBlockHash(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
	}
	indexSizes := func(fd *fileDAOv2) []uint64 {
		return []uint64{fd.hashStore.Size(), fd.blkStore.Size(), fd.sysStore.Size()}
	}
	bufferSlots := func(fd *fileDAOv2) []*block.Store {
		fd.blkBuffer.lock.RLock()
		defer fd.blkBuffer.lock.RUnlock()
		return append([]*block.Store{}, fd.blkBuffer.buffer...)
	}
	// checkBufferRound checks that the staging buffer of a reopened file holds
	// exactly the blocks of the current round
	checkBufferRound := func(r *require.Assertions, fd *fileDAOv2, blks []*block.Block) {
		tip := uint64(len(blks))
		roundStart := tip - (tip-fd.header.Start+1)%fd.header.BlockStoreSize + 1
		for i, v := range bufferSlots(fd) {
			height := roundStart + uint64(i)
			if height > tip {
				r.Nil(v, "slot %d", i)
				continue
			}
			r.NotNil(v, "slot %d", i)
			r.Equal(blks[height-1].HashBlock(), v.Block.HashBlock(), "slot %d", i)
		}
	}
	// failCompressAt makes the n-th compression from now on fail, PutBlock
	// compresses the block first and then the transaction log
	failCompressAt := func(r *require.Assertions, n int) func() {
		calls := 0
		_compress = func(v []byte, comp string) ([]byte, error) {
			calls++
			if calls == n {
				return nil, errors.New("compress failed")
			}
			return compress.Compress(v, comp)
		}
		return func() {
			_compress = compress.Compress
			r.GreaterOrEqual(calls, n, "compression did not fail")
		}
	}

	for _, f := range []struct {
		name   string
		inject func(*require.Assertions, *fileDAOv2) (restore func())
	}{
		{
			"write batch fails",
			func(_ *require.Assertions, fd *fileDAOv2) func() {
				kv := fd.kvStore
				fd.kvStore = failingWriteKVStore{kv}
				return func() { fd.kvStore = kv }
			},
		},
		{
			// fails in putBlock, after the hash index entry is in the batch
			"block compression fails",
			func(r *require.Assertions, _ *fileDAOv2) func() { return failCompressAt(r, 1) },
		},
		{
			// fails in putTransactionLog, after putBlock has returned its apply
			"transaction log compression fails",
			func(r *require.Assertions, _ *fileDAOv2) func() { return failCompressAt(r, 2) },
		},
	} {
		for _, c := range []struct {
			name   string
			height uint64
		}{
			{"staging buffer not full", _blockStoreBatchSize + 4},
			{"staging buffer full", 2 * _blockStoreBatchSize},
		} {
			t.Run(f.name+"/"+c.name, func(t *testing.T) {
				r := require.New(t)
				testPath, err := testutil.PathOfTempFile("test-failed-write")
				r.NoError(err)
				defer testutil.CleanupPath(testPath)

				cfg := db.DefaultConfig
				cfg.DbPath = testPath
				r.Equal(_blockStoreBatchSize, cfg.BlockStoreBatchSize)
				r.NotEmpty(cfg.Compressor)
				fd, err := newFileDAOv2(1, cfg, deser)
				r.NoError(err)
				r.NoError(fd.Start(ctx))
				written, failed := blks[:c.height-1], blks[c.height-1]
				for _, blk := range written {
					r.NoError(fd.PutBlock(ctx, blk))
				}
				sizes, slots := indexSizes(fd), bufferSlots(fd)

				// a failed write leaves the in-memory state unchanged
				restore := f.inject(r, fd)
				r.Error(fd.PutBlock(ctx, failed))
				restore()
				r.Equal(sizes, indexSizes(fd))
				newSlots := bufferSlots(fd)
				for i := range slots {
					r.Same(slots[i], newSlots[i], "slot %d", i)
				}
				checkBlocks(r, fd, written)
				checkNotWritten(r, fd, failed)
				r.NoError(fd.Stop(ctx))

				// and nothing of it reached the file
				fd = openFileDAOv2(cfg, deser)
				r.NoError(fd.Start(ctx))
				r.Equal(sizes, indexSizes(fd))
				checkBufferRound(r, fd, written)
				checkBlocks(r, fd, written)
				checkNotWritten(r, fd, failed)

				// retrying the same block succeeds
				for _, blk := range blks[c.height-1 : 2*_blockStoreBatchSize+2] {
					r.NoError(fd.PutBlock(ctx, blk))
				}
				checkBlocks(r, fd, blks[:2*_blockStoreBatchSize+2])
				r.NoError(fd.Stop(ctx))

				// reopening reloads the same state from the file and writing resumes
				fd = openFileDAOv2(cfg, deser)
				r.NoError(fd.Start(ctx))
				defer fd.Stop(ctx)
				checkBufferRound(r, fd, blks[:2*_blockStoreBatchSize+2])
				checkBlocks(r, fd, blks[:2*_blockStoreBatchSize+2])
				for _, blk := range blks[2*_blockStoreBatchSize+2:] {
					r.NoError(fd.PutBlock(ctx, blk))
				}
				checkBlocks(r, fd, blks)
			})
		}
	}
}
