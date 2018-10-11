// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/errcode"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// StartSubChain represents start sub-chain message
type StartSubChain struct {
	action
	chainID            uint32
	securityDeposit    big.Int
	operationDeposit   big.Int
	startHeight        uint64
	parentHeightOffset uint64
}

// Hash returns the hash of starting sub-chain message
func (start *StartSubChain) Hash() hash.Hash32B {
	// TODO: implement hash generation
	var hash hash.Hash32B
	return hash
}

// StopSubChain represents stop sub-chain message
type StopSubChain struct {
	action
	chainID    uint32
	stopHeight uint64
}

// Hash returns the hash of stopping sub-chain message
func (stop *StopSubChain) Hash() hash.Hash32B {
	// TODO: implement hash generation
	var hash hash.Hash32B
	return hash
}

// PutBlock represents put a sub-chain block message
type PutBlock struct {
	action
	chainID            uint32
	height             uint64
	hash               hash.Hash32B
	actionRoot         hash.Hash32B
	stateRoot          hash.Hash32B
	endorsorSignatures map[keypair.PublicKey][]byte
}

// Hash returns the hash of putting a sub-chain block message
func (put *PutBlock) Hash() hash.Hash32B {
	// TODO: implement hash generation
	var hash hash.Hash32B
	return hash
}

// TODO: we need to remove this duplicate code of blockchain/action/action.go
type action struct {
	version   uint32
	nonce     uint64
	srcAddr   string
	srcPubkey keypair.PublicKey
	dstAddr   string
	gasLimit  uint64
	gasPrice  *big.Int
	signature []byte
}

// Version returns the version
func (act *action) Version() uint32 { return act.version }

// Nonce returns the nonce
func (act *action) Nonce() uint64 { return act.nonce }

// SrcAddr returns the source address
func (act *action) SrcAddr() string { return act.srcAddr }

// SrcPubkey returns the source public key
func (act *action) SrcPubkey() keypair.PublicKey { return act.srcPubkey }

// SetSrcPubkey sets the source public key
func (act *action) SetSrcPubkey(srcPubkey keypair.PublicKey) { act.srcPubkey = srcPubkey }

// DstAddr returns the destination address
func (act *action) DstAddr() string { return act.dstAddr }

// GasLimit returns the gas limit
func (act *action) GasLimit() uint64 { return act.gasLimit }

// GasPrice returns the gas price
func (act *action) GasPrice() *big.Int { return act.gasPrice }

// Signature returns signature bytes
func (act *action) Signature() []byte { return act.signature }

// SetSignature sets the signature bytes
func (act *action) SetSignature(signature []byte) { act.signature = signature }

// IntrinsicGas returns the intrinsic gas of an action
func (start *StartSubChain) IntrinsicGas() (uint64, error) {
	// TODO: implement intrinsic gas calculation
	return 0, errcode.ErrNotImplemented
}
