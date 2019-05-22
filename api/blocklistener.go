package api

import (
	"sync"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// blockListener defines the block listener in subscribed through API
type blockListener struct {
	pendingBlks chan *block.Block
	cancelChan  chan interface{}
	streamMap   sync.Map
}

// Start starts the block listener
func (bl *blockListener) Start() error {
	go func() {
		for {
			select {
			case <-bl.cancelChan:
				bl.streamMap.Range(func(_, value interface{}) bool {
					errChan, ok := value.(chan error)
					if !ok {
						log.S().Panic("streamMap store a value which is not an error channel")
					}
					errChan <- nil

					return true
				})
				return
			case blk := <-bl.pendingBlks:
				var receiptsPb []*iotextypes.Receipt
				for _, receipt := range blk.Receipts {
					receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
				}
				blockInfo := &iotexapi.BlockInfo{
					Block:    blk.ConvertToBlockPb(),
					Receipts: receiptsPb,
				}

				var wg sync.WaitGroup
				bl.streamMap.Range(func(key, value interface{}) bool {
					stream, ok := key.(iotexapi.APIService_StreamBlocksServer)
					if !ok {
						log.S().Panic("streamMap stores a key which is not a stream")
					}
					errChan, ok := value.(chan error)
					if !ok {
						log.S().Panic("streamMap store a value which is not an error channel")
					}
					wg.Add(1)
					go bl.sendBlock(&wg, stream, errChan, blockInfo)
					return true
				})
				wg.Wait()
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
	bl.pendingBlks <- blk
	return nil
}

// AddStream adds a new stream into streamMap
func (bl *blockListener) AddStream(stream iotexapi.APIService_StreamBlocksServer, errChan chan error) error {
	_, loaded := bl.streamMap.LoadOrStore(stream, errChan)
	if loaded {
		return errors.New("stream is already added")
	}
	return nil
}

func newBlockListener() *blockListener {
	return &blockListener{
		pendingBlks: make(chan *block.Block, 64), // Actually 1 should be enough
		cancelChan:  make(chan interface{}),
	}
}

func (bl *blockListener) sendBlock(wg *sync.WaitGroup, stream iotexapi.APIService_StreamBlocksServer, errChan chan error, blockInfo *iotexapi.BlockInfo) {
	if err := stream.Send(&iotexapi.StreamBlocksResponse{Block: blockInfo}); err != nil {
		log.L().Info(
			"Error when streaming the block",
			zap.Uint64("height", blockInfo.GetBlock().GetHeader().GetCore().GetHeight()),
			zap.Error(err),
		)
		bl.streamMap.Delete(stream)
		errChan <- err
	}
	wg.Done()
}
