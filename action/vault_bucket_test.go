// Copyright (c) 2026 IoTeX Foundation
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"math/big"
	"testing"
)

func TestVaultBucketABIRoundTrip(t *testing.T) {
	for name, method := range bucketguard.Draft5Contract.Methods {
		t.Run(name, func(t *testing.T) {
			published, ok := NativeStakingContractABI().Methods[name]
			require.True(t, ok)
			require.Equal(t, method.Sig, published.Sig)
			require.Equal(t, method.ID, published.ID)
			var args []any
			for _, f := range method.Inputs {
				var v any
				switch f.Type.String() {
				case "uint64":
					v = uint64(1)
					if f.Name == "deadline" {
						v = uint64(10)
					}
					if f.Name == "unbindDelay" {
						v = bucketguard.UnbindDelayBlocks
					}
				case "uint32":
					v = uint32(pqcrypto.AlgSLHDSA192s)
				case "uint256":
					v = big.NewInt(100)
				case "bytes32":
					v = [32]byte{1}
				case "address":
					b := _stakingProtocolEthAddr
					b[0] = 1
					v = b
				case "bytes":
					v = bytes.Repeat([]byte{1}, 16224)
					if f.Name == "publicKey" {
						v = bytes.Repeat([]byte{1}, 48)
					}
				}
				args = append(args, v)
			}
			data, err := method.Inputs.Pack(args...)
			require.NoError(t, err)
			data = append(bytes.Clone(method.ID), data...)
			a, err := NewVaultActionFromABIBinary(data)
			require.NoError(t, err)
			require.NoError(t, a.SanityCheck())
			encoded, err := a.EthData()
			require.NoError(t, err)
			require.Equal(t, data, encoded)
			gas, err := a.IntrinsicGas()
			require.NoError(t, err)
			elp := (&EnvelopeBuilder{}).SetNonce(1).SetGasLimit(gas).SetGasPrice(big.NewInt(1)).SetAction(a).Build()
			restored := &envelope{}
			require.NoError(t, restored.LoadProto(elp.Proto()))
			require.IsType(t, a, restored.Action())
			require.NoError(t, restored.Action().SanityCheck())
			got, err := restored.IntrinsicGas()
			require.NoError(t, err)
			require.Equal(t, gas, got)
			to, _ := a.EthTo()
			eth := types.NewTx(&types.LegacyTx{To: to, Gas: gas, GasPrice: big.NewInt(1), Value: new(big.Int), Data: data})
			web, err := (&EnvelopeBuilder{}).BuildStakingAction(eth)
			require.NoError(t, err)
			got, err = web.IntrinsicGas()
			require.NoError(t, err)
			require.Equal(t, gas, got)
			_, err = NewVaultActionFromABIBinary(append(data, 0))
			require.Error(t, err, "reject trailing bytes")
		})
	}
	require.Equal(t, "100000000000000000000", bucketguard.ParamsV2().MinVaultBalance.String())
}

func TestVaultBucketMalformedAmount(t *testing.T) {
	for _, n := range []*big.Int{nil, big.NewInt(-1), new(big.Int).Lsh(big.NewInt(1), 256)} {
		_, err := NewVaultAction(&bucketguard.VaultAction{Method: "vaultCreate", Amount: n})
		require.Error(t, err)
	}
	_, err := NewVaultAction(nil)
	require.Error(t, err)
}

func TestVaultBucketPreservesExecution(t *testing.T) {
	a, err := NewVaultAction(&bucketguard.VaultAction{Method: "vaultInitiateExit"})
	require.NoError(t, err)
	data, err := a.EthData()
	require.NoError(t, err)
	// Native executions submitted before this feature retain their payload type
	// even if the data happens to match a newly allocated staking selector.
	exec := NewExecution(address.StakingProtocolAddr, new(big.Int), data)
	elp := (&EnvelopeBuilder{}).SetGasPrice(big.NewInt(1)).SetAction(exec).Build()
	restored := &envelope{}
	require.NoError(t, restored.LoadProto(elp.Proto()))
	require.IsType(t, &Execution{}, restored.Action())
	core := &iotextypes.ActionCore{}
	a.FillAction(core)
	old := &GuardedBucketOp{}
	require.NoError(t, old.LoadProto(core.GetGuardedBucketOp()))
	require.Error(t, old.SanityCheck(), "carrier cannot be a valid pre-fork action")
	core.GetGuardedBucketOp().Nonce = 1
	_, err = loadVaultProto(core.GetGuardedBucketOp())
	require.Error(t, err, "reject unused carrier fields")
}
