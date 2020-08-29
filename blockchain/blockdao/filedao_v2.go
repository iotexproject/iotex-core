// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	_normal             = 0
	_compressed         = 1
	blockStoreBatchSize = 16
)

// namespace for hash, block, and staging storage
const (
	hashDataNS    = "hsh"
	blockDataNS   = "bdn"
	stagingDataNS = "stg"
)

var (
	bottomHeightKey = []byte("bh")
)

type (
	fileDAOv2 struct {
		compressBlock bool
		bottomHeight  uint64 // height of first block in this DB file
		tipHeight     uint64
		cfg           config.DB
		blkBuffer     *stagingBuffer
		blkCache      *cache.ThreadSafeLruCache
		kvStore       db.KVStore
		batch         batch.KVStoreBatch
		hashStore     db.CountingIndex // store block hash
		blkStore      db.CountingIndex // store raw blocks
		sysStore      db.CountingIndex // store transaction log
	}
)

// newFileDAOv2 creates a new v2 file
func newFileDAOv2(kvStore db.KVStore, bottom uint64, cfg config.DB) (FileDAO, error) {
	if bottom == 0 {
		return nil, ErrNotSupported
	}

	fd := fileDAOv2{
		compressBlock: cfg.CompressData,
		bottomHeight:  bottom,
		tipHeight:     bottom - 1,
		cfg:           cfg,
		blkCache:      cache.NewThreadSafeLruCache(16),
		kvStore:       kvStore,
		batch:         batch.NewBatch(),
	}
	return &fd, nil
}

// openFileDAOv2 opens an existing v2 file
func openFileDAOv2(kvStore db.KVStore, cfg config.DB) (FileDAO, error) {
	fd := fileDAOv2{
		compressBlock: cfg.CompressData,
		cfg:           cfg,
		blkCache:      cache.NewThreadSafeLruCache(16),
		kvStore:       kvStore,
		batch:         batch.NewBatch(),
	}
	return &fd, nil
}

func (fd *fileDAOv2) Start(ctx context.Context) error {
	if err := fd.kvStore.Start(ctx); err != nil {
		return err
	}

	// check start height
	value, err := getValueMustBe8Bytes(fd.kvStore, blockHashHeightMappingNS, bottomHeightKey)
	if err != nil {
		if errors.Cause(err) != db.ErrBucketNotExist && errors.Cause(err) != db.ErrNotExist {
			return errors.Wrap(err, "failed to get bottom height")
		}
		// set start height
		if err = fd.kvStore.Put(blockHashHeightMappingNS, bottomHeightKey, byteutil.Uint64ToBytesBigEndian(fd.bottomHeight)); err != nil {
			return err
		}
	} else {
		fd.bottomHeight = byteutil.BytesToUint64BigEndian(value)
	}

	// check tip height
	value, err = getValueMustBe8Bytes(fd.kvStore, blockHashHeightMappingNS, topHeightKey)
	if err != nil {
		if errors.Cause(err) != db.ErrBucketNotExist && errors.Cause(err) != db.ErrNotExist {
			return errors.Wrap(err, "failed to get tip height")
		}
		// set tip height
		if err = fd.kvStore.Put(blockHashHeightMappingNS, topHeightKey, byteutil.Uint64ToBytesBigEndian(fd.tipHeight)); err != nil {
			return err
		}
	} else {
		fd.tipHeight = byteutil.BytesToUint64BigEndian(value)
	}

	// create counting index for hash, blk, and transaction log
	if fd.hashStore, err = db.NewCountingIndexNX(fd.kvStore, []byte(hashDataNS)); err != nil {
		return err
	}
	if fd.blkStore, err = db.NewCountingIndexNX(fd.kvStore, []byte(blockDataNS)); err != nil {
		return err
	}
	if fd.sysStore, err = db.NewCountingIndexNX(fd.kvStore, []byte(systemLogNS)); err != nil {
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
	value, err := getValueMustBe8Bytes(fd.kvStore, blockHashHeightMappingNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	return byteutil.BytesToUint64BigEndian(value), nil
}

func (fd *fileDAOv2) Bottom() (uint64, error) {
	return fd.bottomHeight, nil
}

func (fd *fileDAOv2) ContainsHeight(height uint64) bool {
	return fd.bottomHeight <= height && height <= fd.tipHeight
}

func (fd *fileDAOv2) GetBlockHash(height uint64) (hash.Hash256, error) {
	if height == fd.bottomHeight-1 {
		return hash.ZeroHash256, nil
	}
	if !fd.ContainsHeight(height) {
		return hash.ZeroHash256, db.ErrNotExist
	}
	h, err := fd.hashStore.Get(height - fd.bottomHeight)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get block hash")
	}
	return hash.BytesToHash256(h), nil
}

func (fd *fileDAOv2) GetBlockHeight(h hash.Hash256) (uint64, error) {
	value, err := getValueMustBe8Bytes(fd.kvStore, blockHashHeightMappingNS, hashKey(h))
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

	value, err := fd.sysStore.Get(height - fd.bottomHeight)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transaction log at height %d", height)
	}
	value, err = decompressDatabytes(value)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transaction log at height %d", height)
	}
	return block.DeserializeSystemLogPb(value)
}

func (fd *fileDAOv2) PutBlock(_ context.Context, blk *block.Block) error {
	if blk.Height() != fd.tipHeight+1 {
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
	fd.tipHeight = blk.Height()
	return nil
}

func (fd *fileDAOv2) DeleteTipBlock() error {
	height, err := fd.Height()
	if err != nil {
		return err
	}

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
	v, err := fd.kvStore.Get(blockHashHeightMappingNS, topHashKey)
	if err != nil {
		return err
	}
	fd.batch.Delete(blockHashHeightMappingNS, hashKey(hash.BytesToHash256(v)), "failed to delete hash -> height mapping")

	// update tip hash/height
	h := hash.ZeroHash256
	if height > fd.bottomHeight {
		h, err = fd.GetBlockHash(height - 1)
		if err != nil {
			return err
		}
	}
	fd.batch.Put(blockHashHeightMappingNS, topHashKey, h[:], "failed to put tip hash")
	fd.batch.Put(blockHashHeightMappingNS, topHeightKey, byteutil.Uint64ToBytesBigEndian(height-1), "failed to put tip height")

	if err := fd.kvStore.WriteBatch(fd.batch); err != nil {
		return err
	}
	fd.tipHeight = height - 1
	return nil
}
