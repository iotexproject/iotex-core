// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestExecuteContractFailure(t *testing.T) {
	ctrl := gomock.NewController(t)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()

	e := action.NewExecution("", big.NewInt(0), nil)
	elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(10)).
		SetGasLimit(testutil.TestGasLimit).SetAction(e).Build()

	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller: identityset.Address(27),
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		Producer: identityset.Address(27),
		GasLimit: testutil.TestGasLimit,
	})
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())

	ctx = protocol.WithBlockchainCtx(protocol.WithFeatureCtx(ctx), protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: 100,
	})
	ctx = WithHelperCtx(ctx, HelperContext{
		GetBlockHash: func(uint64) (hash.Hash256, error) {
			return hash.ZeroHash256, nil
		},
		GetBlockTime: func(uint64) (time.Time, error) {
			return time.Time{}, nil
		},
		DepositGasFunc: func(context.Context, protocol.StateManager, *big.Int, ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			return nil, nil
		},
	})
	retval, receipt, err := ExecuteContract(ctx, sm, elp)
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
	g := genesis.TestDefault()
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
		// Okhotsk - Palau
		{
			action.EmptyAddress,
			21542761,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			22991400,
		},
		// Palau - Quebec
		{
			action.EmptyAddress,
			22991401,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			24838200,
		},
		// after Quebec - Redsea
		{
			action.EmptyAddress,
			24838201,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			26704440,
		},
		// after Redsea - Sumatra
		{
			action.EmptyAddress,
			26704441,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			28516680,
		},
		// after Sumatra - Tsunami
		{
			action.EmptyAddress,
			28516681,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			29275560,
		},
		// after Tsunami - Upernavik
		{
			action.EmptyAddress,
			29275561,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			31174200,
		},
		// after Upernavik - Vanuatu
		{
			action.EmptyAddress,
			31174201,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			33730920,
		},
		// after Vanuatu - Wake
		{
			action.EmptyAddress,
			33730921,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			43730920,
		},
		// after Wake
		{
			action.EmptyAddress,
			43730921,
		},
		{
			"io1pcg2ja9krrhujpazswgz77ss46xgt88afqlk6y",
			1261440000, // = 200*365*24*3600/5, around 200 years later
		},
	}
	now := time.Now()
	getBlockTime := func(height uint64) (time.Time, error) {
		return now.Add(time.Duration(height) * time.Second * 5), nil
	}
	for _, e := range execHeights {
		ex := action.NewExecution(e.contract, big.NewInt(0), nil)
		elp := (&action.EnvelopeBuilder{}).SetNonce(1).SetGasPrice(big.NewInt(10)).
			SetGasLimit(testutil.TestGasLimit).SetAction(ex).Build()

		timestamp, err := getBlockTime(e.height)
		require.NoError(err)
		fCtx := protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			Producer:       identityset.Address(27),
			GasLimit:       testutil.TestGasLimit,
			BlockHeight:    e.height,
			BlockTimeStamp: timestamp,
		}))
		fCtx = WithHelperCtx(fCtx, HelperContext{
			GetBlockHash: func(uint64) (hash.Hash256, error) {
				return hash.ZeroHash256, nil
			},
			GetBlockTime: getBlockTime,
		})
		stateDB, err := prepareStateDB(fCtx, sm)
		require.NoError(err)
		ps, err := newParams(fCtx, elp)
		require.NoError(err)

		evm := vm.NewEVM(ps.context, ps.txCtx, stateDB, ps.chainConfig, ps.evmConfig)
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
		chainRules := evmChainConfig.Rules(ps.context.BlockNumber, g.IsSumatra(e.height), ps.context.Time)
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

		// Redsea = enable ArrowGlacier, GrayGlacier
		isRedsea := g.IsRedsea(e.height)
		require.Equal(big.NewInt(int64(g.RedseaBlockHeight)), evmChainConfig.ArrowGlacierBlock)
		require.Equal(big.NewInt(int64(g.RedseaBlockHeight)), evmChainConfig.GrayGlacierBlock)
		require.Equal(isRedsea, evmChainConfig.IsArrowGlacier(evm.Context.BlockNumber))
		require.Equal(isRedsea, evmChainConfig.IsGrayGlacier(evm.Context.BlockNumber))

		// Sumatra = enable Merge, Shanghai
		isSumatra := g.IsSumatra(e.height)
		require.Equal(isSumatra, chainRules.IsMerge)
		require.Equal(isSumatra, chainRules.IsShanghai)

		// Vanuatu = enable Cancun
		isVanuatu := g.IsVanuatu(e.height)
		require.Equal(isVanuatu, chainRules.IsCancun)
		require.Equal(isVanuatu, evmChainConfig.IsCancun(big.NewInt(int64(e.height)), evm.Context.Time))

		// Prague and Verkle not yet enabled
		require.False(chainRules.IsPrague)
		require.False(evmChainConfig.IsPrague(big.NewInt(int64(e.height)), evm.Context.Time))
		require.False(chainRules.IsVerkle)
		require.False(evmChainConfig.IsVerkle(big.NewInt(int64(e.height)), evm.Context.Time))

		// test basefee
		require.Equal(new(big.Int), evm.Context.BaseFee)
	}
}

func TestEvmError(t *testing.T) {
	r := require.New(t)
	g := genesis.TestDefault().Blockchain

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
		{genesis.TestDefault().BlockGasLimit, 8200300, 1000000, 20000},
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
