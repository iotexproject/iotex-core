// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"sync/atomic"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
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
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
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
		blockStore   BlockDAO
		indexers     []BlockIndexer
		timerFactory *prometheustimer.TimerFactory
		lifecycle    lifecycle.Lifecycle
		headerCache  cache.LRUCache
		bodyCache    cache.LRUCache
		footerCache  cache.LRUCache
		tipHeight    uint64
	}
)

// NewBlockDAOWithIndexersAndCache returns a BlockDAO with indexers which will consume blocks appended, and
// caches which will speed up reading
func NewBlockDAOWithIndexersAndCache(blkStore BlockDAO, indexers []BlockIndexer, cacheSize int) BlockDAO {
	if blkStore == nil {
		return nil
	}

	blockDAO := &blockDAO{
		blockStore: blkStore,
		indexers:   indexers,
	}

	blockDAO.lifecycle.Add(blkStore)
	for _, indexer := range indexers {
		blockDAO.lifecycle.Add(indexer)
	}
	if cacheSize > 0 {
		blockDAO.headerCache = cache.NewThreadSafeLruCache(cacheSize)
		blockDAO.bodyCache = cache.NewThreadSafeLruCache(cacheSize)
		blockDAO.footerCache = cache.NewThreadSafeLruCache(cacheSize)
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

func (dao *blockDAO) Stop(ctx context.Context) error { return dao.lifecycle.OnStop(ctx) }

func (dao *blockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	timer := dao.timerFactory.NewTimer("get_block_hash")
	defer timer.End()
	return dao.blockStore.GetBlockHash(height)
}

func (dao *blockDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	timer := dao.timerFactory.NewTimer("get_block_height")
	defer timer.End()
	return dao.blockStore.GetBlockHeight(hash)
}

func (dao *blockDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	timer := dao.timerFactory.NewTimer("get_block")
	defer timer.End()
	return dao.blockStore.GetBlock(hash)
}

func (dao *blockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	timer := dao.timerFactory.NewTimer("get_block_byheight")
	defer timer.End()
	return dao.blockStore.GetBlockByHeight(height)
}

func (dao *blockDAO) HeaderByHeight(height uint64) (*block.Header, error) {
	if v, ok := lruCacheGet(dao.headerCache, height); ok {
		_cacheMtc.WithLabelValues("hit_header").Inc()
		return v.(*block.Header), nil
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
	_cacheMtc.WithLabelValues("miss_header").Inc()

	header, err := dao.blockStore.Header(h)
	if err != nil {
		return nil, err
	}

	lruCachePut(dao.headerCache, h, header)
	return header, nil
}

func (dao *blockDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	timer := dao.timerFactory.NewTimer("get_receipt")
	defer timer.End()
	return dao.blockStore.GetReceipts(height)
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
	atomic.StoreUint64(&dao.tipHeight, blk.Height())
	header := blk.Header
	lruCachePut(dao.headerCache, blk.Height(), &header)
	lruCachePut(dao.headerCache, header.HashHeader(), &header)
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
