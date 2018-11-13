// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestProtocolValidateSubChainStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	factory := mock_state.NewMockFactory(ctrl)
	factory.EXPECT().AccountState(gomock.Any()).Return(
		&state.Account{Balance: big.NewInt(0).Mul(big.NewInt(2000000000), big.NewInt(blockchain.Iotx))},
		nil,
	).AnyTimes()
	factory.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(&state.SortedSlice{InOperation{ID: uint32(3)}}, nil).
		AnyTimes()
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().GetFactory().Return(factory).AnyTimes()
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()

	defer ctrl.Finish()

	p := NewProtocol(chain)

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
	account, subChainsInOp, err := p.validateStartSubChain(start, nil)
	assert.NotNil(t, account)
	assert.NotNil(t, subChainsInOp)
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
	account, subChainsInOp, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, subChainsInOp)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by main chain"))

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
	account, subChainsInOp, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
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
	account, subChainsInOp, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
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
	account, subChainsInOp, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

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
	account, subChainsInOp, err = p.validateStartSubChain(start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	// operation deposit is more than the owner balance in working set
	ws := mock_state.NewMockWorkingSet(ctrl)
	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(&state.SortedSlice{InOperation{ID: uint32(3)}}, nil).
		Times(1)
	ws.EXPECT().CachedAccountState(gomock.Any()).Return(
		&state.Account{Balance: big.NewInt(0).Mul(big.NewInt(1500000000), big.NewInt(blockchain.Iotx))},
		nil,
	).AnyTimes()
	start = action.NewStartSubChain(
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
	account, subChainsInOp, err = p.validateStartSubChain(start, ws)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	// chain ID is used in the working set
	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Return(&state.SortedSlice{InOperation{ID: uint32(2)}, InOperation{ID: uint32(3)}}, nil).
		Times(1)
	account, subChainsInOp, err = p.validateStartSubChain(start, ws)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by another sub-chain"))
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
	sf, err := state.NewFactory(cfg, state.InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	ctrl := gomock.NewController(t)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
	chain.EXPECT().GetFactory().Return(sf).AnyTimes()
	chain.EXPECT().SubscribeBlockCreation(gomock.Any()).Return(nil).AnyTimes()

	defer func() {
		require.NoError(t, sf.Stop(ctx))
		ctrl.Finish()
	}()

	// Create an account with 2000000000 iotx
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = ws.LoadOrCreateAccountState(
		testaddress.Addrinfo["producer"].RawAddress,
		big.NewInt(0).Mul(big.NewInt(2000000000), big.NewInt(blockchain.Iotx)),
	)
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx = state.WithRunActionsCtx(ctx,
		state.RunActionsCtx{
			ProducerAddr:    testaddress.Addrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)

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
	protocol := NewProtocol(chain)
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
	state, err := sf.State(addr, &SubChain{})
	require.NoError(t, err)
	sc, ok := state.(*SubChain)
	require.True(t, ok)
	assert.Equal(t, uint32(2), sc.ChainID)
	assert.Equal(t, MinSecurityDeposit, sc.SecurityDeposit)
	assert.Equal(t, big.NewInt(0).Mul(big.NewInt(1000000000), big.NewInt(blockchain.Iotx)), sc.OperationDeposit)
	assert.Equal(t, testaddress.Addrinfo["producer"].PublicKey, sc.OwnerPublicKey)
	assert.Equal(t, uint64(110), sc.StartHeight)
	assert.Equal(t, uint64(10), sc.ParentHeightOffset)
	assert.Equal(t, uint64(0), sc.CurrentHeight)
}

func TestNoStartSubChainInGenesis(t *testing.T) {
	cfg := config.Default

	ctx := context.Background()
	bc := blockchain.NewBlockchain(cfg, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	p := NewProtocol(bc)
	bc.GetFactory().AddActionHandlers(p)
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))

	subChainsInOp, err := p.SubChainsInOperation()
	require.NoError(t, err)
	assert.Equal(t, 0, len(subChainsInOp))
}

func TestStartSubChainInGenesis(t *testing.T) {
	cfg := config.Default
	cfg.Chain.EnableSubChainStartInGenesis = true

	ctx := context.Background()
	bc := blockchain.NewBlockchain(cfg, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	p := NewProtocol(bc)
	bc.GetFactory().AddActionHandlers(p)
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))

	scAddr, err := createSubChainAddress(blockchain.Gen.CreatorAddr(1), 0)
	require.NoError(t, err)
	addr := address.New(1, scAddr[:])
	sc, err := p.SubChain(addr)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), sc.ChainID)
	assert.Equal(t, blockchain.ConvertIotxToRau(1000000000), sc.SecurityDeposit)
	assert.Equal(t, blockchain.ConvertIotxToRau(1000000000), sc.OperationDeposit)
	assert.Equal(t, uint64(10), sc.StartHeight)
	assert.Equal(t, uint64(10), sc.ParentHeightOffset)
	subChainsInOp, err := p.SubChainsInOperation()
	require.NoError(t, err)
	_, ok := subChainsInOp.Get(InOperation{ID: uint32(2)}, SortInOperation)
	assert.True(t, ok)
}
