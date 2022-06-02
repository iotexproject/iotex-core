package api

import (
	"encoding/hex"

	"go.uber.org/zap"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type streamHandler func(interface{}) error

type gRPCBlockListener struct {
	streamHandle streamHandler
	errChan      chan error
}

// NewGRPCBlockListener returns a new gRPC block listener
func NewGRPCBlockListener(handler streamHandler, errChan chan error) apitypes.Responder {
	return &gRPCBlockListener{
		streamHandle: handler,
		errChan:      errChan,
	}
}

// Respond to new block
func (bl *gRPCBlockListener) Respond(_ string, blk *block.Block) error {
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
	if err := bl.streamHandle(&iotexapi.StreamBlocksResponse{
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
func (bl *gRPCBlockListener) Exit() {
	bl.errChan <- nil
}

type web3BlockListener struct {
	streamHandle streamHandler
}

// NewWeb3BlockListener returns a new websocket block listener
func NewWeb3BlockListener(handler streamHandler) apitypes.Responder {
	return &web3BlockListener{
		streamHandle: handler,
	}
}

// Respond to new block
func (bl *web3BlockListener) Respond(id string, blk *block.Block) error {
	res := &streamResponse{
		id: id,
		result: &getBlockResultV2{
			blk: blk,
		},
	}
	// send blockInfo thru streaming API
	if err := bl.streamHandle(res); err != nil {
		log.L().Info(
			"Error when streaming the block",
			zap.Uint64("height", blk.Height()),
			zap.Error(err),
		)
		return err
	}
	return nil
}

// Exit send to error channel
func (bl *web3BlockListener) Exit() {}
