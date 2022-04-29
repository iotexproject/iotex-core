package api

import (
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"go.uber.org/zap"

	logfilter "github.com/iotexproject/iotex-core/api/logfilter"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// LogListener defines the log listener in subscribed through API
type LogListener struct {
	stream    iotexapi.APIService_StreamLogsServer
	errChan   chan error
	logFilter *logfilter.LogFilter
}

// NewLogListener returns a new log listener
func NewLogListener(in *logfilter.LogFilter, stream iotexapi.APIService_StreamLogsServer, errChan chan error) *LogListener {
	return &LogListener{
		stream:    stream,
		errChan:   errChan,
		logFilter: in,
	}
}

// Respond to new block
func (ll *LogListener) Respond(blk *block.Block) error {
	if !ll.logFilter.ExistInBloomFilter(blk.LogsBloomfilter()) {
		return nil
	}
	blkHash := blk.HashBlock()
	logs := ll.logFilter.MatchLogs(blk.Receipts)
	// send matched logs thru streaming API
	for _, e := range logs {
		logPb := e.ConvertToLogPb()
		logPb.BlkHash = blkHash[:]
		if err := ll.stream.Send(&iotexapi.StreamLogsResponse{Log: logPb}); err != nil {
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
func (ll *LogListener) Exit() {
	ll.errChan <- nil
}
