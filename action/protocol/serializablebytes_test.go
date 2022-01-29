// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSerializableBytes(t *testing.T) {
	require := require.New(t)
	data := []byte("some data")
	var sb SerializableBytes
	require.NoError(sb.Deserialize(data))
	c, err := sb.Serialize()
	require.NoError(err)
	require.True(bytes.Equal(data, c))
}
