// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/test/identityset"
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

	defer func() {
		require.NoError(t, bc.Stop(ctx))
		ctrl.Finish()
	}()

	ws, err := bc.GetFactory().NewWorkingSet()
	require.NoError(t, err)
	var depositIndex DepositIndex
	require.NoError(t, ws.PutState(depositAddress(10000), &depositIndex))
	bc.GetFactory().Commit(ws)
}

func TestMutateDeposit(t *testing.T) {
	t.Skip()
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	bc := blockchain.NewBlockchain(
		config.Default,
		blockchain.InMemStateFactoryOption(),
		blockchain.InMemDaoOption(),
		blockchain.EnableExperimentalActions(),
	)
	require.NoError(t, bc.Start(ctx))

	defer func() {
		require.NoError(t, bc.Stop(ctx))
		ctrl.Finish()
	}()

	ctx = protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller: identityset.Address(27),
	})
	ws, err := bc.GetFactory().NewWorkingSet()
	require.NoError(t, err)
	require.NoError(t, bc.GetFactory().Commit(ws))

	account1, err := bc.GetFactory().AccountState(identityset.Address(27).String())
	require.NoError(t, err)
	assert.Equal(t, uint64(1), account1.Nonce)

	account2, err := bc.GetFactory().AccountState(identityset.Address(28).String())
	require.NoError(t, err)
	assert.Equal(t, big.NewInt(1000), account2.Balance)

	var di DepositIndex
	err = bc.GetFactory().State(depositAddress(10000), &di)
	require.NoError(t, err)
	var zero DepositIndex
	assert.Equal(t, zero, di)
}
