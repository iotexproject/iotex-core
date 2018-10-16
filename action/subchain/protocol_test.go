// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/stretchr/testify/require"
)

func TestProtocolValidateSubChainStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().TipHeight().Return(uint64(100)).AnyTimes()
	factory := mock_state.NewMockFactory(ctrl)
	factory.EXPECT().LoadOrCreateState(gomock.Any(), gomock.Any()).Return(
		&state.State{Balance: big.NewInt(2000000000)},
		nil,
	).AnyTimes()
	ws := mock_state.NewMockWorkingSet(ctrl)
	ws.EXPECT().LoadOrCreateState(gomock.Any(), gomock.Any()).Return(
		&state.State{Balance: big.NewInt(1500000000)},
		nil,
	).AnyTimes()

	defer ctrl.Finish()

	p := NewProtocol(chain, factory)
	p.subChains[3] = &subChain{}

	start := action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		MinSecurityDeposit,
		big.NewInt(1000000000),
		110,
		10,
		0,
		big.NewInt(0),
	)
	assert.NoError(t, p.validateStartSubChain(start, nil))

	// chain ID is the main chain ID
	start = action.NewStartSubChain(
		1,
		1,
		testaddress.Addrinfo["producer"].RawAddress,
		MinSecurityDeposit,
		big.NewInt(1000000000),
		110,
		10,
		0,
		big.NewInt(0),
	)
	err := p.validateStartSubChain(start, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is the chain ID reserved for main chain"))

	// chain ID is used
	start = action.NewStartSubChain(
		1,
		3,
		testaddress.Addrinfo["producer"].RawAddress,
		MinSecurityDeposit,
		big.NewInt(1000000000),
		110,
		10,
		0,
		big.NewInt(0),
	)
	err = p.validateStartSubChain(start, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by another sub-chain"))

	// security deposit is lower than the minimum
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(500000000),
		big.NewInt(1000000000),
		110,
		10,
		0,
		big.NewInt(0),
	)
	err = p.validateStartSubChain(start, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "security deposit is smaller than the minimal requirement"))

	// security deposit is more than the owner balance
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(2100000000),
		big.NewInt(1000000000),
		110,
		10,
		0,
		big.NewInt(0),
	)
	err = p.validateStartSubChain(start, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "sub-chain owner doesn't have enough balance for security deposit"))

	// operation deposit is more than the owner balance
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(1000000000),
		big.NewInt(1100000000),
		110,
		10,
		0,
		big.NewInt(0),
	)
	err = p.validateStartSubChain(start, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "sub-chain owner doesn't have enough balance for operation deposit"))

	// start height doesn't have enough delay
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(1000000000),
		big.NewInt(1000000000),
		109,
		10,
		0,
		big.NewInt(0),
	)
	err = p.validateStartSubChain(start, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "sub-chain could be started no early than"))
}
