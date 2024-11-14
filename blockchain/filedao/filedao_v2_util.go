// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/compress"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

func (fd *fileDAOv2) populateStagingBuffer() (*stagingBuffer, error) {
	buffer := newStagingBuffer(fd.header.BlockStoreSize, fd.header.Start)
	blockStoreTip := fd.highestBlockOfStoreTip()
	for i := uint64(0); i < fd.header.BlockStoreSize; i++ {
		v, err := fd.kvStore.Get(_headerDataNs, byteutil.Uint64ToBytesBigEndian(i))
		if err != nil {
			if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
				break
			}
			return nil, err
		}

		v, err = decompBytes(v, fd.header.Compressor)
		if err != nil {
			return nil, err
		}
		info, err := fd.deser.DeserializeBlockStore(v)
		if err != nil {
			return nil, err
		}

		// populate to staging buffer, if the block is in latest round
		height := info.Block.Height()
		if height > blockStoreTip {
			if _, err = buffer.Put(height, info); err != nil {
				return nil, err
			}
		} else {
			break
		}
	}
	return buffer, nil
}

func (fd *fileDAOv2) putTipHashHeightMapping(blk *block.Block) error {
	// write height <-> hash mapping
	h := blk.HashBlock()
	if err := addOneEntryToBatch(fd.hashStore, h[:], fd.batch); err != nil {
		return err
	}

	// write hash <-> height mapping
	height := blk.Height()
	fd.batch.Put(_blockHashHeightMappingNS, hashKey(h), byteutil.Uint64ToBytesBigEndian(height), "failed to put hash -> height mapping")

	// update file tip
	ser, err := (&FileTip{Height: height, Hash: h}).Serialize()
	if err != nil {
		return err
	}
	fd.batch.Put(_headerDataNs, _topHeightKey, ser, "failed to put file tip")
	return nil
}

func (fd *fileDAOv2) putBlock(blk *block.Block) error {
	blkInfo := &block.Store{
		Block:    blk,
		Receipts: blk.Receipts,
	}
	ser, err := blkInfo.Serialize()
	if err != nil {
		return err
	}
	blkBytes, err := compBytes(ser, fd.header.Compressor)
	if err != nil {
		return err
	}

	// add to staging buffer
	full, err := fd.blkBuffer.Put(blk.Height(), blkInfo)
	if err != nil {
		return err
	}
	if !full {
		index := fd.blkBuffer.slot(blk.Height())
		fd.batch.Put(_headerDataNs, byteutil.Uint64ToBytesBigEndian(index), blkBytes, "failed to put block")
		return nil
	}

	// pack blocks together, write to block store
	if ser, err = fd.blkBuffer.Serialize(); err != nil {
		return err
	}
	if blkBytes, err = compBytes(ser, fd.header.Compressor); err != nil {
		return err
	}
	return addOneEntryToBatch(fd.blkStore, blkBytes, fd.batch)
}

func (fd *fileDAOv2) putTransactionLog(blk *block.Block) error {
	sysLog := blk.TransactionLog()
	if sysLog == nil {
		sysLog = &block.BlkTransactionLog{}
	}
	logBytes, err := compBytes(sysLog.Serialize(), fd.header.Compressor)
	if err != nil {
		return err
	}
	return addOneEntryToBatch(fd.sysStore, logBytes, fd.batch)
}

func addOneEntryToBatch(c db.CountingIndex, v []byte, b batch.KVStoreBatch) error {
	if err := c.UseBatch(b); err != nil {
		return err
	}
	if err := c.Add(v, true); err != nil {
		return err
	}
	return c.Finalize()
}

func compBytes(v []byte, comp string) ([]byte, error) {
	if comp != "" {
		return compress.Compress(v, comp)
	}
	return v, nil
}

func decompBytes(v []byte, comp string) ([]byte, error) {
	if comp != "" {
		return compress.Decompress(v, comp)
	}
	return v, nil
}

// blockStoreKey is the slot of block in block storage (each item containing blockStorageBatchSize of blocks)
func blockStoreKey(height uint64, header *FileHeader) uint64 {
	if height <= header.Start {
		return 0
	}
	return (height - header.Start) / header.BlockStoreSize
}

// lowestBlockOfStoreTip is the lowest height of the tip of block storage
// used in DeleteTipBlock(), once new tip height drops below this, the tip of block storage can be deleted
func (fd *fileDAOv2) lowestBlockOfStoreTip() uint64 {
	if fd.blkStore.Size() == 0 {
		return 0
	}
	return fd.header.Start + (fd.blkStore.Size()-1)*fd.header.BlockStoreSize
}

// highestBlockOfStoreTip is the highest height of the tip of block storage
func (fd *fileDAOv2) highestBlockOfStoreTip() uint64 {
	if fd.blkStore.Size() == 0 {
		return fd.header.Start - 1
	}
	return fd.header.Start + fd.blkStore.Size()*fd.header.BlockStoreSize - 1
}

func (fd *fileDAOv2) getBlock(height uint64) (*block.Block, error) {
	if !fd.ContainsHeight(height) {
		return nil, db.ErrNotExist
	}
	// check whether block in staging buffer or not
	if blkStore := fd.getFromStagingBuffer(height); blkStore != nil {
		return blkStore.Block, nil
	}
	// read from storage DB
	blockStore, err := fd.getBlockStore(height)
	if err != nil {
		return nil, err
	}
	return fd.deser.BlockFromBlockStoreProto(blockStore)
}

func (fd *fileDAOv2) getReceipt(height uint64) ([]*action.Receipt, error) {
	if !fd.ContainsHeight(height) {
		return nil, db.ErrNotExist
	}
	// check whether block in staging buffer or not
	if blkStore := fd.getFromStagingBuffer(height); blkStore != nil {
		return blkStore.Receipts, nil
	}
	// read from storage DB
	blockStore, err := fd.getBlockStore(height)
	if err != nil {
		return nil, err
	}
	return fd.deser.ReceiptsFromBlockStoreProto(blockStore)
}

func (fd *fileDAOv2) getFromStagingBuffer(height uint64) *block.Store {
	if fd.loadTip().Height-height >= fd.header.BlockStoreSize {
		return nil
	}
	blkStore := fd.blkBuffer.Get(height)
	if blkStore == nil || blkStore.Block.Height() != height {
		return nil
	}
	return blkStore
}

func (fd *fileDAOv2) getBlockStore(height uint64) (*iotextypes.BlockStore, error) {
	// check whether blockStore in read cache or not
	storeKey := blockStoreKey(height, fd.header)
	if value, ok := fd.blkStorePbCache.Get(storeKey); ok {
		pbInfos := value.(*iotextypes.BlockStores)
		return pbInfos.BlockStores[fd.blkBuffer.slot(height)], nil
	}
	// read from storage DB
	value, err := fd.blkStore.Get(storeKey)
	if err != nil {
		return nil, err
	}
	value, err = decompBytes(value, fd.header.Compressor)
	if err != nil {
		return nil, err
	}
	pbStores, err := block.DeserializeBlockStoresPb(value)
	if err != nil {
		return nil, err
	}
	if len(pbStores.BlockStores) != int(fd.header.BlockStoreSize) {
		return nil, ErrDataCorruption
	}
	// add to read cache
	fd.blkStorePbCache.Add(storeKey, pbStores)
	return pbStores.BlockStores[fd.blkBuffer.slot(height)], nil
}
