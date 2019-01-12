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
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestValidateDeposit(t *testing.T) {
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

	p := NewProtocol(chain)

	addr1 := testaddress.IotxAddrinfo["producer"].RawAddress
	addr, err := address.IotxAddressToAddress(addr1)
	require.NoError(t, err)
	addr2 := address.New(2, addr.Payload()).IotxAddress()

	deposit := action.NewCreateDeposit(1, big.NewInt(1000), addr1, addr2, testutil.TestGasLimit, big.NewInt(0))
	_, _, err = p.validateDeposit(deposit, nil)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	_, err = account.LoadOrCreateAccount(
		ws,
		testaddress.IotxAddrinfo["producer"].RawAddress,
		big.NewInt(1000),
	)
	require.NoError(t, err)
	gasLimit := testutil.TestGasLimit
	ctx = protocol.WithRunActionsCtx(ctx,
		protocol.RunActionsCtx{
			ProducerAddr:    testaddress.IotxAddrinfo["producer"].RawAddress,
			GasLimit:        &gasLimit,
			EnableGasCharge: testutil.EnableGasCharge,
		})
	_, _, err = ws.RunActions(ctx, 0, nil)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	deposit1 := action.NewCreateDeposit(1, big.NewInt(2000), addr1, addr2, testutil.TestGasLimit, big.NewInt(0))
	_, _, err = p.validateDeposit(deposit1, nil)
	assert.True(t, strings.Contains(err.Error(), "doesn't have at least required balance"))

	_, _, err = p.validateDeposit(deposit, nil)
	assert.True(t, strings.Contains(err.Error(), "is not on a sub-chain in operation"))

	subChainAddr, err := createSubChainAddress(addr1, 0)
	require.NoError(t, err)
	require.NoError(t, ws.PutState(
		SubChainsInOperationKey,
		SubChainsInOperation{
			InOperation{
				ID:   2,
				Addr: address.New(1, subChainAddr[:]).Bytes(),
			},
		},
	))
	require.NoError(t, sf.Commit(ws))

	_, _, err = p.validateDeposit(deposit, nil)
	assert.NoError(t, err)
}

func TestMutateDeposit(t *testing.T) {
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

	addr1 := testaddress.IotxAddrinfo["producer"].RawAddress
	addr, err := address.IotxAddressToAddress(addr1)
	require.NoError(t, err)
	addr2 := address.New(2, addr.Payload()).IotxAddress()
	subChainAddr, err := createSubChainAddress(addr1, 0)
	require.NoError(t, err)

	ws, err := sf.NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, ws.PutState(
		subChainAddr,
		&SubChain{
			ChainID:            2,
			SecurityDeposit:    big.NewInt(1),
			OperationDeposit:   big.NewInt(2),
			StartHeight:        100,
			ParentHeightOffset: 10,
			OwnerPublicKey:     testaddress.IotxAddrinfo["producer"].PublicKey,
			CurrentHeight:      200,
			DepositCount:       300,
		},
	))
	require.NoError(t, sf.Commit(ws))

	p := NewProtocol(chain)
	act := action.NewCreateDeposit(2, big.NewInt(1000), addr1, addr2, testutil.TestGasLimit, big.NewInt(0))
	receipt, err := p.mutateDeposit(
		act,
		&state.Account{
			Nonce:   1,
			Balance: big.NewInt(2000),
		},
		InOperation{
			ID:   2,
			Addr: address.New(1, subChainAddr[:]).Bytes(),
		},
		ws,
	)
	require.NoError(t, err)
	require.NoError(t, sf.Commit(ws))

	account, err := sf.AccountState(addr1)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), account.Nonce)
	assert.Equal(t, big.NewInt(1000), account.Balance)

	subChain, err := p.SubChain(address.New(1, subChainAddr[:]))
	require.NoError(t, err)
	assert.Equal(t, uint64(301), subChain.DepositCount)

	deposit, err := p.Deposit(address.New(1, subChainAddr[:]), 300)
	require.NoError(t, err)
	assert.Equal(t, address.New(2, addr.Payload()).Bytes(), deposit.Addr)
	assert.Equal(t, big.NewInt(1000), deposit.Amount)
	assert.False(t, deposit.Confirmed)

	require.NotNil(t, receipt)
	assert.Equal(t, uint64(300), enc.MachineEndian.Uint64(receipt.ReturnValue))
	assert.Equal(t, act.Hash(), receipt.Hash)
	assert.Equal(t, uint64(0), receipt.Status)
	gas, err := act.IntrinsicGas()
	assert.NoError(t, err)
	assert.Equal(t, gas, receipt.GasConsumed)
	assert.Equal(t, address.New(1, subChainAddr[:]).IotxAddress(), receipt.ContractAddress)
}
