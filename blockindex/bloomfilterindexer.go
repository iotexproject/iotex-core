// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"runtime"
	"sync"

	"github.com/iotexproject/go-pkgs/bloom"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	filter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/pkg/errors"
)

const (
	// BlockBloomFilterNamespace indicated the kvstore namespace to store block BloomFilters
	BlockBloomFilterNamespace = "BlockBloomFilters"
	// RangeBloomFilterNamespace indicates the kvstore namespace to store range BloomFilters
	RangeBloomFilterNamespace = "RangeBloomFilters"
	// CurrentHeightKey indicates the key of current bf indexer height in underlying DB
	CurrentHeightKey = "CurrentHeight"
)

var (
	// TotalBloomFilterNamespace indicates the kvstore namespace to store total ranges
	TotalBloomFilterNamespace = []byte("TotalBloomFilters")
)

type (
	// BloomFilterIndexer is the interface for bloomfilter indexer
	BloomFilterIndexer interface {
		blockdao.BlockIndexer
		// RangeBloomFilterNumElements returns the number of elements that each rangeBloomfilter indexes
		RangeBloomFilterNumElements() uint64
		// BlockFilterByHeight returns the block-level bloomfilter which includes not only topic but also address of logs info by given block height
		BlockFilterByHeight(uint64) (bloom.BloomFilter, error)
		// FilterBlocksInRange returns the block numbers by given logFilter in range from start to end
		FilterBlocksInRange(*filter.LogFilter, uint64, uint64) ([]uint64, error)
	}

	// bloomfilterIndexer is a struct for bloomfilter indexer
	bloomfilterIndexer struct {
		mutex               sync.RWMutex // mutex for curRangeBloomfilter
		kvStore             db.KVStore
		rangeSize           uint64
		bfSize              uint64
		bfNumHash           uint64
		currRangeBfKey      []byte
		curRangeBloomfilter *bloomRange
		totalRange          db.RangeIndex
	}
)

// NewBloomfilterIndexer creates a new bloomfilterindexer struct by given kvstore and rangebloomfilter size
func NewBloomfilterIndexer(kv db.KVStore, cfg config.Indexer) (BloomFilterIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}

	return &bloomfilterIndexer{
		kvStore:   kv,
		rangeSize: cfg.RangeBloomFilterNumElements,
		bfSize:    cfg.RangeBloomFilterSize,
		bfNumHash: cfg.RangeBloomFilterNumHash,
	}, nil
}

// Start starts the bloomfilter indexer
func (bfx *bloomfilterIndexer) Start(ctx context.Context) error {
	if err := bfx.kvStore.Start(ctx); err != nil {
		return err
	}

	bfx.mutex.Lock()
	defer bfx.mutex.Unlock()
	tipHeightData, err := bfx.kvStore.Get(RangeBloomFilterNamespace, []byte(CurrentHeightKey))
	switch errors.Cause(err) {
	case nil:
		tipHeight := byteutil.BytesToUint64BigEndian(tipHeightData)
		return bfx.initRangeBloomFilter(tipHeight)
	case db.ErrNotExist:
		if err = bfx.kvStore.Put(RangeBloomFilterNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)); err != nil {
			return err
		}
		return bfx.initRangeBloomFilter(0)
	default:
		return err
	}
}

func (bfx *bloomfilterIndexer) initRangeBloomFilter(height uint64) error {
	var (
		err        error
		zero8Bytes = make([]byte, 8)
	)
	bfx.totalRange, err = db.NewRangeIndex(bfx.kvStore, TotalBloomFilterNamespace, zero8Bytes)
	if err != nil {
		return err
	}

	if height > 0 {
		bfx.curRangeBloomfilter, err = bfx.rangeBloomFilter(height)
		if err != nil {
			return err
		}
		// totalRange.Get() is called and err-checked in rangeBloomFilter() above
		bfx.currRangeBfKey, _ = bfx.totalRange.Get(height)
	} else {
		if bfx.curRangeBloomfilter, err = newBloomRange(bfx.bfSize, bfx.bfNumHash); err != nil {
			return err
		}
		bfx.curRangeBloomfilter.SetStart(1)
		bfx.currRangeBfKey = zero8Bytes
	}
	return nil
}

// Stop stops the bloomfilter indexer
func (bfx *bloomfilterIndexer) Stop(ctx context.Context) error {
	bfx.totalRange.Close()
	return bfx.kvStore.Stop(ctx)
}

