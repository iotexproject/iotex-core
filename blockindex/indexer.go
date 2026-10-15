// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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
	// pendingIndex collects the entries of the counting indexes to be added in a
	// batch. Addresses are kept in the order they are first seen, so that the
	// batch is deterministic
	pendingIndex struct {
		blocks      [][]byte
		actions     [][]byte
		addrs       []hash.Hash160
		addrActions map[hash.Hash160][][]byte
	}

	// Indexer is the interface for block indexer
	Indexer interface {
		Start(context.Context) error
		Stop(context.Context) error
		PutBlock(context.Context, *block.Block) error
		PutBlocks(context.Context, []*block.Block) error
		Height() (uint64, error)
		GetBlockHash(height uint64) (hash.Hash256, error)
		GetBlockHeight(hash hash.Hash256) (uint64, error)
		GetBlockIndex(uint64) (*BlockIndex, error)
		GetActionIndex([]byte) (*ActionIndex, error)
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
		genesisHash: genesisHash,
	}
	return &x, nil
}

// Start starts the indexer
func (x *blockIndexer) Start(ctx context.Context) error {
	log.L().Debug("Starting block indexer...")
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
		if err = x.tbk.Add((&BlockIndex{
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

// PutBlocks indexes the blocks in a single batch. The in-memory sizes of the
// total block and action indexes are updated only after the batch is committed,
// so a failure leaves the indexer unchanged, and the same blocks can be put again
func (x *blockIndexer) PutBlocks(ctx context.Context, blks []*block.Block) error {
	x.mutex.Lock()
	defer x.mutex.Unlock()

	b := batch.NewBatch()
	p := newPendingIndex()
	for _, blk := range blks {
		if err := x.putBlock(ctx, b, p, blk); err != nil {
			return err
		}
	}
	apply, err := x.addToBatch(b, p)
	if err != nil {
		return err
	}
	if err := x.kvStore.WriteBatch(b); err != nil {
		return err
	}
	apply()
	return nil
}

// PutBlock index the block
func (x *blockIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	return x.PutBlocks(ctx, []*block.Block{blk})
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
func (x *blockIndexer) GetBlockIndex(height uint64) (*BlockIndex, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	v, err := x.tbk.Get(height)
	if err != nil {
		return nil, err
	}
	b := &BlockIndex{}
	if err := b.Deserialize(v); err != nil {
		return nil, err
	}
	return b, nil
}

// GetActionIndex return the index of action
func (x *blockIndexer) GetActionIndex(h []byte) (*ActionIndex, error) {
	x.mutex.RLock()
	defer x.mutex.RUnlock()

	v, err := x.kvStore.Get(_actionToBlockHashNS, h[_hashOffset:])
	if err != nil {
		return nil, errors.Wrapf(err, "action hash %x not found", h)
	}
	a := &ActionIndex{}
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

// putBlock puts the hash and action mappings of the block into the batch, and
// collects its entries of the counting indexes into p
func (x *blockIndexer) putBlock(ctx context.Context, b batch.KVStoreBatch, p *pendingIndex, blk *block.Block) error {
	// the block to be indexed must be exactly current top + 1, otherwise counting index would not work correctly
	height := blk.Height()
	if expect := x.tbk.Size() + uint64(len(p.blocks)); height != expect {
		return errors.Wrapf(db.ErrInvalid, "wrong block height %d, expecting %d", height, expect)
	}

	// index hash --> height
	hash := blk.HashBlock()
	b.Put(_blockHashToHeightNS, hash[_hashOffset:], byteutil.Uint64ToBytesBigEndian(height), "failed to put hash -> height mapping")

	// index height --> block hash, number of actions, and total transfer amount
	bd := &BlockIndex{
		hash:      hash[:],
		numAction: uint32(len(blk.Actions)),
		tsfAmount: blk.CalculateTransferAmount()}
	p.blocks = append(p.blocks, bd.Serialize())

	// index actions in the block
	fCtx := protocol.MustGetFeatureCtx(protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: blk.Height(),
	})))
	for idx, selp := range blk.Actions {
		actHash, err := selp.Hash()
		if err != nil {
			return err
		}
		// store height of the block, so getReceiptByActionHash() can use height to directly pull receipts
		ad := (&ActionIndex{blkHeight: blk.Height(), txNumber: uint32(idx + 1)}).Serialize()
		b.Put(_actionToBlockHashNS, actHash[_hashOffset:], ad, fmt.Sprintf("failed to put action hash %x", actHash))
		// add to total account index
		p.actions = append(p.actions, actHash[:])
		if err := indexAction(p, actHash, selp, fCtx.TolerateLegacyAddress); err != nil {
			return err
		}
	}
	return nil
}

// addToBatch puts the collected entries of the counting indexes into the batch,
// and returns a func that updates the total block and action indexes once the
// batch is committed. The address indexes are loaded per batch, so they need
// no update
func (x *blockIndexer) addToBatch(b batch.KVStoreBatch, p *pendingIndex) (func(), error) {
	for _, addr := range p.addrs {
		indexer, err := db.LoadCountingIndex(x.kvStore, addr[:])
		if err != nil {
			return nil, err
		}
		indexer.AddToBatch(b, p.addrActions[addr]...)
	}
	applyTbk := x.tbk.AddToBatch(b, p.blocks...)
	applyTac := x.tac.AddToBatch(b, p.actions...)
	return func() {
		applyTac()
		applyTbk()
	}, nil
}

// indexAction builds index for an action
func indexAction(p *pendingIndex, actHash hash.Hash256, elp *action.SealedEnvelope, tolerateLegacyAddress bool) error {
	// add to sender's index
	callerAddrBytes := elp.SrcPubkey().Hash()
	p.addAddrAction(callerAddrBytes, actHash)

	dst, ok := elp.Destination()
	if !ok || dst == "" {
		return nil
	}

	var (
		dstAddr address.Address
		err     error
	)
	if tolerateLegacyAddress {
		dstAddr, err = address.FromStringLegacy(dst)
	} else {
		dstAddr, err = address.FromString(dst)
	}
	if err != nil {
		return err
	}
	dstAddrBytes := dstAddr.Bytes()

	if bytes.Equal(dstAddrBytes, callerAddrBytes) {
		// recipient is same as sender
		return nil
	}

	// add to recipient's index
	p.addAddrAction(dstAddrBytes, actHash)
	return nil
}

func newPendingIndex() *pendingIndex {
	return &pendingIndex{
		addrActions: make(map[hash.Hash160][][]byte),
	}
}

func (p *pendingIndex) addAddrAction(addrBytes []byte, actHash hash.Hash256) {
	addr := hash.BytesToHash160(addrBytes)
	if _, ok := p.addrActions[addr]; !ok {
		p.addrs = append(p.addrs, addr)
	}
	p.addrActions[addr] = append(p.addrActions[addr], actHash[:])
}
