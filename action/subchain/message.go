// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"
	"reflect"

	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/errcode"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// StartSubChain represents start sub-chain message
type StartSubChain struct {
	action
	chainID            uint32
	securityDeposit    *big.Int
	operationDeposit   *big.Int
	startHeight        uint64
	parentHeightOffset uint64
}

// NewStartSubChain instantiates a start sub-chain action struct
func NewStartSubChain(
	nonce uint64,
	chainID uint32,
	ownerAddr string,
	securityDeposit *big.Int,
	operationDeposit *big.Int,
	startHeight uint64,
	parentHeightOffset uint64,
	gasLimit uint64,
	gasPrice *big.Int,
) *StartSubChain {
	return &StartSubChain{
		action: action{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  ownerAddr,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		chainID:            chainID,
		securityDeposit:    securityDeposit,
		operationDeposit:   operationDeposit,
		startHeight:        startHeight,
		parentHeightOffset: parentHeightOffset,
	}
}

// NewStartSubChainFromProto converts a proto message into start sub-chain action
func NewStartSubChainFromProto(actPb *iproto.ActionPb) *StartSubChain {
	if actPb == nil {
		return nil
	}
	startPb := actPb.GetStartSubChain()
	start := StartSubChain{
		action: action{
			version:   actPb.Version,
			nonce:     actPb.Nonce,
			srcAddr:   startPb.OwnerAddress,
			gasLimit:  actPb.GetGasLimit(),
			gasPrice:  big.NewInt(0),
			signature: actPb.Signature,
		},
		chainID:            startPb.ChainID,
		securityDeposit:    big.NewInt(0),
		operationDeposit:   big.NewInt(0),
		startHeight:        startPb.StartHeight,
		parentHeightOffset: startPb.ParentHeightOffset,
	}
	if len(actPb.GasPrice) > 0 {
		start.gasPrice.SetBytes(actPb.GasPrice)
	}
	if len(startPb.SecurityDeposit) > 0 {
		start.securityDeposit.SetBytes(startPb.SecurityDeposit)
	}
	if len(startPb.OperationDeposit) > 0 {
		start.operationDeposit.SetBytes(startPb.OperationDeposit)
	}
	copy(start.srcPubkey[:], startPb.OwnerPublicKey)
	return &start
}

// ChainID returns chain ID
func (start *StartSubChain) ChainID() uint32 { return start.chainID }

// SecurityDeposit returns security deposit
func (start *StartSubChain) SecurityDeposit() *big.Int { return start.securityDeposit }

// OperationDeposit returns operation deposit
func (start *StartSubChain) OperationDeposit() *big.Int { return start.operationDeposit }

// StartHeight returns start height
func (start *StartSubChain) StartHeight() uint64 { return start.startHeight }

// ParentHeightOffset returns parent height offset
func (start *StartSubChain) ParentHeightOffset() uint64 { return start.parentHeightOffset }

// OwnerAddress returns the owner address, which is the wrapper of SrcAddr
func (start *StartSubChain) OwnerAddress() string { return start.SrcAddr() }

// OwnerPublicKey returns the owner public key, which is the wrapper of SrcPubkey
func (start *StartSubChain) OwnerPublicKey() keypair.PublicKey { return start.SrcPubkey() }

// Hash returns the hash of starting sub-chain message
func (start *StartSubChain) Hash() hash.Hash32B {
	stream := []byte(reflect.TypeOf(start).String())
	temp := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, start.version)
	stream = append(stream, temp...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, start.nonce)
	stream = append(stream, temp...)
	temp = make([]byte, 4)
	enc.MachineEndian.PutUint32(temp, start.chainID)
	stream = append(stream, temp...)
	if start.securityDeposit != nil && len(start.securityDeposit.Bytes()) > 0 {
		stream = append(stream, start.securityDeposit.Bytes()...)
	}
	if start.operationDeposit != nil && len(start.operationDeposit.Bytes()) > 0 {
		stream = append(stream, start.operationDeposit.Bytes()...)
	}
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, start.startHeight)
	stream = append(stream, temp...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, start.parentHeightOffset)
	stream = append(stream, temp...)
	stream = append(stream, start.srcAddr...)
	stream = append(stream, start.srcPubkey[:]...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, start.gasLimit)
	stream = append(stream, temp...)
	if start.gasPrice != nil && len(start.gasPrice.Bytes()) > 0 {
		stream = append(stream, start.gasPrice.Bytes()...)
	}
	return blake2b.Sum256(stream)
}

// Proto converts start sub-chain action into a proto message
func (start *StartSubChain) Proto() *iproto.ActionPb {
	// used by account-based model
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_StartSubChain{
			StartSubChain: &iproto.StartSubChainPb{
				ChainID:            start.chainID,
				StartHeight:        start.startHeight,
				ParentHeightOffset: start.parentHeightOffset,
				OwnerAddress:       start.srcAddr,
				OwnerPublicKey:     start.srcPubkey[:],
			},
		},
		Version:   start.version,
		Nonce:     start.nonce,
		GasLimit:  start.gasLimit,
		Signature: start.signature,
	}

	if start.securityDeposit != nil && len(start.securityDeposit.Bytes()) > 0 {
		act.GetStartSubChain().SecurityDeposit = start.securityDeposit.Bytes()
	}
	if start.operationDeposit != nil && len(start.operationDeposit.Bytes()) > 0 {
		act.GetStartSubChain().OperationDeposit = start.operationDeposit.Bytes()
	}
	if start.gasPrice != nil && len(start.gasPrice.Bytes()) > 0 {
		act.GasPrice = start.gasPrice.Bytes()
	}
	return act
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
