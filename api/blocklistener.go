package api

import (
	"encoding/hex"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type streamHandler func(interface{}) (int, error)

type gRPCBlockListener struct {
	sender *streamSender
}

// NewGRPCBlockListener returns a new gRPC block listener
func NewGRPCBlockListener(handler streamHandler, errChan chan error) apitypes.Responder {
	return &gRPCBlockListener{
		sender: newStreamSender("blocks", handler, errChan),
	}
}

// Respond to new block. It only serialises the block and queues it; the write
// itself happens on the subscription's own goroutine, so a peer that stops
// reading cannot stall block commitment. See streamSender.
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
	return bl.sender.enqueue(&iotexapi.StreamBlocksResponse{
		Block:           blockInfo,
		BlockIdentifier: blockID,
	})
}

// Exit ends the subscription and releases the RPC handler
func (bl *gRPCBlockListener) Exit() {
	bl.sender.fail(nil)
}

type web3BlockListener struct {
	sender *streamSender
}

// NewWeb3BlockListener returns a new websocket block listener
func NewWeb3BlockListener(handler streamHandler) apitypes.Responder {
	return newWeb3BlockListener(handler)
}

func newWeb3BlockListener(handler streamHandler) *web3BlockListener {
	return &web3BlockListener{
		// nobody waits on a websocket subscription's outcome: a failed write
		// ends the subscription and the next Respond evicts it
		sender: newStreamSender("web3Blocks", handler, make(chan error, 1)),
	}
}

// Respond to new block. Like the gRPC listener it only queues the block; the
// websocket write happens on the subscription's own goroutine. See streamSender.
func (bl *web3BlockListener) Respond(id string, blk *block.Block) error {
	return bl.sender.enqueue(&streamResponse{
		id: id,
		result: &getBlockResult{
			blk: blk,
		},
	})
}

// Exit ends the subscription
func (bl *web3BlockListener) Exit() {
	bl.sender.fail(nil)
}
