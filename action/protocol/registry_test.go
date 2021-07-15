// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	require := require.New(t)
	reg := NewRegistry()
	// Case I: Normal
	require.NoError(reg.Register("1", nil))
	// Case II: Protocol with ID is already registered
	require.Error(reg.Register("1", nil))
}

func TestFind(t *testing.T) {
	ctrl := gomock.NewController(t)

	require := require.New(t)
	reg := NewRegistry()
	p := NewMockProtocol(ctrl)
	require.NoError(reg.Register("1", p))
	// Case I: Normal
	_, ok := reg.Find("1")
	require.True(ok)
	// Case II: Not exist
	_, ok = reg.Find("0")
	require.False(ok)
	// Case III: Registry stores the item which is not a protocol
	require.NoError(reg.Register("2", nil))
	require.Nil(reg.Find("2"))
}

func TestAll(t *testing.T) {
	ctrl := gomock.NewController(t)
	require := require.New(t)
	reg := NewRegistry()
	p := NewMockProtocol(ctrl)
	require.NoError(reg.Register("1", p))
	// Case I: Normal
	require.Equal(1, len(reg.All()))
	// Case II: Registry stores the item which is not a protocol
	require.NoError(reg.Register("2", nil))
	all := reg.All()
	require.Equal(2, len(all))
	require.Equal(all[0], p)
	require.Nil(all[1])
}
