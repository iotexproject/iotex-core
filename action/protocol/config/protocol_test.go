// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"context"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestProtocol_Handle(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()

	p := NewProtocol(identityset.Address(0), []string{"a", "b"})

	b := action.UpdateActiveProtocolsBuilder{}
	uap := b.SetAdditions([]string{"c"}).SetRemovals([]string{"b"}).Build()

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller:   identityset.Address(0),
		GasPrice: big.NewInt(unit.Qev),
		GasLimit: 100000,
	})
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	receipt, err := p.Handle(ctx, &uap, ws)
	require.NoError(t, err)
	require.NotNil(t, receipt)
	assert.Equal(t, action.SuccessReceiptStatus, receipt.Status)
	assert.Equal(t, 1, len(receipt.Logs))
}

func TestProtocol_ReadActiveProtocols(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	stateDB, err := factory.NewStateDB(cfg, factory.InMemStateDBOption())
	require.NoError(t, err)
	require.NoError(t, stateDB.Start(context.Background()))
	defer func() {
		require.NoError(t, stateDB.Stop(context.Background()))
	}()

	p := NewProtocol(identityset.Address(0), []string{"a", "b"})
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller: identityset.Address(0),
	})
	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	data, err := p.ReadState(ctx, ws, []byte("ActiveProtocols"))
	require.NoError(t, err)
	assert.Equal(t, "a,b", string(data))
}
