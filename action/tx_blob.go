// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

var (
	_ TxCommonInternal = (*BlobTx)(nil)
	_ validateSidecar  = (*BlobTx)(nil)
)

// BlobTx represents EIP-4844 blob transaction
type BlobTx struct {
	chainID    uint32
	nonce      uint64
	gasLimit   uint64
	gasTipCap  *uint256.Int // a.k.a. maxPriorityFeePerGas
	gasFeeCap  *uint256.Int // a.k.a. maxFeePerGas
	accessList types.AccessList
	blob       *BlobTxData
}

// NewBlobTx creates a new blob transaction
func NewBlobTx(
	chainID uint32,
	nonce uint64,
	gasLimit uint64,
	gasTipCap *big.Int,
	gasFeeCap *big.Int,
	accessList types.AccessList,
	blobData *BlobTxData,
) *BlobTx {
	return &BlobTx{
		chainID:    chainID,
		nonce:      nonce,
		gasLimit:   gasLimit,
		gasTipCap:  uint256.MustFromBig(gasTipCap),
		gasFeeCap:  uint256.MustFromBig(gasFeeCap),
		accessList: accessList,
		blob:       blobData,
	}
}

func (tx *BlobTx) TxType() uint32 {
	return BlobTxType
}

func (tx *BlobTx) ChainID() uint32 {
	return tx.chainID
}

func (tx *BlobTx) Nonce() uint64 {
	return tx.nonce
}

func (tx *BlobTx) Gas() uint64 {
	return tx.gasLimit
}

func (tx *BlobTx) GasPrice() *big.Int {
	return tx.feeCap().ToBig()
}

func (tx *BlobTx) GasTipCap() *big.Int {
	return tx.tipCap().ToBig()
}

func (tx *BlobTx) tipCap() *uint256.Int {
	p := uint256.Int{}
	if tx.gasTipCap != nil {
		p.Set(tx.gasTipCap)
	}
	return &p
}

func (tx *BlobTx) GasFeeCap() *big.Int {
	return tx.feeCap().ToBig()
}

func (tx *BlobTx) feeCap() *uint256.Int {
	p := uint256.Int{}
	if tx.gasFeeCap != nil {
		p.Set(tx.gasFeeCap)
	}
	return &p
}

func (tx *BlobTx) EffectiveGasPrice(baseFee *big.Int) *big.Int {
	tip := tx.GasFeeCap()
	if baseFee == nil {
		return tip
	}
	tip.Sub(tip, baseFee)
	if tipCap := tx.GasTipCap(); tip.Cmp(tipCap) > 0 {
		tip.Set(tipCap)
	}
	return tip.Add(tip, baseFee)
}

func (tx *BlobTx) AccessList() types.AccessList {
	return tx.accessList
}

func (tx *BlobTx) BlobGas() uint64 { return tx.blob.gas() }

func (tx *BlobTx) BlobGasFeeCap() *big.Int { return tx.blob.gasFeeCap().ToBig() }

func (tx *BlobTx) BlobHashes() []common.Hash { return tx.blob.hashes() }

func (tx *BlobTx) BlobTxSidecar() *types.BlobTxSidecar { return tx.blob.sidecar }

func (tx *BlobTx) SanityCheck() error {
	if tx.gasTipCap == nil || tx.gasFeeCap == nil {
		return ErrMissRequiredField
	}
	if tx.gasTipCap.Sign() < 0 {
		return ErrNegativeValue
	}
	if tx.gasFeeCap.Sign() < 0 {
		return ErrNegativeValue
	}
	if tx.gasFeeCap.Cmp(tx.gasTipCap) < 0 {
		return ErrGasTipOverFeeCap
	}
	return tx.blob.SanityCheck()
}

func (tx *BlobTx) ValidateSidecar() error { return tx.blob.ValidateSidecar() }

func (tx *BlobTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		TxType:   BlobTxType,
		Nonce:    tx.nonce,
		GasLimit: tx.gasLimit,
		ChainID:  tx.chainID,
	}
	if tx.gasFeeCap != nil {
		actCore.GasFeeCap = tx.gasFeeCap.String()
	}
	if tx.gasTipCap != nil {
		actCore.GasTipCap = tx.gasTipCap.String()
	}
	if len(tx.accessList) > 0 {
		actCore.AccessList = toAccessListProto(tx.accessList)
	}
	actCore.BlobTxData = tx.blob.toProto()
	return &actCore
}

func (tx *BlobTx) ProtoForRawHash() *iotextypes.ActionCore {
	actCore := tx.toProto()
	if actCore.BlobTxData != nil {
		actCore.BlobTxData.BlobTxSidecar = nil
	}
	return actCore
}

func (tx *BlobTx) fromProto(pb *iotextypes.ActionCore) error {
	if pb.TxType != BlobTxType {
		return errors.Wrapf(ErrInvalidProto, "wrong tx type = %d", pb.TxType)
	}
	var (
		feeCap   = new(uint256.Int)
		tipCap   = new(uint256.Int)
		blobData *BlobTxData
		err      error
	)
	if feeCapStr := pb.GetGasFeeCap(); len(feeCapStr) > 0 {
		if err := feeCap.SetFromDecimal(feeCapStr); err != nil {
			return errors.Errorf("invalid feeCap %s", feeCapStr)
		}
	}
	if tipCapStr := pb.GetGasTipCap(); len(tipCapStr) > 0 {
		if err := tipCap.SetFromDecimal(tipCapStr); err != nil {
			return errors.Errorf("invalid tipCap %s", tipCapStr)
		}
	}
	if blobPb := pb.GetBlobTxData(); blobPb != nil {
		if blobData, err = fromProtoBlobTxData(blobPb); err != nil {
			return err
		}
	}
	tx.nonce = pb.GetNonce()
	tx.gasLimit = pb.GetGasLimit()
	tx.chainID = pb.GetChainID()
	tx.gasFeeCap = feeCap
	tx.gasTipCap = tipCap
	tx.accessList = fromAccessListProto(pb.GetAccessList())
	tx.blob = blobData
	return nil
}

func (tx *BlobTx) setNonce(n uint64) {
	tx.nonce = n
}

func (tx *BlobTx) setGas(gas uint64) {
	tx.gasLimit = gas
}

func (tx *BlobTx) setChainID(n uint32) {
	tx.chainID = n
}

func (tx *BlobTx) toEthTx(to *common.Address, value *big.Int, data []byte) *types.Transaction {
	return types.NewTx(&types.BlobTx{
		Nonce:      tx.nonce,
		GasTipCap:  tx.tipCap(),
		GasFeeCap:  tx.feeCap(),
		Gas:        tx.gasLimit,
		To:         *to,
		Value:      uint256.MustFromBig(value),
		Data:       data,
		AccessList: tx.accessList,
		BlobFeeCap: uint256.MustFromBig(tx.BlobGasFeeCap()),
		BlobHashes: tx.BlobHashes(),
		Sidecar:    tx.BlobTxSidecar(),
	})
}
