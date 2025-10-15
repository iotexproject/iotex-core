// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"fmt"
	"sync"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type (
	// ReceiptIndexer defines a struct to store correct receipts for poluted blocks
	ReceiptIndexer struct {
		mu        sync.RWMutex
		height    uint64
		endHeight uint64
		kvstore   db.KVStore
	}
)

const (
	_receiptsNS    = "receipts"
	_receiptMetaNS = "meta"
)

var _heightKey = []byte("height")

// ErrIndexOutOfRange indicates the receipt does not exist in the indexer
var ErrIndexOutOfRange = errors.New("index out of range")

// NewReceiptIndexer creates a new receipt indexer
func NewReceiptIndexer(kvstore db.KVStore, endHeight uint64) *ReceiptIndexer {
	return &ReceiptIndexer{
		endHeight: endHeight,
		kvstore:   kvstore,
	}
}

// Start starts the receipt indexer
func (ri *ReceiptIndexer) Start(ctx context.Context) error {
	ri.mu.Lock()
	defer ri.mu.Unlock()
	if err := ri.kvstore.Start(ctx); err != nil {
		return err
	}
	value, err := ri.kvstore.Get(_receiptMetaNS, _heightKey)
	switch errors.Cause(err) {
	case nil:
	case db.ErrNotExist, db.ErrBucketNotExist:
		return nil
	default:
		return err
	}
	ri.height = byteutil.BytesToUint64(value)
	return nil
}

// Stop stops the receipt indexer
func (ri *ReceiptIndexer) Stop(ctx context.Context) error {
	ri.mu.Lock()
	defer ri.mu.Unlock()

	return ri.kvstore.Stop(ctx)
}

// Height returns the end height of the receipt indexer
func (ri *ReceiptIndexer) Height() (uint64, error) {
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	return ri.height, nil
}

// PutBlock puts the receipts of the block into kvstore
func (ri *ReceiptIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	height := blk.Height()
	if height > ri.endHeight {
		return nil
	}
	key := byteutil.Uint64ToBytes(height)
	if blk.Receipts == nil {
		return errors.Errorf("receipts of block %d is nil", height)
	}
	ri.mu.Lock()
	defer ri.mu.Unlock()
	logIndex := uint32(0)
	receipts := iotextypes.Receipts{}
	for i, receipt := range blk.Receipts {
		cr := receipt.CloneFixed()
		logIndex = cr.UpdateIndex(uint32(i), logIndex)
		receipts.Receipts = append(receipts.Receipts, cr.ConvertToReceiptPb())
	}
	receiptsBytes, err := proto.Marshal(&receipts)
	if err != nil {
		return err
	}

	b := batch.NewBatch()
	b.Put(_receiptsNS, key, receiptsBytes, fmt.Sprintf("failed to write receipts for block %d", height))
	if height > ri.height {
		ri.height = height
		b.Put(_receiptMetaNS, _heightKey, key, "failed to write receipt indexer height")
	}
	return ri.kvstore.WriteBatch(b)
}

// Receipts returns the receipts of the block at the given height
func (ri *ReceiptIndexer) Receipts(height uint64) ([]*action.Receipt, error) {
	key := byteutil.Uint64ToBytes(height)
	ri.mu.RLock()
	defer ri.mu.RUnlock()
	if height > ri.endHeight {
		return nil, errors.Wrapf(ErrIndexOutOfRange, "height %d > endHeight %d", height, ri.endHeight)
	}
	value, err := ri.kvstore.Get(_receiptsNS, key)
	if err != nil {
		return nil, err
	}
	receiptsPb := &iotextypes.Receipts{}
	if err := proto.Unmarshal(value, receiptsPb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block receipts")
	}

	var receipts []*action.Receipt
	for _, receiptPb := range receiptsPb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		receipts = append(receipts, receipt)
	}
	return receipts, nil
}
