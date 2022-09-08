// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func initStandaloneProtocol(ctrl *gomock.Controller) (Protocol, context.Context, protocol.StateManager, error) {
	p := NewStandaloneProtocol(identityset.Address(0))
	registry := protocol.NewRegistry()
	err := registry.Register("rolldpos", rolldpos.NewProtocol(36, 36, 20))
	if err != nil {
		return nil, nil, nil, err
	}
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), registry),
		genesis.Default,
	)
	ctx = protocol.WithActionCtx(
		ctx,
		protocol.ActionCtx{},
	)
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{},
	)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := batch.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			val, err := cb.Get("state", cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			return 0, state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(account interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, err := state.Serialize(account)
			if err != nil {
				return 0, err
			}
			cb.Put("state", cfg.Key, ss, "failed to put state")
			return 0, nil
		}).AnyTimes()
	return p, ctx, sm, nil
}

func TestProtocol_Handle_WithStandalone(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	p, ctx, sm, err := initStandaloneProtocol(ctrl)
	require.NoError(err)

	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	receipt, err := p.Handle(ctx, nil, sm)
	require.Nil(receipt)
	require.NoError(err)
}
