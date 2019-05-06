// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package hash

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHash(t *testing.T) {
	require := require.New(t)

	tests := []struct {
		msg  string
		hash string
	}{
		{"", "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"},
		{"abc", "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45"},
	}

	for _, test := range tests {
		h := Hash256b([]byte(test.msg))
		e, _ := hex.DecodeString(test.hash)
		require.Equal(e, h[:])
		h1 := Hash160b([]byte(test.msg))
		require.Equal(e[12:], h1[:])
		require.Equal(h, BytesToHash256(append([]byte{1, 2, 3, 4}, h[:]...)))
		require.Equal(h1, BytesToHash160(append([]byte{1, 2, 3, 4}, h1[:]...)))
	}
}