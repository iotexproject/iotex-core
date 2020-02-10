// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	blockNS       = "blockNS"
	blockReceipNS = "blockReceipNS"
	blockTopNS    = "blockTopNS"
	topBlockKey   = []byte("tbk")
	epochLength   = uint64(360)
)

// CheckPoint is the check point struct
type CheckPoint struct {
	kv db.KVStore
}

// NewCheckPoint return new CheckPointDB
func NewCheckPoint(kv db.KVStore, epochLen uint64) *CheckPoint {
	p := &CheckPoint{
		kv,
	}
	epochLength = epochLen
	return p
}

func (pb *CheckPoint) writeBlock(blk *block.Block, key []byte) error {
	blkHeight := blk.Height()
	blkSer, err := blk.Serialize()
	if err != nil {
		return err
	}
	bat := batch.NewCachedBatch()

	bat.Put(blockNS, key, blkSer, "failed to put block")
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			bat.Put(blockReceipNS, key, receiptsBytes, "failed to put receipts")
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	heightValue := byteutil.Uint64ToBytes(blk.Height())
	bat.Put(blockTopNS, topBlockKey, heightValue, "failed to put block")
	return pb.kv.WriteBatch(bat)
}

func (pb *CheckPoint) delBlock(height uint64) error {
	heightKey := byteutil.Uint64ToBytes(height)
	batch := batch.NewCachedBatch()
	batch.Delete(blockNS, heightKey, "failed to del block")
	batch.Delete(blockReceipNS, heightKey, "failed to del receipts")
	return pb.kv.WriteBatch(batch)
}

// ReceiveBlock write block to db
func (pb *CheckPoint) ReceiveBlock(blk *block.Block) error {
	heightKey := byteutil.Uint64ToBytes(blk.Height())
	err := pb.writeBlock(blk, heightKey)
	if err != nil {
		return err
	}
	if blk.Height() > epochLength {
		pb.delBlock(blk.Height() - epochLength)
	}
	return nil
}

// GetTopBlock get top block
func GetTopBlock(kv db.KVStore) (*block.Block, error) {
	heightValue, err := kv.Get(blockTopNS, topBlockKey)
	if err != nil {
		return nil, err
	}
	return GetBlock(kv, heightValue)
}

// GetBlock get block by height
func GetBlock(kv db.KVStore, heightValue []byte) (*block.Block, error) {
	blkSer, err := kv.Get(blockNS, heightValue)
	if err != nil {
		return nil, err
	}
	blk := &block.Block{}
	if err := blk.Deserialize(blkSer); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block")
	}
	receipts, err := kv.Get(blockReceipNS, heightValue)
	if err != nil {
		return nil, err
	}
	receiptsPb := &iotextypes.Receipts{}
	if err := proto.Unmarshal(receipts, receiptsPb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block receipts")
	}
	for _, receiptPb := range receiptsPb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		blk.Receipts = append(blk.Receipts, receipt)
	}
	return blk, nil
}
