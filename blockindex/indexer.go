// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"bytes"
	"context"
	"math/big"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

// the NS/bucket name here are used in index.db, which is separate from chain.db
// still we use 2-byte NS/bucket name here, to clearly differentiate from those (3-byte) in BlockDAO
const (
	// first 12-byte of hash is cut off, only last 20-byte is written to DB to reduce storage
	_hashOffset          = 12
	_blockHashToHeightNS = "hh"
	_actionToBlockHashNS = "ab"
)

var (
	_totalBlocksBucket  = []byte("bk")
	_totalActionsBucket = []byte("ac")
	// ErrActionIndexNA indicates action index is not supported
	ErrActionIndexNA = errors.New("action index not supported")
)

type (
	addrIndex map[hash.Hash160]db.CountingIndex

	// Indexer is the interface for block indexer
	Indexer interface {
		Start(context.Context) error
		Stop(context.Context) error
		PutBlock(context.Context, *block.Block) error
		PutBlocks(context.Context, []*block.Block) error
		DeleteTipBlock(context.Context, *block.Block) error
		Height() (uint64, error)
		GetBlockHash(height uint64) (hash.Hash256, error)
		GetBlockHeight(hash hash.Hash256) (uint64, error)
		GetBlockIndex(uint64) (*blockIndex, error)
		GetActionIndex([]byte) (*actionIndex, error)
		GetTotalActions() (uint64, error)
		GetActionHashFromIndex(uint64, uint64) ([][]byte, error)
		GetActionCountByAddress(hash.Hash160) (uint64, error)
		GetActionsByAddress(hash.Hash160, uint64, uint64) ([][]byte, error)
	}

	// blockIndexer implements the Indexer interface
	blockIndexer struct {
		mutex       sync.RWMutex
		genesisHash hash.Hash256
		kvStore     db.KVStoreWithRange
		batch       batch.KVStoreBatch
		dirtyAddr   addrIndex
		tbk         db.CountingIndex
		tac         db.CountingIndex
	}
)

// NewIndexer creates a new indexer
func NewIndexer(kv db.KVStore, genesisHash hash.Hash256) (Indexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	kvRange, ok := kv.(db.KVStoreWithRange)
	if !ok {
		return nil, errors.New("indexer can only be created from KVStoreWithRange")
	}
	x := blockIndexer{
		kvStore:     kvRange,
		batch:       batch.NewBatch(),
		dirtyAddr:   make(addrIndex),
		genesisHash: genesisHash,
	}
	return &x, nil
}

// Start starts the indexer
func (x *blockIndexer) Start(ctx context.Context) error {
	if err := x.kvStore.Start(ctx); err != nil {
		return err
	}
	// create the total block and action index
	var err error
	if x.tbk, err = db.NewCountingIndexNX(x.kvStore, _totalBlocksBucket); err != nil {
		return err
	}
	if x.tbk.Size() == 0 {
		// insert genesis block
		if err = x.tbk.Add((&blockIndex{
			x.genesisHash[:],
			0,
			big.NewInt(0)}).Serialize(), false); err != nil {
			return err
		}
	}
	x.tac, err = db.NewCountingIndexNX(x.kvStore, _totalActionsBucket)
	return err
}

// Stop stops the indexer
func (x *blockIndexer) Stop(ctx context.Context) error {
	return x.kvStore.Stop(ctx)
}

// PutBlocks writes the batch to DB
func (x *blockIndexer) PutBlocks(ctx context.Context, blks []*block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()
	for _, blk := range blks {
		if err := x.putBlock(ctx, blk); err != nil {
			// TODO: Revert changes
			return err
		}
	}
	return x.commit()
}

// PutBlock index the block
func (x *blockIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	if err := x.putBlock(ctx, blk); err != nil {
		return err
	}
	return x.commit()
}

