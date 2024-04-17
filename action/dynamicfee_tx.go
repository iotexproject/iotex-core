// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	_ hasDestination      = (*Execution)(nil)
	_ EthCompatibleAction = (*Execution)(nil)
	_ TxData              = (*DynamicFeeTx)(nil)
)

// DynamicFeeTx represents an EIP-1559 transaction
type DynamicFeeTx struct {
	ex        Execution
	gasTipCap *big.Int // a.k.a. maxPriorityFeePerGas
	gasFeeCap *big.Int // a.k.a. maxFeePerGas
}

func (tx *DynamicFeeTx) Nonce() uint64 {
	return tx.ex.nonce
}

func (tx *DynamicFeeTx) GasLimit() uint64 {
	return tx.ex.gasLimit
}

func (tx *DynamicFeeTx) GasPrice() *big.Int {
	return tx.gasFeeCap
}

func (tx *DynamicFeeTx) Amount() *big.Int {
	return tx.ex.amount
}

func (tx *DynamicFeeTx) To() *common.Address {
	return tx.ex.To()
}

func (tx *DynamicFeeTx) Data() []byte {
	return tx.ex.data
}

func (tx *DynamicFeeTx) AccessList() types.AccessList {
	return tx.ex.accessList
}

// Serialize returns a raw byte stream of this tx
func (tx *DynamicFeeTx) Serialize() []byte {
	return byteutil.Must(proto.Marshal(tx.Proto()))
}

func (tx *DynamicFeeTx) Proto() *iotextypes.DynamicFeeTx {
	act := iotextypes.DynamicFeeTx{
		Execution: tx.ex.Proto(),
		GasTipCap: tx.gasTipCap.String(),
		GasFeeCap: tx.gasFeeCap.String(),
	}
	return &act
}

func (tx *DynamicFeeTx) LoadProto(pbAct *iotextypes.DynamicFeeTx) error {
	if pbAct == nil {
		return ErrNilProto
	}
	if tx == nil {
		return ErrNilAction
	}
	if gasTip := pbAct.GasTipCap; gasTip == "" {
		tx.gasTipCap = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(gasTip, 10)
		if !ok {
			return errors.Errorf("invalid amount %s", gasTip)
		}
		tx.gasTipCap = amount
	}
	if gasFee := pbAct.GasFeeCap; gasFee == "" {
		tx.gasFeeCap = big.NewInt(0)
	} else {
		amount, ok := new(big.Int).SetString(gasFee, 10)
		if !ok {
			return errors.Errorf("invalid amount %s", gasFee)
		}
		tx.gasFeeCap = amount
	}
	return tx.ex.LoadProto(pbAct.Execution)
}

func (tx *DynamicFeeTx) SanityCheck() error {
	// Reject tx of negative tip and fee
	if tx.gasTipCap.Sign() < 0 {
		return errors.Wrap(ErrInvalidAmount, "negative gasTipCap")
	}
	if tx.gasFeeCap.Sign() < 0 {
		return errors.Wrap(ErrInvalidAmount, "negative gasFeeCap")
	}
	// check the execution
	return tx.ex.SanityCheck()
}

func (tx *DynamicFeeTx) Cost() (*big.Int, error) {
	maxExecFee := new(big.Int).Mul(tx.gasFeeCap, new(big.Int).SetUint64(tx.ex.gasLimit))
	return maxExecFee.Add(tx.ex.amount, maxExecFee), nil
}

func (tx *DynamicFeeTx) IntrinsicGas() (uint64, error) {
	return tx.ex.IntrinsicGas()
}

func (tx *DynamicFeeTx) SetEnvelopeContext(elp Envelope) {
	tx.ex.SetEnvelopeContext(elp)
}
