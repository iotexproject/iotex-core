// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"sync"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/logger"
)

type bCheckinResult int

const (
	bCheckinValid bCheckinResult = iota + 1
	bCheckinLower
	bCheckinExisting
	bCheckinHigher
)

// blockBuffer is used to keep in-coming block in order.
type blockBuffer struct {
	mu              sync.RWMutex
	blocks          map[uint64]*blockchain.Block
	bc              blockchain.Blockchain
	ap              actpool.ActPool
	size            uint64
	startHeight     uint64
	confirmedHeight uint64
}

// Flush tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) Flush(blk *blockchain.Block) (bool, bCheckinResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	l := logger.With().Uint64("recvHeight", blk.Height()).Uint64("startHeight", b.startHeight).Uint64("confirmedHeight", b.confirmedHeight).Str("source", "blockBuffer").Logger()

	var (
		syncedHeight uint64
		moved        bool
	)

	// check
	h := blk.Height()
	if h <= b.confirmedHeight {
		return moved, bCheckinLower
	}
	if h < b.startHeight {
		if err := commitBlock(b.bc, b.ap, blk); err != nil {
			return moved, bCheckinLower
		}

		b.confirmedHeight = h
		b.startHeight = h + 1
		l.Info().Uint64("confirmedHeight", b.confirmedHeight).Msg("replace old dummy.")
		return true, bCheckinValid
	}
	if b.blocks[h] != nil {
		return moved, bCheckinExisting
	}
	if b.startHeight+b.size <= h {
		return moved, bCheckinHigher
	}
	b.blocks[h] = blk

	for syncHeight := b.startHeight; syncHeight < b.startHeight+b.size; syncHeight++ {
		if b.blocks[syncHeight] == nil {
			continue
		}
		if err := commitBlock(b.bc, b.ap, b.blocks[syncHeight]); err == nil {
			syncedHeight = syncHeight
			b.confirmedHeight = syncedHeight
			l.Info().Uint64("syncedHeight", syncedHeight).Msg("Successfully committed block.")
			delete(b.blocks, syncHeight)
			continue
		} else {
			l.Error().Err(err).Msg("Failed to commit the block")
		}

		// unable to commit, check reason
		if blk, err := b.bc.GetBlockByHeight(syncHeight); err == nil {
			if blk.HashBlock() == b.blocks[syncHeight].HashBlock() {
				// same existing block
				syncedHeight = syncHeight
				b.confirmedHeight = syncedHeight
			}
			delete(b.blocks, syncHeight)
		} else {
			th := b.bc.TipHeight()
			if syncHeight == th+1 {
				// bad block or forked here
				l.Error().Uint64("syncHeight", syncHeight).
					Uint64("syncedHeight", syncedHeight).
					Uint64("tipHeight", th).
					Msg("Failed to commit next block.")
				delete(b.blocks, syncHeight)
			}
			// otherwise block is higher than currently height
		}
	}
	if syncedHeight != 0 {
		b.startHeight = syncedHeight + 1
		moved = true
	}

	// clean up on memory leak
	if len(b.blocks) > int(b.size)*2 {
		l.Warn().Int("bufferSize", len(b.blocks)).Msg("blockBuffer is leaking memory.")
		for h := range b.blocks {
			if h < b.startHeight {
				delete(b.blocks, h)
			}
		}
	}
	return moved, bCheckinValid
}

// GetBlocksIntervalsToSync returns groups of syncBlocksInterval are missing upto targetHeight.
func (b *blockBuffer) GetBlocksIntervalsToSync(targetHeight uint64) []syncBlocksInterval {
	var (
		start    uint64
		startSet bool
		bi       []syncBlocksInterval
	)

	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.confirmedHeight >= targetHeight {
		return bi
	}

	if targetHeight >= b.startHeight+b.size {
		targetHeight = b.startHeight + b.size - 1
	}

	for h := b.confirmedHeight + 1; h <= targetHeight; h++ {
		if b.blocks[h] == nil {
			if !startSet {
				start = h
				startSet = true
			}
			continue
		}
		if startSet {
			bi = append(bi, syncBlocksInterval{Start: start, End: h - 1})
			startSet = false
		}
	}

	// handle last block
	if b.blocks[targetHeight] == nil {
		if !startSet {
			start = targetHeight
		}
		bi = append(bi, syncBlocksInterval{Start: start, End: targetHeight})
	}
	return bi
}