// DeleteBlock deletes a block's index
func (x *blockIndexer) DeleteTipBlock(ctx context.Context, blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// the block to be deleted must be exactly current top, otherwise counting index would not work correctly
	height := blk.Height()
	if height != x.tbk.Size()-1 {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, x.tbk.Size()-1)
	}
	// delete hash --> height
	hash := blk.HashBlock()
	x.batch.Delete(_blockHashToHeightNS, hash[_hashOffset:], "failed to delete block at height %d", height)
	// delete from total block index
	if err := x.tbk.Revert(1); err != nil {
		return err
	}

	// delete action index
	for _, selp := range blk.Actions {
		actHash, err := selp.Hash()
		if err != nil {
			return err
		}
		x.batch.Delete(_actionToBlockHashNS, actHash[_hashOffset:], "failed to delete action hash %x", actHash)
		if err := x.indexAction(actHash, selp, false); err != nil {
			return err
		}
	}
	// delete from total action index
	if err := x.tac.Revert(uint64(len(blk.Actions))); err != nil {
		return err
	}
	if err := x.kvStore.WriteBatch(x.batch); err != nil {
		return err
	}
	x.batch.Clear()
	return nil
}

// Height return the blockchain height
func (x *blockIndexer) Height() (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()
	return x.tbk.Size() - 1, nil
}

// GetBlockHash returns the block hash by height
func (x *blockIndexer) GetBlockHash(height uint64) (hash.Hash256, error) {
	index, err := x.GetBlockIndex(height)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get block hash")
	}
	return hash.BytesToHash256(index.Hash()), nil
}

// GetBlockHeight returns the block height by hash
func (x *blockIndexer) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	value, err := x.kvStore.Get(_blockHashToHeightNS, hash[_hashOffset:])
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	if len(value) == 0 {
		return 0, errors.Wrapf(db.ErrNotExist, "height missing for block with hash = %x", hash)
	}
	return byteutil.BytesToUint64BigEndian(value), nil
}

// GetBlockIndex return the index of block
func (x *blockIndexer) GetBlockIndex(height uint64) (*blockIndex, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	v, err := x.tbk.Get(height)
	if err != nil {
		return nil, err
	}
	b := &blockIndex{}
	if err := b.Deserialize(v); err != nil {
		return nil, err
	}
	return b, nil
}

// GetActionIndex return the index of action
func (x *blockIndexer) GetActionIndex(h []byte) (*actionIndex, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	v, err := x.kvStore.Get(_actionToBlockHashNS, h[_hashOffset:])
	if err != nil {
		return nil, err
	}
	a := &actionIndex{}
	if err := a.Deserialize(v); err != nil {
		return nil, err
	}
	return a, nil
}

// GetTotalActions return total number of all actions
func (x *blockIndexer) GetTotalActions() (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()
	return x.tac.Size(), nil
}

// GetActionHashFromIndex return hash of actions[start, start+count)
func (x *blockIndexer) GetActionHashFromIndex(start, count uint64) ([][]byte, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	return x.tac.Range(start, count)
}

// GetActionCountByAddress return total number of actions of an address
func (x *blockIndexer) GetActionCountByAddress(addrBytes hash.Hash160) (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	addr, err := db.GetCountingIndex(x.kvStore, addrBytes[:])
	if err != nil {
		if errors.Cause(err) == db.ErrBucketNotExist || errors.Cause(err) == db.ErrNotExist {
			return 0, nil
		}
		return 0, err
	}
	return addr.Size(), nil
}

// GetActionsByAddress return hash of an address's actions[start, start+count)
func (x *blockIndexer) GetActionsByAddress(addrBytes hash.Hash160, start, count uint64) ([][]byte, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	addr, err := db.GetCountingIndex(x.kvStore, addrBytes[:])
	if err != nil {
		return nil, err
	}
	total := addr.Size()
	if start >= total {
		return nil, errors.Wrapf(db.ErrInvalid, "start = %d >= total = %d", start, total)
	}
	if start+count > total {
		count = total - start
	}
	return addr.Range(start, count)
}

