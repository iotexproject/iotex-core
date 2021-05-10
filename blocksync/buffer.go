// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"sync"

	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// blockBuffer is used to keep in-coming block in order.
type blockBuffer struct {
	mu           sync.RWMutex
	blockQueues  map[uint64]*uniQueue
	bufferSize   uint64
	intervalSize uint64
}

type syncBlocksInterval struct {
	Start uint64
	End   uint64
}

func (b *blockBuffer) contains(height uint64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	_, ok := b.blockQueues[height]

	return ok
}

func (b *blockBuffer) delete(height uint64) []*block.Block {
	b.mu.Lock()
	defer b.mu.Unlock()
	queue, ok := b.blockQueues[height]
	if !ok {
		return nil
	}
	blks := queue.dequeAll()
	delete(b.blockQueues, height)

	return blks
}

func (b *blockBuffer) cleanup(height uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	size := len(b.blockQueues)
	if size > int(b.bufferSize)*2 {
		log.L().Warn("blockBuffer is leaking memory.", zap.Int("bufferSize", size))
		newQueues := map[uint64]*uniQueue{}
		for h := range b.blockQueues {
			if h > height {
				newQueues[h] = b.blockQueues[h]
			}
		}
		b.blockQueues = newQueues
	}
}

// AddBlock tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) AddBlock(tipHeight uint64, blk *block.Block) {
	blkHeight := blk.Height()
	if blkHeight > tipHeight && blkHeight <= tipHeight+b.bufferSize {
		b.mu.Lock()
		defer b.mu.Unlock()
		if _, ok := b.blockQueues[blkHeight]; !ok {
			b.blockQueues[blkHeight] = &uniQueue{
				blocks: []*block.Block{},
				hashes: map[hash.Hash256]bool{},
			}
		}
		b.blockQueues[blkHeight].enque(blk)
	}
}

func (b *blockBuffer) Flush(tipHeight uint64, commitBlock func(*block.Block) error) uint64 {
	syncedHeight := tipHeight
	log.L().Debug("Flush blocks", zap.Uint64("tipHeight", tipHeight), zap.String("source", "blockBuffer"))
	for syncedHeight <= tipHeight+b.bufferSize {
		committed := false
		blks := b.delete(syncedHeight + 1)
		for _, blk := range blks {
			if blk == nil {
				continue
			}
			if err := commitBlock(blk); err != nil {
				log.L().Debug("failed to commit block", zap.Error(err))
				continue
			}
			syncedHeight++
			committed = true
			break
		}
		if committed == false {
			break
		}
	}

	// clean up on memory leak
	b.cleanup(syncedHeight)

	return syncedHeight
}

// GetBlocksIntervalsToSync returns groups of syncBlocksInterval are missing upto targetHeight.
func (b *blockBuffer) GetBlocksIntervalsToSync(confirmedHeight uint64, targetHeight uint64) []syncBlocksInterval {
	var (
		start    uint64
		startSet bool
		bi       []syncBlocksInterval
	)

	// The sync range shouldn't go beyond tip height + buffer size to avoid being too aggressive
	if targetHeight > confirmedHeight+b.bufferSize {
		targetHeight = confirmedHeight + b.bufferSize
	}
	// The sync range should at least contain one interval to speculatively fetch missing blocks
	if targetHeight < confirmedHeight+b.intervalSize {
		targetHeight = confirmedHeight + b.intervalSize
	}

	var iLen uint64
	for h := confirmedHeight + 1; h <= targetHeight; h++ {
		if !b.contains(h) {
			iLen++
			if !startSet {
				start = h
				startSet = true
			}
			if iLen >= b.intervalSize {
				bi = append(bi, syncBlocksInterval{Start: start, End: h})
				startSet = false
				iLen = 0
			}
			continue
		}
		if startSet {
			bi = append(bi, syncBlocksInterval{Start: start, End: h - 1})
			startSet = false
			iLen = 0
		}
	}

	// handle last interval
	if startSet {
		bi = append(bi, syncBlocksInterval{Start: start, End: targetHeight})
	}
	return bi
}
