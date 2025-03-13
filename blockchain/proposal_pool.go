package blockchain

import (
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

// proposalPool is a pool of draft blocks
type proposalPool struct {
	blocks map[hash.Hash256]*block.Block
	forks  map[hash.Hash256]time.Time
	mu     sync.Mutex
}

func newProposalPool() *proposalPool {
	return &proposalPool{
		blocks: make(map[hash.Hash256]*block.Block),
		forks:  make(map[hash.Hash256]time.Time),
	}
}

// AddBlock adds a block to the draft pool
func (d *proposalPool) AddBlock(blk *block.Block) {
	d.mu.Lock()
	defer d.mu.Unlock()
	hash := blk.HashBlock()
	if _, ok := d.blocks[hash]; ok {
		return
	}
	d.blocks[hash] = blk
	prevHash := blk.PrevHash()
	if _, ok := d.forks[prevHash]; ok {
		delete(d.forks, prevHash)
	}
	d.forks[hash] = blk.Timestamp()
}

// ReceiveBlock a block has been confirmed and remove all drafts that are lower than the confirmed block
func (d *proposalPool) ReceiveBlock(blk *block.Block) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	fork, err := d.forkAt(blk)
	if err != nil {
		return err
	}
	forkTime := d.blocks[fork].Timestamp()
	for f := range d.forks {
		start := d.blocks[f]
		if f == fork {
			start = blk
		}
		for b := start; b != nil; b = d.blocks[b.PrevHash()] {
			delete(d.blocks, b.HashBlock())
		}
	}
	d.forks = make(map[hash.Hash256]time.Time)
	d.forks[fork] = forkTime

	return nil
}

// Block returns the latest block at the given height
func (d *proposalPool) Block(height uint64) *block.Block {
	d.mu.Lock()
	defer d.mu.Unlock()
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
	return blk
}

// Tip returns the tip height of the draft pool
func (d *proposalPool) Tip() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	var tip uint64
	for h := range d.blocks {
		if tip < d.blocks[h].Height() {
			tip = d.blocks[h].Height()
		}
	}
	return tip
}

func (d *proposalPool) isForkedAt(blk *block.Block, fork hash.Hash256) bool {
	hash := blk.HashBlock()
	for b := d.blocks[fork]; b != nil; b = d.blocks[b.PrevHash()] {
		if hash == b.HashBlock() {
			return true
		}
	}
	return false
}

func (d *proposalPool) forkAt(blk *block.Block) (hash.Hash256, error) {
	blkHash := blk.HashBlock()
	if _, ok := d.blocks[blkHash]; !ok {
		d.blocks[blkHash] = blk
		if _, ok := d.forks[blk.PrevHash()]; ok {
			delete(d.forks, blk.PrevHash())
		}
		d.forks[blkHash] = blk.Timestamp()
		return blkHash, nil
	}
	for h := range d.forks {
		if d.isForkedAt(blk, h) {
			return h, nil
		}
	}
	return hash.ZeroHash256, errors.Errorf("block %x exists in the draft pool but not in the fork pool, this should not happen", blkHash[:])
}
