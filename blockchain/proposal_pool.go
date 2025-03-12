package blockchain

import (
	"sync"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

// proposalPool is a pool of draft blocks
type proposalPool struct {
	blocks map[hash.Hash256]*block.Block
	mu     sync.Mutex
}

func newProposalPool() *proposalPool {
	return &proposalPool{
		blocks: make(map[hash.Hash256]*block.Block),
	}
}

// AddBlock adds a block to the draft pool
func (d *proposalPool) AddBlock(blk *block.Block) {
	d.mu.Lock()
	d.blocks[blk.HashBlock()] = blk
	d.mu.Unlock()
}

// ReceiveBlock a block has been confirmed and remove all drafts that are lower than the confirmed block
func (d *proposalPool) ReceiveBlock(blk *block.Block) error {
	d.mu.Lock()
	toDels := make([]hash.Hash256, 0)
	for h := range d.blocks {
		if d.blocks[h].Height() <= blk.Height() {
			toDels = append(toDels, h)
		}
	}
	for _, h := range toDels {
		delete(d.blocks, h)
	}
	d.mu.Unlock()
	return nil
}

// Block returns the latest block at the given height
func (d *proposalPool) Block(height uint64) *block.Block {
	d.mu.Lock()
	var blk *block.Block
	for _, b := range d.blocks {
		if b.Height() != height {
			continue
		} else if blk == nil {
			blk = b
		} else if b.Timestamp().After(blk.Timestamp()) {
			blk = b
		}
	}
	d.mu.Unlock()
	return blk
}

// Tip returns the tip height of the draft pool
func (d *proposalPool) Tip() uint64 {
	d.mu.Lock()
	var tip uint64
	for h := range d.blocks {
		if tip < d.blocks[h].Height() {
			tip = d.blocks[h].Height()
		}
	}
	d.mu.Unlock()
	return tip
}
