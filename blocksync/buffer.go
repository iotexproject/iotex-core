// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"sync"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// blockBuffer is used to keep in-coming block in order.
type blockBuffer struct {
	mu           sync.RWMutex
	blocks       map[uint64]*block.Block
	bufferSize   uint64
	intervalSize uint64
}

type syncBlocksInterval struct {
	Start uint64
	End   uint64
}

func (b *blockBuffer) load(height uint64) *block.Block {
	b.mu.RLock()
	defer b.mu.RUnlock()
	blk, ok := b.blocks[height]
	if !ok {
		return nil
	}

	return blk
}

func (b *blockBuffer) delete(height uint64) *block.Block {
	b.mu.Lock()
	defer b.mu.Unlock()
	blk, ok := b.blocks[height]
	if !ok {
		return nil
	}
	delete(b.blocks, height)

	return blk
}

func (b *blockBuffer) cleanup(height uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	size := len(b.blocks)
	if size > int(b.bufferSize)*2 {
		log.L().Warn("blockBuffer is leaking memory.", zap.Int("bufferSize", size))
		newBlocks := map[uint64]*block.Block{}
		for h := range b.blocks {
			if h > height {
				newBlocks[h] = b.blocks[h]
			}
		}
		b.blocks = newBlocks
	}
}

// AddBlock tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) AddBlock(tipHeight uint64, blk *block.Block) {
	blkHeight := blk.Height()
	if blkHeight > tipHeight && blkHeight <= tipHeight+b.bufferSize {
		b.mu.Lock()
		defer b.mu.Unlock()

		if _, ok := b.blocks[blkHeight]; !ok {
			b.blocks[blkHeight] = blk
		}
	}
}

func (b *blockBuffer) Flush(tipHeight uint64, commitBlock func(*block.Block) error) uint64 {
	syncedHeight := tipHeight
	log.L().Debug("Flush blocks", zap.Uint64("tipHeight", tipHeight), zap.String("source", "blockBuffer"))
	for syncedHeight <= tipHeight+b.bufferSize {
		blk := b.delete(syncedHeight + 1)
		if blk == nil {
			break
		}
		if err := commitBlock(blk); err == nil {
			syncedHeight++
		} else {
			log.L().Debug("failed to commit block", zap.Error(err))
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
		if b.load(h) == nil {
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
