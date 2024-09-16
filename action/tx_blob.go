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

// BlobTx represents EIP-4844 blob transaction
type BlobTx struct {
	chainID    uint32
	nonce      uint64
	gasLimit   uint64
	gasTipCap  *big.Int // a.k.a. maxPriorityFeePerGas
	gasFeeCap  *big.Int // a.k.a. maxFeePerGas
	accessList types.AccessList
	blob       *BlobTxData
}

func (tx *BlobTx) Version() uint32 {
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
	return tx.feeCap()
}

func (tx *BlobTx) GasTipCap() *big.Int {
	p := &big.Int{}
	if tx.gasTipCap == nil {
		return p
	}
	return p.Set(tx.gasTipCap)
}

func (tx *BlobTx) GasFeeCap() *big.Int {
	return tx.feeCap()
}

func (tx *BlobTx) feeCap() *big.Int {
	p := &big.Int{}
	if tx.gasFeeCap == nil {
		return p
	}
	return p.Set(tx.gasFeeCap)
}

func (tx *BlobTx) AccessList() types.AccessList {
	return tx.accessList
}

func (tx *BlobTx) BlobGas() uint64 { return tx.blob.blobGas() }

func (tx *BlobTx) BlobGasFeeCap() *big.Int { return tx.blob.BlobGasFeeCap() }

func (tx *BlobTx) BlobHashes() []common.Hash { return tx.blob.BlobHashes() }

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
	if tx.gasTipCap.BitLen() > 256 {
		return errors.Wrap(ErrValueVeryHigh, "tip cap is too high")
	}
	if tx.gasFeeCap.BitLen() > 256 {
		return errors.Wrap(ErrValueVeryHigh, "fee cap is too high")
	}
	return tx.blob.SanityCheck()
}

func (tx *BlobTx) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		Version:  BlobTxType,
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

func (tx *BlobTx) fromProto(pb *iotextypes.ActionCore) error {
	var (
		feeCap   *big.Int
		tipCap   *big.Int
		acl      types.AccessList
		blobData *BlobTxData
		err      error
	)
	if feeCapStr := pb.GetGasFeeCap(); len(feeCapStr) > 0 {
		v, ok := big.NewInt(0).SetString(feeCapStr, 10)
		if !ok {
			return errors.Errorf("invalid feeCap %s", feeCapStr)
		}
		feeCap = v
	}
	if tipCapStr := pb.GetGasTipCap(); len(tipCapStr) > 0 {
		v, ok := big.NewInt(0).SetString(tipCapStr, 10)
		if !ok {
			return errors.Errorf("invalid tipCap %s", tipCapStr)
		}
		tipCap = v
	}
	if aclp := pb.GetAccessList(); len(aclp) > 0 {
		acl = fromAccessListProto(aclp)
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
	tx.accessList = acl
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
