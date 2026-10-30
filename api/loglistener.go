package api

import (
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/api/logfilter"
	apitypes "github.com/iotexproject/iotex-core/v2/api/types"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
	logFilter    *logfilter.LogFilter
	streamHandle streamHandler
}

// NewWeb3LogListener returns a new websocket block listener
func NewWeb3LogListener(filter *logfilter.LogFilter, handler streamHandler) apitypes.Responder {
	return &web3LogListener{
		logFilter:    filter,
		streamHandle: handler,
	}
}

// Respond to new block
func (ll *web3LogListener) Respond(id string, blk *block.Block) error {
	if !ll.logFilter.ExistInBloomFilter(blk.LogsBloomfilter()) {
		return nil
	}
	blkHash := blk.HashBlock()
	logs := ll.logFilter.MatchLogs(blk.Receipts)

	for _, e := range logs {
		res := &streamResponse{
			id: id,
			result: &getLogsResult{
				blockHash: blkHash,
				log:       e,
			},
		}
		if _, err := ll.streamHandle(res); err != nil {
			log.L().Info(
				"Error when streaming the block",
				zap.Uint64("height", blk.Height()),
				zap.Error(err),
			)
			return err
		}
	}
	return nil
}

// Exit send to error channel
func (ll *web3LogListener) Exit() {}
