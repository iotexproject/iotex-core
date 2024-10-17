// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
)

// vars
var (
	_cacheMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_blockdao_cache",
			Help: "IoTeX blockdao cache counter.",
		},
		[]string{"result"},
	)
)

type (
	// BlockDAO represents the block data access object
	BlockDAO interface {
		BlockStore
		GetBlob(hash.Hash256) (*types.BlobTxSidecar, string, error)
		GetBlobsByHeight(uint64) ([]*types.BlobTxSidecar, []string, error)
	}

	BlockStore interface {
		Start(context.Context) error
		Stop(context.Context) error
		Height() (uint64, error)
		GetBlockHash(uint64) (hash.Hash256, error)
		GetBlockHeight(hash.Hash256) (uint64, error)
		GetBlock(hash.Hash256) (*block.Block, error)
		GetBlockByHeight(uint64) (*block.Block, error)
		GetReceipts(uint64) ([]*action.Receipt, error)
		ContainsTransactionLog() bool
		TransactionLogs(uint64) (*iotextypes.TransactionLogs, error)
		PutBlock(context.Context, *block.Block) error
		Header(hash.Hash256) (*block.Header, error)
		HeaderByHeight(uint64) (*block.Header, error)
		FooterByHeight(uint64) (*block.Footer, error)
	}

	blockDAO struct {
		blockStore   BlockStore
		blobStore    BlobStore
		indexers     []BlockIndexer
		timerFactory *prometheustimer.TimerFactory
		lifecycle    lifecycle.Lifecycle
		headerCache  cache.LRUCache
		footerCache  cache.LRUCache
		receiptCache cache.LRUCache
		blockCache   cache.LRUCache
		heightToHash sync.Map
		tipHeight    uint64
	}
)

type Option func(*blockDAO)

func WithBlobStore(bs BlobStore) Option {
	return func(dao *blockDAO) {
		dao.blobStore = bs
	}
}

