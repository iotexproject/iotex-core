// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package systemlog

import (
	"context"
	"sync"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/systemlog/systemlogpb"
)

const (
	indexerStatus = "is"
	evmTransferNS = "et"
)

var (
	blockHeightPrefix = []byte("bh.")
	actionHashPrefix  = []byte("ah.")
	tipBlockHeightKey = []byte("tipHeight")
)

// Indexer is the indexer for system log
type Indexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStoreWithRange
}

// NewIndexer creates a new indexer
func NewIndexer(kv db.KVStore) (*Indexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	kvRange, ok := kv.(db.KVStoreWithRange)
	if !ok {
		return nil, errors.New("indexer can only be created from KVStoreWithRange")
	}
	return &Indexer{kvStore: kvRange}, nil
}

func (x *Indexer) tipHeight() (uint64, error) {
	value, err := x.kvStore.Get(indexerStatus, tipBlockHeightKey)
	if err != nil {
		return uint64(0), err
	}
	return byteutil.BytesToUint64(value), nil
}

// Start starts the indexer
func (x *Indexer) Start(ctx context.Context) error {
	if err := x.kvStore.Start(ctx); err != nil {
		return err
	}
	_, err := x.tipHeight()
	switch errors.Cause(err) {
	case db.ErrNotExist:
		if err := x.kvStore.Put(indexerStatus, tipBlockHeightKey, byteutil.Uint64ToBytes(0)); err != nil {
			return errors.Wrap(err, "failed to initialize tip block height")
		}
	case nil:
		break
	default:
		return err
	}

	return nil
}

// Stop stops the indexer
func (x *Indexer) Stop(ctx context.Context) error {
	return x.kvStore.Stop(ctx)
}

// Height returns the tip height of the indexer
func (x *Indexer) Height() (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()
	return x.tipHeight()
}

// PutBlock indexes the block
func (x *Indexer) PutBlock(_ context.Context, blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// the block to be indexed must be exactly current top + 1
	tipHeight, err := x.tipHeight()
	if err != nil {
		return err
	}
	height := blk.Height()
	if height != tipHeight+1 {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, tipHeight+1)
	}
	batch := batch.NewBatch()
	actionHashList := &systemlogpb.ActionHashList{}
	for _, receipt := range blk.Receipts {
		if receipt.Status == uint64(iotextypes.ReceiptStatus_Failure) {
			continue
		}

		evmTransferList := &iotextypes.EvmTransferList{}
		for _, l := range receipt.Logs {
			if action.IsSystemLog(l) {
				// TODO: switch different kinds of system log
				fromAddr, err := address.FromBytes(l.Topics[1][12:])
				if err != nil {
					return errors.Wrap(err, "failed to convert IoTeX address")
				}
				toAddr, err := address.FromBytes(l.Topics[2][12:])
				if err != nil {
					return errors.Wrap(err, "failed to convert IoTeX address")
				}
				evmTransferList.EvmTransfers = append(evmTransferList.EvmTransfers, &iotextypes.EvmTransfer{
					Amount: l.Data,
					From:   fromAddr.String(),
					To:     toAddr.String(),
				})
			}
		}

		if len(evmTransferList.EvmTransfers) > 0 {
			data, err := proto.Marshal(evmTransferList)
			if err != nil {
				return errors.Wrap(err, "failed to serialize EvmTransferList")
			}

			actionKey := actionKey(receipt.ActionHash)
			batch.Put(evmTransferNS, actionKey, data,
				"failed to put actionHash -> EvmTransferList mapping")

			actionHashList.ActionHashList = append(actionHashList.ActionHashList, receipt.ActionHash[:])
		}

	}
	if len(actionHashList.ActionHashList) > 0 {
		blockKey := blockKey(blk.Height())
		data, err := proto.Marshal(actionHashList)
		if err != nil {
			return errors.Wrap(err, "failed to serialize blockEvmTransfer")
		}
		batch.Put(evmTransferNS, blockKey, data,
			"failed to put blockHeight -> actionHashList mapping")
	}
	batch.Put(
		indexerStatus,
		tipBlockHeightKey,
		byteutil.Uint64ToBytes(blk.Height()),
		"failed to update tip block height",
	)

	return x.kvStore.WriteBatch(batch)
}

