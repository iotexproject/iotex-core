// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnc(t *testing.T) {
	require := require.New(t)

	data := []struct {
		input string
		key   string
	}{
		{"Foo", "Boo"},
		{"Bar", "Car"},
		{"Bar", ""},
		{"", "Car"},
		{"Long input with more than 16 characters", "Car"},
	}
	for _, d := range data {
		enc, err := Encrypt([]byte(d.input), HashSHA256([]byte(d.key)))
		require.NoError(err)

		dec, err := Decrypt(enc, HashSHA256([]byte(d.key)))
		require.NoError(err)

		require.Equal(string(dec), d.input)
	}
}
