// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/cache"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	blockNS                  = "blk"
	blockHashHeightMappingNS = "h2h"
	blockHeaderNS            = "bhr"
	blockBodyNS              = "bbd"
	blockFooterNS            = "bfr"
	receiptsNS               = "rpt"
)

var (
	topHeightKey       = []byte("th")
	topHashKey         = []byte("ts")
	hashPrefix         = []byte("ha.")
	heightPrefix       = []byte("he.")
	heightToFileBucket = []byte("h2f")
)

var (
	cacheMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_blockdao_cache",
			Help: "IoTeX blockdao cache counter.",
		},
		[]string{"result"},
	)
	patternLen = len("00000000.db")
	suffixLen  = len(".db")
	// ErrNotOpened indicates db is not opened
	ErrNotOpened = errors.New("DB is not opened")
)

type (
	// BlockDAO represents the block data access object
	BlockDAO interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		GetBlockHash(uint64) (hash.Hash256, error)
		GetBlockHeight(hash.Hash256) (uint64, error)
		GetBlock(hash.Hash256) (*block.Block, error)
		GetBlockByHeight(uint64) (*block.Block, error)
		GetTipHeight() uint64
		Header(hash.Hash256) (*block.Header, error)
		Body(hash.Hash256) (*block.Body, error)
		Footer(hash.Hash256) (*block.Footer, error)
		GetActionByActionHash(hash.Hash256, uint64) (action.SealedEnvelope, error)
		GetReceiptByActionHash(hash.Hash256, uint64) (*action.Receipt, error)
		GetReceipts(uint64) ([]*action.Receipt, error)
		PutBlock(*block.Block) error
		DeleteBlockToTarget(uint64) error
		IndexFile(uint64, []byte) error
		GetFileIndex(uint64) ([]byte, error)
		KVStore() db.KVStore
		HeaderByHeight(uint64) (*block.Header, error)
		FooterByHeight(uint64) (*block.Footer, error)
	}

	// BlockIndexer defines an interface to accept block to build index
	BlockIndexer interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		PutBlock(blk *block.Block) error
		DeleteTipBlock(blk *block.Block) error
	}

	blockDAO struct {
		compressBlock bool
		kvStore       db.KVStore
		indexers      []BlockIndexer
		htf           db.RangeIndex
		kvStores      sync.Map //store like map[index]db.KVStore,index from 1...N
		topIndex      atomic.Value
		timerFactory  *prometheustimer.TimerFactory
		lifecycle     lifecycle.Lifecycle
		headerCache   *cache.ThreadSafeLruCache
		bodyCache     *cache.ThreadSafeLruCache
		footerCache   *cache.ThreadSafeLruCache
		cfg           config.DB
		mutex         sync.RWMutex // for create new db file
		tipHeight     uint64
	}
)

