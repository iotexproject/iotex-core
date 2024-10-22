package apitypes

import (
	"encoding/json"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

// MaxResponseSize is the max size of response
var MaxResponseSize = 1024 * 1024 * 100 // 100MB

type (
	// Web3ResponseWriter is writer for web3 request
	Web3ResponseWriter interface {
		Write(interface{}) (int, error)
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
	// BlobSidecarResult is the result of get blob sidecar
	BlobSidecarResult struct {
		BlobSidecar *types.BlobTxSidecar `json:"blobSidecar"`
		BlockNumber uint64               `json:"blockHeight"`
		BlockHash   common.Hash          `json:"blockHash"`
		TxIndex     uint64               `json:"txIndex"`
		TxHash      common.Hash          `json:"txHash"`
	}
)

// responseWriter for server
type responseWriter struct {
	writeHandler func(interface{}) (int, error)
}

// NewResponseWriter returns a new responseWriter
func NewResponseWriter(handler func(interface{}) (int, error)) Web3ResponseWriter {
	return &responseWriter{handler}
}

func (w *responseWriter) Write(in interface{}) (int, error) {
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
		writer: singleWriter,
		buf:    make([]json.RawMessage, 0),
	}
}

// Write adds data into batch buffer
func (w *BatchWriter) Write(in interface{}) (int, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return 0, err
	}
	w.totalSize += len(raw)
	if w.totalSize > MaxResponseSize {
		return w.totalSize, errors.New("response size exceeds limit")
	}
	w.buf = append(w.buf, raw)
	return w.totalSize, nil
}

// Flush writes data in batch buffer
func (w *BatchWriter) Flush() error {
	_, err := w.writer.Write(w.buf)
	return err
}
