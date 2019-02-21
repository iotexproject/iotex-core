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
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/test/mock/mock_explorer"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestValidateDeposit(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(t, bc.Start(ctx))
	exp := mock_explorer.NewMockExplorer(ctrl)

	p := NewProtocol(bc, exp)
	deposit := action.NewSettleDeposit(
		1,
		big.NewInt(1000),
		10000,
		testaddress.Addrinfo["alfa"].String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)

	defer func() {
		require.NoError(t, bc.Stop(ctx))
		ctrl.Finish()
	}()

	exp.EXPECT().GetDeposits(gomock.Any(), gomock.Any(), gomock.Any()).Return([]explorer.Deposit{}, nil).Times(1)
	err := p.validateDeposit(deposit, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "deposits found instead of"))

	exp.EXPECT().GetDeposits(gomock.Any(), gomock.Any(), gomock.Any()).Return([]explorer.Deposit{
		{
			Amount:    "100",
			Address:   testaddress.Addrinfo["alfa"].String(),
			Confirmed: false,
		},
	}, nil).Times(2)

	err = p.validateDeposit(deposit, nil)
	assert.NoError(t, err)

	ws, err := bc.GetFactory().NewWorkingSet()
	require.NoError(t, err)
	var depositIndex DepositIndex
	require.NoError(t, ws.PutState(depositAddress(10000), &depositIndex))
	bc.GetFactory().Commit(ws)
	err = p.validateDeposit(deposit, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is already settled"))

	exp.EXPECT().GetDeposits(gomock.Any(), gomock.Any(), gomock.Any()).Return([]explorer.Deposit{
		{
			Amount:    "100",
			Address:   testaddress.Addrinfo["alfa"].String(),
			Confirmed: true,
		},
	}, nil).Times(1)
	err = p.validateDeposit(deposit, nil)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "is already confirmed"))

}

func TestMutateDeposit(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
	)
	require.NoError(t, bc.Start(ctx))
	exp := mock_explorer.NewMockExplorer(ctrl)

	p := NewProtocol(bc, exp)
	deposit := action.NewSettleDeposit(
		1,
		big.NewInt(1000),
		10000,
		testaddress.Addrinfo["alfa"].String(),
		testutil.TestGasLimit,
		big.NewInt(0),
	)

	defer func() {
		require.NoError(t, bc.Stop(ctx))
		ctrl.Finish()
	}()

	ctx = protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller: testaddress.Addrinfo["producer"],
	})
	ws, err := bc.GetFactory().NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, p.mutateDeposit(ctx, deposit, ws))
	require.NoError(t, bc.GetFactory().Commit(ws))

	account1, err := bc.GetFactory().AccountState(testaddress.Addrinfo["producer"].String())
	require.NoError(t, err)
	assert.Equal(t, uint64(1), account1.Nonce)

	account2, err := bc.GetFactory().AccountState(testaddress.Addrinfo["alfa"].String())
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1000), account2.Balance)

	var di DepositIndex
	err = bc.GetFactory().State(depositAddress(10000), &di)
	require.NoError(t, err)
	var zero DepositIndex
	assert.Equal(t, zero, di)
}
