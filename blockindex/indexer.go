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
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	hashOffset          = 12
	blockHashToHeightNS = "hh"
	actionToBlockHashNS = "ab"
)

var (
	totalBlocksBucket  = []byte("tbk")
	totalActionsBucket = []byte("tac")
)

type (
	addrIndex map[hash.Hash160]db.CountingIndex

	// Indexer is the interface for block indexer
	Indexer interface {
		Start(context.Context) error
		Stop(context.Context) error
		Commit() error
		IndexBlock(*block.Block, bool) error
		IndexAction(*block.Block) error
		DeleteBlockIndex(*block.Block) error
		DeleteActionIndex(*block.Block) error
		RevertBlocks(uint64) error
		GetBlockchainHeight() (uint64, error)
		GetBlockHash(height uint64) (hash.Hash256, error)
		GetBlockHeight(hash hash.Hash256) (uint64, error)
		GetBlockIndex(uint64) (*blockIndex, error)
		GetActionIndex([]byte) (*actionIndex, error)
		GetTotalActions() (uint64, error)
		GetActionHashFromIndex(uint64, uint64) ([][]byte, error)
		GetBlockHeightByActionHash(hash.Hash256) (uint64, error)
		GetActionCountByAddress(hash.Hash160) (uint64, error)
		GetActionsByAddress(hash.Hash160, uint64, uint64) ([][]byte, error)
	}

	// blockIndexer implements the Indexer interface
	blockIndexer struct {
		mutex     sync.RWMutex
		kvstore   db.KVStore
		batch     db.KVStoreBatch
		dirtyAddr addrIndex
		tbk       db.CountingIndex
		tac       db.CountingIndex
	}
)

// NewIndexer creates a new indexer
func NewIndexer(kv db.KVStore) (Indexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvstore")
	}
	x := blockIndexer{
		kvstore:   kv,
		batch:     db.NewBatch(),
		dirtyAddr: make(addrIndex),
	}
	return &x, nil
}

// Start starts the indexer
func (x *blockIndexer) Start(ctx context.Context) error {
	if err := x.kvstore.Start(ctx); err != nil {
		return err
	}
	// create the total block and action index
	var err error
	if x.tbk, err = x.kvstore.CreateCountingIndexNX(totalBlocksBucket); err != nil {
		return err
	}
	x.tac, err = x.kvstore.CreateCountingIndexNX(totalActionsBucket)
	return err
}

// Stop stops the indexer
func (x *blockIndexer) Stop(ctx context.Context) error {
	return x.kvstore.Stop(ctx)
}

// Commit writes the batch to DB
func (x *blockIndexer) Commit() error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	return x.commit()
}

// IndexBlock index the block
func (x *blockIndexer) IndexBlock(blk *block.Block, batch bool) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// the block to be indexed must be exactly current top + 1, otherwise counting index would not work correctly
	height := blk.Height()
	if height != x.tbk.Size()+1 {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, x.tbk.Size()+1)
	}
	// index hash --> height
	hash := blk.HashBlock()
	var err error
	if !batch {
		err = x.kvstore.Put(blockHashToHeightNS, hash[hashOffset:], byteutil.Uint64ToBytesBigEndian(height))
	} else {
		x.batch.Put(blockHashToHeightNS, hash[hashOffset:], byteutil.Uint64ToBytesBigEndian(height), "failed to put hash -> height mapping")
	}
	if err != nil {
		return errors.Wrap(err, "failed to put hash --> height index")
	}
	// index height --> block hash, number of actions, and total transfer amount
	bd := &blockIndex{
		hash:      hash[:],
		numAction: uint32(len(blk.Actions)),
		tsfAmount: blk.CalculateTransferAmount()}
	return x.tbk.Add(bd.Serialize(), batch)
}

// IndexAction index the actions in the block
func (x *blockIndexer) IndexAction(blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// store height of the block, so getReceiptByActionHash() can use height to directly pull receipts
	ad := (&actionIndex{
		blkHeight: blk.Height()}).Serialize()
	// index actions in the block
	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		x.batch.Put(actionToBlockHashNS, actHash[hashOffset:], ad, "failed to put action hash %x", actHash)
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

// DeleteBlockIndex deletes a block's index
func (x *blockIndexer) DeleteBlockIndex(blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// the block to be deleted must be exactly current top, otherwise counting index would not work correctly
	height := blk.Height()
	if height != x.tbk.Size() {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, x.tbk.Size())
	}
	// delete hash --> height
	hash := blk.HashBlock()
	x.kvstore.Delete(blockHashToHeightNS, hash[hashOffset:])
	// delete from total block index
	return x.tbk.Revert(1)
}