// NewBlockDAO instantiates a block DAO
func NewBlockDAO(kvStore db.KVStore, indexers []BlockIndexer, compressBlock bool, cfg config.DB) BlockDAO {
	blockDAO := &blockDAO{
		compressBlock: compressBlock,
		kvStore:       kvStore,
		indexers:      indexers,
		cfg:           cfg,
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
	blockDAO.lifecycle.Add(kvStore)
	for _, indexer := range indexers {
		blockDAO.lifecycle.Add(indexer)
	}

	return blockDAO
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start(ctx context.Context) error {
	err := dao.lifecycle.OnStart(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start child services")
	}
	// set init height value
	if _, err = dao.kvStore.Get(blockNS, topHeightKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err := dao.kvStore.Put(blockNS, topHeightKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for top height")
		}
	}
	tipHeight, err := dao.getTipHeight()
	if err != nil {
		return err
	}
	atomic.StoreUint64(&dao.tipHeight, tipHeight)
	return dao.initStores()
}

func (dao *blockDAO) initStores() error {
	cfg := dao.cfg
	model, dir := getFileNameAndDir(cfg.DbPath)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	var maxN uint64
	for _, file := range files {
		name := file.Name()
		lens := len(name)
		if lens < patternLen || !strings.Contains(name, model) {
			continue
		}
		num := name[lens-patternLen : lens-suffixLen]
		n, err := strconv.Atoi(num)
		if err != nil {
			continue
		}
		if _, _, err := dao.openDB(uint64(n)); err != nil {
			return err
		}
		if uint64(n) > maxN {
			maxN = uint64(n)
		}
	}
	if maxN == 0 {
		maxN = 1
	}
	dao.topIndex.Store(maxN)
	return nil
}

func (dao *blockDAO) Stop(ctx context.Context) error { return dao.lifecycle.OnStop(ctx) }

func (dao *blockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	return dao.getBlockHash(height)
}

func (dao *blockDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	return dao.getBlockHeight(hash)
}

func (dao *blockDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	return dao.getBlock(hash)
}

func (dao *blockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	hash, err := dao.getBlockHash(height)
	if err != nil {
		return nil, err
	}
	return dao.getBlock(hash)
}

func (dao *blockDAO) HeaderByHeight(height uint64) (*block.Header, error) {
	hash, err := dao.getBlockHash(height)
	if err != nil {
		return nil, err
	}
	return dao.header(hash)
}

func (dao *blockDAO) FooterByHeight(height uint64) (*block.Footer, error) {
	hash, err := dao.getBlockHash(height)
	if err != nil {
		return nil, err
	}
	return dao.footer(hash)
}

func (dao *blockDAO) GetTipHeight() uint64 {
	return atomic.LoadUint64(&dao.tipHeight)
}

func (dao *blockDAO) Header(h hash.Hash256) (*block.Header, error) {
	return dao.header(h)
}

func (dao *blockDAO) Body(h hash.Hash256) (*block.Body, error) {
	return dao.body(h)
}

func (dao *blockDAO) Footer(h hash.Hash256) (*block.Footer, error) {
	return dao.footer(h)
}

func (dao *blockDAO) GetActionByActionHash(h hash.Hash256, height uint64) (action.SealedEnvelope, error) {
	bh, err := dao.getBlockHash(height)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	blk, err := dao.body(bh)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	for _, act := range blk.Actions {
		if act.Hash() == h {
			return act, nil
		}
	}
	return action.SealedEnvelope{}, errors.Errorf("block %d does not have action %x", height, h)
}

func (dao *blockDAO) GetReceiptByActionHash(h hash.Hash256, height uint64) (*action.Receipt, error) {
	receipts, err := dao.getReceipts(height)
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

func (dao *blockDAO) GetReceipts(blkHeight uint64) ([]*action.Receipt, error) {
	return dao.getReceipts(blkHeight)
}

func (dao *blockDAO) PutBlock(blk *block.Block) error {
	err := func() error {
		dao.mutex.Lock()
		defer dao.mutex.Unlock()
		if err := dao.putBlock(blk); err != nil {
			return err
		}
		atomic.StoreUint64(&dao.tipHeight, blk.Height())
		return nil
	}()
	if err != nil {
		return err
	}
	// index the block if there's indexer
	for _, indexer := range dao.indexers {
		if indexer == nil {
			continue
		}
		if err := indexer.PutBlock(blk); err != nil {
			return err
		}
	}

	return nil
}

func (dao *blockDAO) DeleteBlockToTarget(targetHeight uint64) error {
	dao.mutex.Lock()
	defer dao.mutex.Unlock()
	tipHeight, err := dao.getTipHeight()
	if err != nil {
		return err
	}
	for tipHeight > targetHeight {
		// Obtain tip block hash
		h, err := dao.getTipHash()
		if err != nil {
			return errors.Wrap(err, "failed to get tip block hash")
		}
		blk, err := dao.getBlock(h)
		if err != nil {
			return errors.Wrap(err, "failed to get tip block")
		}
		// delete block index if there's indexer
		for _, indexer := range dao.indexers {
			if indexer == nil {
				continue
			}
			if err := indexer.DeleteTipBlock(blk); err != nil {
				return err
			}
		}

		if err := dao.deleteTipBlock(); err != nil {
			return err
		}
		tipHeight--
		atomic.StoreUint64(&dao.tipHeight, tipHeight)
	}
	return nil
}

func (dao *blockDAO) IndexFile(height uint64, index []byte) error {
	dao.mutex.Lock()
	defer dao.mutex.Unlock()

	if dao.htf == nil {
		htf, err := db.NewRangeIndex(dao.kvStore, heightToFileBucket, make([]byte, 8))
		if err != nil {
			return err
		}
		dao.htf = htf
	}
	return dao.htf.Insert(height, index)
}

// GetFileIndex return the db filename
func (dao *blockDAO) GetFileIndex(height uint64) ([]byte, error) {
	dao.mutex.RLock()
	defer dao.mutex.RUnlock()

	if dao.htf == nil {
		htf, err := db.NewRangeIndex(dao.kvStore, heightToFileBucket, make([]byte, 8))
		if err != nil {
			return nil, err
		}
		dao.htf = htf
	}
	return dao.htf.Get(height)
}

func (dao *blockDAO) KVStore() db.KVStore {
	return dao.kvStore
}

// getBlockHash returns the block hash by height
func (dao *blockDAO) getBlockHash(height uint64) (hash.Hash256, error) {
	h := hash.ZeroHash256
	if height == 0 {
		return h, nil
	}
	key := heightKey(height)
	value, err := dao.kvStore.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return h, errors.Wrap(err, "failed to get block hash")
	}
	if len(h) != len(value) {
		return h, errors.Wrapf(err, "blockhash is broken with length = %d", len(value))
	}
	copy(h[:], value)
	return h, nil
}

// getBlockHeight returns the block height by hash
func (dao *blockDAO) getBlockHeight(hash hash.Hash256) (uint64, error) {
	key := hashKey(hash)
	value, err := dao.kvStore.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	if len(value) == 0 {
		return 0, errors.Wrapf(db.ErrNotExist, "height missing for block with hash = %x", hash)
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getBlock returns a block
func (dao *blockDAO) getBlock(hash hash.Hash256) (*block.Block, error) {
	header, err := dao.header(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", hash)
	}
	body, err := dao.body(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", hash)
	}
	footer, err := dao.footer(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", hash)
	}
	return &block.Block{
		Header: *header,
		Body:   *body,
		Footer: *footer,
	}, nil
}

func (dao *blockDAO) header(h hash.Hash256) (*block.Header, error) {
	if dao.headerCache != nil {
		header, ok := dao.headerCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_header").Inc()
			return header.(*block.Header), nil
		}
		cacheMtc.WithLabelValues("miss_header").Inc()
	}
	value, err := dao.getBlockValue(blockHeaderNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_header")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block header %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block header %x is missing", h)
	}
	header := &block.Header{}
	if err := header.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block header %x", h)
	}
	if dao.headerCache != nil {
		dao.headerCache.Add(h, header)
	}
	return header, nil
}

func (dao *blockDAO) body(h hash.Hash256) (*block.Body, error) {
	if dao.bodyCache != nil {
		body, ok := dao.bodyCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_body").Inc()
			return body.(*block.Body), nil
		}
		cacheMtc.WithLabelValues("miss_body").Inc()
	}
	value, err := dao.getBlockValue(blockBodyNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_body")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block body %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block body %x is missing", h)
	}
	body := &block.Body{}
	if err := body.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block body %x", h)
	}
	if dao.bodyCache != nil {
		dao.bodyCache.Add(h, body)
	}
	return body, nil
}

