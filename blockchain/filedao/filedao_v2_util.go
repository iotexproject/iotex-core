// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

func (fd *fileDAOv2) populateStagingBuffer() (*stagingBuffer, error) {
	buffer := newStagingBuffer(fd.header.BlockStoreSize, fd.deser)
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
			if _, err = buffer.Put(stagingKey(height, fd.header), v); err != nil {
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
	index := stagingKey(blk.Height(), fd.header)
	full, err := fd.blkBuffer.Put(index, ser)
	if err != nil {
		return err
	}
	if !full {
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

// stagingKey is the position of block in the staging buffer
func stagingKey(height uint64, header *FileHeader) uint64 {
	return (height - header.Start) % header.BlockStoreSize
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

func (fd *fileDAOv2) getBlockStore(height uint64) (*block.Store, error) {
	if !fd.ContainsHeight(height) {
		return nil, db.ErrNotExist
	}

	// check whether block in staging buffer or not
	storeKey := blockStoreKey(height, fd.header)
	if storeKey >= fd.blkStore.Size() {
		return fd.blkBuffer.Get(stagingKey(height, fd.header))
	}

	// check whether block in read cache or not
	if value, ok := fd.blkCache.Get(storeKey); ok {
		pbInfos := value.(*iotextypes.BlockStores)
		return fd.deser.FromBlockStoreProto(pbInfos.BlockStores[stagingKey(height, fd.header)])
	}

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
	fd.blkCache.Add(storeKey, pbStores)
	return fd.deser.FromBlockStoreProto(pbStores.BlockStores[stagingKey(height, fd.header)])
}
