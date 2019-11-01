// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestExecuteContractFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cm := mock_chainmanager.NewMockChainManager(ctrl)
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	store := db.NewMemKVStore()
	sm.EXPECT().GetDB().Return(store).AnyTimes()
	cb := db.NewCachedBatch()
	sm.EXPECT().GetCachedBatch().Return(cb).AnyTimes()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()

	e, err := action.NewExecution(
		"",
		1,
		big.NewInt(0),
		testutil.TestGasLimit,
		big.NewInt(10),
		nil,
	)
	require.NoError(t, err)

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller:   identityset.Address(27),
		Producer: identityset.Address(27),
		GasLimit: testutil.TestGasLimit,
	})

	retval, receipt, err := ExecuteContract(ctx, sm, e, cm, config.NewHeightUpgrade(config.Default))
	require.Nil(t, retval)
	require.Nil(t, receipt)
	require.Error(t, err)
}

func TestConstantinople(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cm := mock_chainmanager.NewMockChainManager(ctrl)
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	store := db.NewMemKVStore()
	sm.EXPECT().GetDB().Return(store).AnyTimes()
	cb := db.NewCachedBatch()
	sm.EXPECT().GetCachedBatch().Return(cb).AnyTimes()

	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		Caller:   identityset.Address(27),
		Producer: identityset.Address(27),
		GasLimit: testutil.TestGasLimit,
	})
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	hu := config.NewHeightUpgrade(config.Default)

	execHeights := []struct {
		contract string
		height   uint64
	}{
		// before Pacific
		{
			action.EmptyAddress,
			20000,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			20000,
		},
		// Pacific -- Aleutian
		{
			action.EmptyAddress,
			442000,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			442000,
		},
		// after Aleutian
		{
			action.EmptyAddress,
			872000,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			872000,
		},
	}

	for _, e := range execHeights {
		ex, err := action.NewExecution(
			e.contract,
			1,
			big.NewInt(0),
			testutil.TestGasLimit,
			big.NewInt(10),
			nil,
		)
		require.NoError(err)
		raCtx.BlockHeight = e.height
		stateDB := NewStateDBAdapter(cm, sm, hu, e.height, ex.Hash())
		ps, err := NewParams(raCtx, ex, stateDB, hu)
		require.NoError(err)

		var config vm.Config
		chainConfig := getChainConfig(genesis.Default.BeringBlockHeight)
		evm := vm.NewEVM(ps.context, stateDB, chainConfig, config)

		require.Equal(false, evm.ChainConfig().IsHomestead(evm.BlockNumber))
		require.Equal(false, evm.ChainConfig().IsByzantium(evm.BlockNumber))
		require.Equal(true, evm.ChainConfig().IsConstantinople(evm.BlockNumber))
		require.Equal(true, evm.ChainConfig().IsPetersburg(evm.BlockNumber))

		// verify chainRules
		chainRules := chainConfig.Rules(ps.context.BlockNumber)
		require.Equal(false, chainRules.IsHomestead)
		require.Equal(false, chainRules.IsByzantium)
		require.Equal(true, chainRules.IsConstantinople)
		require.Equal(true, chainRules.IsPetersburg)

		// verify iotex configs in chain config block
		require.Equal(big.NewInt(int64(genesis.Default.BeringBlockHeight)), evm.ChainConfig().BeringBlock)
		require.True(evm.IsPreBering())
	}
}
