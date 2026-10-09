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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestValidateAuthorizationChainID checks how an EIP-7702 authorization's chain
// ID is compared with the chain's. Before the next hardfork only the low 64
// bits of the authorization's chain ID are compared, so a chain ID that
// differs from the chain's only above bit 63 is accepted; from the next
// hardfork the full 256-bit value is compared and it is refused. A zero chain
// ID, the chain's own ID and an unrelated one are treated the same either way.
func TestValidateAuthorizationChainID(t *testing.T) {
	const evmNetworkID = 4689
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	evm := vm.NewEVM(vm.BlockContext{}, newTransferStateDB(t), &params.ChainConfig{ChainID: big.NewInt(evmNetworkID)}, vm.Config{})

	aliased := new(uint256.Int).Lsh(uint256.NewInt(1), 64)
	aliased.Add(aliased, uint256.NewInt(evmNetworkID))
	require.Equal(t, uint64(evmNetworkID), aliased.Uint64())

	signed := func(chainID *uint256.Int) *types.SetCodeAuthorization {
		auth, err := types.SignSetCode(key, types.SetCodeAuthorization{
			ChainID: *chainID,
			Address: common.HexToAddress("0x1234"),
			Nonce:   0,
		})
		require.NoError(t, err)
		return &auth
	}
	validate := func(auth *types.SetCodeAuthorization, compareFullChainID bool) error {
		_, err := validateAuthorization(evm, newTransferStateDB(t), auth, nil, 1, compareFullChainID)
		return err
	}
	// the outcome for the chain's own ID, with nothing else wrong
	matching := validate(signed(uint256.NewInt(evmNetworkID)), false)

	for _, tt := range []struct {
		name           string
		chainID        *uint256.Int
		mismatchBefore bool
		mismatchFrom   bool
	}{
		{"zero", uint256.NewInt(0), false, false},
		{"own chain ID", uint256.NewInt(evmNetworkID), false, false},
		{"other chain ID", uint256.NewInt(evmNetworkID + 1), true, true},
		{"own chain ID above bit 63", aliased, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			auth := signed(tt.chainID)
			for _, c := range []struct {
				compareFullChainID bool
				wantMismatch       bool
			}{
				{false, tt.mismatchBefore},
				{true, tt.mismatchFrom},
			} {
				err := validate(auth, c.compareFullChainID)
				if c.wantMismatch {
					r.ErrorContains(err, "does not match current chain ID")
					continue
				}
				// gets past the chain ID check exactly as the chain's own ID does
				if matching == nil {
					r.NoError(err)
				} else {
					r.EqualError(err, matching.Error())
				}
			}
		})
	}
}

// TestExecuteContractAuthorizationChainIDAboveBit63 runs a SetCode transaction
// carrying an authorization whose chain ID matches the chain's only in its low
// 64 bits. Before the next hardfork the authorization applies and installs the
// delegation; from the next hardfork it is skipped.
func TestExecuteContractAuthorizationChainIDAboveBit63(t *testing.T) {
	const evmNetworkID = 4689
	var (
		caller = identityset.Address(28)
		target = common.HexToAddress("0x1234")
	)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	authority := crypto.PubkeyToAddress(key.PublicKey)
	aliased := new(uint256.Int).Lsh(uint256.NewInt(1), 64)
	aliased.Add(aliased, uint256.NewInt(evmNetworkID))

	for _, tt := range []struct {
		name           string
		gateHeight     uint64
		wantDelegation bool
	}{
		{"before the next hardfork", math.MaxUint64, true},
		{"from the next hardfork", 1, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			sm, err := initMockStateManager(gomock.NewController(t))
			r.NoError(err)
			acc, err := accountutil.LoadOrCreateAccount(sm, caller)
			r.NoError(err)
			r.NoError(acc.AddBalance(big.NewInt(1_000_000_000)))
			r.NoError(accountutil.StoreAccount(sm, caller, acc))

			var deposits []*big.Int
			ctx := pragueExecutionCtx(t, tt.gateHeight, caller, &deposits, nil)
			r.Equal(uint32(evmNetworkID), protocol.MustGetBlockchainCtx(ctx).EvmNetworkID)
			auth, err := types.SignSetCode(key, types.SetCodeAuthorization{
				ChainID: *aliased,
				Address: target,
				Nonce:   0,
			})
			r.NoError(err)
			elp := (&action.EnvelopeBuilder{}).SetTxType(action.SetCodeTxType).
				SetNonce(acc.PendingNonce()).SetGasLimit(100_000).
				SetDynamicGas(big.NewInt(1), big.NewInt(1)).
				SetAuthList([]types.SetCodeAuthorization{auth}).
				SetAction(action.NewExecution(caller.String(), big.NewInt(0), nil)).Build()
			_, receipt, err := ExecuteContract(ctx, sm, elp)
			r.NoError(err)
			r.Equal(uint64(iotextypes.ReceiptStatus_Success), receipt.Status)

			stateDB, err := NewStateDBAdapter(sm, protocol.MustGetBlockCtx(ctx).BlockHeight, hash.ZeroHash256)
			r.NoError(err)
			code := stateDB.GetCode(authority)
			if tt.wantDelegation {
				r.Equal(types.AddressToDelegation(target), code)
			} else {
				r.Empty(code)
			}
		})
	}
}
