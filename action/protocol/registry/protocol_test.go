// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package registry

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	prtcl "github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/testutil"
)

type dummyProtocol struct {
	addr address.Address
	tags [][]byte
}

func newDummyProtocol(addr address.Address, tags [][]byte) *dummyProtocol {
	return &dummyProtocol{
		addr: addr,
		tags: tags,
	}
}

func (p *dummyProtocol) Address() address.Address { return p.addr }

func (p *dummyProtocol) Tags() [][]byte { return p.tags }

func (p *dummyProtocol) Handle(context.Context, action.Action, prtcl.StateManager) (*action.Receipt, error) {
	return nil, nil
}

func (p *dummyProtocol) Validate(context.Context, action.Action) error { return nil }

func TestProtocol(t *testing.T) {
	registry := NewProtocol()

	// Register a protocol
	addr1 := testutil.NewTestAddress()
	p1 := newDummyProtocol(addr1, nil)
	require.NoError(t, registry.Register(p1))
	_, ok := registry.Get(addr1)
	assert.True(t, ok)

	// Register another protocol on the same address will fail
	p2 := newDummyProtocol(addr1, nil)
	require.Error(t, registry.Register(p2))

	// Get a protocol by a non-existing address
	addr2 := testutil.NewTestAddress()
	_, ok = registry.Get(addr2)
	assert.False(t, ok)

	// Register 2nd protocol
	p2 = newDummyProtocol(addr2, [][]byte{[]byte("hello"), []byte("world")})
	require.NoError(t, registry.Register(p2))

	// Register 3rd protocol
	addr3 := testutil.NewTestAddress()
	p3 := newDummyProtocol(addr3, [][]byte{[]byte("hello"), []byte("iotex")})
	require.NoError(t, registry.Register(p3))

	// Filtering by hello would return ether p2 or p3
	protocol1, ok := registry.FindFirst(func(protocol prtcl.Protocol) bool {
		for _, tag := range protocol.Tags() {
			if bytes.Equal(tag, []byte("hello")) {
				return true
			}
		}
		return false
	})
	assert.True(t, ok)
	assert.True(t, protocol1 == p2 || protocol1 == p3)

	// Filtering by world would return p2
	protocol2, ok := registry.FindFirst(func(protocol prtcl.Protocol) bool {
		for _, tag := range protocol.Tags() {
			if bytes.Equal(tag, []byte("world")) {
				return true
			}
		}
		return false
	})
	assert.True(t, ok)
	assert.Equal(t, p2, protocol2)

	// Filtering by other wouldn't find anything
	_, ok = registry.FindFirst(func(protocol prtcl.Protocol) bool {
		for _, tag := range protocol.Tags() {
			if bytes.Equal(tag, []byte("other")) {
				return true
			}
		}
		return false
	})
	assert.False(t, ok)
}
