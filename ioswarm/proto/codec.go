package proto

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto" // ensure default proto codec is loaded
)

// jsonCodec provides JSON marshaling for ioswarm internal use.
// Use a distinct name "ioswarm-json" to avoid overriding the default "proto" codec.
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
	return "ioswarm-json" // Use distinct name to avoid conflict with standard proto codec
}
