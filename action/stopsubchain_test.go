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

func TestStopSubChain(t *testing.T) {
	assertStop := func(stop *StopSubChain) {
		assert.Equal(t, uint32(version.ProtocolVersion), stop.version)
		assert.Equal(t, uint64(1), stop.Nonce())
		assert.Equal(t, "aaaa", stop.ChainAddress())
		assert.Equal(t, uint64(10003), stop.StopHeight())
		assert.Equal(t, uint64(10005), stop.GasLimit())
		assert.Equal(t, big.NewInt(10006), stop.GasPrice())
	}
	stop := NewStopSubChain(
		1,
		"aaaa",
		10003,
		10005,
		big.NewInt(10006),
	)
	require.NotNil(t, stop)
	assertStop(stop)
}

func TestStopSubChainProto(t *testing.T) {
	assertStop := func(stop *StopSubChain) {
		assert.Equal(t, uint64(10003), stop.StopHeight())
	}

	stop := NewStopSubChain(
		1,
		"aaaa",
		10003,
		10005,
		big.NewInt(10006),
	)
	require.NotNil(t, stop)
	assertStop(stop)

	stopPb := stop.Proto()
	require.NotNil(t, stopPb)
	stop = &StopSubChain{}
	assert.NoError(t, stop.LoadProto(stopPb))
	require.NotNil(t, stop)
	assertStop(stop)
}
