// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"sync/atomic"
	"unsafe"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// namespace for hash, block, and header storage
const (
	_hashDataNS   = "hsh"
	_blockDataNS  = "bdn"
	_headerDataNs = "hdr"
)

var (
	_fileHeaderKey = []byte("fh")
)

type (
	// fileDAOv2 handles chain db file after file split activation at v1.1.2
	fileDAOv2 struct {
		filename  string
		header    *FileHeader
		tip       *FileTip
		blkBuffer *stagingBuffer
		blkCache  cache.LRUCache
		kvStore   db.KVStore
		batch     batch.KVStoreBatch
		hashStore db.CountingIndex // store block hash
		blkStore  db.CountingIndex // store raw blocks
		sysStore  db.CountingIndex // store transaction log
		deser     *block.Deserializer
	}
)

// newFileDAOv2 creates a new v2 file
func newFileDAOv2(bottom uint64, cfg db.Config, deser *block.Deserializer) (*fileDAOv2, error) {
	if bottom == 0 {
		return nil, ErrNotSupported
	}

	fd := fileDAOv2{
		filename: cfg.DbPath,
		header: &FileHeader{
			Version:        FileV2,
			Compressor:     cfg.Compressor,
			BlockStoreSize: uint64(cfg.BlockStoreBatchSize),
			Start:          bottom,
		},
		tip: &FileTip{
			Height: bottom - 1,
		},
		blkCache: cache.NewThreadSafeLruCache(16),
		kvStore:  db.NewBoltDB(cfg),
		batch:    batch.NewBatch(),
		deser:    deser,
	}
	return &fd, nil
}

// openFileDAOv2 opens an existing v2 file
func openFileDAOv2(cfg db.Config, deser *block.Deserializer) *fileDAOv2 {
	return &fileDAOv2{
		filename: cfg.DbPath,
		blkCache: cache.NewThreadSafeLruCache(16),
		kvStore:  db.NewBoltDB(cfg),
		batch:    batch.NewBatch(),
		deser:    deser,
	}
}

func (fd *fileDAOv2) Start(ctx context.Context) error {
	if err := fd.kvStore.Start(ctx); err != nil {
		return err
	}

	// check file header
	header, err := ReadHeaderV2(fd.kvStore)
	if err != nil {
		if errors.Cause(err) != db.ErrBucketNotExist && errors.Cause(err) != db.ErrNotExist {
			return errors.Wrap(err, "failed to get file header")
		}
		// write file header and tip
		if err = WriteHeaderV2(fd.kvStore, fd.header); err != nil {
			return err
		}
		if err = WriteTip(fd.kvStore, _headerDataNs, _topHeightKey, fd.tip); err != nil {
			return err
		}
	} else {
		fd.header = header
		// read file tip
		if fd.tip, err = ReadTip(fd.kvStore, _headerDataNs, _topHeightKey); err != nil {
			return err
		}
	}

	// create counting index for hash, blk, and transaction log
	if fd.hashStore, err = db.NewCountingIndexNX(fd.kvStore, []byte(_hashDataNS)); err != nil {
		return err
	}
	if fd.blkStore, err = db.NewCountingIndexNX(fd.kvStore, []byte(_blockDataNS)); err != nil {
		return err
	}
	if fd.sysStore, err = db.NewCountingIndexNX(fd.kvStore, []byte(_systemLogNS)); err != nil {
		return err
	}

	// populate staging buffer
	if fd.blkBuffer, err = fd.populateStagingBuffer(); err != nil {
		return err
	}
	return nil
}

func (fd *fileDAOv2) Stop(ctx context.Context) error {
	return fd.kvStore.Stop(ctx)
}

func (fd *fileDAOv2) Height() (uint64, error) {
	tip := fd.loadTip()
	return tip.Height, nil
}

func (fd *fileDAOv2) Bottom() (uint64, error) {
	return fd.header.Start, nil
}

func (fd *fileDAOv2) ContainsHeight(height uint64) bool {
	return fd.header.Start <= height && height <= fd.loadTip().Height
}

func (fd *fileDAOv2) GetBlockHash(height uint64) (hash.Hash256, error) {
	if height == 0 {
		return block.GenesisHash(), nil
	}
	if !fd.ContainsHeight(height) {
		return hash.ZeroHash256, db.ErrNotExist
	}
	h, err := fd.hashStore.Get(height - fd.header.Start)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get block hash")
	}
	return hash.BytesToHash256(h), nil
}

