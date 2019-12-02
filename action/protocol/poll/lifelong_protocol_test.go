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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

func initLifeLongDelegateProtocol(ctrl *gomock.Controller) (Protocol, context.Context, protocol.StateManager, error) {
	genesisConfig := config.Default.Genesis
	delegates := genesisConfig.Delegates
	p := NewLifeLongDelegatesProtocol(delegates)
	registry := protocol.NewRegistry()
	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{
			Genesis:  config.Default.Genesis,
			Registry: registry,
		},
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
		func(addrHash hash.Hash160, account interface{}) error {
			val, err := cb.Get("state", addrHash[:])
			if err != nil {
				return state.ErrStateNotExist
			}
			return state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			ss, err := state.Serialize(account)
			if err != nil {
				return err
			}
			cb.Put("state", addrHash[:], ss, "failed to put state")
			return nil
		}).AnyTimes()
	return p, ctx, sm, nil
}

func TestCreateGenesisStates_WithLifeLong(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, ctx, sm, err := initLifeLongDelegateProtocol(ctrl)
	require.NoError(err)

	require.NoError(p.CreateGenesisStates(ctx, sm))
}

func TestProtocol_Handle_WithLifeLong(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, ctx, sm, err := initLifeLongDelegateProtocol(ctrl)
	require.NoError(err)

	receipt, error := p.Handle(ctx, nil, sm)
	require.Nil(receipt)
	require.NoError(error)
}

func TestProtocol_Validate_WithLifeLong(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, ctx, _, err := initLifeLongDelegateProtocol(ctrl)
	require.NoError(err)
	require.NoError(p.Validate(ctx, nil))
}