// Height returns the tipHeight from underlying DB
func (bfx *bloomfilterIndexer) Height() (uint64, error) {
	h, err := bfx.kvStore.Get(RangeBloomFilterNamespace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, err
	}
	return byteutil.BytesToUint64BigEndian(h), nil
}

// PutBlock processes new block by adding logs into rangebloomfilter, and if necessary, updating underlying DB
func (bfx *bloomfilterIndexer) PutBlock(ctx context.Context, blk *block.Block) (err error) {
	bfx.mutex.Lock()
	defer bfx.mutex.Unlock()
	bfx.addLogsToRangeBloomFilter(ctx, blk.Height(), blk.Receipts)
	// commit into DB and update tipHeight
	if err := bfx.commit(blk.Height(), bfx.calculateBlockBloomFilter(ctx, blk.Receipts)); err != nil {
		return err
	}
	if bfx.curRangeBloomfilter.NumElements() >= bfx.rangeSize {
		nextIndex := byteutil.BytesToUint64BigEndian(bfx.currRangeBfKey) + 1
		bfx.currRangeBfKey = byteutil.Uint64ToBytesBigEndian(nextIndex)
		if err := bfx.totalRange.Insert(blk.Height()+1, bfx.currRangeBfKey); err != nil {
			return errors.Wrapf(err, "failed to write next bloomfilter index")
		}
		if bfx.curRangeBloomfilter, err = newBloomRange(bfx.bfSize, bfx.bfNumHash); err != nil {
			return err
		}
		bfx.curRangeBloomfilter.SetStart(blk.Height() + 1)
	}
	return nil
}

// DeleteTipBlock deletes tip height from underlying DB if necessary
func (bfx *bloomfilterIndexer) DeleteTipBlock(blk *block.Block) (err error) {
	bfx.mutex.Lock()
	defer bfx.mutex.Unlock()
	height := blk.Height()
	if err := bfx.delete(height); err != nil {
		return err
	}
	bfx.curRangeBloomfilter = nil
	return nil
}

// RangeBloomFilterNumElements returns the number of elements that each rangeBloomfilter indexes
func (bfx *bloomfilterIndexer) RangeBloomFilterNumElements() uint64 {
	bfx.mutex.RLock()
	defer bfx.mutex.RUnlock()
	return bfx.rangeSize
}

// BlockFilterByHeight returns the block-level bloomfilter which includes not only topic but also address of logs info by given block height
func (bfx *bloomfilterIndexer) BlockFilterByHeight(height uint64) (bloom.BloomFilter, error) {
	bfBytes, err := bfx.kvStore.Get(BlockBloomFilterNamespace, byteutil.Uint64ToBytesBigEndian(height))
	if err != nil {
		return nil, err
	}
	bf, err := bloom.NewBloomFilter(bfx.bfSize, bfx.bfNumHash)
	if err != nil {
		return nil, err
	}
	// load data into dummy bloomFilter
	if err := bf.FromBytes(bfBytes); err != nil {
		return nil, err
	}

	return bf, nil
}

// FilterBlocksInRange returns the block numbers by given logFilter in range [start, end]
// TODO: pass pagination argument in
func (bfx *bloomfilterIndexer) FilterBlocksInRange(l *filter.LogFilter, start, end uint64) ([]uint64, error) {
	memMetrics()
	if start == 0 || end == 0 || end < start {
		return nil, errors.New("start/end height should be bigger than zero")
	}

	b, err := bfx.totalRange.Get(start)
	if err != nil {
		return nil, err
	}
	startIndex := byteutil.BytesToUint64BigEndian(b)
	if b, err = bfx.totalRange.Get(end); err != nil {
		return nil, err
	}
	endIndex := byteutil.BytesToUint64BigEndian(b)

	blockNumbers := make([]uint64, 0)
	br, err := newBloomRange(bfx.bfSize, bfx.bfNumHash)
	if err != nil {
		return nil, err
	}
	// TODO: optimized with goroutine, br to be opt with sync.Pool
	// TODO: endIndex + 1 needs to be considered
	for ; startIndex <= endIndex; startIndex++ {
		bfKey := byteutil.Uint64ToBytesBigEndian(startIndex)
		bfBytes, err := bfx.kvStore.Get(RangeBloomFilterNamespace, bfKey)
		if err != nil {
			return nil, err
		}
		if err := br.FromBytes(bfBytes); err != nil {
			return nil, err
		}
		bfBytes = nil // mark data from database can be free
		// runtime.GC()

		if l.ExistInBloomFilterv2(br.BloomFilter) {
			searchStart := br.Start()
			if start > searchStart {
				searchStart = start
			}
			searchEnd := br.End()
			if end < searchEnd {
				searchEnd = end
			}
			blockNumbers = append(blockNumbers, l.SelectBlocksFromRangeBloomFilter(br.BloomFilter, searchStart, searchEnd)...)
		}
	}
	memMetrics()
	return blockNumbers, nil
}

