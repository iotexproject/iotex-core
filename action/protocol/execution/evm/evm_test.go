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

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestExecuteContractFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
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

	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller: identityset.Address(27),
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		Producer: identityset.Address(27),
		GasLimit: testutil.TestGasLimit,
	})
	ctx = genesis.WithGenesisContext(ctx, genesis.Default)

	retval, receipt, err := ExecuteContract(ctx, sm, e,
		func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
		func(context.Context, protocol.StateManager, *big.Int) (*action.TransactionLog, error) {
			return nil, nil
		})
	require.Nil(t, retval)
	require.Nil(t, receipt)
	require.Error(t, err)
}

func TestConstantinople(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller: identityset.Address(27),
	})

	g := genesis.Default
	ctx = genesis.WithGenesisContext(ctx, g)

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
		// Aleutian -- Bering
		{
			action.EmptyAddress,
			872000,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			872000,
		},
		// Bering -- Greenland
		{
			action.EmptyAddress,
			5550000,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			5550000,
		},
		// Greenland -- Hawaii
		{
			action.EmptyAddress,
			6544441,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			6544441,
		},
		// Hawaii -- Iceland
		{
			action.EmptyAddress,
			11267641,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			11267641,
		},
		// after Iceland
		{
			action.EmptyAddress,
			21267641,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			21267641,
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

		stateDB := NewStateDBAdapter(sm, e.height, !g.IsAleutian(e.height), g.IsGreenland(e.height), hash.ZeroHash256)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
			BlockHeight: e.height,
		})
		ps, err := newParams(ctx, ex, stateDB, func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		})
		require.NoError(err)

		var evmConfig vm.Config
		chainConfig := getChainConfig(g, e.height)
		evm := vm.NewEVM(ps.context, stateDB, chainConfig, evmConfig)

		evmChainConfig := evm.ChainConfig()
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsHomestead(evm.BlockNumber))
		require.False(evmChainConfig.IsDAOFork(evm.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsEIP150(evm.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsEIP158(evm.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsEIP155(evm.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsByzantium(evm.BlockNumber))
		require.True(evmChainConfig.IsConstantinople(evm.BlockNumber))
		require.True(evmChainConfig.IsPetersburg(evm.BlockNumber))

		// verify chainRules
		chainRules := evmChainConfig.Rules(ps.context.BlockNumber)
		require.Equal(g.IsGreenland(e.height), chainRules.IsHomestead)
		require.Equal(g.IsGreenland(e.height), chainRules.IsEIP150)
		require.Equal(g.IsGreenland(e.height), chainRules.IsEIP158)
		require.Equal(g.IsGreenland(e.height), chainRules.IsEIP155)
		require.Equal(g.IsGreenland(e.height), chainRules.IsByzantium)
		require.True(chainRules.IsConstantinople)
		require.True(chainRules.IsPetersburg)

		// verify iotex configs in chain config block
		require.Equal(big.NewInt(int64(genesis.Default.BeringBlockHeight)), evmChainConfig.BeringBlock)
		require.Equal(big.NewInt(int64(genesis.Default.GreenlandBlockHeight)), evmChainConfig.GreenlandBlock)
		require.Equal(!g.IsBering(e.height), evm.IsPreBering())

		// iceland = support chainID + enable Istanbul and Muir Glacier
		if g.IsIceland(e.height) {
			require.EqualValues(config.EVMNetworkID(), evmChainConfig.ChainID.Uint64())
			require.True(evmChainConfig.IsIstanbul(evm.BlockNumber))
			require.True(evmChainConfig.IsMuirGlacier(evm.BlockNumber))
			require.True(chainRules.IsIstanbul)
		} else {
			require.Nil(evmChainConfig.ChainID)
			require.False(evmChainConfig.IsIstanbul(evm.BlockNumber))
			require.False(evmChainConfig.IsMuirGlacier(evm.BlockNumber))
			require.False(chainRules.IsIstanbul)
		}
	}
}

func TestGasEstimate(t *testing.T) {
	require := require.New(t)

	for _, v := range []struct {
		gas, consume, refund uint64
		data                 []byte
	}{
		{config.Default.Genesis.BlockGasLimit, 8200300, 1000000, make([]byte, 20000)},
		{1000000, 245600, 100000, make([]byte, 5600)},
		{500000, 21000, 10000, make([]byte, 36)},
	} {
		evmLeftOver, remainingGas, err := gasExecuteInEVM(v.gas, v.consume, v.refund, v.data)
		require.NoError(err)
		gasConsumed := v.gas - remainingGas
		require.Equal(v.gas-evmLeftOver, gasConsumed+v.refund)

		// if we use gasConsumed + refund, EVM will consume the exact amount of gas
		evmLeftOver, _, err = gasExecuteInEVM(gasConsumed+v.refund, v.consume, v.refund, v.data)
		require.NoError(err)
		require.Zero(evmLeftOver)
	}
}

// gasExecuteInEVM performs gas calculation during EVM execution
func gasExecuteInEVM(gas, consume, refund uint64, data []byte) (uint64, uint64, error) {
	remainingGas := gas

	intriGas, err := intrinsicGas(data)
	if err != nil {
		return 0, 0, err
	}
	if remainingGas < intriGas {
		return 0, 0, ErrInconsistentNonce
	}
	remainingGas -= intriGas

	// simulate EVM consumes 'consume' amount of gas
	if remainingGas < consume {
		return 0, 0, ErrInconsistentNonce
	}
	remainingGas -= consume
	return remainingGas, remainingGas + refund, nil
}
