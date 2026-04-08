// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"sync/atomic"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// Namespace constants for windowBlockStore Pebble namespaces.
const (
	_wBlkNS  = "wBlk"  // height(8B BE) → serialized block.Store (block + receipts)
	_wTxlNS  = "wTxl"  // height(8B BE) → serialized BlkTransactionLog
	_wH2HNS  = "wH2H"  // blockHash(32B) → height(8B BE)
	_wHHNS   = "wHH"   // height(8B BE) → blockHash(32B)
	_wMetaNS = "wMeta" // metadata: "tip" → height(8B BE)
)

var _tipKey = []byte("tip")

// windowBlockStore is a BlockStore that retains only the most recent `windowSize`
// blocks on disk.  Older blocks are evicted automatically on each PutBlock call.
// All data is persisted to a Pebble KVStore so the window survives restarts.
type windowBlockStore struct {
	lifecycle.Readiness

	kv         db.KVStore
	deser      *block.Deserializer
	windowSize uint64
	tip        uint64 // cached, updated atomically
}

// NewWindowBlockStore creates a windowBlockStore that keeps only the last
// windowSize blocks.  cfg is passed to db.CreateKVStore to open (or create) the
// Pebble database at the given path.
func NewWindowBlockStore(cfg db.Config, path string, windowSize uint64, deser *block.Deserializer) (BlockStore, error) {
	if windowSize == 0 {
		return nil, errors.New("windowSize must be > 0")
	}
	kv, err := db.CreateKVStore(cfg, path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create window block store KVStore")
	}
	return &windowBlockStore{
		kv:         kv,
		deser:      deser,
		windowSize: windowSize,
	}, nil
}

// Start opens the underlying KVStore and restores the cached tip height.
func (w *windowBlockStore) Start(ctx context.Context) error {
	if err := w.kv.Start(ctx); err != nil {
		return err
	}
	raw, err := w.kv.Get(_wMetaNS, _tipKey)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist || errors.Cause(err) == db.ErrBucketNotExist {
			// empty store
			return w.TurnOn()
		}
		return err
	}
	atomic.StoreUint64(&w.tip, byteutil.BytesToUint64BigEndian(raw))
	return w.TurnOn()
}

// Stop closes the underlying KVStore.
func (w *windowBlockStore) Stop(ctx context.Context) error {
	if err := w.TurnOff(); err != nil {
		return err
	}
	return w.kv.Stop(ctx)
}

// Height returns the current tip height (0 if no blocks have been stored).
func (w *windowBlockStore) Height() (uint64, error) {
	return atomic.LoadUint64(&w.tip), nil
}

// PutBlock writes the block (and its receipts / transaction log) to the store
// and evicts the block that falls outside the window.
func (w *windowBlockStore) PutBlock(_ context.Context, blk *block.Block) error {
	height := blk.Height()
	blkHash := blk.HashBlock()
	heightKey := byteutil.Uint64ToBytesBigEndian(height)

	// Serialize block + receipts together.
	blkStore := &block.Store{Block: blk, Receipts: blk.Receipts}
	blkBytes, err := blkStore.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block store")
	}

	// Serialize transaction log.
	txLog := blk.TransactionLog()
	var txlBytes []byte
	if txLog != nil {
		txlBytes = txLog.Serialize()
	} else {
		txlBytes = (&block.BlkTransactionLog{}).Serialize()
	}

	b := batch.NewBatch()
	b.Put(_wBlkNS, heightKey, blkBytes, "put block")
	b.Put(_wTxlNS, heightKey, txlBytes, "put tx log")
	b.Put(_wH2HNS, blkHash[:], heightKey, "put hash->height")
	b.Put(_wHHNS, heightKey, blkHash[:], "put height->hash")
	b.Put(_wMetaNS, _tipKey, heightKey, "put tip")

	// Evict the block that just left the window.
	if height > w.windowSize {
		evictHeight := height - w.windowSize
		evictKey := byteutil.Uint64ToBytesBigEndian(evictHeight)
		evictHash, err := w.kv.Get(_wHHNS, evictKey)
		if err == nil {
			b.Delete(_wH2HNS, evictHash, "evict hash->height")
			b.Delete(_wHHNS, evictKey, "evict height->hash")
			b.Delete(_wBlkNS, evictKey, "evict block")
			b.Delete(_wTxlNS, evictKey, "evict tx log")
		}
		// If the entry doesn't exist (already evicted or never stored), skip silently.
	}

	if err := w.kv.WriteBatch(b); err != nil {
		return errors.Wrap(err, "failed to write block batch")
	}
	atomic.StoreUint64(&w.tip, height)
	return nil
}

