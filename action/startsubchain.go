// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"reflect"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// StartSubChainIntrinsicGas is the intrinsic gas for start sub-chain action
	StartSubChainIntrinsicGas = uint64(1000)
)

// StartSubChain represents start sub-chain message
type StartSubChain struct {
	abstractAction
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
		abstractAction: abstractAction{
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

// LoadProto converts a proto message into start sub-chain action
func (start *StartSubChain) LoadProto(actPb *iproto.ActionPb) {
	if actPb == nil {
		return
	}
	startPb := actPb.GetStartSubChain()
	start.version = actPb.Version
	start.nonce = actPb.Nonce
	start.srcAddr = startPb.OwnerAddress
	start.gasLimit = actPb.GetGasLimit()
	start.gasPrice = big.NewInt(0)
	start.signature = actPb.Signature
	start.chainID = startPb.ChainID
	start.securityDeposit = big.NewInt(0)
	start.operationDeposit = big.NewInt(0)
	start.startHeight = startPb.StartHeight
	start.parentHeightOffset = startPb.ParentHeightOffset
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

// ByteStream returns the byte representation of sub-chain action
func (start *StartSubChain) ByteStream() []byte {
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
	return stream
}

// Hash returns the hash of starting sub-chain message
func (start *StartSubChain) Hash() hash.Hash32B {
	return blake2b.Sum256(start.ByteStream())
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

// IntrinsicGas returns the intrinsic gas of a start sub-chain action
func (start *StartSubChain) IntrinsicGas() (uint64, error) {
	return StartSubChainIntrinsicGas, nil
}

// Cost returns the total cost of a start sub-chain action
func (start *StartSubChain) Cost() (*big.Int, error) {
	intrinsicGas, err := start.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the start-sub chain action")
	}
	fee := big.NewInt(0).Mul(start.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}
