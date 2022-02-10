// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package compress

import (
	"bytes"
	"compress/gzip"
	"io"

	"github.com/golang/snappy"
	"github.com/pkg/errors"
)

// constants
const (
	Gzip   = "Gzip"
	Snappy = "Snappy"
)

// error definition
var (
	ErrInputEmpty = errors.New("input cannot be empty")
)

// Compress compresses input according to compressor
func Compress(value []byte, compressor string) ([]byte, error) {
	if value == nil {
		return nil, ErrInputEmpty
	}

	switch compressor {
	case Gzip:
		return CompGzip(value)
	case Snappy:
		return CompSnappy(value)
	default:
		panic("unsupported compressor")
	}
}

// Decompress decompresses input according to compressor
func Decompress(value []byte, compressor string) ([]byte, error) {
	switch compressor {
	case Gzip:
		return DecompGzip(value)
	case Snappy:
		return DecompSnappy(value)
	default:
		panic("unsupported compressor")
	}
}

// CompGzip uses gzip to compress the input bytes
func CompGzip(data []byte) ([]byte, error) {
	var bb bytes.Buffer
	w, err := gzip.NewWriterLevel(&bb, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	_, err = w.Write(data)
	if err != nil {
		w.Close()
		return nil, err
	}
	w.Close()
	output := bb.Bytes()
	return output, nil
}

// DecompGzip uses gzip to uncompress the input bytes
func DecompGzip(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	r.Close()
	return io.ReadAll(r)
}

// CompSnappy uses Snappy to compress the input bytes
func CompSnappy(data []byte) ([]byte, error) {
	return snappy.Encode(nil, data), nil
}

// DecompSnappy uses Snappy to decompress the input bytes
func DecompSnappy(data []byte) ([]byte, error) {
	v, err := snappy.Decode(nil, data)
	if len(v) == 0 {
		v = []byte{}
	}
	return v, err
}
