package blockchain

import (
	"sync"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	Prepare struct {
		draftBlocks map[hash.Hash256]chan *mintResult
		mu          sync.Mutex
	}
	mintResult struct {
		blk *block.Block
		err error
	}
)

func newPrepare() *Prepare {
	return &Prepare{
		draftBlocks: make(map[hash.Hash256]chan *mintResult),
	}
}

func (d *Prepare) PrepareBlock(prevHash []byte, mintFn func() (*block.Block, error)) {
	d.mu.Lock()
	res := make(chan *mintResult, 1)
	d.draftBlocks[hash.BytesToHash256(prevHash)] = res
	d.mu.Unlock()

	blk, err := mintFn()
	res <- &mintResult{blk: blk, err: err}
}

func (d *Prepare) WaitBlock(prevHash []byte) (*block.Block, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	hash := hash.Hash256(prevHash)
	if ch, ok := d.draftBlocks[hash]; ok {
		// wait for the draft block
		res := <-ch
		delete(d.draftBlocks, hash)
		if res.err == nil {
			return res.blk, nil
		}
		return nil, res.err
	}
	return nil, nil
}

func (d *Prepare) ReceiveBlock(blk *block.Block) error {
	d.mu.Lock()
	keys := make([]hash.Hash256, 0, len(d.draftBlocks))
	for hash, ch := range d.draftBlocks {
		select {
		case res := <-ch:
			if res.err != nil {
				keys = append(keys, hash)
			} else if res.blk.Height() <= blk.Height() {
				keys = append(keys, hash)
			}
		default:
		}
	}
	for _, key := range keys {
		delete(d.draftBlocks, key)
	}
	d.mu.Unlock()
	return nil
}
