// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/bloom"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"

	"github.com/iotexproject/iotex-core/action"
	filter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// BlockBloomFilterNamespace indicated the kvstore namespace to store block BloomFilters
	BlockBloomFilterNamespace = "BlockBloomFilters"
	// RangeBloomFilterNamespace indicates the kvstore namespace to store range BloomFilters
	RangeBloomFilterNamespace = "RangeBloomFilters"
	// CurrentHeightKey indicates the key of current bf indexer height in underlying DB
	CurrentHeightKey = "CurrentHeight"
)

const (
	_maxBlockRange = 1e6
	_workerNumbers = 5
)

var (
	// TotalBloomFilterNamespace indicates the kvstore namespace to store total ranges
	TotalBloomFilterNamespace = []byte("TotalBloomFilters")

	errRangeTooLarge = errors.New("block range is too large")

	_queryTimeout = 20 * time.Second
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
		FilterBlocksInRange(*filter.LogFilter, uint64, uint64, uint64) ([]uint64, error)
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

	jobDesc struct {
		idx uint64
		key []byte
	}
)

// NewBloomfilterIndexer creates a new bloomfilterindexer struct by given kvstore and rangebloomfilter size
func NewBloomfilterIndexer(kv db.KVStore, cfg Config) (BloomFilterIndexer, error) {
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
	if bfx.curRangeBloomfilter, err = newBloomRange(bfx.bfSize, bfx.bfNumHash); err != nil {
		return err
	}
	if height > 0 {
		bfx.currRangeBfKey, err = bfx.totalRange.Get(height)
		if err != nil {
			return err
		}
		if err := bfx.loadBloomRangeFromDB(bfx.curRangeBloomfilter, bfx.currRangeBfKey); err != nil {
			return err
		}
	} else {
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
func (bfx *bloomfilterIndexer) DeleteTipBlock(_ context.Context, blk *block.Block) (err error) {
	bfx.mutex.Lock()
	defer bfx.mutex.Unlock()
	height := blk.Height()
	if err := bfx.kvStore.Delete(BlockBloomFilterNamespace, byteutil.Uint64ToBytesBigEndian(height)); err != nil {
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
	if err := bf.FromBytes(bfBytes); err != nil {
		return nil, err
	}
	return bf, nil
}

// FilterBlocksInRange returns the block numbers by given logFilter in range [start, end].
// Result blocks are limited when pagination is larger than 0
func (bfx *bloomfilterIndexer) FilterBlocksInRange(l *filter.LogFilter, start, end uint64, pagination uint64) ([]uint64, error) {
	if start == 0 || end == 0 || end < start {
		return nil, errors.New("start/end height should be bigger than zero")
	}
	if end-start > _maxBlockRange {
		return nil, errRangeTooLarge
	}
	var (
		startIndex, endIndex uint64
		err                  error
	)
	if startIndex, err = bfx.getIndexByHeight(start); err != nil {
		return nil, err
	}
	if endIndex, err = bfx.getIndexByHeight(end); err != nil {
		return nil, err
	}

	var (
		ctx, cancel = context.WithTimeout(context.Background(), _queryTimeout)
		blkNums     = make([][]uint64, endIndex-startIndex+1)
		jobs        = make(chan jobDesc, endIndex-startIndex+1)
		eg          *errgroup.Group
		bufPool     sync.Pool
	)
	defer cancel()
	eg, ctx = errgroup.WithContext(ctx)

	// create pool for BloomRange object reusing
	if _, err := newBloomRange(bfx.bfSize, bfx.bfNumHash); err != nil {
		return nil, err
	}
	bufPool = sync.Pool{
		New: func() interface{} {
			br, _ := newBloomRange(bfx.bfSize, bfx.bfNumHash)
			return br
		},
	}

	for w := 0; w < _workerNumbers; w++ {
		eg.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case job, ok := <-jobs:
					if !ok {
						return nil
					}
					br := bufPool.Get().(*bloomRange)
					if err := bfx.loadBloomRangeFromDB(br, job.key); err != nil {
						bufPool.Put(br)
						return err
					}
					if l.ExistInBloomFilterv2(br.BloomFilter) {
						searchStart := br.Start()
						if start > searchStart {
							searchStart = start
						}
						searchEnd := br.End()
						if end < searchEnd {
							searchEnd = end
						}
						blkNums[job.idx] = l.SelectBlocksFromRangeBloomFilter(br.BloomFilter, searchStart, searchEnd)
					}
					bufPool.Put(br)
				}
			}
		})
	}

	// send job to job chan
	for idx := startIndex; idx <= endIndex; idx++ {
		jobs <- jobDesc{idx - startIndex, byteutil.Uint64ToBytesBigEndian(idx)}
	}
	close(jobs)

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// collect results from goroutines
	ret := []uint64{}
	for i := range blkNums {
		if len(blkNums[i]) > 0 {
			ret = append(ret, blkNums[i]...)
			if pagination > 0 && uint64(len(ret)) > pagination {
				return ret, nil
			}
		}
	}
	return ret, nil
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

func (bfx *bloomfilterIndexer) loadBloomRangeFromDB(br *bloomRange, bfKey []byte) error {
	if br == nil {
		return errors.New("bloomRange is empty")
	}
	bfBytes, err := bfx.kvStore.Get(RangeBloomFilterNamespace, bfKey)
	if err != nil {
		return err
	}
	return br.FromBytes(bfBytes)
}

func (bfx *bloomfilterIndexer) getIndexByHeight(height uint64) (uint64, error) {
	val, err := bfx.totalRange.Get(height)
	if err != nil {
		return 0, err
	}
	return byteutil.BytesToUint64BigEndian(val), nil
}
