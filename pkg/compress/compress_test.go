// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package compress

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompressDataBytes(t *testing.T) {
	r := require.New(t)

	// cannot compress nil nor decompress empty byte slice
	_, err := Compress(nil, "")
	r.Equal(ErrInputEmpty, err)
	_, err = Decompress([]byte{}, Gzip)
	r.Error(err)
	_, err = Decompress([]byte{}, Snappy)
	r.Error(err)
	r.Panics(func() { Compress([]byte{}, "invalid") })
	r.Panics(func() { Decompress([]byte{}, "invalid") })

	zero := [32]byte{}
	blkHash, _ := hex.DecodeString("22cd0c2d1f7d65298cec7599e2d0e3c650dd8b4ed2b1c816d909026c60d785b2")
	compressTests := [][]byte{
		[]byte{},
		[]byte{1, 2, 3},
		zero[:],
		blkHash,
		[]byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`1234567890-=~!@#$%^&*()_+å∫ç∂´´©˙ˆˆ˚¬µ˜˜πœ®ß†¨¨∑≈¥Ω[]',./{}|:<>?"),
	}
	for _, ser := range compressTests {
		for _, compress := range []string{Gzip, Snappy} {
			v, err := Compress(ser, compress)
			r.NoError(err)

			ser1, err := Decompress(v, compress)
			r.NoError(err)
			r.Equal(ser, ser1)
		}
	}
}
