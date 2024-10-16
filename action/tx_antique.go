// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

var _ TxCommonInternal = (*AntiqueTx)(nil)

// AntiqueTx is same as LegacyTx, with the only difference that version = 0
type AntiqueTx struct {
	LegacyTx
	version uint32
}

// NewAntiqueTx creates a new antique transaction
func NewAntiqueTx(chainID, version uint32, nonce uint64, gasLimit uint64, gasPrice *big.Int) *AntiqueTx {
	return &AntiqueTx{
		LegacyTx: LegacyTx{
			chainID:  chainID,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		version: version,
	}
}

func (tx *AntiqueTx) TxType() uint32 {
	return AntiqueTxType
}

func (tx *AntiqueTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		TxType:   AntiqueTxType,
		Version:  tx.version,
		Nonce:    tx.nonce,
		GasLimit: tx.gasLimit,
		ChainID:  tx.chainID,
	}
	if tx.gasPrice != nil {
		actCore.GasPrice = tx.gasPrice.String()
	}
	return &actCore
}

func (tx *AntiqueTx) fromProto(pb *iotextypes.ActionCore) error {
	if pb.TxType != AntiqueTxType {
		return errors.Wrapf(ErrInvalidProto, "wrong tx type = %d", pb.TxType)
	}
	var gasPrice big.Int
	if price := pb.GetGasPrice(); len(price) > 0 {
		if _, ok := gasPrice.SetString(price, 10); !ok {
			return errors.Errorf("invalid gasPrice %s", price)
		}
	}
	tx.version = pb.GetVersion()
	tx.chainID = pb.GetChainID()
	tx.nonce = pb.GetNonce()
	tx.gasLimit = pb.GetGasLimit()
	tx.gasPrice = &gasPrice
	return nil
}