// NewBlockDAOWithIndexersAndCache returns a BlockDAO with indexers which will consume blocks appended, and
// caches which will speed up reading
func NewBlockDAOWithIndexersAndCache(blkStore BlockStore, indexers []BlockIndexer, cacheSize int, opts ...Option) BlockDAO {
	if blkStore == nil {
		return nil
	}

	blockDAO := &blockDAO{
		blockStore: blkStore,
		indexers:   indexers,
	}
	for _, opt := range opts {
		opt(blockDAO)
	}

	blockDAO.lifecycle.Add(blkStore)
	if blockDAO.blobStore != nil {
		blockDAO.lifecycle.Add(blockDAO.blobStore)
	}
	for _, indexer := range indexers {
		blockDAO.lifecycle.Add(indexer)
	}
	if cacheSize > 0 {
		blockDAO.headerCache = cache.NewThreadSafeLruCache(cacheSize)
		blockDAO.footerCache = cache.NewThreadSafeLruCache(cacheSize)
		blockDAO.receiptCache = cache.NewThreadSafeLruCache(cacheSize)
		blockDAO.blockCache = cache.NewThreadSafeLruCacheWithOnEvicted(cacheSize, func(_ cache.Key, value interface{}) {
			blockDAO.heightToHash.Delete(value.(*block.Block).Height())
		})
	}
	timerFactory, err := prometheustimer.New(
		"iotex_block_dao_perf",
		"Performance of block DAO",
		[]string{"type"},
		[]string{"default"},
	)
	if err != nil {
		return nil
	}
	blockDAO.timerFactory = timerFactory
	return blockDAO
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start(ctx context.Context) error {
	err := dao.lifecycle.OnStart(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start child services")
	}

	tipHeight, err := dao.blockStore.Height()
	if err != nil {
		return err
	}
	atomic.StoreUint64(&dao.tipHeight, tipHeight)
	return dao.checkIndexers(ctx)
}

func (dao *blockDAO) checkIndexers(ctx context.Context) error {
	checker := NewBlockIndexerChecker(dao)
	for i, indexer := range dao.indexers {
		if err := checker.CheckIndexer(ctx, indexer, 0, func(height uint64) {
			if height%5000 == 0 {
				log.L().Info(
					"indexer is catching up.",
					zap.Int("indexer", i),
					zap.Uint64("height", height),
				)
			}
		}); err != nil {
			return err
		}
		log.L().Info(
			"indexer is up to date.",
			zap.Int("indexer", i),
		)
	}
	return nil
}

func (dao *blockDAO) Stop(ctx context.Context) error {
	return dao.lifecycle.OnStop(ctx)
}

func (dao *blockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	if v, ok := dao.heightToHash.Load(height); ok {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return v.(hash.Hash256), nil
	}
	_cacheMtc.WithLabelValues("miss_block").Inc()
	timer := dao.timerFactory.NewTimer("get_block_hash")
	defer timer.End()
	return dao.blockStore.GetBlockHash(height)
}

func (dao *blockDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	if blk, ok := lruCacheGet(dao.blockCache, hash); ok {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return blk.(*block.Block).Height(), nil
	}
	_cacheMtc.WithLabelValues("miss_block").Inc()
	timer := dao.timerFactory.NewTimer("get_block_height")
	defer timer.End()
	return dao.blockStore.GetBlockHeight(hash)
}

func (dao *blockDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	if blk, ok := lruCacheGet(dao.blockCache, hash); ok {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return blk.(*block.Block), nil
	}
	_cacheMtc.WithLabelValues("miss_block").Inc()

	timer := dao.timerFactory.NewTimer("get_block")
	defer timer.End()
	blk, err := dao.blockStore.GetBlock(hash)
	if err != nil {
		return nil, err
	}
	lruCachePut(dao.blockCache, hash, blk)
	dao.heightToHash.Store(blk.Height(), hash)
	return blk, nil
}

func (dao *blockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	if blk := dao.getBlockByHeightFromCache(height); blk != nil {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return blk, nil
	}
	_cacheMtc.WithLabelValues("miss_block").Inc()
	timer := dao.timerFactory.NewTimer("get_block_byheight")
	defer timer.End()
	blk, err := dao.blockStore.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	hash := blk.HashBlock()
	lruCachePut(dao.blockCache, hash, blk)
	dao.heightToHash.Store(blk.Height(), hash)
	return blk, nil
}

func (dao *blockDAO) getBlockByHeightFromCache(height uint64) *block.Block {
	if v, ok := dao.heightToHash.Load(height); ok {
		if blk, ok := lruCacheGet(dao.blockCache, v.(hash.Hash256)); ok {
			return blk.(*block.Block)
		}
	}
	return nil
}

func (dao *blockDAO) HeaderByHeight(height uint64) (*block.Header, error) {
	if v, ok := lruCacheGet(dao.headerCache, height); ok {
		_cacheMtc.WithLabelValues("hit_header").Inc()
		return v.(*block.Header), nil
	}
	if blk := dao.getBlockByHeightFromCache(height); blk != nil {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return &blk.Header, nil
	}
	_cacheMtc.WithLabelValues("miss_header").Inc()
	header, err := dao.blockStore.HeaderByHeight(height)
	if err != nil {
		return nil, err
	}

	lruCachePut(dao.headerCache, height, header)
	return header, nil
}

func (dao *blockDAO) FooterByHeight(height uint64) (*block.Footer, error) {
	if v, ok := lruCacheGet(dao.footerCache, height); ok {
		_cacheMtc.WithLabelValues("hit_footer").Inc()
		return v.(*block.Footer), nil
	}
	if blk := dao.getBlockByHeightFromCache(height); blk != nil {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return &blk.Footer, nil
	}
	_cacheMtc.WithLabelValues("miss_footer").Inc()
	footer, err := dao.blockStore.FooterByHeight(height)
	if err != nil {
		return nil, err
	}

	lruCachePut(dao.footerCache, height, footer)
	return footer, nil
}

func (dao *blockDAO) Height() (uint64, error) {
	return dao.blockStore.Height()
}

func (dao *blockDAO) Header(h hash.Hash256) (*block.Header, error) {
	if header, ok := lruCacheGet(dao.headerCache, h); ok {
		_cacheMtc.WithLabelValues("hit_header").Inc()
		return header.(*block.Header), nil
	}
	if header, ok := lruCacheGet(dao.blockCache, h); ok {
		_cacheMtc.WithLabelValues("hit_block").Inc()
		return &header.(*block.Block).Header, nil
	}
	_cacheMtc.WithLabelValues("miss_header").Inc()
	header, err := dao.blockStore.Header(h)
	if err != nil {
		return nil, err
	}

	lruCachePut(dao.headerCache, h, header)
	return header, nil
}

func (dao *blockDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	if receipts, ok := lruCacheGet(dao.receiptCache, height); ok {
		_cacheMtc.WithLabelValues("hit_receipts").Inc()
		return receipts.([]*action.Receipt), nil
	}
	_cacheMtc.WithLabelValues("miss_receipts").Inc()
	timer := dao.timerFactory.NewTimer("get_receipt")
	defer timer.End()
	receipts, err := dao.blockStore.GetReceipts(height)
	if err != nil {
		return nil, err
	}
	lruCachePut(dao.receiptCache, height, receipts)
	return receipts, nil
}

func (dao *blockDAO) ContainsTransactionLog() bool {
	return dao.blockStore.ContainsTransactionLog()
}

func (dao *blockDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	timer := dao.timerFactory.NewTimer("get_transactionlog")
	defer timer.End()
	return dao.blockStore.TransactionLogs(height)
}

func (dao *blockDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	timer := dao.timerFactory.NewTimer("put_block")
	if err := dao.blockStore.PutBlock(ctx, blk); err != nil {
		timer.End()
		return err
	}
	if dao.blobStore != nil && blk.HasBlob() {
		if err := dao.blobStore.PutBlock(blk); err != nil {
			timer.End()
			return err
		}
	}
	atomic.StoreUint64(&dao.tipHeight, blk.Height())
	header := blk.Header
	hash := blk.HashBlock()
	lruCachePut(dao.headerCache, blk.Height(), &header)
	lruCachePut(dao.headerCache, header.HashHeader(), &header)
	lruCachePut(dao.blockCache, hash, blk)
	dao.heightToHash.Store(blk.Height(), hash)
	timer.End()

	// index the block if there's indexer
	timer = dao.timerFactory.NewTimer("index_block")
	defer timer.End()
	for _, indexer := range dao.indexers {
		if err := indexer.PutBlock(ctx, blk); err != nil {
			return err
		}
	}
	return nil
}

func (dao *blockDAO) GetBlob(h hash.Hash256) (*types.BlobTxSidecar, string, error) {
	if dao.blobStore == nil {
		return nil, "", errors.Wrap(db.ErrNotExist, "blob store is not available")
	}
	return dao.blobStore.GetBlob(h)
}

func (dao *blockDAO) GetBlobsByHeight(height uint64) ([]*types.BlobTxSidecar, []string, error) {
	if dao.blobStore == nil {
		return nil, nil, errors.Wrap(db.ErrNotExist, "blob store is not available")
	}
	tip := atomic.LoadUint64(&dao.tipHeight)
	if height > tip {
		return nil, nil, errors.Wrapf(db.ErrNotExist, "requested height %d higher than current tip %d", height, tip)
	}
	return dao.blobStore.GetBlobsByHeight(height)
}

func lruCacheGet(c cache.LRUCache, key interface{}) (interface{}, bool) {
	if c != nil {
		return c.Get(key)
	}
	return nil, false
}

func lruCachePut(c cache.LRUCache, k, v interface{}) {
	if c != nil {
		c.Add(k, v)
	}
}
