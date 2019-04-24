// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/config/configpb"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestUpdateActiveProtocols(t *testing.T) {
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
	// Get the init list
	pids, err := p.ActiveProtocols(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, pids)
	// Update the list
	l, err := p.UpdateActiveProtocols(ctx, ws, []string{"a", "c"}, []string{"b", "d"})
	require.NoError(t, err)
	uapLog := configpb.RegistryLog{}
	require.NoError(t, proto.Unmarshal(l.Data, &uapLog))
	assert.Equal(t, []string{"c"}, uapLog.Additions)
	assert.Equal(t, []string{"b"}, uapLog.Removals)
	require.NoError(t, stateDB.Commit(ws))

	ws, err = stateDB.NewWorkingSet()
	require.NoError(t, err)
	// Get the updated list
	pids, err = p.ActiveProtocols(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "c"}, pids)
}

func TestNoAuthorization(t *testing.T) {
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
		Caller: identityset.Address(1),
	})

	ws, err := stateDB.NewWorkingSet()
	require.NoError(t, err)
	_, err = p.UpdateActiveProtocols(ctx, ws, []string{"a", "c"}, []string{"b", "d"})
	require.Error(t, err)
}
