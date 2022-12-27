package api

import (
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/api/logfilter"
	apitypes "github.com/iotexproject/iotex-core/api/types"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type gRPCLogListener struct {
	logFilter    *logfilter.LogFilter
	streamHandle streamHandler
	errChan      chan error
}

// NewGRPCLogListener returns a new log listener
func NewGRPCLogListener(in *logfilter.LogFilter, handler streamHandler, errChan chan error) apitypes.Responder {
	return &gRPCLogListener{
		logFilter:    in,
		streamHandle: handler,
		errChan:      errChan,
	}
}

// Respond to new block
func (ll *gRPCLogListener) Respond(_ string, blk *block.Block) error {
	if !ll.logFilter.ExistInBloomFilter(blk.LogsBloomfilter()) {
		return nil
	}
	blkHash := blk.HashBlock()
	logs := ll.logFilter.MatchLogs(blk.Receipts)
	// send matched logs thru streaming API
	for _, e := range logs {
		logPb := e.ConvertToLogPb()
		logPb.BlkHash = blkHash[:]
		if err := ll.streamHandle(&iotexapi.StreamLogsResponse{Log: logPb}); err != nil {
			ll.errChan <- err
			log.L().Info("error streaming the log",
				zap.Uint64("height", e.BlockHeight),
				zap.Error(err))
			return err
		}
	}
	return nil
}

// Exit send to error channel
func (ll *gRPCLogListener) Exit() {
	ll.errChan <- nil
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
		if err := ll.streamHandle(res); err != nil {
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
