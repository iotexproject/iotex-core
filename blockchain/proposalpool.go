package blockchain

import (
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// proposalPool is a pool of draft blocks
type proposalPool struct {
	// blocks is a map of draft blocks
	// key is the hash of the block
	blocks map[hash.Hash256]*block.Block
	// forks is a map of draft blocks that are forks
	// key is the hash of the tip block of the fork
	// value is the timestamp of the block
	forks map[hash.Hash256]time.Time
	// root is the hash of the tip block of the blockchain
	root hash.Hash256
	mu   sync.Mutex
}

func newProposalPool() *proposalPool {
	return &proposalPool{
		blocks: make(map[hash.Hash256]*block.Block),
		forks:  make(map[hash.Hash256]time.Time),
	}
}

func (d *proposalPool) Init(root hash.Hash256) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.root = root
}

// AddBlock adds a block to the draft pool
func (d *proposalPool) AddBlock(blk *block.Block) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// nothing to do if the block already exists
	hash := blk.HashBlock()
	if _, ok := d.blocks[hash]; ok {
		return nil
	}
	// it must be a new tip of any fork, or make a new fork
	prevHash := blk.PrevHash()
	if _, ok := d.forks[prevHash]; ok {
		delete(d.forks, prevHash)
	} else if prevHash != d.root {
		return errors.Errorf("block %x is not a tip of any fork", prevHash[:])
	}
	d.forks[hash] = blk.Timestamp()
	d.blocks[hash] = blk
	log.L().Debug("added block to draft pool", log.Hex("hash", hash[:]), zap.Uint64("height", blk.Height()), zap.Time("timestamp", blk.Timestamp()))
	return nil
}

// ReceiveBlock a block has been confirmed and remove all the blocks that is previous to it, or other forks
func (d *proposalPool) ReceiveBlock(blk *block.Block) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// find the fork that contains this block
	fork, err := d.forkAt(blk)
	if err != nil {
		return err
	}

	// remove blocks in other forks or older blocks in the same fork
	for f := range d.forks {
		start := d.blocks[f]
		if f == fork {
			start = blk
		}
		for b := start; b != nil; b = d.blocks[b.PrevHash()] {
			ha := b.HashBlock()
			log.L().Debug("remove block from draft pool", log.Hex("hash", ha[:]), zap.Uint64("height", b.Height()), zap.Time("timestamp", b.Timestamp()))
			delete(d.blocks, b.HashBlock())
		}
	}
	// reset forks to only this one
	if forkTime, ok := d.forks[fork]; ok && d.blocks[fork] != nil {
		d.forks = map[hash.Hash256]time.Time{fork: forkTime}
	} else {
		d.forks = map[hash.Hash256]time.Time{}
	}
	d.root = blk.HashBlock()
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

// BlockByHash returns the block by hash
func (d *proposalPool) BlockByHash(hash hash.Hash256) *block.Block {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.blocks[hash]
}

// Tip returns the tip height of the latest fork
func (d *proposalPool) Tip() uint64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	var fork hash.Hash256
	for f := range d.forks {
		if fork == hash.ZeroHash256 || d.forks[f].After(d.forks[fork]) {
			fork = f
		}
	}
	var tip uint64
	if b := d.blocks[fork]; b != nil {
		tip = b.Height()
	}
	return tip
}

func (d *proposalPool) forkAt(blk *block.Block) (hash.Hash256, error) {
	blkHash := blk.HashBlock()
	// If this block isn't in the pool, just return it
	if _, ok := d.blocks[blkHash]; !ok {
		return blkHash, nil
	}
	// Otherwise, find which fork chain contains it
	for forkTip := range d.forks {
		for b := d.blocks[forkTip]; b != nil; b = d.blocks[b.PrevHash()] {
			if blkHash == b.HashBlock() {
				return forkTip, nil
			}
		}
	}
	return hash.ZeroHash256, errors.Errorf("block %x exists in the draft pool but not in the fork pool, this should not happen", blkHash[:])
}
