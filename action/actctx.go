// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
)

// AbstractAction is an abstract implementation of Action interface
type AbstractAction struct {
	version  uint32
	chainID  uint32
	nonce    uint64
	gasLimit uint64
	gasPrice *big.Int
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
func (act *AbstractAction) SetEnvelopeContext(elp Envelope) {
	if act == nil {
		return
	}
	act.version = elp.Version()
	act.chainID = elp.ChainID()
	act.nonce = elp.Nonce()
	act.gasLimit = elp.GasLimit()
	act.gasPrice = elp.GasPrice()
}

// SanityCheck validates the variables in the action
func (act *AbstractAction) SanityCheck() error {
	// Reject execution of negative gas price
	if act.GasPrice().Sign() < 0 {
		return ErrNegativeValue
	}
	return nil
}
