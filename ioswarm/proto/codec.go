package proto

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto" // ensure default proto codec is loaded first
)

// jsonCodec replaces the default "proto" codec with JSON marshaling.
// This allows our plain Go structs to work with gRPC without protoc-generated code.
// In production, replace with proper protobuf types.
type jsonCodec struct{}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string {
	return "proto"
}