func (dao *blockDAO) footer(h hash.Hash256) (*block.Footer, error) {
	if dao.footerCache != nil {
		footer, ok := dao.footerCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_footer").Inc()
			return footer.(*block.Footer), nil
		}
		cacheMtc.WithLabelValues("miss_footer").Inc()
	}
	value, err := dao.getBlockValue(blockFooterNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_footer")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block footer %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block footer %x is missing", h)
	}
	footer := &block.Footer{}
	if err := footer.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block footer %x", h)
	}
	if dao.footerCache != nil {
		dao.footerCache.Add(h, footer)
	}
	return footer, nil
}

// getTipHeight returns the blockchain height
func (dao *blockDAO) getTipHeight() (uint64, error) {
	value, err := dao.kvStore.Get(blockNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "blockchain height missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getTipHash returns the blockchain tip hash
func (dao *blockDAO) getTipHash() (hash.Hash256, error) {
	value, err := dao.kvStore.Get(blockNS, topHashKey)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get tip hash")
	}
	return hash.BytesToHash256(value), nil
}

func (dao *blockDAO) getReceipts(blkHeight uint64) ([]*action.Receipt, error) {
	kvStore, _, err := dao.getDBFromHeight(blkHeight)
	if err != nil {
		return nil, err
	}
	value, err := kvStore.Get(receiptsNS, byteutil.Uint64ToBytes(blkHeight))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipts of block %d", blkHeight)
	}
	if len(value) == 0 {
		return nil, errors.Wrap(db.ErrNotExist, "block receipts missing")
	}
	receiptsPb := &iotextypes.Receipts{}
	if err := proto.Unmarshal(value, receiptsPb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block receipts")
	}
	var blockReceipts []*action.Receipt
	for _, receiptPb := range receiptsPb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		blockReceipts = append(blockReceipts, receipt)
	}
	return blockReceipts, nil
}

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *block.Block) error {
	blkHeight := blk.Height()
	h, err := dao.getBlockHash(blkHeight)
	if h != hash.ZeroHash256 && err == nil {
		return errors.Errorf("block %d already exist", blkHeight)
	}

	serHeader, err := blk.Header.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block header")
	}
	serBody, err := blk.Body.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block body")
	}
	serFooter, err := blk.Footer.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block footer")
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("compress_header")
		serHeader, err = compress.Compress(serHeader)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block header")
		}
		timer = dao.timerFactory.NewTimer("compress_body")
		serBody, err = compress.Compress(serBody)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block body")
		}
		timer = dao.timerFactory.NewTimer("compress_footer")
		serFooter, err = compress.Compress(serFooter)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block footer")
		}
	}
	batchForBlock := batch.NewBatch()
	hash := blk.HashBlock()
	batchForBlock.Put(blockHeaderNS, hash[:], serHeader, "failed to put block header")
	batchForBlock.Put(blockBodyNS, hash[:], serBody, "failed to put block body")
	batchForBlock.Put(blockFooterNS, hash[:], serFooter, "failed to put block footer")
	kv, _, err := dao.getTopDB(blkHeight)
	if err != nil {
		return err
	}
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			batchForBlock.Put(receiptsNS, byteutil.Uint64ToBytes(blkHeight), receiptsBytes, "failed to put receipts")
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	if err := kv.WriteBatch(batchForBlock); err != nil {
		return err
	}

	b := batch.NewBatch()
	heightValue := byteutil.Uint64ToBytes(blkHeight)
	hashKey := hashKey(hash)
	b.Put(blockHashHeightMappingNS, hashKey, heightValue, "failed to put hash -> height mapping")
	heightKey := heightKey(blkHeight)
	b.Put(blockHashHeightMappingNS, heightKey, hash[:], "failed to put height -> hash mapping")
	tipHeight, err := dao.kvStore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	if blkHeight > enc.MachineEndian.Uint64(tipHeight) {
		b.Put(blockNS, topHeightKey, heightValue, "failed to put top height")
		b.Put(blockNS, topHashKey, hash[:], "failed to put top hash")
	}
	return dao.kvStore.WriteBatch(b)
}