// DeleteTipBlock deletes a block's evm transfer
func (x *Indexer) DeleteTipBlock(blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	tipHeight, err := x.tipHeight()
	if err != nil {
		return err
	}
	// the block to be deleted must be exactly current top
	height := blk.Height()
	if height != tipHeight {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, tipHeight)
	}
	batch := batch.NewBatch()

	// delete blockEvmTransfer and actionEvmTransfer
	blockKey := blockKey(blk.Height())
	data, err := x.kvStore.Get(evmTransferNS, blockKey)
	if err != nil {
		return errors.Wrapf(err, "failed to get ActionHashList of block %d", height)
	}
	batch.Delete(evmTransferNS, blockKey, "failed to delete blockHeight -> ActionHashList")

	actionHashList := &systemlogpb.ActionHashList{}
	if err := proto.Unmarshal(data, actionHashList); err != nil {
		return errors.Wrap(err, "failed to deserialize ActionHashList")
	}

	for _, actionHash := range actionHashList.ActionHashList {
		actionKey := actionKey(hash.BytesToHash256(actionHash))
		batch.Delete(evmTransferNS, actionKey, "failed to delete actionHash -> EvmTransferList")
	}
	batch.Put(
		indexerStatus,
		tipBlockHeightKey,
		byteutil.Uint64ToBytes(blk.Height()-1),
		"failed to update tip block height",
	)

	return x.kvStore.WriteBatch(batch)
}

// GetEvmTransfersByActionHash queries evm transfers by action hash
func (x *Indexer) GetEvmTransfersByActionHash(actionHash hash.Hash256) (*iotextypes.ActionEvmTransfer, error) {
	data, err := x.kvStore.Get(evmTransferNS, actionKey(actionHash))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get evm transfers by action hash")
	}

	etl := &iotextypes.EvmTransferList{}
	if err := proto.UnmarshalMerge(data, etl); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize ActionEvmTransfer")
	}
	pb := &iotextypes.ActionEvmTransfer{
		ActionHash:      actionHash[:],
		NumEvmTransfers: uint64(len(etl.EvmTransfers)),
		EvmTransfers:    etl.EvmTransfers,
	}

	return pb, nil
}

// GetEvmTransfersByBlockHeight queries evm transfers by block height
func (x *Indexer) GetEvmTransfersByBlockHeight(blockHeight uint64) (*iotextypes.BlockEvmTransfer, error) {
	data, err := x.kvStore.Get(evmTransferNS, blockKey(blockHeight))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get evm transfers by block height")
	}

	actionHashList := &systemlogpb.ActionHashList{}
	if err := proto.Unmarshal(data, actionHashList); err != nil {
		return nil, errors.Wrap(err, "failed to serialize ActionHashList")
	}

	pb := &iotextypes.BlockEvmTransfer{BlockHeight: blockHeight}
	for _, actionHash := range actionHashList.ActionHashList {
		data, err := x.kvStore.Get(evmTransferNS, actionKey(hash.BytesToHash256(actionHash)))
		if err != nil {
			return nil, errors.Wrap(err, "failed to get evm transfer by action hash")
		}

		etl := &iotextypes.EvmTransferList{}
		if err := proto.Unmarshal(data, etl); err != nil {
			return nil, errors.Wrap(err, "failed to deserialize ActionEvmTransfer")
		}

		pb.ActionEvmTransfers = append(pb.ActionEvmTransfers, &iotextypes.ActionEvmTransfer{
			ActionHash:      actionHash,
			NumEvmTransfers: uint64(len(etl.EvmTransfers)),
			EvmTransfers:    etl.EvmTransfers,
		})

		pb.NumEvmTransfers += uint64(len(etl.EvmTransfers))
	}

	return pb, nil
}

func blockKey(height uint64) []byte {
	return append(blockHeightPrefix, byteutil.Uint64ToBytes(height)...)
}

func actionKey(h hash.Hash256) []byte {
	return append(actionHashPrefix, h[:]...)
}
