// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package compress

import (
	"bytes"
	"compress/gzip"
	"io/ioutil"
)

// Compress uses gzip to compress the input bytes
func Compress(data []byte) ([]byte, error) {
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

// Decompress uses gzip to uncompress the input bytes
func Decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	r.Close()
	return ioutil.ReadAll(r)
}
