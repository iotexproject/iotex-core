// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/vote"

	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestProtocolValidateSubChainStart(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	factory := mock_factory.NewMockFactory(ctrl)
	factory.EXPECT().AccountState(gomock.Any()).Return(
		&state.Account{Balance: big.NewInt(0).Mul(big.NewInt(2000000), big.NewInt(unit.Iotx))},
		nil,
	).AnyTimes()
	factory.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := &SubChainsInOperation{InOperation{ID: uint32(3)}}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).
		AnyTimes()
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().GetFactory().Return(factory).AnyTimes()
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()

	defer ctrl.Finish()

	p := NewProtocol(chain)

	start := action.NewStartSubChain(
		1,
		2,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err := p.validateStartSubChain(testaddress.Addrinfo["producer"], start, nil)
	assert.NotNil(t, account)
	assert.NoError(t, err)
	require.NotNil(t, subChainsInOp)

	// chain ID is the main chain ID
	start = action.NewStartSubChain(
		1,
		1,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, nil)
	assert.Nil(t, subChainsInOp)
	assert.Nil(t, account)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by main chain"))

	// chain ID is used
	start = action.NewStartSubChain(
		1,
		3,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by another sub-chain"))

	// security deposit is lower than the minimum
	start = action.NewStartSubChain(
		1,
		2,
		big.NewInt(0).Mul(big.NewInt(500000), big.NewInt(unit.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "security deposit is smaller than the minimal requirement"))

	// security deposit is more than the owner balance
	start = action.NewStartSubChain(
		1,
		2,
		big.NewInt(0).Mul(big.NewInt(2100000), big.NewInt(unit.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	// operation deposit is more than the owner balance
	start = action.NewStartSubChain(
		1,
		2,
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		big.NewInt(0).Mul(big.NewInt(1100000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, nil)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	// operation deposit is more than the owner balance in working set
	ws := mock_factory.NewMockWorkingSet(ctrl)
	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := SubChainsInOperation{InOperation{ID: uint32(3)}}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(1)
	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := &state.Account{Balance: big.NewInt(0).Mul(big.NewInt(1500000), big.NewInt(unit.Iotx))}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(1)
	start = action.NewStartSubChain(
		1,
		2,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, ws)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	// chain ID is used in the working set
	ws.EXPECT().
		State(gomock.Any(), gomock.Any()).
		Do(func(_ hash.Hash160, s interface{}) error {
			out := SubChainsInOperation{InOperation{ID: uint32(2)}, InOperation{ID: uint32(3)}}
			data, err := state.Serialize(out)
			if err != nil {
				return err
			}
			return state.Deserialize(s, data)
		}).Times(1)
	account, subChainsInOp, err = p.validateStartSubChain(testaddress.Addrinfo["producer"], start, ws)
	assert.Nil(t, account)
	assert.Nil(t, subChainsInOp)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is used by another sub-chain"))
}

func TestCreateSubChainAddress(t *testing.T) {
	t.Parallel()

	addr1, err := createSubChainAddress(testaddress.Addrinfo["producer"].String(), 1)
	assert.NoError(t, err)
	addr2, err := createSubChainAddress(testaddress.Addrinfo["producer"].String(), 2)
	assert.NoError(t, err)
	// Same owner address but different nonce
	assert.NotEqual(t, addr1, addr2)
	addr3, err := createSubChainAddress(testaddress.Addrinfo["alfa"].String(), 1)
	assert.NoError(t, err)
	// Same nonce but different owner address
	assert.NotEqual(t, addr1, addr3)
}

func TestHandleStartSubChain(t *testing.T) {
	t.Parallel()

	cfg := config.Default
	ctx := context.Background()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(t, err)
	require.NoError(t, sf.Start(ctx))
	ctrl := gomock.NewController(t)
	chain := mock_blockchain.NewMockBlockchain(ctrl)
	chain.EXPECT().ChainID().Return(uint32(1)).AnyTimes()
	chain.EXPECT().GetFactory().Return(sf).AnyTimes()
	chain.EXPECT().AddSubscriber(gomock.Any()).Return(nil).AnyTimes()

	defer func() {
		require.NoError(t, sf.Stop(ctx))
		ctrl.Finish()
	}()

	// Create an account with 2000000000 iotx
	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = accountutil.LoadOrCreateAccount(
		ws,
		testaddress.Addrinfo["producer"].String(),
		big.NewInt(0).Mul(big.NewInt(2000000), big.NewInt(unit.Iotx)),
	)
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			Producer: testaddress.Addrinfo["producer"],
			Caller:   testaddress.Addrinfo["producer"],
			GasLimit: gasLimit,
		})
	_, err = ws.RunActions(ctx, 0, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	ws, err = sf.NewWorkingSet()
	require.NoError(t, err)

	// Prepare start sub-chain action
	start := action.NewStartSubChain(
		1,
		2,
		MinSecurityDeposit,
		big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)),
		110,
		10,
		0,
		big.NewInt(0),
	)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(1).SetGasLimit(10).SetAction(start).Build()
	_, err = action.Sign(elp, testaddress.Keyinfo["producer"].PriKey)
	require.NoError(t, err)

	// Handle the action
	protocol := NewProtocol(chain)
	require.NoError(t, protocol.handleStartSubChain(ctx, start, ws))
	require.NoError(t, sf.Commit(ws))

	// Check the owner state
	account, err := sf.AccountState(testaddress.Addrinfo["producer"].String())
	require.NoError(t, err)
	assert.Equal(t, uint64(1), account.Nonce)
	assert.Equal(t, big.NewInt(0), account.Balance)

	// Check the sub-chain state
	addr, err := createSubChainAddress(testaddress.Addrinfo["producer"].String(), 1)
	require.NoError(t, err)
	var sc SubChain
	err = sf.State(addr, &sc)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), sc.ChainID)
	assert.Equal(t, MinSecurityDeposit, sc.SecurityDeposit)
	assert.Equal(t, big.NewInt(0).Mul(big.NewInt(1000000), big.NewInt(unit.Iotx)), sc.OperationDeposit)
	assert.Equal(t, testaddress.Keyinfo["producer"].PubKey, sc.OwnerPublicKey)
	assert.Equal(t, uint64(110), sc.StartHeight)
	assert.Equal(t, uint64(10), sc.ParentHeightOffset)
	assert.Equal(t, uint64(0), sc.CurrentHeight)
}

func TestNoStartSubChainInGenesis(t *testing.T) {
	cfg := config.Default
	ctx := context.Background()
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	p := NewProtocol(bc)
	bc.GetFactory().AddActionHandlers(p)
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))

	subChainsInOp, err := p.SubChainsInOperation()
	require.NoError(t, err)
	assert.Equal(t, 0, len(subChainsInOp))
}

