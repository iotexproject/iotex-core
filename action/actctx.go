// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
)

// AbstractAction is an abstract implementation of Action interface
type AbstractAction struct {
	version    uint32
	chainID    uint32
	nonce      uint64
	gasLimit   uint64
	gasPrice   *big.Int
	gasTipCap  *big.Int
	gasFeeCap  *big.Int
	accessList types.AccessList
}

// Version returns the version
func (act *AbstractAction) Version() uint32 { return act.version }

// ChainID returns the chainID
func (act *AbstractAction) ChainID() uint32 { return act.chainID }

// Nonce returns the nonce
func (act *AbstractAction) Nonce() uint64 { return act.nonce }

// SetNonce sets gaslimit
func (act *AbstractAction) SetNonce(val uint64) {
	act.nonce = val
}

// GasLimit returns the gas limit
func (act *AbstractAction) GasLimit() uint64 { return act.gasLimit }

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

// SetEnvelopeContext sets the struct according to input
func (act *AbstractAction) SetEnvelopeContext(in *AbstractAction) {
	if act == nil {
		return
	}
	*act = *in
	if in.gasPrice != nil {
		act.gasPrice = new(big.Int).Set(in.gasPrice)
	}
	if in.gasTipCap != nil {
		act.gasTipCap = new(big.Int).Set(in.gasTipCap)
	}
	if in.gasFeeCap != nil {
		act.gasFeeCap = new(big.Int).Set(in.gasFeeCap)
	}
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
