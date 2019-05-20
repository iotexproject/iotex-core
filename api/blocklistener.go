package api

import "github.com/iotexproject/iotex-core/blockchain/block"

// blockListener defines the block listener in subscribed through API
type blockListener struct {
	pendingBlks chan *block.Block
	cancelChan  chan interface{}
}

// HandleBlock handles the block
func (abl *blockListener) HandleBlock(blk *block.Block) error {
	abl.pendingBlks <- blk
	return nil
}
