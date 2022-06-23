// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

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
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/db"
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
		filedao.FileDAO
		GetActionByActionHash(hash.Hash256, uint64) (action.SealedEnvelope, uint32, error)
		GetReceiptByActionHash(hash.Hash256, uint64) (*action.Receipt, error)
		DeleteBlockToTarget(uint64) error
	}

	blockDAO struct {
		blockStore   filedao.FileDAO
		indexers     []BlockIndexer
		timerFactory *prometheustimer.TimerFactory
		lifecycle    lifecycle.Lifecycle
		headerCache  *cache.ThreadSafeLruCache
		bodyCache    *cache.ThreadSafeLruCache
		footerCache  *cache.ThreadSafeLruCache
		tipHeight    uint64
	}
)

// NewBlockDAO instantiates a block DAO
func NewBlockDAO(indexers []BlockIndexer, evmNetworkID uint32, cfg db.Config) BlockDAO {
	blkStore, err := filedao.NewFileDAO(cfg, evmNetworkID)
	if err != nil {
		log.L().Fatal(err.Error(), zap.Any("cfg", cfg))
		return nil
	}
	return createBlockDAO(blkStore, indexers, cfg)
}

// NewBlockDAOInMemForTest creates a in-memory block DAO for testing
func NewBlockDAOInMemForTest(indexers []BlockIndexer) BlockDAO {
	blkStore, err := filedao.NewFileDAOInMemForTest()
	if err != nil {
		return nil
	}
	return createBlockDAO(blkStore, indexers, db.Config{MaxCacheSize: 16})
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

func (dao *blockDAO) GetActionByActionHash(h hash.Hash256, height uint64) (action.SealedEnvelope, uint32, error) {
	blk, err := dao.blockStore.GetBlockByHeight(height)
	if err != nil {
		return action.SealedEnvelope{}, 0, err
	}
	for i, act := range blk.Actions {
		actHash, err := act.Hash()
		if err != nil {
			return action.SealedEnvelope{}, 0, errors.Errorf("hash failed for block %d", height)
		}
		if actHash == h {
			return act, uint32(i), nil
		}
	}
	return action.SealedEnvelope{}, 0, errors.Errorf("block %d does not have action %x", height, h)
}

func (dao *blockDAO) GetReceiptByActionHash(h hash.Hash256, height uint64) (*action.Receipt, error) {
	receipts, err := dao.blockStore.GetReceipts(height)
	if err != nil {
		return nil, err
	}
	for _, r := range receipts {
		if r.ActionHash == h {
			return r, nil
		}
	}
	return nil, errors.Errorf("receipt of action %x isn't found", h)
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

func (dao *blockDAO) DeleteTipBlock() error {
	timer := dao.timerFactory.NewTimer("del_block")
	defer timer.End()
	return dao.blockStore.DeleteTipBlock()
}

func (dao *blockDAO) DeleteBlockToTarget(targetHeight uint64) error {
	tipHeight, err := dao.blockStore.Height()
	if err != nil {
		return err
	}
	ctx := context.Background()
	for tipHeight > targetHeight {
		blk, err := dao.blockStore.GetBlockByHeight(tipHeight)
		if err != nil {
			return errors.Wrap(err, "failed to get tip block")
		}
		// delete block index if there's indexer
		for _, indexer := range dao.indexers {
			if err := indexer.DeleteTipBlock(ctx, blk); err != nil {
				return err
			}
		}

		if err := dao.blockStore.DeleteTipBlock(); err != nil {
			return err
		}
		// purge from cache
		h := blk.HashBlock()
		lruCacheDel(dao.headerCache, tipHeight)
		lruCacheDel(dao.headerCache, h)
		lruCacheDel(dao.bodyCache, tipHeight)
		lruCacheDel(dao.bodyCache, h)
		lruCacheDel(dao.footerCache, tipHeight)
		lruCacheDel(dao.footerCache, h)

		tipHeight--
		atomic.StoreUint64(&dao.tipHeight, tipHeight)
	}
	return nil
}

func createBlockDAO(blkStore filedao.FileDAO, indexers []BlockIndexer, cfg db.Config) BlockDAO {
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
	if cfg.MaxCacheSize > 0 {
		blockDAO.headerCache = cache.NewThreadSafeLruCache(cfg.MaxCacheSize)
		blockDAO.bodyCache = cache.NewThreadSafeLruCache(cfg.MaxCacheSize)
		blockDAO.footerCache = cache.NewThreadSafeLruCache(cfg.MaxCacheSize)
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

func lruCacheGet(c *cache.ThreadSafeLruCache, key interface{}) (interface{}, bool) {
	if c != nil {
		return c.Get(key)
	}
	return nil, false
}

func lruCachePut(c *cache.ThreadSafeLruCache, k, v interface{}) {
	if c != nil {
		c.Add(k, v)
	}
}

func lruCacheDel(c *cache.ThreadSafeLruCache, k interface{}) {
	if c != nil {
		c.Remove(k)
	}
}
