// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"context"
	"sync"

	"github.com/iotexproject/go-pkgs/bloom"

	"github.com/iotexproject/iotex-core/action"
	filter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/pkg/errors"
)

const (
	// BlockBloomFilterNamespace indicated the kvstore namespace to store BlockBloomFilterNamespace
	BlockBloomFilterNamespace = "BlockBloomFilters"
	// RangeBloomFilterNamespace indicates the kvstore namespace to store RangeBloomFilters
	RangeBloomFilterNamespace = "RangeBloomFilters"
	// CurrentHeightKey indicates the key of current bf indexer height in underlying DB
	CurrentHeightKey = "CurrentHeight"
)

type (
	// BloomFilterIndexer is the interface for bloomfilter indexer
	BloomFilterIndexer interface {
		blockdao.BlockIndexer
		// RangeBloomFilterSize returns the number of blocks that each rangeBloomfilter includes
		RangeBloomFilterSize() uint64
		// BloomFilterByHeight returns the block-level bloomfilter which includes not only topic but also address of logs info by given block height
		BloomFilterByHeight(uint64) (bloom.BloomFilter, error)
		// FilterBlocksInRange returns the block numbers by given logFilter in range from start to end
		FilterBlocksInRange(*filter.LogFilter, uint64, uint64) ([]uint64, error)
	}

	// bloomfilterIndexer is a struct for bloomfilter indexer
	bloomfilterIndexer struct {
		mutex               sync.RWMutex // mutex for curRangeBloomfilter
		kvStore             db.KVStore
		rangeSize           uint64
		curRangeBloomfilter bloom.BloomFilter
	}
)

// NewBloomfilterIndexer creates a new bloomfilterindexer struct by given kvstore and rangebloomfilter size
func NewBloomfilterIndexer(kv db.KVStore, rangeSize uint64) (BloomFilterIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	return &bloomfilterIndexer{
		kvStore:   kv,
		rangeSize: rangeSize,
	}, nil
}

// Start starts the bloomfilter indexer
func (bfx *bloomfilterIndexer) Start(ctx context.Context) error {
	if err := bfx.kvStore.Start(ctx); err != nil {
		return err
	}
	bfx.mutex.Lock()
	defer bfx.mutex.Unlock()
	tipHeight, err := bfx.kvStore.Get(BlockBloomFilterNamespace, []byte(CurrentHeightKey))
	switch errors.Cause(err) {
	case nil:
		bfx.curRangeBloomfilter, err = bfx.rangeBloomFilter(byteutil.BytesToUint64(tipHeight))
		if err != nil {
			return err
		}
	case db.ErrNotExist:
		if err = bfx.kvStore.Put(BlockBloomFilterNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(0)); err != nil {
			return err
		}
		bfx.curRangeBloomfilter, err = bloom.NewBloomFilter(2048, 3)
		if err != nil {
			return err
		}
	default:
		return err
	}
	return nil
}

// Stop stops the bloomfilter indexer
func (bfx *bloomfilterIndexer) Stop(ctx context.Context) error {
	return bfx.kvStore.Stop(ctx)
}

