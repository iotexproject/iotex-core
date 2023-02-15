package apitypes

import (
	"encoding/json"
	"errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

// MaxResponseSize is the max size of response
var MaxResponseSize = 1024 * 1024 * 100 // 100MB

type (
	// Web3ResponseWriter is writer for web3 request
	Web3ResponseWriter interface {
		Write(interface{}) error
	}

	// Responder responds to new block
	Responder interface {
		Respond(string, *block.Block) error
		Exit()
	}

	// Listener pass new block to all responders
	Listener interface {
		Start() error
		Stop() error
		ReceiveBlock(*block.Block) error
		AddResponder(Responder) (string, error)
		RemoveResponder(string) (bool, error)
	}

	// BlockWithReceipts includes block and its receipts
	BlockWithReceipts struct {
		Block    *block.Block
		Receipts []*action.Receipt
	}
)

// responseWriter for server
type responseWriter struct {
	writeHandler func(interface{}) error
}

// NewResponseWriter returns a new responseWriter
func NewResponseWriter(handler func(interface{}) error) Web3ResponseWriter {
	return &responseWriter{handler}
}

func (w *responseWriter) Write(in interface{}) error {
	return w.writeHandler(in)
}

// BatchWriter for multiple web3 requests
type BatchWriter struct {
	totalSize int
	writer    Web3ResponseWriter
	buf       []json.RawMessage
}

// NewBatchWriter returns a new BatchWriter
func NewBatchWriter(singleWriter Web3ResponseWriter) *BatchWriter {
	return &BatchWriter{
		writer:    singleWriter,
		buf:       make([]json.RawMessage, 0),
	}
}

// Write adds data into batch buffer
func (w *BatchWriter) Write(in interface{}) error {
	raw, err := json.Marshal(in)
	if err != nil {
		return err
	}
	w.totalSize += len(raw)
	if w.totalSize > _maxResponseSize {
		return errors.New("response size exceeds limit")
	}
	w.buf = append(w.buf, raw)
	return nil
}

// Flush writes data in batch buffer
func (w *BatchWriter) Flush() error {
	return w.writer.Write(w.buf)
}
