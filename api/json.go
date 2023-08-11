package api

import (
	"encoding/json"
	"fmt"

	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/grpc/status"
)

const (
	vsn              = "2.0"
	defaultErrorCode = -32000
)

var null = json.RawMessage("null")

// error response. Which one it is depends on the fields.
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc,omitempty"`
	ID      int64           `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (obj *jsonrpcMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(*obj)
}

func successMessage(result interface{}) *jsonrpcMessage {
	msg := &jsonrpcMessage{Version: vsn, Result: null}
	if result != nil {
		msg.Result, _ = json.Marshal(result)
	}
	return msg
}

func errorMessage(err error) *jsonrpcMessage {
	msg := &jsonrpcMessage{Version: vsn, Error: &jsonError{
		Code:    defaultErrorCode,
		Message: err.Error(),
	}}
	de, ok := err.(rpc.DataError)
	if ok {
		msg.Error.Data = de.ErrorData()
	}
	ec, ok := err.(rpc.Error)
	if ok {
		msg.Error.Code = ec.ErrorCode()
		return msg
	}
	if s, ok := status.FromError(err); ok {
		msg.Error.Code, msg.Error.Message = int(s.Code()), s.Message()
	} else {
		msg.Error.Code, msg.Error.Message = -32603, err.Error()
	}
	return msg
}

type jsonError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (err *jsonError) Error() string {
	if err.Message == "" {
		return fmt.Sprintf("json-rpc error %d", err.Code)
	}
	return err.Message
}

func (err *jsonError) ErrorCode() int {
	return err.Code
}

func (err *jsonError) ErrorData() interface{} {
	return err.Data
}