// deleteTipBlock deletes the tip block
func (dao *blockDAO) deleteTipBlock() error {
	// First obtain tip height from db
	height, err := dao.getTipHeight()
	if err != nil {
		return errors.Wrap(err, "failed to get tip height")
	}
	if height == 0 {
		// should not delete genesis block
		return errors.New("cannot delete genesis block")
	}
	// Obtain tip block hash
	hash, err := dao.getTipHash()
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}

	b := batch.NewBatch()
	batchForBlock := batch.NewBatch()
	whichDB, _, err := dao.getDBFromHeight(height)
	if err != nil {
		return err
	}
	// Delete hash -> block mapping
	batchForBlock.Delete(blockHeaderNS, hash[:], "failed to delete block header")
	if dao.headerCache != nil {
		dao.headerCache.Remove(hash)
	}
	batchForBlock.Delete(blockBodyNS, hash[:], "failed to delete block body")
	if dao.bodyCache != nil {
		dao.bodyCache.Remove(hash)
	}
	batchForBlock.Delete(blockFooterNS, hash[:], "failed to delete block footer")
	if dao.footerCache != nil {
		dao.footerCache.Remove(hash)
	}
	// delete receipt
	batchForBlock.Delete(receiptsNS, byteutil.Uint64ToBytes(height), "failed to delete receipt")
	// Delete hash -> height mapping
	hashKey := hashKey(hash)
	b.Delete(blockHashHeightMappingNS, hashKey, "failed to delete hash -> height mapping")

	// Delete height -> hash mapping
	heightKey := heightKey(height)
	b.Delete(blockHashHeightMappingNS, heightKey, "failed to delete height -> hash mapping")

	// Update tip height
	b.Put(blockNS, topHeightKey, byteutil.Uint64ToBytes(height-1), "failed to put top height")

	// Update tip hash
	hash2, err := dao.getBlockHash(height - 1)
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}
	b.Put(blockNS, topHashKey, hash2[:], "failed to put top hash")

	if err := dao.kvStore.WriteBatch(b); err != nil {
		return err
	}
	return whichDB.WriteBatch(batchForBlock)
}

// getDBFromHash returns db of this block stored
func (dao *blockDAO) getDBFromHash(h hash.Hash256) (db.KVStore, uint64, error) {
	height, err := dao.getBlockHeight(h)
	if err != nil {
		return nil, 0, err
	}
	return dao.getDBFromHeight(height)
}

