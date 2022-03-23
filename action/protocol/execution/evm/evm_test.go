// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

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

	ctx = protocol.WithFeatureCtx(ctx)
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
		// Iceland -- Jutland
		{
			action.EmptyAddress,
			12289321,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			12289321,
		},
		// Jutland - Kamchatka
		{
			action.EmptyAddress,
			13685401,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			13816440,
		},
		// Kamchatka - LordHowe
		{
			action.EmptyAddress,
			13816441,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			13979160,
		},
		// LordHowe - Midway
		{
			action.EmptyAddress,
			13979161,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			16509240,
		},
		// after Midway
		{
			action.EmptyAddress,
			16509241,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			math.MaxUint64,
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
		opt := []StateDBAdapterOption{}
		if !g.IsAleutian(e.height) {
			opt = append(opt, NotFixTopicCopyBugOption())
		}
		if g.IsGreenland(e.height) {
			opt = append(opt, AsyncContractTrieOption())
		}
		if g.IsKamchatka(e.height) {
			opt = append(opt, FixSnapshotOrderOption())
		}
		stateDB := NewStateDBAdapter(sm, e.height, hash.ZeroHash256, opt...)

		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
			BlockHeight: e.height,
		})
		ctx = protocol.WithFeatureCtx(ctx)
		ps, err := newParams(ctx, ex, stateDB, func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		})
		require.NoError(err)

		var evmConfig vm.Config
		chainConfig := getChainConfig(g.Blockchain, e.height)
		evm := vm.NewEVM(ps.context, ps.txCtx, stateDB, chainConfig, evmConfig)

		evmChainConfig := evm.ChainConfig()
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsHomestead(evm.Context.BlockNumber))
		require.False(evmChainConfig.IsDAOFork(evm.Context.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsEIP150(evm.Context.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsEIP158(evm.Context.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsEIP155(evm.Context.BlockNumber))
		require.Equal(g.IsGreenland(e.height), evmChainConfig.IsByzantium(evm.Context.BlockNumber))
		require.True(evmChainConfig.IsConstantinople(evm.Context.BlockNumber))
		require.True(evmChainConfig.IsPetersburg(evm.Context.BlockNumber))

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
		require.Equal(big.NewInt(int64(g.BeringBlockHeight)), evmChainConfig.BeringBlock)
		require.Equal(big.NewInt(int64(g.GreenlandBlockHeight)), evmChainConfig.GreenlandBlock)
		require.Equal(!g.IsBering(e.height), evm.IsPreBering())

		// iceland = support chainID + enable Istanbul and Muir Glacier
		isIceland := g.IsIceland(e.height)
		if isIceland {
			require.EqualValues(config.EVMNetworkID(), evmChainConfig.ChainID.Uint64())
		} else {
			require.Nil(evmChainConfig.ChainID)
		}
		require.Equal(big.NewInt(int64(g.IcelandBlockHeight)), evmChainConfig.MuirGlacierBlock)
		require.Equal(big.NewInt(int64(g.IcelandBlockHeight)), evmChainConfig.IstanbulBlock)
		require.Equal(isIceland, evmChainConfig.IsIstanbul(evm.Context.BlockNumber))
		require.Equal(isIceland, evmChainConfig.IsMuirGlacier(evm.Context.BlockNumber))
		require.Equal(isIceland, chainRules.IsIstanbul)

		// enable Berlin and London
		isBerlin := g.IsToBeEnabled(e.height)
		require.Equal(isBerlin, evmChainConfig.IsBerlin(evm.Context.BlockNumber))
		require.Equal(isBerlin, chainRules.IsBerlin)
		isLondon := g.IsToBeEnabled(e.height)
		require.Equal(isLondon, evmChainConfig.IsLondon(evm.Context.BlockNumber))
		require.Equal(isLondon, chainRules.IsLondon)
		require.False(evmChainConfig.IsCatalyst(evm.Context.BlockNumber))
		require.False(chainRules.IsCatalyst)
	}
}

func TestEvmError(t *testing.T) {
	r := require.New(t)
	g := genesis.Default.Blockchain

	beringTests := []struct {
		evmError error
		status   iotextypes.ReceiptStatus
	}{
		{vm.ErrOutOfGas, iotextypes.ReceiptStatus_ErrOutOfGas},
		{vm.ErrCodeStoreOutOfGas, iotextypes.ReceiptStatus_ErrCodeStoreOutOfGas},
		{vm.ErrDepth, iotextypes.ReceiptStatus_ErrDepth},
		{vm.ErrContractAddressCollision, iotextypes.ReceiptStatus_ErrContractAddressCollision},
		{vm.ErrExecutionReverted, iotextypes.ReceiptStatus_ErrExecutionReverted},
		{vm.ErrMaxCodeSizeExceeded, iotextypes.ReceiptStatus_ErrMaxCodeSizeExceeded},
		{vm.ErrWriteProtection, iotextypes.ReceiptStatus_ErrWriteProtection},
		{errors.New("unknown"), iotextypes.ReceiptStatus_ErrUnknown},
	}
	for _, v := range beringTests {
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight), uint64(v.status))
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight-1), uint64(v.status))
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight), uint64(v.status))
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight-1), uint64(iotextypes.ReceiptStatus_Failure))
	}

	jutlandTests := []struct {
		evmError error
		status   iotextypes.ReceiptStatus
	}{
		{vm.ErrInsufficientBalance, iotextypes.ReceiptStatus_ErrInsufficientBalance},
		{vm.ErrInvalidJump, iotextypes.ReceiptStatus_ErrInvalidJump},
		{vm.ErrReturnDataOutOfBounds, iotextypes.ReceiptStatus_ErrReturnDataOutOfBounds},
		{vm.ErrGasUintOverflow, iotextypes.ReceiptStatus_ErrGasUintOverflow},
		{errors.New("unknown"), iotextypes.ReceiptStatus_ErrUnknown},
	}
	for _, v := range jutlandTests {
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight), uint64(v.status))
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight-1), uint64(iotextypes.ReceiptStatus_ErrUnknown))
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight), uint64(iotextypes.ReceiptStatus_ErrUnknown))
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight-1), uint64(iotextypes.ReceiptStatus_Failure))
	}
}

func TestGasEstimate(t *testing.T) {
	require := require.New(t)

	for _, v := range []struct {
		gas, consume, refund, size uint64
	}{
		{config.Default.Genesis.BlockGasLimit, 8200300, 1000000, 20000},
		{1000000, 245600, 100000, 5600},
		{500000, 21000, 10000, 36},
	} {
		evmLeftOver, remainingGas, err := gasExecuteInEVM(v.gas, v.consume, v.refund, v.size)
		require.NoError(err)
		gasConsumed := v.gas - remainingGas
		require.Equal(v.gas-evmLeftOver, gasConsumed+v.refund)

		// if we use gasConsumed + refund, EVM will consume the exact amount of gas
		evmLeftOver, _, err = gasExecuteInEVM(gasConsumed+v.refund, v.consume, v.refund, v.size)
		require.NoError(err)
		require.Zero(evmLeftOver)
	}
}

// gasExecuteInEVM performs gas calculation during EVM execution
func gasExecuteInEVM(gas, consume, refund, size uint64) (uint64, uint64, error) {
	remainingGas := gas

	intriGas, err := intrinsicGas(size, nil)
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
