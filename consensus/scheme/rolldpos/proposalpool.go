package rolldpos

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
	// nodes is a map of draft proposal blocks
	// key is the hash of the block
	nodes map[hash.Hash256]*block.Block
	// leaves is a map of tip blocks of forks
	// key is the hash of the tip block of the fork
	// value is the timestamp of the block
	leaves map[hash.Hash256]time.Time
	// root is the hash of the tip block of the blockchain
	root hash.Hash256
	mu   sync.Mutex
}

func newProposalPool() *proposalPool {
	return &proposalPool{
		nodes:  make(map[hash.Hash256]*block.Block),
		leaves: make(map[hash.Hash256]time.Time),
	}
}

func (d *proposalPool) Init(root hash.Hash256) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.root = root
	log.L().Debug("proposal pool initialized", log.Hex("root", root[:]))
}

// AddBlock adds a block to the draft pool
func (d *proposalPool) AddBlock(blk *block.Block) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	// nothing to do if the block already exists
	hash := blk.HashBlock()
	if _, ok := d.nodes[hash]; ok {
		return nil
	}
	// it must be a new tip of any fork, or make a new fork
	prevHash := blk.PrevHash()
	if _, ok := d.leaves[prevHash]; ok {
		delete(d.leaves, prevHash)
	} else if prevHash != d.root {
		return errors.Errorf("block %x is not a tip of any fork", prevHash[:])
	}
	d.leaves[hash] = blk.Timestamp()
	d.nodes[hash] = blk
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
	for f := range d.leaves {
		start := d.nodes[f]
		if f == fork {
			start = blk
		}
		for b := start; b != nil; b = d.nodes[b.PrevHash()] {
			ha := b.HashBlock()
			log.L().Debug("remove block from draft pool", log.Hex("hash", ha[:]), zap.Uint64("height", b.Height()), zap.Time("timestamp", b.Timestamp()))
			delete(d.nodes, b.HashBlock())
		}
	}
	// reset forks to only this one
	if forkTime, ok := d.leaves[fork]; ok && d.nodes[fork] != nil {
		d.leaves = map[hash.Hash256]time.Time{fork: forkTime}
	} else {
		d.leaves = map[hash.Hash256]time.Time{}
	}
	d.root = blk.HashBlock()
	return nil
}

// Block returns the latest block at the given height
func (d *proposalPool) Block(height uint64) *block.Block {
	d.mu.Lock()
	defer d.mu.Unlock()
	var blk *block.Block
	for _, b := range d.nodes {
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
	return d.nodes[hash]
}

func (d *proposalPool) BlockByTime(prevHash hash.Hash256, timestamp time.Time) *block.Block {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, b := range d.nodes {
		if b.PrevHash() == prevHash && b.Timestamp().Equal(timestamp) {
			return b
		}
	}
	return nil
}

func (d *proposalPool) forkAt(blk *block.Block) (hash.Hash256, error) {
	blkHash := blk.HashBlock()
	// If this block isn't in the pool, just return it
	if _, ok := d.nodes[blkHash]; !ok {
		return blkHash, nil
	}
	// Otherwise, find which fork chain contains it
	for forkTip := range d.leaves {
		for b := d.nodes[forkTip]; b != nil; b = d.nodes[b.PrevHash()] {
			if blkHash == b.HashBlock() {
				return forkTip, nil
			}
		}
	}
	return hash.ZeroHash256, errors.Errorf("block %x exists in the draft pool but not in the fork pool, this should not happen", blkHash[:])
}
