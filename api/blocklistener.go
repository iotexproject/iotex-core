package api

import (
	"encoding/hex"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// blockListener defines the block listener in subscribed through API
type blockListener struct {
	stream  iotexapi.APIService_StreamBlocksServer
	errChan chan error
}

// NewBlockListener returns a new block listener
func NewBlockListener(stream iotexapi.APIService_StreamBlocksServer, errChan chan error) Responder {
	return &blockListener{
		stream:  stream,
		errChan: errChan,
	}
}

// Respond to new block
func (bl *blockListener) Respond(blk *block.Block) error {
	var receiptsPb []*iotextypes.Receipt
	for _, receipt := range blk.Receipts {
		receiptsPb = append(receiptsPb, receipt.ConvertToReceiptPb())
	}
	blockInfo := &iotexapi.BlockInfo{
		Block:    blk.ConvertToBlockPb(),
		Receipts: receiptsPb,
	}
	h := blk.HashBlock()
	blockID := &iotextypes.BlockIdentifier{
		Hash:   hex.EncodeToString(h[:]),
		Height: blk.Height(),
	}
	// send blockInfo thru streaming API
	if err := bl.stream.Send(&iotexapi.StreamBlocksResponse{
		Block:           blockInfo,
		BlockIdentifier: blockID,
	}); err != nil {
		log.L().Info(
			"Error when streaming the block",
			zap.Uint64("height", blockInfo.GetBlock().GetHeader().GetCore().GetHeight()),
			zap.Error(err),
		)
		bl.errChan <- err
		return err
	}
	return nil
}

// Exit send to error channel
func (bl *blockListener) Exit() {
	bl.errChan <- nil
}