// GetBlockHash returns the hash of the block at the given height.
func (w *windowBlockStore) GetBlockHash(height uint64) (hash.Hash256, error) {
	if height == 0 {
		return block.GenesisHash(), nil
	}
	raw, err := w.kv.Get(_wHHNS, byteutil.Uint64ToBytesBigEndian(height))
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(db.ErrNotExist, "block hash at height %d not found", height)
	}
	return hash.BytesToHash256(raw), nil
}

// GetBlockHeight returns the height of the block with the given hash.
func (w *windowBlockStore) GetBlockHeight(h hash.Hash256) (uint64, error) {
	raw, err := w.kv.Get(_wH2HNS, h[:])
	if err != nil {
		return 0, errors.Wrapf(db.ErrNotExist, "block height for hash %x not found", h)
	}
	return byteutil.BytesToUint64BigEndian(raw), nil
}

// GetBlock returns the block with the given hash.
func (w *windowBlockStore) GetBlock(h hash.Hash256) (*block.Block, error) {
	height, err := w.GetBlockHeight(h)
	if err != nil {
		return nil, err
	}
	return w.GetBlockByHeight(height)
}

// GetBlockByHeight returns the block at the given height.
func (w *windowBlockStore) GetBlockByHeight(height uint64) (*block.Block, error) {
	if height == 0 {
		return block.GenesisBlock(), nil
	}
	s, err := w.getBlockStore(height)
	if err != nil {
		return nil, err
	}
	return s.Block, nil
}

// GetReceipts returns the receipts for the block at the given height.
func (w *windowBlockStore) GetReceipts(height uint64) ([]*action.Receipt, error) {
	s, err := w.getBlockStore(height)
	if err != nil {
		return nil, err
	}
	return s.Receipts, nil
}

// Header returns the block header for the block with the given hash.
func (w *windowBlockStore) Header(h hash.Hash256) (*block.Header, error) {
	blk, err := w.GetBlock(h)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

// HeaderByHeight returns the block header at the given height.
func (w *windowBlockStore) HeaderByHeight(height uint64) (*block.Header, error) {
	blk, err := w.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

// FooterByHeight returns the block footer at the given height.
func (w *windowBlockStore) FooterByHeight(height uint64) (*block.Footer, error) {
	blk, err := w.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return &blk.Footer, nil
}

// ContainsTransactionLog reports whether transaction logs are stored.
func (w *windowBlockStore) ContainsTransactionLog() bool { return true }

// TransactionLogs returns the transaction logs for the block at the given height.
func (w *windowBlockStore) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	raw, err := w.kv.Get(_wTxlNS, byteutil.Uint64ToBytesBigEndian(height))
	if err != nil {
		return nil, errors.Wrapf(db.ErrNotExist, "transaction logs at height %d not found", height)
	}
	return block.DeserializeSystemLogPb(raw)
}

// getBlockStore retrieves and deserializes the block.Store for the given height.
func (w *windowBlockStore) getBlockStore(height uint64) (*block.Store, error) {
	raw, err := w.kv.Get(_wBlkNS, byteutil.Uint64ToBytesBigEndian(height))
	if err != nil {
		return nil, errors.Wrapf(db.ErrNotExist, "block at height %d not found (outside window or never stored)", height)
	}
	return w.deser.DeserializeBlockStore(raw)
}

var _ BlockStore = (*windowBlockStore)(nil)
