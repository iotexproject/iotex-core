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
	// PutBlockIntrinsicGas is the intrinsic gas for put block action
	PutBlockIntrinsicGas = uint64(1000)
)

// PutBlock represents put a sub-chain block action
type PutBlock struct {
	abstractAction
	chainID      uint32
	height       uint64
	hash         hash.Hash32B
	actionRoot   hash.Hash32B
	stateRoot    hash.Hash32B
	producerAddr string
	endorsements []*Endorsement
}

// Endorsement represent a endorsement
type Endorsement struct {
	endorser  keypair.PublicKey
	signature []byte
}

// NewPutBlock instantiates a put block action
func NewPutBlock(
	nonce uint64,
	chainID uint32,
	height uint64,
	hash hash.Hash32B,
	actionRoot hash.Hash32B,
	stateRoot hash.Hash32B,
	producerAddr string,
	endorsements []*Endorsement,
	gasLimit uint64,
	gasPrice *big.Int,
) *PutBlock {
	return &PutBlock{
		abstractAction: abstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  producerAddr,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		chainID:      chainID,
		height:       height,
		hash:         hash,
		actionRoot:   actionRoot,
		stateRoot:    stateRoot,
		endorsements: endorsements,
	}
}

// ChainID returns the chain ID
func (put *PutBlock) ChainID() uint32 { return put.chainID }

// Height returns the height
func (put *PutBlock) Height() uint64 { return put.height }

// BlockHash returns the block hash
func (put *PutBlock) BlockHash() hash.Hash32B { return put.hash }

// ActionRoot returns the action root
func (put *PutBlock) ActionRoot() hash.Hash32B { return put.actionRoot }

// StateRoot returns the state root
func (put *PutBlock) StateRoot() hash.Hash32B { return put.stateRoot }

// ProducerAddress returns the producer address, which is the wrapper of SrcAddr
func (put *PutBlock) ProducerAddress() string { return put.SrcAddr() }

// ProducerPublicKey returns the producer public key, which is the wrapper of SrcPubkey
func (put *PutBlock) ProducerPublicKey() keypair.PublicKey { return put.SrcPubkey() }

// ByteStream returns the byte representation of a put block action
func (put *PutBlock) ByteStream() []byte {
	stream := []byte(reflect.TypeOf(put).String())
	temp := make([]byte, 4)
	enc.MachineEndian.PutUint32(stream, put.version)
	stream = append(stream, temp...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, put.nonce)
	stream = append(stream, temp...)
	temp = make([]byte, 4)
	enc.MachineEndian.PutUint32(temp, put.chainID)
	stream = append(stream, temp...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, put.height)
	stream = append(stream, temp...)
	stream = append(stream, put.hash[:]...)
	stream = append(stream, put.actionRoot[:]...)
	stream = append(stream, put.stateRoot[:]...)
	for _, endorsement := range put.endorsements {
		stream = append(stream, endorsement.endorser[:]...)
		stream = append(stream, endorsement.signature...)
	}
	stream = append(stream, put.srcAddr...)
	stream = append(stream, put.srcPubkey[:]...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, put.gasLimit)
	stream = append(stream, temp...)
	if put.gasPrice != nil && len(put.gasPrice.Bytes()) > 0 {
		stream = append(stream, put.gasPrice.Bytes()...)
	}
	return stream
}

// Hash returns the hash of a put block action
func (put *PutBlock) Hash() hash.Hash32B {
	return blake2b.Sum256(put.ByteStream())
}

// Proto converts put block action into a proto message
func (put *PutBlock) Proto() *iproto.ActionPb {
	endorsementPbs := make([]*iproto.EndorsementPb, len(put.endorsements))
	for i, endorsement := range put.endorsements {
		endorsementPbs[i] = &iproto.EndorsementPb{
			Endorser:  endorsement.endorser[:],
			Signature: endorsement.signature,
		}
	}
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_PutBlock{
			PutBlock: &iproto.PutBlockPb{
				ChainID:           put.chainID,
				Height:            put.height,
				Hash:              put.hash[:],
				ActionRoot:        put.actionRoot[:],
				StateRoot:         put.stateRoot[:],
				ProducerAddress:   put.srcAddr,
				ProducerPublicKey: put.srcPubkey[:],
				Endorsements:      endorsementPbs,
			},
		},
		Version:   put.version,
		Nonce:     put.nonce,
		GasLimit:  put.gasLimit,
		Signature: put.signature,
	}
	if put.gasPrice != nil && len(put.gasPrice.Bytes()) > 0 {
		act.GasPrice = put.gasPrice.Bytes()
	}
	return act
}

// FromProto converts proto message into a put block action
func (put *PutBlock) FromProto(actPb *iproto.ActionPb) {
	if actPb == nil {
		return
	}
	putPb := actPb.GetPutBlock()
	put.version = actPb.Version
	put.nonce = actPb.Nonce
	put.gasLimit = actPb.GasLimit
	copy(put.signature, actPb.Signature)
	put.gasPrice = big.NewInt(0)
	if len(actPb.GasPrice) > 0 {
		put.gasPrice.SetBytes(actPb.GasPrice)
	}
	put.chainID = putPb.ChainID
	put.height = putPb.Height
	copy(put.hash[:], putPb.Hash)
	copy(put.actionRoot[:], putPb.ActionRoot)
	copy(put.stateRoot[:], putPb.StateRoot)
	put.srcAddr = putPb.ProducerAddress
	copy(put.srcPubkey[:], putPb.ProducerPublicKey)
	for _, endorsementPb := range putPb.Endorsements {
		var pk keypair.PublicKey
		copy(pk[:], endorsementPb.Endorser)
		put.endorsements = append(put.endorsements, &Endorsement{
			endorser:  pk,
			signature: endorsementPb.Signature,
		})
	}
}

// IntrinsicGas returns the intrinsic gas of a put block action
func (put *PutBlock) IntrinsicGas() (uint64, error) {
	return PutBlockIntrinsicGas, nil
}

// Cost returns the total cost of a put block action
func (put *PutBlock) Cost() (*big.Int, error) {
	intrinsicGas, err := put.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the put block action")
	}
	fee := big.NewInt(0).Mul(put.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}