func (x *blockIndexer) putBlock(ctx context.Context, blk *block.Block) error {
	// the block to be indexed must be exactly current top + 1, otherwise counting index would not work correctly
	height := blk.Height()
	if height != x.tbk.Size() {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, x.tbk.Size())
	}

	// index hash --> height
	hash := blk.HashBlock()
	x.batch.Put(_blockHashToHeightNS, hash[_hashOffset:], byteutil.Uint64ToBytesBigEndian(height), "failed to put hash -> height mapping")

	// index height --> block hash, number of actions, and total transfer amount
	bd := &blockIndex{
		hash:      hash[:],
		numAction: uint32(len(blk.Actions)),
		tsfAmount: blk.CalculateTransferAmount()}
	if err := x.tbk.UseBatch(x.batch); err != nil {
		return err
	}
	if err := x.tbk.Add(bd.Serialize(), true); err != nil {
		return errors.Wrapf(err, "failed to put block %d index", height)
	}

	// store height of the block, so getReceiptByActionHash() can use height to directly pull receipts
	ad := (&actionIndex{
		blkHeight: blk.Height()}).Serialize()
	if err := x.tac.UseBatch(x.batch); err != nil {
		return err
	}
	// index actions in the block
	for _, selp := range blk.Actions {
		actHash, err := selp.Hash()
		if err != nil {
			return err
		}
		x.batch.Put(_actionToBlockHashNS, actHash[_hashOffset:], ad, "failed to put action hash %x", actHash)
		// add to total account index
		if err := x.tac.Add(actHash[:], true); err != nil {
			return err
		}
		if err := x.indexAction(actHash, selp, true); err != nil {
			return err
		}
	}
	return nil
}

// commit writes the changes
func (x *blockIndexer) commit() error {
	var commitErr error
	for k, v := range x.dirtyAddr {
		if commitErr == nil {
			if err := v.Finalize(); err != nil {
				commitErr = err
			}
		}
		delete(x.dirtyAddr, k)
	}
	if commitErr != nil {
		return commitErr
	}
	// total block and total action index
	if err := x.tbk.Finalize(); err != nil {
		return err
	}
	if err := x.tac.Finalize(); err != nil {
		return err
	}
	if err := x.kvStore.WriteBatch(x.batch); err != nil {
		return err
	}
	x.batch.Clear()
	return nil
}

// getIndexerForAddr returns the counting indexer for an address
// if batch is true, the indexer will be placed into a dirty map, to be committed later
func (x *blockIndexer) getIndexerForAddr(addr []byte, batch bool) (db.CountingIndex, error) {
	if !batch {
		return db.NewCountingIndexNX(x.kvStore, addr)
	}
	address := hash.BytesToHash160(addr)
	indexer, ok := x.dirtyAddr[address]
	if !ok {
		// create indexer for addr if not exist
		var err error
		indexer, err = db.NewCountingIndexNX(x.kvStore, addr)
		if err != nil {
			return nil, err
		}
		if err := indexer.UseBatch(x.batch); err != nil {
			return nil, err
		}
		x.dirtyAddr[address] = indexer
	}
	return indexer, nil
}

// indexAction builds index for an action
func (x *blockIndexer) indexAction(actHash hash.Hash256, elp action.SealedEnvelope, insert bool) error {
	// add to sender's index
	callerAddrBytes := elp.SrcPubkey().Hash()
	sender, err := x.getIndexerForAddr(callerAddrBytes, insert)
	if err != nil {
		return err
	}
	if insert {
		err = sender.Add(actHash[:], insert)
	} else {
		err = sender.Revert(1)
	}
	if err != nil {
		return err
	}

	dst, ok := elp.Destination()
	if !ok || dst == "" {
		return nil
	}
	dstAddr, err := address.FromString(dst)
	if err != nil {
		return err
	}
	dstAddrBytes := dstAddr.Bytes()

	if bytes.Equal(dstAddrBytes, callerAddrBytes) {
		// recipient is same as sender
		return nil
	}

	// add to recipient's index
	recipient, err := x.getIndexerForAddr(dstAddrBytes, insert)
	if err != nil {
		return err
	}
	if insert {
		err = recipient.Add(actHash[:], insert)
	} else {
		err = recipient.Revert(1)
	}
	return err
}
