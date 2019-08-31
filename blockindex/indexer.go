// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"bytes"
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	hashOffset                = 12
	blockHashHeightMappingNS  = "h2h"
	blockActionBlockMappingNS = "a2b"
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
		DeleteIndex(*block.Block) error
		GetBlockchainHeight() (uint64, error)
		GetBlockHash(height uint64) (hash.Hash256, error)
		GetBlockHeight(hash hash.Hash256) (uint64, error)
		GetBlockIndex(uint64) (*blockIndex, error)
		GetActionIndex([]byte) (*actionIndex, error)
		GetTotalActions() (uint64, error)
		GetActionHashFromIndex(uint64, uint64) ([][]byte, error)
		GetBlockHashByActionHash(hash.Hash256) (hash.Hash256, error)
		GetActionCountByAddress(hash.Hash160) (uint64, error)
		GetActionsByAddress(hash.Hash160, uint64, uint64) ([][]byte, error)
	}

	// blockIndexer implements the Indexer interface
	blockIndexer struct {
		kvstore   db.KVStore
		batch     db.KVStoreBatch
		dirtyAddr addrIndex
	}
)

// NewIndexer creates a new indexer
func NewIndexer(kv db.KVStore) (Indexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvstore")
	}
	x := blockIndexer{
		kv, db.NewBatch(), make(addrIndex),
	}
	return &x, nil
}

// Start starts the indexer
func (x *blockIndexer) Start(ctx context.Context) error {
	return x.kvstore.Start(ctx)
}

// Stop stops the indexer
func (x *blockIndexer) Stop(ctx context.Context) error {
	return x.kvstore.Stop(ctx)
}

// Commit writes the batch to DB
func (x *blockIndexer) Commit() error {
	for _, v := range x.dirtyAddr {
		if err := v.Commit(); err != nil {
			return err
		}
	}
	x.dirtyAddr = nil
	x.dirtyAddr = make(addrIndex)
	return x.kvstore.Commit(x.batch)
}

// IndexBlock index the block
func (x *blockIndexer) IndexBlock(blk *block.Block, batch bool) error {
	index, err := x.getIndexerForAddr(totalBlocksBucket, batch)
	if err != nil {
		return err
	}

	// TODO: check height+1
	height := blk.Height()

	// index hash --> height
	hash := blk.HashBlock()
	if !batch {
		err = x.kvstore.Put(blockHashHeightMappingNS, hash[hashOffset:], byteutil.Uint64ToBytesBigEndian(height))
	} else {
		x.batch.Put(blockHashHeightMappingNS, hash[hashOffset:], byteutil.Uint64ToBytesBigEndian(height), "failed to put hash -> height mapping")
	}
	if err != nil {
		errors.Wrap(err, "failed to put hash --> height index")
	}

	// index height --> block hash, number of actions, and total transfer amount
	bd := &blockIndex{hash[:], uint32(len(blk.Actions)), blk.CalculateTransferAmount()}
	return index.Add(bd.Serialize(), batch)
}

// IndexAction index the actions in the block
func (x *blockIndexer) IndexAction(blk *block.Block) error {
	// get indexer for total actions
	total, err := x.getIndexerForAddr(totalActionsBucket, true)
	if err != nil {
		return err
	}

	// store hash/height of the block, so getReceiptByActionHash() can use height to directly pull receipts
	hash := blk.HashBlock()
	ad := (&actionIndex{blk.Height(), hash[:]}).Serialize()

	// index actions in the block
	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		x.batch.Put(blockActionBlockMappingNS, actHash[hashOffset:], ad, "failed to put action hash %x", actHash)
		// add to total account index
		if err := total.Add(actHash[:], true); err != nil {
			return err
		}
		if err := x.indexAction(actHash, selp, true); err != nil {
			return err
		}
	}
	return nil
}

// DeleteIndex deletes a block's index
func (x *blockIndexer) DeleteIndex(blk *block.Block) error {
	// delete hash --> height
	hash := blk.HashBlock()
	x.batch.Delete(blockHashHeightMappingNS, hash[hashOffset:], "failed to delete hash --> height index")

	// delete action index
	for _, selp := range blk.Actions {
		actHash := selp.Hash()
		x.batch.Delete(blockActionBlockMappingNS, actHash[hashOffset:], "failed to delete action hash %x", actHash)
		if err := x.indexAction(actHash, selp, false); err != nil {
			return err
		}
	}
	if err := x.Commit(); err != nil {
		return err
	}

	// delete block index
	index, err := x.getIndexerForAddr(totalBlocksBucket, false)
	if err != nil {
		return err
	}
	if err := index.Revert(1); err != nil {
		return err
	}

	// delete from total action index
	total, err := x.getIndexerForAddr(totalActionsBucket, false)
	if err != nil {
		return err
	}
	return total.Revert(uint64(len(blk.Actions)))
}

// GetBlockchainHeight return the blockchain height
func (x *blockIndexer) GetBlockchainHeight() (uint64, error) {
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
	value, err := x.kvstore.Get(blockHashHeightMappingNS, hash[hashOffset:])
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
	index, err := x.kvstore.CountingIndex(totalBlocksBucket)
	if err != nil {
		return nil, err
	}
	v, err := index.Get(height - 1)
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
	v, err := x.kvstore.Get(blockActionBlockMappingNS, h[hashOffset:])
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
	total, err := x.kvstore.CountingIndex(totalActionsBucket)
	if err != nil {
		return 0, err
	}
	return total.Size(), nil
}

// GetActionHashFromIndex return hash of actions[start, start+count)
func (x *blockIndexer) GetActionHashFromIndex(start, count uint64) ([][]byte, error) {
	total, err := x.kvstore.CountingIndex(totalActionsBucket)
	if err != nil {
		return nil, err
	}
	return total.Range(start, count)
}

// GetBlockHashByActionHash return block hash of an action
func (x *blockIndexer) GetBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error) {
	v, err := x.kvstore.Get(blockActionBlockMappingNS, h[hashOffset:])
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to get action %x", h)
	}
	ad := &actionIndex{}
	if err := ad.Deserialize(v); err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to decode action %x", h)
	}
	if len(ad.blkHash) == 0 {
		return hash.ZeroHash256, errors.Wrapf(db.ErrNotExist, "action %x missing", h)
	}
	return hash.BytesToHash256(ad.blkHash), nil
}

// GetActionCountByAddress return total number of actions of an address
func (x *blockIndexer) GetActionCountByAddress(addrBytes hash.Hash160) (uint64, error) {
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
	address, err := x.kvstore.CountingIndex(addrBytes[:])
	if err != nil {
		return nil, err
	}
	total := address.Size()
	if start+count > total {
		count = total - start
	}
	return address.Range(start, count)
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
