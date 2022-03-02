// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mptrie

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func validate(sb sortedBytes) bool {
	for i := 1; i < len(sb); i++ {
		if sb[i-1] >= sb[i] {
			return false
		}
	}
	return true
}

func TestSortedBytes(t *testing.T) {
	require := require.New(t)
	sb := sortedBytes{byte('v'), byte('d'), byte('y'), byte('x'), byte('b'), byte('a')}
	require.Equal(6, sb.Len())
	require.False(validate(sb))
	sb.Resort()
	require.True(validate(sb))
	sb.Add(byte('c'))
	require.Equal(7, sb.Len())
	require.True(validate(sb))
	sb.Add(byte('b'))
	require.Equal(7, sb.Len())
	require.True(validate(sb))
	sb.Add(byte('z'))
	require.Equal(8, sb.Len())
	require.True(validate(sb))
	sb.Delete(byte('c'))
	require.Equal(7, sb.Len())
	require.True(validate(sb))
	sb.Delete(byte('a'))
	require.Equal(6, sb.Len())
	require.True(validate(sb))
	sb.Delete(byte('b'))
	require.Equal(5, sb.Len())
	require.True(validate(sb))
	sb.Delete(byte('b'))
	require.Equal(5, sb.Len())
	require.True(validate(sb))
	sb.Delete(byte('u'))
	require.Equal(5, sb.Len())
	require.True(validate(sb))
	sb.Delete(byte('z'))
	require.Equal(4, sb.Len())
	require.True(validate(sb))
}
