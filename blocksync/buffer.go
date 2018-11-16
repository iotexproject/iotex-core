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
	bCheckinSkipNil
)

// blockBuffer is used to keep in-coming block in order.
type blockBuffer struct {
	mu     sync.RWMutex
	blocks map[uint64]*blockchain.Block
	bc     blockchain.Blockchain
	ap     actpool.ActPool
	size   uint64
}

// Flush tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) Flush(blk *blockchain.Block) (bool, bCheckinResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if blk == nil {
		return false, bCheckinSkipNil
	}
	confirmedHeight := b.bc.TipHeight()
	// check
	h := blk.Height()
	if h <= confirmedHeight {
		return false, bCheckinLower
	}
	if _, ok := b.blocks[h]; ok {
		return false, bCheckinExisting
	}
	if h > confirmedHeight+b.size {
		return false, bCheckinHigher
	}
	b.blocks[h] = blk
	l := logger.With().
		Uint64("recvHeight", blk.Height()).
		Uint64("confirmedHeight", confirmedHeight).
		Str("source", "blockBuffer").
		Logger()
	syncedHeight := confirmedHeight
	for confirmedHeight+b.size > syncedHeight {
		blk, ok := b.blocks[syncedHeight+1]
		if !ok {
			break
		}
		delete(b.blocks, syncedHeight+1)
		if err := commitBlock(b.bc, b.ap, blk); err != nil {
			l.Error().Err(err).Uint64("syncHeight", syncedHeight+1).
				Msg("Failed to commit the block.")
			// unable to commit, check reason
			committedBlk, err := b.bc.GetBlockByHeight(syncedHeight + 1)
			if err != nil || committedBlk.HashBlock() != blk.HashBlock() {
				break
			}
		}
		syncedHeight++
		l.Info().Uint64("syncedHeight", syncedHeight).Msg("Successfully committed block.")
	}

	// clean up on memory leak
	if len(b.blocks) > int(b.size)*2 {
		l.Warn().Int("bufferSize", len(b.blocks)).Msg("blockBuffer is leaking memory.")
		for h := range b.blocks {
			if h <= confirmedHeight {
				delete(b.blocks, h)
			}
		}
	}

	return syncedHeight >= blk.Height(), bCheckinValid
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

	confirmedHeight := b.bc.TipHeight()
	if confirmedHeight >= targetHeight {
		return bi
	}

	if targetHeight > confirmedHeight+b.size {
		targetHeight = confirmedHeight + b.size
	}

	for h := confirmedHeight + 1; h <= targetHeight; h++ {
		if _, ok := b.blocks[h]; !ok {
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
	if _, ok := b.blocks[targetHeight]; !ok {
		if !startSet {
			start = targetHeight
		}
		bi = append(bi, syncBlocksInterval{Start: start, End: targetHeight})
	}
	return bi
}
