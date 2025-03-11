package blockchain

import (
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	Prepare struct {
		tasks       map[hash.Hash256]chan *mintResult
		draftBlocks map[uint64]*block.Block
		mu          sync.Mutex
	}
	mintResult struct {
		blk *block.Block
		err error
	}
)

func newPrepare() *Prepare {
	return &Prepare{
		tasks:       make(map[hash.Hash256]chan *mintResult),
		draftBlocks: make(map[uint64]*block.Block),
	}
}

func (d *Prepare) PrepareBlock(prevHash []byte, mintFn func() (*block.Block, error)) {
	d.mu.Lock()
	if _, ok := d.tasks[hash.BytesToHash256(prevHash)]; ok {
		log.L().Debug("draft block already exists", log.Hex("prevHash", prevHash))
		d.mu.Unlock()
		return
	}
	res := make(chan *mintResult, 1)
	d.tasks[hash.BytesToHash256(prevHash)] = res
	d.mu.Unlock()

	go func() {
		blk, err := mintFn()
		res <- &mintResult{blk: blk, err: err}
		log.L().Debug("prepare mint returned", zap.Error(err))
	}()
}

func (d *Prepare) WaitBlock(prevHash []byte) (*block.Block, error) {
	d.mu.Lock()
	hash := hash.Hash256(prevHash)
	if ch, ok := d.tasks[hash]; ok {
		d.mu.Unlock()
		res := <-ch
		d.mu.Lock()
		delete(d.tasks, hash)
		d.mu.Unlock()
		return res.blk, res.err
	}
	d.mu.Unlock()
	return nil, nil
}

func (d *Prepare) AddDraftBlock(blk *block.Block) {
	d.mu.Lock()
	d.draftBlocks[blk.Height()] = blk
	d.mu.Unlock()
}

func (d *Prepare) ReceiveBlock(blk *block.Block) error {
	d.mu.Lock()
	delete(d.tasks, blk.PrevHash())
	delete(d.draftBlocks, blk.Height())
	d.mu.Unlock()
	return nil
}

func (d *Prepare) Block(height uint64) *block.Block {
	d.mu.Lock()
	blk := d.draftBlocks[height]
	d.mu.Unlock()
	return blk
}

func (d *Prepare) Tip() uint64 {
	d.mu.Lock()
	var tip uint64
	for h := range d.draftBlocks {
		if h > tip {
			tip = h
		}
	}
	d.mu.Unlock()
	return tip
}
