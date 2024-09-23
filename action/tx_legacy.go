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
)

// LegacyTx is the legacy transaction
type LegacyTx struct {
	chainID  uint32
	nonce    uint64
	gasLimit uint64
	gasPrice *big.Int
}

func (tx *LegacyTx) Version() uint32 {
	return LegacyTxType
}

func (tx *LegacyTx) ChainID() uint32 {
	return tx.chainID
}

func (tx *LegacyTx) Nonce() uint64 {
	return tx.nonce
}

func (tx *LegacyTx) Gas() uint64 {
	return tx.gasLimit
}

func (tx *LegacyTx) GasPrice() *big.Int {
	return tx.price()
}

func (tx *LegacyTx) price() *big.Int {
	p := &big.Int{}
	if tx.gasPrice == nil {
		return p
	}
	return p.Set(tx.gasPrice)
}

func (tx *LegacyTx) AccessList() types.AccessList {
	return nil
}

func (tx *LegacyTx) GasTipCap() *big.Int {
	return tx.price()
}

func (tx *LegacyTx) GasFeeCap() *big.Int {
	return tx.price()
}

func (tx *LegacyTx) BlobGas() uint64 { return 0 }

func (tx *LegacyTx) BlobGasFeeCap() *big.Int { return nil }

func (tx *LegacyTx) BlobHashes() []common.Hash { return nil }

func (tx *LegacyTx) BlobTxSidecar() *types.BlobTxSidecar { return nil }

func (tx *LegacyTx) SanityCheck() error {
	// Reject execution of negative gas price
	if tx.gasPrice != nil && tx.gasPrice.Sign() < 0 {
		return ErrNegativeValue
	}
	return nil
}

func (tx *LegacyTx) withoutSidecar() TxCommonWithProto {
	return tx
}

func (tx *LegacyTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		Version:  LegacyTxType,
		Nonce:    tx.nonce,
		GasLimit: tx.gasLimit,
		ChainID:  tx.chainID,
	}
	if tx.gasPrice != nil {
		actCore.GasPrice = tx.gasPrice.String()
	}
	return &actCore
}

func fromProtoLegacyTx(pb *iotextypes.ActionCore) (*LegacyTx, error) {
	var tx LegacyTx
	tx.nonce = pb.GetNonce()
	tx.gasLimit = pb.GetGasLimit()
	tx.chainID = pb.GetChainID()
	tx.gasPrice = &big.Int{}
	if price := pb.GetGasPrice(); len(price) > 0 {
		var ok bool
		if tx.gasPrice, ok = tx.gasPrice.SetString(price, 10); !ok {
			return nil, errors.Errorf("invalid gasPrice %s", price)
		}
	}
	return &tx, nil
}

func (tx *LegacyTx) setNonce(n uint64) {
	tx.nonce = n
}

func (tx *LegacyTx) setGas(gas uint64) {
	tx.gasLimit = gas
}

func (tx *LegacyTx) setChainID(n uint32) {
	tx.chainID = n
}
