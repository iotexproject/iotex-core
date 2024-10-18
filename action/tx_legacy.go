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

var _ TxCommonInternal = (*LegacyTx)(nil)

// LegacyTx is the legacy transaction
type LegacyTx struct {
	chainID  uint32
	version  uint32
	nonce    uint64
	gasLimit uint64
	gasPrice *big.Int
}

// NewLegacyTx creates a new legacy transaction
func NewLegacyTx(chainID uint32, nonce uint64, gasLimit uint64, gasPrice *big.Int) *LegacyTx {
	return &LegacyTx{
		chainID:  chainID,
		nonce:    nonce,
		gasLimit: gasLimit,
		gasPrice: gasPrice,
	}
}

func (tx *LegacyTx) TxType() uint32 {
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

func (tx *LegacyTx) EffectiveGasPrice(_ *big.Int) *big.Int {
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

func (tx *LegacyTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		TxType:   LegacyTxType,
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

func (tx *LegacyTx) fromProto(pb *iotextypes.ActionCore) error {
	if pb.TxType != LegacyTxType {
		return errors.Wrapf(ErrInvalidProto, "wrong tx type = %d", pb.TxType)
	}
	var gasPrice big.Int
	if price := pb.GetGasPrice(); len(price) > 0 {
		if _, ok := gasPrice.SetString(price, 10); !ok {
			return errors.Errorf("invalid gasPrice %s", price)
		}
	}
	tx.chainID = pb.GetChainID()
	tx.version = pb.GetVersion()
	tx.nonce = pb.GetNonce()
	tx.gasLimit = pb.GetGasLimit()
	tx.gasPrice = &gasPrice
	return nil
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

func (tx *LegacyTx) toEthTx(to *common.Address, value *big.Int, data []byte) *types.Transaction {
	return types.NewTx(&types.LegacyTx{
		Nonce:    tx.nonce,
		GasPrice: tx.price(),
		Gas:      tx.gasLimit,
		To:       to,
		Value:    value,
		Data:     data,
	})
}
