package api

import (
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/api/logfilter"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type gRPCLogListener struct {
	logFilter *logfilter.LogFilter
	sender    *streamSender
}

// NewGRPCLogListener returns a new log listener
func NewGRPCLogListener(in *logfilter.LogFilter, handler streamHandler, errChan chan error) apitypes.Responder {
	return &gRPCLogListener{
		logFilter: in,
		sender:    newStreamSender("logs", handler, errChan),
	}
}

// Respond to new block. Matching logs are queued, not written inline; see
// streamSender for why the write must not happen on this goroutine.
func (ll *gRPCLogListener) Respond(_ string, blk *block.Block) error {
	if !ll.logFilter.ExistInBloomFilter(blk.LogsBloomfilter()) {
		return nil
	}
	blkHash := blk.HashBlock()
	logs := ll.logFilter.MatchLogs(blk.Receipts)
	if len(logs) == 0 {
		return nil
	}
	// enqueue one block's matched logs as a single group so a block with many
	// matching logs occupies one queue slot, not one per log — a legal per-block
	// burst must not be mistaken for a slow consumer.
	msgs := make([]interface{}, 0, len(logs))
	for _, e := range logs {
		logPb := e.ConvertToLogPb()
		logPb.BlkHash = blkHash[:]
		msgs = append(msgs, &iotexapi.StreamLogsResponse{Log: logPb})
	}
	return ll.sender.enqueue(msgs...)
}

// Exit ends the subscription and releases the RPC handler
func (ll *gRPCLogListener) Exit() {
	ll.sender.fail(nil)
}

type web3LogListener struct {
	logFilter *logfilter.LogFilter
	sender    *streamSender
}

// NewWeb3LogListener returns a new websocket block listener
func NewWeb3LogListener(filter *logfilter.LogFilter, handler streamHandler) apitypes.Responder {
	return newWeb3LogListener(filter, handler)
}

func newWeb3LogListener(filter *logfilter.LogFilter, handler streamHandler) *web3LogListener {
	return &web3LogListener{
		logFilter: filter,
		// nobody waits on a websocket subscription's outcome: a failed write
		// ends the subscription and the next Respond evicts it
		sender: newStreamSender("web3Logs", handler, make(chan error, 1)),
	}
}

// Respond to new block. Matching logs are queued as one group, not written
// inline; see streamSender.
func (ll *web3LogListener) Respond(id string, blk *block.Block) error {
	if !ll.logFilter.ExistInBloomFilter(blk.LogsBloomfilter()) {
		return nil
	}
	blkHash := blk.HashBlock()
	logs := ll.logFilter.MatchLogs(blk.Receipts)
	if len(logs) == 0 {
		return nil
	}
	msgs := make([]interface{}, 0, len(logs))
	for _, e := range logs {
		msgs = append(msgs, &streamResponse{
			id: id,
			result: &getLogsResult{
				blockHash: blkHash,
				log:       e,
			},
		})
	}
	return ll.sender.enqueue(msgs...)
}

// Exit ends the subscription
func (ll *web3LogListener) Exit() {
	ll.sender.fail(nil)
}