func TestStartSubChainInGenesis(t *testing.T) {
	t.Skip()
	cfg := config.Default

	ctx := context.Background()
	registry := protocol.Registry{}
	acc := account.NewProtocol()
	registry.Register(account.ProtocolID, acc)
	bc := blockchain.NewBlockchain(
		cfg,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	vp := vote.NewProtocol(bc)
	registry.Register(vote.ProtocolID, vp)
	p := NewProtocol(bc)
	bc.GetFactory().AddActionHandlers(p)
	require.NoError(t, bc.Start(ctx))
	defer require.NoError(t, bc.Stop(ctx))
	scAddr, err := createSubChainAddress(identityset.Address(0).String(), 0)
	require.NoError(t, err)
	addr, err := address.FromBytes(scAddr[:])
	require.NoError(t, err)
	sc, err := p.SubChain(addr)
	require.NoError(t, err)
	assert.Equal(t, uint32(2), sc.ChainID)
	assert.Equal(t, unit.ConvertIotxToRau(1000000), sc.SecurityDeposit)
	assert.Equal(t, unit.ConvertIotxToRau(1000000), sc.OperationDeposit)
	assert.Equal(t, uint64(10), sc.StartHeight)
	assert.Equal(t, uint64(10), sc.ParentHeightOffset)
	subChainsInOp, err := p.SubChainsInOperation()
	require.NoError(t, err)
	fmt.Println(len(subChainsInOp))
	_, ok := subChainsInOp.Get(uint32(2))
	assert.True(t, ok)
}
