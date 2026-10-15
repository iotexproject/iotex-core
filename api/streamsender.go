// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package api

import (
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	// streamSendQueueSize bounds how many per-block message groups one streaming
	// subscription may have pending. A consumer that falls further behind is
	// dropped rather than allowed to apply backpressure. One block enqueues at
	// most one group regardless of how many messages it produces, so a single
	// block's legitimate burst (e.g. a block with many matching logs) never
	// overflows the queue on its own.
	streamSendQueueSize = 64
)

// errSlowStreamConsumer is returned to the chain listener so it evicts the
// subscription; the RPC handler turns it into a ResourceExhausted status.
var errSlowStreamConsumer = errors.New("stream consumer too slow, dropping subscription")

// streamSender decouples Responder.Respond from the actual stream write.
//
// Respond runs on the single goroutine that fans every committed block out to
// every subscriber, and that goroutine is what blockchain.commitBlock waits on
// while CommitBlock holds the chain write lock. A gRPC stream write, by
// contrast, can block for as long as the peer is not reading: HTTP/2 flow
// control has no deadline and keepalive does not help, because the peer's
// transport answers PINGs whether or not the application reads. Writing to the
// stream directly from Respond would therefore couple block commitment to the
// slowest stream consumer.
//
// So Respond only enqueues, and never blocks: a bounded queue is drained by one
// goroutine per subscription. A blocked write now costs exactly that one
// goroutine, which unblocks when the RPC handler returns and the stream is torn
// down. Overflow drops the subscription rather than slowing the fan-out.
type streamSender struct {
	kind    string
	handle  streamHandler
	errChan chan error
	queue   chan []interface{}
	closed  chan struct{}
	once    sync.Once
}

func newStreamSender(kind string, handle streamHandler, errChan chan error) *streamSender {
	s := &streamSender{
		kind:    kind,
		handle:  handle,
		errChan: errChan,
		queue:   make(chan []interface{}, streamSendQueueSize),
		closed:  make(chan struct{}),
	}
	go s.run()
	return s
}

// enqueue hands one block's worth of messages to the writer goroutine as a
// single queue slot. It never blocks: if the queue is full the subscription is
// terminated and the error is returned so the caller evicts this responder.
//
// Grouping per block is deliberate: a block that legitimately produces many
// messages (e.g. more than streamSendQueueSize matching logs) occupies exactly
// one slot, so a single-block burst can never be mistaken for a slow consumer.
// Only a consumer that falls behind by streamSendQueueSize *blocks* is dropped.
func (s *streamSender) enqueue(msgs ...interface{}) error {
	if len(msgs) == 0 {
		return nil
	}
	select {
	case <-s.closed:
		return errSlowStreamConsumer
	default:
	}
	select {
	case s.queue <- msgs:
		return nil
	default:
		apiStreamDropMtc.WithLabelValues(s.kind).Inc()
		log.L().Warn("stream consumer is not keeping up, dropping subscription",
			zap.String("stream", s.kind), zap.Int("queuedBlocks", cap(s.queue)))
		s.fail(errSlowStreamConsumer)
		return errSlowStreamConsumer
	}
}

func (s *streamSender) run() {
	for {
		select {
		case <-s.closed:
			return
		case group := <-s.queue:
			for _, msg := range group {
				if _, err := s.handle(msg); err != nil {
					log.L().Info("error writing to stream",
						zap.String("stream", s.kind), zap.Error(err))
					s.fail(err)
					return
				}
			}
		}
	}
}

// fail ends the subscription exactly once and reports err to the RPC handler.
// The send is non-blocking and errChan is never closed, so neither Exit nor a
// write error can deadlock a caller that is holding the chain listener lock.
func (s *streamSender) fail(err error) {
	s.once.Do(func() {
		close(s.closed)
		select {
		case s.errChan <- err:
		default:
		}
	})
}