func (bfx *bloomfilterIndexer) rangeBloomFilter(blockNumber uint64) (*bloomRange, error) {
	rangeBloomfilterKey, err := bfx.totalRange.Get(blockNumber)
	if err != nil {
		return nil, err
	}
	bfBytes, err := bfx.kvStore.Get(RangeBloomFilterNamespace, rangeBloomfilterKey)
	if err != nil {
		return nil, err
	}
	br, err := newBloomRange(bfx.bfSize, bfx.bfNumHash)
	if err != nil {
		return nil, err
	}
	if err := br.FromBytes(bfBytes); err != nil {
		return nil, err
	}
	return br, nil
}
func (bfx *bloomfilterIndexer) delete(blockNumber uint64) error {
	// TODO: remove delete from indexer interface
	return bfx.kvStore.Delete(BlockBloomFilterNamespace, byteutil.Uint64ToBytesBigEndian(blockNumber))
}

func (bfx *bloomfilterIndexer) commit(blockNumber uint64, blkBloomfilter bloom.BloomFilter) error {
	bfx.curRangeBloomfilter.SetEnd(blockNumber)
	bfBytes, err := bfx.curRangeBloomfilter.Bytes()
	if err != nil {
		return err
	}
	b := batch.NewBatch()
	b.Put(RangeBloomFilterNamespace, bfx.currRangeBfKey, bfBytes, "failed to put range bloom filter")
	b.Put(BlockBloomFilterNamespace, byteutil.Uint64ToBytesBigEndian(blockNumber), blkBloomfilter.Bytes(), "failed to put block bloom filter")
	b.Put(RangeBloomFilterNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytesBigEndian(blockNumber), "failed to put current height")
	b.AddFillPercent(RangeBloomFilterNamespace, 1.0)
	b.AddFillPercent(BlockBloomFilterNamespace, 1.0)
	return bfx.kvStore.WriteBatch(b)
}

// TODO: improve performance
func (bfx *bloomfilterIndexer) calculateBlockBloomFilter(ctx context.Context, receipts []*action.Receipt) bloom.BloomFilter {
	bloom, _ := bloom.NewBloomFilter(2048, 3)
	for _, receipt := range receipts {
		for _, l := range receipt.Logs() {
			bloom.Add([]byte(l.Address))
			for i, topic := range l.Topics {
				bloom.Add(append(byteutil.Uint64ToBytes(uint64(i)), topic[:]...)) //position-sensitive
			}
		}
	}
	return bloom
}

// TODO: improve performance
func (bfx *bloomfilterIndexer) addLogsToRangeBloomFilter(ctx context.Context, blockNumber uint64, receipts []*action.Receipt) {
	Heightkey := append([]byte(filter.BlockHeightPrefix), byteutil.Uint64ToBytes(blockNumber)...)

	for _, receipt := range receipts {
		for _, l := range receipt.Logs() {
			bfx.curRangeBloomfilter.Add([]byte(l.Address))
			bfx.curRangeBloomfilter.Add(append(Heightkey, []byte(l.Address)...)) // concatenate with block number
			for i, topic := range l.Topics {
				bfx.curRangeBloomfilter.Add(append(byteutil.Uint64ToBytes(uint64(i)), topic[:]...)) //position-sensitive
				bfx.curRangeBloomfilter.Add(append(Heightkey, topic[:]...))                         // concatenate with block number
			}
		}
	}
}

func memMetrics() {
	bToMb := func(b uint64) uint64 {
		return b / 1024
	}
	var memStat runtime.MemStats
	runtime.ReadMemStats(&memStat)
	log.L().Info("MemInfo",
		zap.Uint64("allocatedHeapObjects", bToMb(memStat.Alloc)),
		zap.Uint64("totalAllocatedHeapObjects", bToMb(memStat.TotalAlloc)),
		zap.Uint64("stackInUse", bToMb(memStat.StackInuse)),
		zap.Uint64("stackFromOS", bToMb(memStat.StackSys)),
		zap.Uint64("heapInUse", bToMb(memStat.HeapInuse)),
		zap.Uint64("heapFromOS", bToMb(memStat.HeapSys)),
		zap.Uint64("heapIdle", bToMb(memStat.HeapIdle)),
		zap.Uint64("heapReleased", bToMb(memStat.HeapReleased)),
	)
}