// Height returns the tipHeight from underlying DB
func (bfx *bloomfilterIndexer) Height() (uint64, error) {
	h, err := bfx.kvStore.Get(BlockBloomFilterNamespace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, err
	}
	return byteutil.BytesToUint64(h), nil
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
	if blk.Height()%bfx.rangeSize == 0 {
		bfx.curRangeBloomfilter, err = bloom.NewBloomFilter(2048, 3)
		if err != nil {
			return errors.Wrapf(err, "Can not create new bloomfilter")
		}
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
	if (height-1)%bfx.rangeSize == 0 {
		bfx.curRangeBloomfilter, err = bloom.NewBloomFilter(2048, 3)
		if err != nil {
			return err
		}
	} else {
		bfx.curRangeBloomfilter, err = bfx.rangeBloomFilter(height - 1)
		if err != nil {
			return err
		}
	}
	return nil
}

// RangeBloomFilterSize returns the number of blocks that each rangeBloomfilter includes
func (bfx *bloomfilterIndexer) RangeBloomFilterSize() uint64 {
	bfx.mutex.RLock()
	defer bfx.mutex.RUnlock()
	return bfx.rangeSize
}

// BloomFilterByHeight returns the block-level bloomfilter which includes not only topic but also address of logs info by given block height
func (bfx *bloomfilterIndexer) BloomFilterByHeight(height uint64) (bloom.BloomFilter, error) {
	return bfx.blockBloomFilter(height)
}

// FilterBlocksInRange returns the block numbers by given logFilter in range (start, end]
func (bfx *bloomfilterIndexer) FilterBlocksInRange(l *filter.LogFilter, start, end uint64) ([]uint64, error) {
	bfx.mutex.RLock()
	defer bfx.mutex.RUnlock()
	blockNumbers := make([]uint64, 0)
	queryHeight := (start/bfx.rangeSize + 1) * bfx.rangeSize // round up
	for queryHeight <= end {
		bigBloom, err := bfx.rangeBloomFilter(queryHeight)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get rangeBloomFilter from indexer by given height %d", queryHeight)
		}
		if l.ExistInBloomFilterv2(bigBloom) {
			blocks := l.SelectBlocksFromRangeBloomFilter(bigBloom, queryHeight-bfx.rangeSize+1, queryHeight)
			for _, num := range blocks {
				if num > start && num <= end {
					blockNumbers = append(blockNumbers, num)
				}
			}
		}
		queryHeight += bfx.rangeSize
	}
	// in case that mode of end is not 0, we need to get range Bloomfilter [end-(end%bfx.rangeSize)+1, end]
	if end%bfx.rangeSize != 0 {
		bigBloom, err := bfx.rangeBloomFilter(end)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get rangeBloomFilter from indexer by given height %d", end)
		}
		if l.ExistInBloomFilterv2(bigBloom) {
			blocks := l.SelectBlocksFromRangeBloomFilter(bigBloom, end-(end%bfx.rangeSize)+1, end)
			blockNumbers = append(blockNumbers, blocks...)
		}
	}

	return blockNumbers, nil
}

// rangeBloomFilter reads rangebloomfilter by given block number from underlying DB
func (bfx *bloomfilterIndexer) rangeBloomFilter(blockNumber uint64) (bloom.BloomFilter, error) {
	bfBytes, err := bfx.kvStore.Get(RangeBloomFilterNamespace, byteutil.Uint64ToBytes(blockNumber))
	if err != nil {
		return nil, err
	}
	return bloom.BloomFilterFromBytes(bfBytes, 2048, 3)
}

// blockBloomFilter reads block bloomfilter by given block number from underlying DB
func (bfx *bloomfilterIndexer) blockBloomFilter(blockNumber uint64) (bloom.BloomFilter, error) {
	bfBytes, err := bfx.kvStore.Get(BlockBloomFilterNamespace, byteutil.Uint64ToBytes(blockNumber))
	if err != nil {
		return nil, err
	}
	return bloom.BloomFilterFromBytes(bfBytes, 2048, 3)
}

func (bfx *bloomfilterIndexer) delete(blockNumber uint64) error {
	if err := bfx.kvStore.Delete(RangeBloomFilterNamespace, byteutil.Uint64ToBytes(blockNumber)); err != nil {
		return errors.Wrapf(err, "failed to delete RangeBloomFilter")
	}
	if err := bfx.kvStore.Delete(BlockBloomFilterNamespace, byteutil.Uint64ToBytes(blockNumber)); err != nil {
		return errors.Wrapf(err, "failed to delete BlockBloomFilter")
	}
	return bfx.kvStore.Put(BlockBloomFilterNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(blockNumber-1))
}

func (bfx *bloomfilterIndexer) commit(blockNumber uint64, blkBloomfilter bloom.BloomFilter) error {
	if err := bfx.kvStore.Put(RangeBloomFilterNamespace, byteutil.Uint64ToBytes(blockNumber), bfx.curRangeBloomfilter.Bytes()); err != nil {
		return errors.Wrapf(err, "failed to put RangeBloomFilter")
	}
	if err := bfx.kvStore.Put(BlockBloomFilterNamespace, byteutil.Uint64ToBytes(blockNumber), blkBloomfilter.Bytes()); err != nil {
		return errors.Wrapf(err, "failed to put BlockBloomFilter")
	}
	return bfx.kvStore.Put(BlockBloomFilterNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(blockNumber))
}

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
	return
}