func (dao *blockDAO) getTopDB(blkHeight uint64) (kvStore db.KVStore, index uint64, err error) {
	if dao.cfg.SplitDBSizeMB == 0 || blkHeight <= dao.cfg.SplitDBHeight {
		return dao.kvStore, 0, nil
	}
	topIndex := dao.topIndex.Load().(uint64)
	file, dir := getFileNameAndDir(dao.cfg.DbPath)
	if err != nil {
		return
	}
	longFileName := dir + "/" + file + fmt.Sprintf("-%08d", topIndex) + ".db"
	dat, err := os.Stat(longFileName)
	if err != nil && os.IsNotExist(err) {
		// index the height --> file index mapping
		if err = dao.IndexFile(blkHeight, byteutil.Uint64ToBytesBigEndian(topIndex)); err != nil {
			return
		}
		// db file does not exist, create it
		return dao.openDB(topIndex)
	}
	// other errors except file does not exist
	if err != nil {
		return
	}
	// file exists,but need create new db
	if uint64(dat.Size()) > dao.cfg.SplitDBSize() {
		kvStore, index, err = dao.openDB(topIndex + 1)
		dao.topIndex.Store(index)
		// index the height --> file index mapping
		err = dao.IndexFile(blkHeight, byteutil.Uint64ToBytesBigEndian(topIndex))
		return
	}
	// db exist,need load from kvStores
	kv, ok := dao.kvStores.Load(topIndex)
	if ok {
		kvStore, ok = kv.(db.KVStore)
		if !ok {
			err = errors.New("db convert error")
		}
		index = topIndex
		return
	}
	// file exists,but not opened
	return dao.openDB(topIndex)
}

func (dao *blockDAO) getDBFromHeight(blkHeight uint64) (kvStore db.KVStore, index uint64, err error) {
	if dao.cfg.SplitDBSizeMB == 0 {
		return dao.kvStore, 0, nil
	}
	if blkHeight <= dao.cfg.SplitDBHeight {
		return dao.kvStore, 0, nil
	}
	// get file index
	value, err := dao.GetFileIndex(blkHeight)
	if err != nil {
		return
	}
	return dao.getDBFromIndex(byteutil.BytesToUint64BigEndian(value))
}

func (dao *blockDAO) getDBFromIndex(idx uint64) (kvStore db.KVStore, index uint64, err error) {
	if idx == 0 {
		return dao.kvStore, 0, nil
	}
	kv, ok := dao.kvStores.Load(idx)
	if ok {
		kvStore, ok = kv.(db.KVStore)
		if !ok {
			err = errors.New("db convert error")
		}
		index = idx
		return
	}
	// if user rm some db files manully,then call this method will create new file
	return dao.openDB(idx)
}

// getBlockValue get block's data from db,if this db failed,it will try the previous one
func (dao *blockDAO) getBlockValue(blockNS string, h hash.Hash256) ([]byte, error) {
	whichDB, index, err := dao.getDBFromHash(h)
	if err != nil {
		return nil, err
	}
	value, err := whichDB.Get(blockNS, h[:])
	if errors.Cause(err) == db.ErrNotExist {
		idx := index - 1
		if index == 0 {
			idx = 0
		}
		db, _, err := dao.getDBFromIndex(idx)
		if err != nil {
			return nil, err
		}
		value, err = db.Get(blockNS, h[:])
		if err != nil {
			return nil, err
		}
	}
	return value, err
}

// openDB open file if exists, or create new file
func (dao *blockDAO) openDB(idx uint64) (kvStore db.KVStore, index uint64, err error) {
	if idx == 0 {
		return dao.kvStore, 0, nil
	}
	dao.mutex.Lock()
	defer dao.mutex.Unlock()
	cfg := dao.cfg
	model, _ := getFileNameAndDir(cfg.DbPath)
	name := model + fmt.Sprintf("-%08d", idx) + ".db"

	// open or create this db file
	cfg.DbPath = path.Dir(cfg.DbPath) + "/" + name
	kvStore = db.NewBoltDB(cfg)
	dao.kvStores.Store(idx, kvStore)
	err = kvStore.Start(context.Background())
	if err != nil {
		return
	}
	dao.lifecycle.Add(kvStore)
	index = idx
	return
}

func getFileNameAndDir(p string) (fileName, dir string) {
	var withSuffix, suffix string
	withSuffix = path.Base(p)
	suffix = path.Ext(withSuffix)
	fileName = strings.TrimSuffix(withSuffix, suffix)
	dir = path.Dir(p)
	return
}

func hashKey(h hash.Hash256) []byte {
	return append(hashPrefix, h[:]...)
}

func heightKey(height uint64) []byte {
	return append(heightPrefix, byteutil.Uint64ToBytes(height)...)
}
