// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

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

	ctx = protocol.WithBlockchainCtx(protocol.WithFeatureCtx(ctx), protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: 100,
	})
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

	evmNetworkID := uint32(100)
	g := genesis.Default
	ctx = protocol.WithBlockchainCtx(genesis.WithGenesisContext(ctx, g), protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: evmNetworkID,
	})

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
		// Midway - NewFoundland
		{
			action.EmptyAddress,
			16509241,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			17662680,
		},
		// NewFoundland - Okhotsk
		{
			action.EmptyAddress,
			17662681,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			21542760,
		},
		// after Okhotsk
		{
			action.EmptyAddress,
			21542761,
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

		fCtx := protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			Producer:    identityset.Address(27),
			GasLimit:    testutil.TestGasLimit,
			BlockHeight: e.height,
		}))
		stateDB, err := prepareStateDB(fCtx, sm)
		require.NoError(err)
		ps, err := newParams(fCtx, ex, stateDB, func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		})
		require.NoError(err)

		var evmConfig vm.Config
		chainConfig := getChainConfig(g.Blockchain, e.height, ps.evmNetworkID)
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
		chainRules := evmChainConfig.Rules(ps.context.BlockNumber, false)
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

		// iceland = support chainID + enable Istanbul and Muir Glacier
		isIceland := g.IsIceland(e.height)
		if isIceland {
			require.EqualValues(evmNetworkID, evmChainConfig.ChainID.Uint64())
		} else {
			require.Nil(evmChainConfig.ChainID)
		}
		require.Equal(big.NewInt(int64(g.IcelandBlockHeight)), evmChainConfig.MuirGlacierBlock)
		require.Equal(big.NewInt(int64(g.IcelandBlockHeight)), evmChainConfig.IstanbulBlock)
		require.Equal(isIceland, evmChainConfig.IsIstanbul(evm.Context.BlockNumber))
		require.Equal(isIceland, evmChainConfig.IsMuirGlacier(evm.Context.BlockNumber))
		require.Equal(isIceland, chainRules.IsIstanbul)

		// Okhotsk = enable Berlin and London
		isOkhotsk := g.IsOkhotsk(e.height)
		require.Equal(big.NewInt(int64(g.OkhotskBlockHeight)), evmChainConfig.BerlinBlock)
		require.Equal(big.NewInt(int64(g.OkhotskBlockHeight)), evmChainConfig.LondonBlock)
		require.Equal(isOkhotsk, evmChainConfig.IsBerlin(evm.Context.BlockNumber))
		require.Equal(isOkhotsk, chainRules.IsBerlin)
		require.Equal(isOkhotsk, evmChainConfig.IsLondon(evm.Context.BlockNumber))
		require.Equal(isOkhotsk, chainRules.IsLondon)
		require.False(chainRules.IsMerge)

		// test basefee
		require.Equal(new(big.Int), evm.Context.BaseFee)
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
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.OkhotskBlockHeight), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.OkhotskBlockHeight-1), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight-1), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight-1), iotextypes.ReceiptStatus_Failure)
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
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.OkhotskBlockHeight), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.OkhotskBlockHeight-1), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight-1), iotextypes.ReceiptStatus_ErrUnknown)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight), iotextypes.ReceiptStatus_ErrUnknown)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight-1), iotextypes.ReceiptStatus_Failure)
	}

	newTests := []struct {
		evmError error
		status   iotextypes.ReceiptStatus
	}{
		{vm.ErrInvalidCode, iotextypes.ReceiptStatus_ErrInvalidCode},
	}
	for _, v := range newTests {
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.OkhotskBlockHeight), v.status)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.OkhotskBlockHeight-1), iotextypes.ReceiptStatus_ErrUnknown)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight), iotextypes.ReceiptStatus_ErrUnknown)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.JutlandBlockHeight-1), iotextypes.ReceiptStatus_ErrUnknown)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight), iotextypes.ReceiptStatus_ErrUnknown)
		r.Equal(evmErrToErrStatusCode(v.evmError, g, g.BeringBlockHeight-1), iotextypes.ReceiptStatus_Failure)
	}
}

func TestGasEstimate(t *testing.T) {
	require := require.New(t)

	for _, v := range []struct {
		gas, consume, refund, size uint64
	}{
		{genesis.Default.BlockGasLimit, 8200300, 1000000, 20000},
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
