// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package evm

import (
	"math"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestGasPriceOpcode deploys a contract whose init code returns what the
// GASPRICE opcode reports as the deployed code, and reads it back. Before the
// next hardfork a dynamic-fee transaction sees its fee cap; from the next
// hardfork it sees the effective gas price, min(feeCap, baseFee+tipCap). A
// legacy transaction sees its gas price either way.
func TestGasPriceOpcode(t *testing.T) {
	var (
		caller  = identityset.Address(28)
		baseFee = big.NewInt(10)
		feeCap  = big.NewInt(100)
		tipCap  = big.NewInt(5)
		// GASPRICE PUSH0 MSTORE PUSH1 0x20 PUSH0 RETURN
		initCode = []byte{0x3a, 0x5f, 0x52, 0x60, 0x20, 0x5f, 0xf3}
		gasLimit = uint64(200_000)
	)
	dynamicFeeTx := func(nonce uint64) action.Envelope {
		return action.NewEnvelope(action.NewDynamicFeeTx(1, nonce, gasLimit, feeCap, tipCap, nil),
			action.NewExecution("", big.NewInt(0), initCode))
	}
	legacyTx := func(nonce uint64) action.Envelope {
		return action.NewEnvelope(action.NewLegacyTx(1, nonce, gasLimit, feeCap),
			action.NewExecution("", big.NewInt(0), initCode))
	}
	for _, tt := range []struct {
		name       string
		gateHeight uint64
		elp        func(uint64) action.Envelope
		want       *big.Int
	}{
		{"dynamic fee before the next hardfork", math.MaxUint64, dynamicFeeTx, feeCap},
		{"dynamic fee from the next hardfork", 1, dynamicFeeTx, new(big.Int).Add(baseFee, tipCap)},
		{"legacy before the next hardfork", math.MaxUint64, legacyTx, feeCap},
		{"legacy from the next hardfork", 1, legacyTx, feeCap},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			sm, err := initMockStateManager(gomock.NewController(t))
			r.NoError(err)
			acc, err := accountutil.LoadOrCreateAccount(sm, caller)
			r.NoError(err)
			balance := new(big.Int).Mul(feeCap, new(big.Int).SetUint64(gasLimit))
			r.NoError(acc.AddBalance(balance))
			r.NoError(accountutil.StoreAccount(sm, caller, acc))

			var deposits []*big.Int
			ctx := pragueExecutionCtx(t, tt.gateHeight, caller, &deposits, func(blkCtx *protocol.BlockCtx) {
				blkCtx.BaseFee = new(big.Int).Set(baseFee)
			})
			_, receipt, err := ExecuteContract(ctx, sm, tt.elp(acc.PendingNonce()))
			r.NoError(err)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)
			// what the sender pays is unchanged either way
			r.Equal(tt.elp(0).EffectiveGasPrice(baseFee), receipt.EffectiveGasPrice)

			contract, err := address.FromString(receipt.ContractAddress)
			r.NoError(err)
			stateDB, err := NewStateDBAdapter(sm, protocol.MustGetBlockCtx(ctx).BlockHeight, hash.ZeroHash256)
			r.NoError(err)
			code := stateDB.GetCode(common.BytesToAddress(contract.Bytes()))
			r.Len(code, 32)
			r.Equal(tt.want, new(big.Int).SetBytes(code))
		})
	}
}