func (fd *fileDAOv2) GetBlockHeight(h hash.Hash256) (uint64, error) {
	if h == block.GenesisHash() {
		return 0, nil
	}
	value, err := getValueMustBe8Bytes(fd.kvStore, _blockHashHeightMappingNS, hashKey(h))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	return byteutil.BytesToUint64BigEndian(value), nil
}

func (fd *fileDAOv2) GetBlock(h hash.Hash256) (*block.Block, error) {
	height, err := fd.GetBlockHeight(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block")
	}
	return fd.GetBlockByHeight(height)
}

func (fd *fileDAOv2) GetBlockByHeight(height uint64) (*block.Block, error) {
	if height == 0 {
		return block.GenesisBlock(), nil
	}
	blkInfo, err := fd.getBlockStore(height)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block at height %d", height)
	}
	return blkInfo.Block, nil
}

func (fd *fileDAOv2) GetReceipts(height uint64) ([]*action.Receipt, error) {
	blkInfo, err := fd.getBlockStore(height)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipts at height %d", height)
	}
	return blkInfo.Receipts, nil
}

func (fd *fileDAOv2) ContainsTransactionLog() bool {
	return true
}

func (fd *fileDAOv2) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	if !fd.ContainsHeight(height) {
		return nil, ErrNotSupported
	}

	value, err := fd.sysStore.Get(height - fd.header.Start)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transaction log at height %d", height)
	}
	value, err = decompBytes(value, fd.header.Compressor)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transaction log at height %d", height)
	}
	return block.DeserializeSystemLogPb(value)
}

func (fd *fileDAOv2) PutBlock(_ context.Context, blk *block.Block) error {
	tip := fd.loadTip()
	if blk.Height() != tip.Height+1 {
		return ErrInvalidTipHeight
	}

	// write tip hash and hash-height mapping
	if err := fd.putTipHashHeightMapping(blk); err != nil {
		return errors.Wrap(err, "failed to write hash-height mapping")
	}

	// write block data
	if err := fd.putBlock(blk); err != nil {
		return errors.Wrap(err, "failed to write block")
	}

	// write receipt and transaction log
	if err := fd.putTransactionLog(blk); err != nil {
		return errors.Wrap(err, "failed to write receipt")
	}

	if err := fd.kvStore.WriteBatch(fd.batch); err != nil {
		return errors.Wrapf(err, "failed to put block at height %d", blk.Height())
	}
	fd.batch.Clear()
	// update file tip
	tip = &FileTip{Height: blk.Height(), Hash: blk.HashBlock()}
	fd.storeTip(tip)
	return nil
}

func (fd *fileDAOv2) DeleteTipBlock() error {
	tip := fd.loadTip()
	height := tip.Height

	if !fd.ContainsHeight(height) {
		// cannot delete block that does not exist
		return ErrNotSupported
	}

	// delete hash
	if err := fd.hashStore.Revert(1); err != nil {
		return err
	}
	// delete tip of block storage, if new tip height < lowest block stored in it
	if height-1 < fd.lowestBlockOfStoreTip() {
		if err := fd.blkStore.Revert(1); err != nil {
			return err
		}
	}
	// delete transaction log
	if err := fd.sysStore.Revert(1); err != nil {
		return err
	}

	// delete hash -> height mapping
	fd.batch.Delete(_blockHashHeightMappingNS, hashKey(tip.Hash), "failed to delete hash -> height mapping")

	// update file tip
	var (
		h   = hash.ZeroHash256
		err error
	)
	if height > fd.header.Start {
		h, err = fd.GetBlockHash(height - 1)
		if err != nil {
			return err
		}
	}
	tip = &FileTip{Height: height - 1, Hash: h}
	ser, err := tip.Serialize()
	if err != nil {
		return err
	}
	fd.batch.Put(_headerDataNs, _topHeightKey, ser, "failed to put file tip")

	if err := fd.kvStore.WriteBatch(fd.batch); err != nil {
		return err
	}
	fd.batch.Clear()
	fd.storeTip(tip)
	return nil
}

func (fd *fileDAOv2) loadTip() *FileTip {
	p := (*unsafe.Pointer)(unsafe.Pointer(&fd.tip))
	return (*FileTip)(atomic.LoadPointer(p))
}

func (fd *fileDAOv2) storeTip(tip *FileTip) {
	p := (*unsafe.Pointer)(unsafe.Pointer(&fd.tip))
	atomic.StorePointer(p, unsafe.Pointer(tip))
}
