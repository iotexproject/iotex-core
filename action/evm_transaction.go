// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
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

	TxData interface {
		Nonce() uint64
		GasLimit() uint64
		GasPrice() *big.Int
		Amount() *big.Int
		To() *common.Address
		Data() []byte
		AccessList() types.AccessList
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
	return tx.inner.GasLimit()
}

func (tx *EvmTransaction) GasPrice() *big.Int {
	return tx.inner.GasPrice()
}

func (tx *EvmTransaction) Value() *big.Int {
	return tx.inner.Amount()
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
