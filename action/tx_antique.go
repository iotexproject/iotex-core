// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var _ TxCommonInternal = (*AntiqueTx)(nil)

// AntiqueTx is same as LegacyTx, with the only difference that version = 0
type AntiqueTx struct {
	LegacyTx
}

// NewAntiqueTx creates a new antique transaction
func NewAntiqueTx(chainID uint32, nonce uint64, gasLimit uint64, gasPrice *big.Int) *AntiqueTx {
	return &AntiqueTx{
		LegacyTx{
			chainID:  chainID,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
	}
}

func (tx *AntiqueTx) Version() uint32 {
	return AntiqueTxType
}

func (tx *AntiqueTx) toProto() *iotextypes.ActionCore {
	actCore := tx.LegacyTx.toProto()
	actCore.Version = AntiqueTxType
	return actCore
}