// DeleteActionIndex deletes action index in a block
func (x *blockIndexer) DeleteActionIndex(blk *block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	// delete action index
	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		x.batch.Delete(actionToBlockHashNS, actHash[hashOffset:], "failed to delete action hash %x", actHash)
		if err := x.indexAction(actHash, selp, false); err != nil {
			return err
		}
	}
	if err := x.commit(); err != nil {
		return err
	}
	// delete from total action index
	return x.tac.Revert(uint64(len(blk.Actions)))
}

// RevertBlocks revert the top 'n' blocks
func (x *blockIndexer) RevertBlocks(n uint64) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	if n == 0 || n > x.tbk.Size() {
		return nil
	}
	return x.tbk.Revert(n)
}

// GetBlockchainHeight return the blockchain height
func (x *blockIndexer) GetBlockchainHeight() (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	index, err := x.kvstore.CountingIndex(totalBlocksBucket)
	if err != nil {
		if errors.Cause(err) == db.ErrBucketNotExist {
			// counting index does not exist yet
			return 0, nil
		}
		return 0, err
	}
	return index.Size(), nil
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

	value, err := x.kvstore.Get(blockHashToHeightNS, hash[hashOffset:])
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

	if height == 0 {
		return &blockIndex{hash.ZeroHash256[:], 0, big.NewInt(0)}, nil
	}
	v, err := x.tbk.Get(height - 1)
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

	v, err := x.kvstore.Get(actionToBlockHashNS, h[hashOffset:])
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

	total, err := x.kvstore.CountingIndex(totalActionsBucket)
	if err != nil {
		return 0, err
	}
	return total.Size(), nil
}

// GetActionHashFromIndex return hash of actions[start, start+count)
func (x *blockIndexer) GetActionHashFromIndex(start, count uint64) ([][]byte, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	return x.tac.Range(start, count)
}

// GetBlockHeightByActionHash return block height of an action
func (x *blockIndexer) GetBlockHeightByActionHash(h hash.Hash256) (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	v, err := x.kvstore.Get(actionToBlockHashNS, h[hashOffset:])
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get action %x", h)
	}
	ad := &actionIndex{}
	if err := ad.Deserialize(v); err != nil {
		return 0, errors.Wrapf(err, "failed to decode action %x", h)
	}
	return ad.blkHeight, nil
}

// GetActionCountByAddress return total number of actions of an address
func (x *blockIndexer) GetActionCountByAddress(addrBytes hash.Hash160) (uint64, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	address, err := x.kvstore.CountingIndex(addrBytes[:])
	if err != nil {
		if errors.Cause(err) == db.ErrBucketNotExist {
			return 0, nil
		}
		return 0, err
	}
	return address.Size(), nil
}

// GetActionsByAddress return hash of an address's actions[start, start+count)
func (x *blockIndexer) GetActionsByAddress(addrBytes hash.Hash160, start, count uint64) ([][]byte, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	address, err := x.kvstore.CountingIndex(addrBytes[:])
	if err != nil {
		return nil, err
	}
	total := address.Size()
	if start >= total {
		return nil, errors.Wrapf(db.ErrInvalid, "start = %d >= total = %d", start, total)
	}
	if start+count > total {
		count = total - start
	}
	return address.Range(start, count)
}

// commit() writes the changes
func (x *blockIndexer) commit() error {
	var commitErr error
	for k, v := range x.dirtyAddr {
		if commitErr == nil {
			if err := v.Commit(); err != nil {
				commitErr = err
			}
		}
		delete(x.dirtyAddr, k)
	}
	if commitErr != nil {
		return commitErr
	}
	// total block and total action index
	if err := x.tbk.Commit(); err != nil {
		return err
	}
	if err := x.tac.Commit(); err != nil {
		return err
	}
	return x.kvstore.Commit(x.batch)
}

// getIndexerForAddr returns the counting indexer for an address
// if batch is true, the indexer will be placed into a dirty map, to be committed later
func (x *blockIndexer) getIndexerForAddr(addr []byte, batch bool) (db.CountingIndex, error) {
	if !batch {
		return x.kvstore.CreateCountingIndexNX(addr)
	}
	address := hash.BytesToHash160(addr)
	indexer, ok := x.dirtyAddr[address]
	if !ok {
		// create indexer for addr if not exist
		var err error
		indexer, err = x.kvstore.CreateCountingIndexNX(addr)
		if err != nil {
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

	if bytes.Compare(dstAddrBytes, callerAddrBytes) == 0 {
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
