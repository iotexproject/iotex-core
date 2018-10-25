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
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestStopSubChain(t *testing.T) {
	addr := testaddress.Addrinfo["producer"]
	assertStop := func(stop *StopSubChain) {
		assert.Equal(t, uint32(version.ProtocolVersion), stop.version)
		assert.Equal(t, uint64(1), stop.Nonce())
		assert.Equal(t, uint32(10000), stop.ChainID())
		assert.Equal(t, addr.RawAddress, stop.SrcAddr())
		assert.Equal(t, uint64(10003), stop.StopHeight())
		assert.Equal(t, uint64(10005), stop.GasLimit())
		assert.Equal(t, big.NewInt(10006), stop.GasPrice())
	}
	stop, err := NewStopSubChain(
		addr.RawAddress,
		1,
		10000,
		"aaaa",
		10003,
		10005,
		big.NewInt(10006),
	)
	require.Nil(t, err)
	require.NotNil(t, stop)
	assertStop(stop)

	stopPb := stop.Proto()
	require.NotNil(t, stopPb)
	stop = &StopSubChain{}
	stop.LoadProto(stopPb)
	require.NotNil(t, stop)
	assertStop(stop)
}
