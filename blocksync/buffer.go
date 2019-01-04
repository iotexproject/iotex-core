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
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
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
	mu           sync.RWMutex
	blocks       map[uint64]*block.Block
	bc           blockchain.Blockchain
	ap           actpool.ActPool
	size         uint64
	commitHeight uint64 // last commit block height
}

// CommitHeight return the last commit block height
func (b *blockBuffer) CommitHeight() uint64 {
	return b.commitHeight
}

// Flush tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) Flush(blk *block.Block) (bool, bCheckinResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if blk == nil {
		return false, bCheckinSkipNil
	}
	confirmedHeight := b.bc.TipHeight()
	// check
	blkHeight := blk.Height()
	if blkHeight <= confirmedHeight {
		return false, bCheckinLower
	}
	if _, ok := b.blocks[blkHeight]; ok {
		return false, bCheckinExisting
	}
	if blkHeight > confirmedHeight+b.size {
		return false, bCheckinHigher
	}
	b.blocks[blkHeight] = blk
	l := logger.With().
		Uint64("recvHeight", blkHeight).
		Uint64("confirmedHeight", confirmedHeight).
		Str("source", "blockBuffer").
		Logger()
	var heightToSync uint64
	for heightToSync = confirmedHeight + 1; heightToSync <= confirmedHeight+b.size; heightToSync++ {
		blk, ok := b.blocks[heightToSync]
		if !ok {
			break
		}
		delete(b.blocks, heightToSync)
		if err := commitBlock(b.bc, b.ap, blk); err != nil {
			if err == db.ErrAlreadyExist {
				l.Info().Uint64("syncHeight", heightToSync).Msg("Block already exists.")
			} else {
				l.Error().Err(err).Uint64("syncHeight", heightToSync).Msg("Failed to commit the block.")
				break
			}
		}
		b.commitHeight = heightToSync
		l.Info().Uint64("syncedHeight", heightToSync).Msg("Successfully committed block.")
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

	return heightToSync > blkHeight, bCheckinValid
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

// bufSize return the size of buffer
func (b *blockBuffer) bufSize() uint64 {
	return b.size
}
