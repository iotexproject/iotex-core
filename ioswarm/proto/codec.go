package proto

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
	_ "google.golang.org/grpc/encoding/proto" // ensure default proto codec is loaded
)

// CodecName is the content-subtype registered for the ioswarm JSON codec.
// Clients must use grpc.ForceCodec(JSONCodec{}) (or set content-subtype to this name)
// to interoperate with ioswarm services that exchange the plain Go structs in this package.
const CodecName = "ioswarm-json"

// JSONCodec provides JSON marshaling for ioswarm internal use.
// Use a distinct name "ioswarm-json" to avoid overriding the default "proto" codec.
// In production, replace with proper protobuf types.
type JSONCodec struct{}

func init() {
	encoding.RegisterCodec(JSONCodec{})
}

func (JSONCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (JSONCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (JSONCodec) Name() string {
	return CodecName
}
