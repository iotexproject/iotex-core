// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

func initLifeLongDelegateProtocol(ctrl *gomock.Controller) (Protocol, context.Context, protocol.StateManager, error) {
	genesisConfig := genesis.TestDefault()
	delegates := genesisConfig.Delegates
	p := NewLifeLongDelegatesProtocol(delegates)
	registry := protocol.NewRegistry()
	err := registry.Register("rolldpos", rolldpos.NewProtocol(36, 36, 20))
	if err != nil {
		return nil, nil, nil, err
	}
	ctx := genesis.WithGenesisContext(
		protocol.WithRegistry(context.Background(), registry),
		genesisConfig,
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

func TestCreateGenesisStates_WithLifeLong(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	p, ctx, sm, err := initLifeLongDelegateProtocol(ctrl)
	require.NoError(err)

	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(ctx)
	require.NoError(p.CreateGenesisStates(ctx, sm))
}

func TestProtocol_Handle_WithLifeLong(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	p, ctx, sm, err := initLifeLongDelegateProtocol(ctrl)
	require.NoError(err)

	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	elp := (&action.EnvelopeBuilder{}).SetAction(&action.GrantReward{}).Build()
	receipt, error := p.Handle(ctx, elp, sm)
	require.Nil(receipt)
	require.NoError(error)
}
