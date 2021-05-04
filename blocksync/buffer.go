// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
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
	cs           consensus.Consensus
	bufferSize   uint64
	intervalSize uint64
}

func (b *blockBuffer) store(blk *block.Block) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	height := blk.Height()
	if _, ok := b.blocks[height]; ok {
		return false
	}
	b.blocks[height] = blk

	return true
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

func (b *blockBuffer) cleanup(l *zap.Logger, height uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	size := len(b.blocks)
	if size > int(b.bufferSize)*2 {
		l.Warn("blockBuffer is leaking memory.", zap.Int("bufferSize", size))
		for h := range b.blocks {
			if h <= height {
				b.delete(h)
			}
		}
	}
}

// Flush tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) Flush(ctx context.Context, blk *block.Block) (uint64, bCheckinResult) {
	if blk == nil {
		return 0, bCheckinSkipNil
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	hu := config.NewHeightUpgrade(&bcCtx.Genesis)
	confirmedHeight := b.bc.TipHeight()
	blkHeight := blk.Height()
	if blkHeight <= confirmedHeight {
		return confirmedHeight, bCheckinLower
	}
	if blkHeight > confirmedHeight+b.bufferSize {
		return confirmedHeight, bCheckinHigher
	}
	if !b.store(blk) {
		return confirmedHeight, bCheckinExisting
	}
	l := log.L().With(
		zap.Uint64("recvHeight", blkHeight),
		zap.Uint64("confirmedHeight", confirmedHeight),
		zap.String("source", "blockBuffer"))
	syncedHeight := confirmedHeight
	for syncedHeight <= confirmedHeight+b.bufferSize {
		blk = b.delete(syncedHeight + 1)
		if blk == nil {
			break
		}
		retries := 1
		if hu.IsPre(config.Hawaii, blk.Height()) {
			retries = 4
		}
		if b.commitBlock(blk, l, retries) {
			syncedHeight++
		} else {
			break
		}
	}

	// clean up on memory leak
	b.cleanup(l, syncedHeight)

	return syncedHeight, bCheckinValid
}

func (b *blockBuffer) commitBlock(blk *block.Block, l *zap.Logger, retries int) bool {
	var err error
	for i := 0; i < retries; i++ {
		switch err = commitBlock(b.bc, b.cs, blk); errors.Cause(err) {
		case nil:
			l.Info("Successfully committed block.", zap.Uint64("syncedHeight", blk.Height()))
			return true
		case blockchain.ErrInvalidTipHeight:
			return true
		case block.ErrDeltaStateMismatch:
			continue
		case poll.ErrProposedDelegatesLength, poll.ErrDelegatesNotAsExpected, db.ErrNotExist:
			l.Debug("Failed to commit the block.", zap.Error(err), zap.Uint64("syncHeight", blk.Height()))
			return false
		default:
			l.Error("Failed to commit the block.", zap.Error(err), zap.Uint64("syncHeight", blk.Height()))
			return false
		}
	}
	return false
}

// GetBlocksIntervalsToSync returns groups of syncBlocksInterval are missing upto targetHeight.
func (b *blockBuffer) GetBlocksIntervalsToSync(targetHeight uint64) []syncBlocksInterval {
	var (
		start    uint64
		startSet bool
		bi       []syncBlocksInterval
	)

	confirmedHeight := b.bc.TipHeight()
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

// bufSize return the bufferSize of buffer
func (b *blockBuffer) bufSize() uint64 {
	return b.bufferSize
}
