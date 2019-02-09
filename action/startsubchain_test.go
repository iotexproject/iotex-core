// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/version"
)

func TestStartSubChain(t *testing.T) {
	assertStart := func(start *StartSubChain) {
		assert.Equal(t, uint32(version.ProtocolVersion), start.version)
		assert.Equal(t, uint64(1), start.Nonce())
		assert.Equal(t, uint32(10000), start.ChainID())
		assert.Equal(t, big.NewInt(10001), start.SecurityDeposit())
		assert.Equal(t, big.NewInt(10002), start.OperationDeposit())
		assert.Equal(t, uint64(10003), start.StartHeight())
		assert.Equal(t, uint64(10004), start.ParentHeightOffset())
		assert.Equal(t, uint64(10005), start.GasLimit())
		assert.Equal(t, big.NewInt(10006), start.GasPrice())
	}
	start := NewStartSubChain(
		1,
		10000,
		big.NewInt(10001),
		big.NewInt(10002),
		10003,
		10004,
		10005,
		big.NewInt(10006),
	)
	require.NotNil(t, start)
	assertStart(start)
}

func TestStartSubChainProto(t *testing.T) {
	assertStart := func(start *StartSubChain) {
		assert.Equal(t, uint32(10000), start.ChainID())
		assert.Equal(t, big.NewInt(10001), start.SecurityDeposit())
		assert.Equal(t, big.NewInt(10002), start.OperationDeposit())
		assert.Equal(t, uint64(10003), start.StartHeight())
		assert.Equal(t, uint64(10004), start.ParentHeightOffset())
	}

	start := &StartSubChain{
		chainID:            uint32(10000),
		securityDeposit:    big.NewInt(10001),
		operationDeposit:   big.NewInt(10002),
		startHeight:        uint64(10003),
		parentHeightOffset: uint64(10004),
	}

	startPb := start.Proto()
	require.NotNil(t, startPb)
	start = &StartSubChain{}
	assert.NoError(t, start.LoadProto(startPb))
	require.NotNil(t, start)
	assertStart(start)
}
