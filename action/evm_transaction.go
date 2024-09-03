// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

type (
	// EvmTransaction represents an action to be executed by EVM protocol
	// as of now 3 types of transactions are supported:
	// 1. Legacy transaction
	// 2. EIP-2930 access list transaction
	// 3. EIP-4844 shard blob transaction
	EvmTransaction struct {
		inner TxData
	}

	// TxData is the interface required to execute a transaction by EVM
	// It follows the same-name interface in go-ethereum
	TxData interface {
		TxCommon
		Value() *big.Int
		To() *common.Address
		Data() []byte
	}

	TxCommon interface {
		Nonce() uint64
		Gas() uint64
		GasPrice() *big.Int
		TxDynamicGas
		AccessList() types.AccessList
	}

	TxDynamicGas interface {
		GasTipCap() *big.Int
		GasFeeCap() *big.Int
	}
)

func NewEvmTx(a Action) *EvmTransaction {
	tx := new(EvmTransaction)
	switch act := a.(type) {
	case *Execution:
		tx.inner = act
	default:
		panic("unsupported action type")
	}
	return tx
}

func (tx *EvmTransaction) Nonce() uint64 {
	return tx.inner.Nonce()
}

func (tx *EvmTransaction) Gas() uint64 {
	return tx.inner.Gas()
}

func (tx *EvmTransaction) GasPrice() *big.Int {
	return tx.inner.GasPrice()
}

func (tx *EvmTransaction) GasTipCap() *big.Int {
	return tx.inner.GasTipCap()
}

func (tx *EvmTransaction) GasFeeCap() *big.Int {
	return tx.inner.GasFeeCap()
}

func (tx *EvmTransaction) Value() *big.Int {
	return tx.inner.Value()
}

func (tx *EvmTransaction) To() *common.Address {
	return tx.inner.To()
}

func (tx *EvmTransaction) Data() []byte {
	return tx.inner.Data()
}

func (tx *EvmTransaction) AccessList() types.AccessList {
	return tx.inner.AccessList()
}

func toAccessListProto(list types.AccessList) []*iotextypes.AccessTuple {
	if len(list) == 0 {
		return nil
	}
	proto := make([]*iotextypes.AccessTuple, len(list))
	for i, v := range list {
		proto[i] = &iotextypes.AccessTuple{}
		proto[i].Address = hex.EncodeToString(v.Address.Bytes())
		if numKey := len(v.StorageKeys); numKey > 0 {
			proto[i].StorageKeys = make([]string, numKey)
			for j, key := range v.StorageKeys {
				proto[i].StorageKeys[j] = hex.EncodeToString(key.Bytes())
			}
		}
	}
	return proto
}

func fromAccessListProto(list []*iotextypes.AccessTuple) types.AccessList {
	if len(list) == 0 {
		return nil
	}
	accessList := make(types.AccessList, len(list))
	for i, v := range list {
		accessList[i].Address = common.HexToAddress(v.Address)
		if numKey := len(v.StorageKeys); numKey > 0 {
			accessList[i].StorageKeys = make([]common.Hash, numKey)
			for j, key := range v.StorageKeys {
				accessList[i].StorageKeys[j] = common.HexToHash(key)
			}
		}
	}
	return accessList
}

// EffectiveGas returns the effective gas
func EffectiveGasTip(tx TxDynamicGas, baseFee *big.Int) (*big.Int, error) {
	tip := tx.GasTipCap()
	if baseFee == nil {
		return tip, nil
	}
	effectiveGas := tx.GasFeeCap()
	effectiveGas.Sub(effectiveGas, baseFee)
	if effectiveGas.Sign() < 0 {
		return effectiveGas, ErrGasFeeCapTooLow
	}
	// effective gas = min(tip, feeCap - baseFee)
	if effectiveGas.Cmp(tip) <= 0 {
		return effectiveGas, nil
	}
	return tip, nil
}
