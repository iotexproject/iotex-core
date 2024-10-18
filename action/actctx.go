// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

// AbstractAction is an abstract implementation of Action interface
type AbstractAction struct {
	version    uint32
	chainID    uint32
	txType     uint32
	nonce      uint64
	gasLimit   uint64
	gasPrice   *big.Int
	gasTipCap  *big.Int
	gasFeeCap  *big.Int
	accessList types.AccessList
	blobData   *BlobTxData
}

// ChainID returns the chainID
func (act *AbstractAction) ChainID() uint32 { return act.chainID }

// Nonce returns the nonce
func (act *AbstractAction) Nonce() uint64 { return act.nonce }

// SetNonce sets gaslimit
func (act *AbstractAction) SetNonce(val uint64) {
	act.nonce = val
}

// Gas returns the gas limit
func (act *AbstractAction) Gas() uint64 { return act.gasLimit }

// SetGasLimit sets gaslimit
func (act *AbstractAction) SetGasLimit(val uint64) {
	act.gasLimit = val
}

// GasPrice returns the gas price
func (act *AbstractAction) GasPrice() *big.Int {
	p := &big.Int{}
	if act.gasPrice == nil {
		return p
	}
	return p.Set(act.gasPrice)
}

// SetGasPrice sets gaslimit
func (act *AbstractAction) SetGasPrice(val *big.Int) {
	act.gasPrice = val
}

// GasTipCap returns the gas tip cap
func (act *AbstractAction) GasTipCap() *big.Int {
	if act.gasTipCap == nil {
		return act.GasPrice()
	}
	return new(big.Int).Set(act.gasTipCap)
}

// GasFeeCap returns the gas fee cap
func (act *AbstractAction) GasFeeCap() *big.Int {
	if act.gasFeeCap == nil {
		return act.GasPrice()
	}
	return new(big.Int).Set(act.gasFeeCap)
}

// BasicActionSize returns the basic size of action
func (act *AbstractAction) BasicActionSize() uint32 {
	// VersionSizeInBytes + NonceSizeInBytes + GasSizeInBytes
	size := 4 + 8 + 8
	if act.gasPrice != nil && len(act.gasPrice.Bytes()) > 0 {
		size += len(act.gasPrice.Bytes())
	}

	return uint32(size)
}

// SanityCheck validates the variables in the action
func (act *AbstractAction) SanityCheck() error {
	// Reject execution of negative gas price
	if act.gasPrice != nil && act.gasPrice.Sign() < 0 {
		return ErrNegativeValue
	}
	if act.gasTipCap != nil && act.gasTipCap.Sign() < 0 {
		return ErrNegativeValue
	}
	if act.gasFeeCap != nil && act.gasFeeCap.Sign() < 0 {
		return ErrNegativeValue
	}
	return nil
}

func (act *AbstractAction) toProto() *iotextypes.ActionCore {
	actCore := iotextypes.ActionCore{
		Version:  act.version,
		Nonce:    act.nonce,
		GasLimit: act.gasLimit,
		ChainID:  act.chainID,
		TxType:   act.txType,
	}
	if act.gasPrice != nil {
		actCore.GasPrice = act.gasPrice.String()
	}
	if act.gasTipCap != nil {
		actCore.GasTipCap = act.gasTipCap.String()
	}
	if act.gasFeeCap != nil {
		actCore.GasFeeCap = act.gasFeeCap.String()
	}
	if act.accessList != nil {
		actCore.AccessList = toAccessListProto(act.accessList)
	}
	return &actCore
}

func (act *AbstractAction) fromProto(pb *iotextypes.ActionCore) error {
	act.version = pb.GetVersion()
	act.nonce = pb.GetNonce()
	act.gasLimit = pb.GetGasLimit()
	act.chainID = pb.GetChainID()
	act.txType = pb.GetTxType()

	var ok bool
	if price := pb.GetGasPrice(); price == "" {
		act.gasPrice = &big.Int{}
	} else {
		if act.gasPrice, ok = new(big.Int).SetString(price, 10); !ok {
			return errors.Errorf("invalid gasPrice %s", price)
		}
	}
	if gasTip := pb.GetGasTipCap(); gasTip == "" {
		act.gasTipCap = nil
	} else {
		if act.gasTipCap, ok = new(big.Int).SetString(gasTip, 10); !ok {
			return errors.Errorf("invalid gasTipCap %s", gasTip)
		}
	}
	if gasFee := pb.GetGasFeeCap(); gasFee == "" {
		act.gasFeeCap = nil
	} else {
		if act.gasFeeCap, ok = new(big.Int).SetString(gasFee, 10); !ok {
			return errors.Errorf("invalid gasFeeCap %s", gasFee)
		}
	}
	if acl := pb.GetAccessList(); acl != nil {
		act.accessList = fromAccessListProto(acl)
	}
	return nil
}

func (act *AbstractAction) convertToTx() TxCommonInternal {
	switch act.txType {
	case LegacyTxType:
		tx := LegacyTx{
			chainID:  act.chainID,
			version:  act.version,
			nonce:    act.nonce,
			gasLimit: act.gasLimit,
			gasPrice: &big.Int{},
		}
		if act.gasPrice != nil {
			tx.gasPrice.Set(act.gasPrice)
		}
		return &tx
	case AccessListTxType:
		tx := AccessListTx{
			chainID:    act.chainID,
			nonce:      act.nonce,
			gasLimit:   act.gasLimit,
			gasPrice:   &big.Int{},
			accessList: act.accessList,
		}
		if act.gasPrice != nil {
			tx.gasPrice.Set(act.gasPrice)
		}
		return &tx
	case DynamicFeeTxType:
		return &DynamicFeeTx{
			chainID:    act.chainID,
			nonce:      act.nonce,
			gasLimit:   act.gasLimit,
			gasTipCap:  act.gasTipCap,
			gasFeeCap:  act.gasFeeCap,
			accessList: act.accessList,
		}
	case BlobTxType:
		return &BlobTx{
			chainID:    act.chainID,
			nonce:      act.nonce,
			gasLimit:   act.gasLimit,
			gasTipCap:  uint256.MustFromBig(act.gasTipCap),
			gasFeeCap:  uint256.MustFromBig(act.gasFeeCap),
			accessList: act.accessList,
			blob:       act.blobData,
		}
	default:
		panic(fmt.Sprintf("unsupported action type = %d", act.txType))
	}
}

func (act *AbstractAction) validateTx() error {
	switch act.txType {
	case LegacyTxType, AccessListTxType, DynamicFeeTxType, BlobTxType:
		// these are allowed tx types
	default:
		return errors.Wrapf(ErrInvalidAct, "unsupported tx type = %d", act.txType)
	}
	if act.txType == BlobTxType && act.blobData == nil {
		return errors.Wrap(ErrInvalidAct, "blob tx with empty blob")
	}
	return nil
}
