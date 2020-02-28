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
	tipBlockHeightKey = []byte("h")
)

// Indexer is the indexer for system log
type Indexer struct {
	mutex          sync.RWMutex
	kvStore        db.KVStoreWithRange
	batch          batch.KVStoreBatch
	tipBlockHeight uint64
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
	return &Indexer{
		kvStore: kvRange,
		batch:   batch.NewBatch(),
	}, nil
}

// Start starts the indexer
func (x *Indexer) Start(ctx context.Context) error {
	if _, err := x.kvStore.Get(indexerStatus, tipBlockHeightKey); err != nil && errors.Cause(err) == db.ErrNotExist {
		if err := x.kvStore.Put(indexerStatus, tipBlockHeightKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to initialize tip block height")
		}
	}
	value, err := x.kvStore.Get(indexerStatus, tipBlockHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get tip block height")
	}
	x.tipBlockHeight = byteutil.BytesToUint64(value)

	return x.kvStore.Start(ctx)
}

// Stop stops the indexer
func (x *Indexer) Stop(ctx context.Context) error {
	return x.kvStore.Stop(ctx)
}

// Commit writes the batch to DB
func (x *Indexer) Commit() error {
	x.mutex.Lock()
	defer x.mutex.Unlock()
	x.batch.Put(indexerStatus, tipBlockHeightKey, byteutil.Uint64ToBytes(x.tipBlockHeight),
		"failed to update tip block height")
	return x.kvStore.WriteBatch(x.batch)
}

// PutBlock indexes the block
func (x *Indexer) PutBlock(blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// the block to be indexed must be exactly current top + 1
	height := blk.Height()
	if height != x.tipBlockHeight+1 {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, x.tipBlockHeight+1)
	}

	actionHashList := &systemlogpb.ActionHashList{}
	for _, receipt := range blk.Receipts {
		evmTransferList := &systemlogpb.EvmTransferList{}
		for _, l := range receipt.Logs {
			if action.IsSystemLog(l) {
				// TODO: switch different kinds of system log
				fromAddr, err := address.FromBytes(l.Topics[1][:])
				if err != nil {
					return errors.Wrap(err, "failed to convert IoTeX address")
				}
				toAddr, err := address.FromBytes(l.Topics[2][:])
				if err != nil {
					return errors.Wrap(err, "failed to convert IoTeX address")
				}
				evmTransferList.EvmTransferList = append(evmTransferList.EvmTransferList, &systemlogpb.EvmTransfer{
					Amount: l.Data,
					From:   fromAddr.String(),
					To:     toAddr.String(),
				})
			}
		}

		if len(evmTransferList.EvmTransferList) > 0 {
			data, err := proto.Marshal(evmTransferList)
			if err != nil {
				return errors.Wrap(err, "failed to serialize EvmTransferList")
			}

			actionKey := actionKey(receipt.ActionHash)
			x.batch.Put(evmTransferNS, actionKey, data,
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
		x.batch.Put(evmTransferNS, blockKey, data,
			"failed to put blockHeight -> actionHashList mapping")
	}

	x.tipBlockHeight++

	return nil
}

// DeleteTipBlock deletes a block's evm transfer
func (x *Indexer) DeleteTipBlock(blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// the block to be deleted must be exactly current top
	height := blk.Height()
	if height != x.tipBlockHeight {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, x.tipBlockHeight)
	}

	// delete blockEvmTransfer and actionEvmTransfer
	blockKey := blockKey(blk.Height())
	data, err := x.kvStore.Get(evmTransferNS, blockKey)
	if err != nil {
		return errors.Wrapf(err, "failed to get ActionHashList of block %d", height)
	}
	x.batch.Delete(evmTransferNS, blockKey, "failed to delete blockHeight -> ActionHashList")

	actionHashList := &systemlogpb.ActionHashList{}
	if err := proto.Unmarshal(data, actionHashList); err != nil {
		return errors.Wrap(err, "failed to deserialize ActionHashList")
	}

	for _, actionHash := range actionHashList.ActionHashList {
		actionKey := actionKey(hash.BytesToHash256(actionHash))
		x.batch.Delete(evmTransferNS, actionKey, "failed to delete actionHash -> EvmTransferList")
	}

	x.tipBlockHeight--

	return nil
}

// GetEvmTransferByActionHash queries evm transfers by action hash
func (x *Indexer) GetEvmTransferByActionHash(actionHash hash.Hash256) (*systemlogpb.ActionEvmTransfer, error) {
	data, err := x.kvStore.Get(evmTransferNS, actionKey(actionHash))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get evm transfers by action hash")
	}

	pb := &systemlogpb.ActionEvmTransfer{}
	if err := proto.UnmarshalMerge(data, pb); err != nil {
		return nil, errors.Wrap(err, "failed to deserialize ActionEvmTransfer")
	}

	return pb, nil
}

// GetEvmTransferByBlockHeight queries evm transfers by block height
func (x *Indexer) GetEvmTransferByBlockHeight(blockHeight uint64) (*systemlogpb.BlockEvmTransfer, error) {
	data, err := x.kvStore.Get(evmTransferNS, blockKey(blockHeight))
	if err != nil {
		return nil, errors.Wrap(err, "failed to get evm transfers by block height")
	}

	actionHashList := &systemlogpb.ActionHashList{}
	if err := proto.Unmarshal(data, actionHashList); err != nil {
		return nil, errors.Wrap(err, "failed to serialize ActionHashList")
	}

	pb := &systemlogpb.BlockEvmTransfer{BlockHeight: blockHeight, NumEvmTransfer: 0}
	for _, actionHash := range actionHashList.ActionHashList {
		data, err := x.kvStore.Get(evmTransferNS, actionKey(hash.BytesToHash256(actionHash)))
		if err != nil {
			return nil, errors.Wrap(err, "failed to get evm transfer by action hash")
		}

		apb := &systemlogpb.ActionEvmTransfer{}
		if err := proto.Unmarshal(data, apb); err != nil {
			return nil, errors.Wrap(err, "failed to deserialize ActionEvmTransfer")
		}

		pb.ActionEvmTransferList = append(pb.ActionEvmTransferList, apb)
		pb.NumEvmTransfer += apb.NumEvmTransfer
	}

	return pb, nil
}

func blockKey(height uint64) []byte {
	return append(blockHeightPrefix, byteutil.Uint64ToBytes(height)...)
}

func actionKey(h hash.Hash256) []byte {
	return append(actionHashPrefix, h[:]...)
}
