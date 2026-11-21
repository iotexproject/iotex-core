// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// pragueExecutionCtx returns a context for running an execution at a height
// past Yap (so Prague rules, and the EIP-7623 data floor, apply), with the
// next hardfork scheduled at gateHeight. Every gas fee deposit made through the
// helper context is recorded in deposits.
func pragueExecutionCtx(t *testing.T, gateHeight uint64, caller address.Address, deposits *[]*big.Int, mod func(*protocol.BlockCtx)) context.Context {
	t.Helper()
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = gateHeight
	height := g.YapBlockHeight + 1
	blkCtx := protocol.BlockCtx{
		BlockHeight:    height,
		BlockTimeStamp: time.Unix(1700000000, 0),
		Producer:       identityset.Address(27),
		GasLimit:       g.BlockGasLimitByHeight(height),
	}
	if mod != nil {
		mod(&blkCtx)
	}
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, blkCtx)
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:     caller,
		ActionHash: hash.Hash256b([]byte("next-fork")),
	})
	ctx = protocol.WithBlockchainCtx(protocol.WithFeatureCtx(ctx), protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: 4689,
	})
	return WithHelperCtx(ctx, HelperContext{
		GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		GetBlockTime: func(uint64) (time.Time, error) { return time.Time{}, nil },
		DepositGasFunc: func(_ context.Context, _ protocol.StateManager, amount *big.Int, opts ...protocol.DepositOption) ([]*action.TransactionLog, error) {
			cfg := protocol.DepositOptionCfg{}
			for _, opt := range opts {
				opt(&cfg)
			}
			total := new(big.Int).Set(amount)
			if cfg.PriorityFee != nil {
				total.Add(total, cfg.PriorityFee)
			}
			*deposits = append(*deposits, total)
			return nil, nil
		},
	})
}

// TestExecuteContractFloorDataGasShortfall covers an execution whose gas limit
// covers the intrinsic gas but not the EIP-7623 data floor. Before the next
// hardfork the EVM returns ErrFloorDataGas, which abandons the block it is in;
// from the next hardfork it settles a failure receipt that consumes the whole
// gas limit and bumps the sender's nonce.
func TestExecuteContractFloorDataGasShortfall(t *testing.T) {
	var (
		caller   = identityset.Address(28)
		to       = identityset.Address(29)
		gasPrice = big.NewInt(10)
		balance  = big.NewInt(1_000_000_000)
		data     = make([]byte, 100)
	)
	exec := action.NewExecution(to.String(), big.NewInt(0), data)
	intrinsicGas, err := exec.IntrinsicGas()
	require.NoError(t, err)
	floorDataGas, err := action.FloorDataGas(data)
	require.NoError(t, err)
	// above the intrinsic gas, so admission's intrinsic check passes, and
	// below the data floor
	gasLimit := (intrinsicGas + floorDataGas) / 2
	require.True(t, intrinsicGas <= gasLimit && gasLimit < floorDataGas)

	for _, tt := range []struct {
		name       string
		gateHeight uint64
	}{
		{"before the next hardfork", math.MaxUint64},
		{"from the next hardfork", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			sm, err := initMockStateManager(gomock.NewController(t))
			r.NoError(err)
			acc, err := accountutil.LoadOrCreateAccount(sm, caller)
			r.NoError(err)
			r.NoError(acc.AddBalance(balance))
			r.NoError(accountutil.StoreAccount(sm, caller, acc))
			nonce := acc.PendingNonce()

			var deposits []*big.Int
			ctx := pragueExecutionCtx(t, tt.gateHeight, caller, &deposits, nil)
			elp := (&action.EnvelopeBuilder{}).SetNonce(nonce).SetGasPrice(gasPrice).
				SetGasLimit(gasLimit).SetAction(exec).Build()
			_, receipt, err := ExecuteContract(ctx, sm, elp)
			after, loadErr := accountutil.LoadAccount(sm, caller)
			r.NoError(loadErr)

			if tt.gateHeight == math.MaxUint64 {
				r.ErrorIs(err, action.ErrFloorDataGas)
				r.Nil(receipt)
				r.Empty(deposits)
				r.Equal(nonce, after.PendingNonce())
				return
			}
			r.NoError(err)
			r.NotNil(receipt)
			r.Equal(uint64(iotextypes.ReceiptStatus_ErrOutOfGas), receipt.Status)
			r.Equal(gasLimit, receipt.GasConsumed)
			r.Len(deposits, 1)
			r.Equal(new(big.Int).Mul(gasPrice, new(big.Int).SetUint64(gasLimit)), deposits[0])
			r.Equal(nonce+1, after.PendingNonce())
			// the deposit taken before the check is handed back in full; the
			// fee itself goes through the deposit function
			r.Equal(balance, after.Balance)
		})
	}
}

// TestExecuteContractAtFloorDataGas checks that an execution whose gas limit
// covers the data floor is unaffected by the next hardfork.
func TestExecuteContractAtFloorDataGas(t *testing.T) {
	var (
		caller = identityset.Address(28)
		to     = identityset.Address(29)
		data   = make([]byte, 100)
	)
	floorDataGas, err := action.FloorDataGas(data)
	require.NoError(t, err)
	var receipts []*action.Receipt
	for _, gateHeight := range []uint64{math.MaxUint64, 1} {
		r := require.New(t)
		sm, err := initMockStateManager(gomock.NewController(t))
		r.NoError(err)
		acc, err := accountutil.LoadOrCreateAccount(sm, caller)
		r.NoError(err)
		r.NoError(acc.AddBalance(big.NewInt(1_000_000_000)))
		r.NoError(accountutil.StoreAccount(sm, caller, acc))

		var deposits []*big.Int
		ctx := pragueExecutionCtx(t, gateHeight, caller, &deposits, nil)
		elp := (&action.EnvelopeBuilder{}).SetNonce(acc.PendingNonce()).SetGasPrice(big.NewInt(10)).
			SetGasLimit(floorDataGas).SetAction(action.NewExecution(to.String(), big.NewInt(0), data)).Build()
		_, receipt, err := ExecuteContract(ctx, sm, elp)
		r.NoError(err)
		r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
		receipts = append(receipts, receipt)
	}
	require.Equal(t, receipts[0], receipts[1])
}
