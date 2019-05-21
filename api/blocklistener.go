package api

import (
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// blockListener defines the block listener in subscribed through API
type blockListener struct {
	pendingBlks chan *block.Block
	cancelChan  chan interface{}
	streamList  []iotexapi.APIService_StreamBlocksServer
}

// Start starts the block listener
func (bl *blockListener) Start() error {
	go func() {
		for {
			select {
			case <-bl.cancelChan:
				return
			case blk := <-bl.pendingBlks:
				var receiptsPb []*iotextypes.Receipt
				for _, receipt := range blk.Receipts {
					receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
				}
				for i := len(bl.streamList) - 1; i >= 0; i-- {
					if err := bl.streamList[i].Send(&iotexapi.StreamBlocksResponse{
						Block: &iotexapi.BlockInfo{
							Block:    blk.ConvertToBlockPb(),
							Receipts: receiptsPb,
						},
					}); err != nil {
						log.L().Info(
							"Error when streaming the block",
							zap.Uint64("height", blk.Height()),
							zap.Error(err),
						)
						bl.streamList = append(bl.streamList[:i], bl.streamList[i+1:]...)
					}
				}
			}
		}
	}()
	return nil
}

// Stop stops the block listener
func (bl *blockListener) Stop() error {
	close(bl.cancelChan)
	return nil
}

// HandleBlock handles the block
func (bl *blockListener) HandleBlock(blk *block.Block) error {
	if len(bl.streamList) > 0 {
		bl.pendingBlks <- blk
	}
	return nil
}
