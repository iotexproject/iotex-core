// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package batch

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWriteInfo(t *testing.T) {
	ns := "namespace"
	key := []byte("key")
	value := []byte("value")
	ef := "fail to put %x"
	wi := NewWriteInfo(Put, ns, key, value, fmt.Sprintf(ef, key))
	require.Equal(t, ns, wi.Namespace())
	require.Equal(t, Put, wi.WriteType())
	require.True(t, bytes.Equal(key, wi.Key()))
	require.True(t, bytes.Equal(value, wi.Value()))
	require.Equal(t, fmt.Sprintf(ef, key), wi.Error())
	require.True(t, bytes.Equal([]byte{110, 97, 109, 101, 115, 112, 97, 99, 101, 107, 101, 121, 118, 97, 108, 117, 101}, wi.SerializeWithoutWriteType()))
	require.True(t, bytes.Equal([]byte{0, 110, 97, 109, 101, 115, 112, 97, 99, 101, 107, 101, 121, 118, 97, 108, 117, 101}, wi.Serialize()))
}
