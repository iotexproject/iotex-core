// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestProtocolValidateSubChainStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().TipHeight().Return(uint64(100)).AnyTimes()
	factory := mock_state.NewMockFactory(ctrl)
	factory.EXPECT().AccountState(gomock.Any()).Return(
		&state.Account{Balance: big.NewInt(0).Mul(big.NewInt(2000000000), big.NewInt(blockchain.Iotx))},
		nil,
	).AnyTimes()
	ws := mock_state.NewMockWorkingSet(ctrl)
	ws.EXPECT().CachedAccountState(gomock.Any()).Return(
		&state.Account{Balance: big.NewInt(0).Mul(big.NewInt(1500000000), big.NewInt(blockchain.Iotx))},
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
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, err := p.validateStartSubChain(start, nil)
	assert.NotNil(t, account)
	assert.NoError(t, err)

	// chain ID is the main chain ID
	start = action.NewStartSubChain(
		1,
		1,
		testaddress.Addrinfo["producer"].RawAddress,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is the chain ID reserved for main chain"))

	// chain ID is used
	start = action.NewStartSubChain(
		1,
		3,
		testaddress.Addrinfo["producer"].RawAddress,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by another sub-chain"))

	// security deposit is lower than the minimum
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(500000000), big.NewInt(blockchain.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "security deposit is smaller than the minimal requirement"))

	// security deposit is more than the owner balance
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(2100000000), big.NewInt(blockchain.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "sub-chain owner doesn't have enough balance for security deposit"))

	// operation deposit is more than the owner balance
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1100000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "sub-chain owner doesn't have enough balance for operation deposit"))

	// start height doesn't have enough delay
	start = action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		109,
		10,
		0,
		big.NewInt(0),
	)
	account, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "sub-chain could be started no early than"))
}

func TestCreateSubChainAddress(t *testing.T) {
	t.Parallel()

	addr1, err := createSubChainAddress(testaddress.Addrinfo["producer"].RawAddress, 1)
	assert.NoError(t, err)
	addr2, err := createSubChainAddress(testaddress.Addrinfo["producer"].RawAddress, 2)
	assert.NoError(t, err)
	// Same owner address but different nonce
	assert.NotEqual(t, addr1, addr2)
	addr3, err := createSubChainAddress(testaddress.Addrinfo["alfa"].RawAddress, 1)
	assert.NoError(t, err)
	// Same nonce but different owner address
	assert.NotEqual(t, addr1, addr3)
}

func TestHandleStartSubChain(t *testing.T) {
	t.Parallel()

	cfg := config.Default
	ctx := context.Background()
	sf, err := state.NewFactory(&cfg, state.InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	defer require.NoError(t, sf.Stop(ctx))

	// Create an account with 2000000000 iotx
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = ws.LoadOrCreateAccountState(
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(2000000000), big.NewInt(blockchain.Iotx)),
	)
	require.NoError(t, err)
	_, err = ws.RunActions(0, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().TipHeight().Return(uint64(0)).AnyTimes()

	// Prepare start sub-chain action
	start := action.NewStartSubChain(
		1,
		2,
		testaddress.Addrinfo["producer"].RawAddress,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	assert.NoError(t, action.Sign(start, testaddress.Addrinfo["producer"].PrivateKey))

	// Handle the action
	protocol := NewProtocol(chain, sf)
	require.NoError(t, protocol.handleStartSubChain(start, ws))
	require.NoError(t, sf.Commit(ws))

	// Check the owner state
	account, err := sf.AccountState(testaddress.Addrinfo["producer"].RawAddress)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), account.Nonce)
	assert.Equal(t, big.NewInt(0), account.Balance)

	// Check the sub-chain state
	addr, err := createSubChainAddress(testaddress.Addrinfo["producer"].RawAddress, 1)
	require.NoError(t, err)
	state, err := sf.State(addr, &subChain{})
	require.NoError(t, err)
	sc, ok := state.(*subChain)
	require.True(t, ok)
	assert.Equal(t, uint32(2), sc.ChainID)
	assert.Equal(t, MinSecurityDeposit, sc.SecurityDeposit)
	assert.Equal(t, big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)), sc.OperationDeposit)
	assert.Equal(t, testaddress.Addrinfo["producer"].PublicKey, sc.OwnerPublicKey)
	assert.Equal(t, uint64(110), sc.StartHeight)
	assert.Equal(t, uint64(10), sc.ParentHeightOffset)
	assert.Equal(t, uint64(0), sc.CurrentHeight)

}
