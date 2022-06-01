package apitypes

import "github.com/iotexproject/iotex-core/blockchain/block"

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
	writer Web3ResponseWriter
	buf    []interface{}
}

// NewBatchWriter returns a new BatchWriter
func NewBatchWriter(singleWriter Web3ResponseWriter) *BatchWriter {
	return &BatchWriter{
		writer: singleWriter,
		buf:    make([]interface{}, 0),
	}
}

// Write adds data into batch buffer
func (w *BatchWriter) Write(in interface{}) error {
	w.buf = append(w.buf, in)
	return nil
}

// Flush writes data in batch buffer
func (w *BatchWriter) Flush() error {
	return w.writer.Write(w.buf)
}
