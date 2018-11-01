// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// PutBlockIntrinsicGas is the instrinsic gas for put block action.
const PutBlockIntrinsicGas = uint64(1000)

// PutBlock represents put a sub-chain block message.
type PutBlock struct {
	AbstractAction

	subChainAddress string
	height          uint64
	roots           map[string]hash.Hash32B
	producerAddress string
}

// NewPutBlock instantiates a putting sub-chain block action struct.
func NewPutBlock(
	nonce uint64,
	subChainAddress string,
	producerAddress string,
	height uint64,
	roots map[string]hash.Hash32B,
	gasLimit uint64,
	gasPrice *big.Int,
) *PutBlock {
	return &PutBlock{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  producerAddress,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		subChainAddress: subChainAddress,
		height:          height,
		roots:           roots,
		producerAddress: producerAddress,
	}
}

// LoadProto converts a proto message into put block action.
func (pb *PutBlock) LoadProto(actPb *iproto.ActionPb) {
	if pb == nil {
		return
	}
	*pb = PutBlock{}

	if actPb == nil {
		return
	}
	putBlockPb := actPb.GetPutBlock()
	if putBlockPb == nil {
		return
	}

	pb.version = actPb.Version
	pb.nonce = actPb.Nonce
	pb.srcAddr = putBlockPb.ProducerAddress
	copy(pb.srcPubkey[:], putBlockPb.ProducerPublicKey)
	pb.gasLimit = actPb.GasLimit
	pb.gasPrice = big.NewInt(0)
	if len(actPb.GasPrice) > 0 {
		pb.gasPrice.SetBytes(actPb.GasPrice)
	}

	pb.subChainAddress = putBlockPb.SubChainAddress
	pb.height = putBlockPb.Height
	pb.producerAddress = putBlockPb.ProducerAddress

	pb.roots = make(map[string]hash.Hash32B)
	for k, v := range putBlockPb.Roots {
		pb.roots[k] = byteutil.BytesTo32B(v)
	}
}

// Proto converts put sub-chain block action into a proto message.
func (pb *PutBlock) Proto() *iproto.ActionPb {
	// used by account-based model
	act := &iproto.ActionPb{
		Action: &iproto.ActionPb_PutBlock{
			PutBlock: &iproto.PutBlockPb{
				SubChainAddress:   pb.subChainAddress,
				Height:            pb.height,
				ProducerAddress:   pb.producerAddress,
				ProducerPublicKey: pb.srcPubkey[:],
			},
		},
		Version:   pb.version,
		Nonce:     pb.nonce,
		GasLimit:  pb.gasLimit,
		Signature: pb.signature,
	}

	putBlockPb := act.GetPutBlock()
	putBlockPb.Roots = make(map[string][]byte)
	for k, v := range pb.roots {
		putBlockPb.Roots[k] = make([]byte, len(v))
		copy(putBlockPb.Roots[k], v[:])
	}

	if pb.gasPrice != nil && len(pb.gasPrice.Bytes()) > 0 {
		act.GasPrice = pb.gasPrice.Bytes()
	}

	return act
}

// SubChainAddress returns sub chain address.
func (pb *PutBlock) SubChainAddress() string { return pb.subChainAddress }

// Height returns put block height.
func (pb *PutBlock) Height() uint64 { return pb.height }

// Roots return merkel roots put in.
func (pb *PutBlock) Roots() map[string]hash.Hash32B { return pb.roots }

// ProducerAddress return producer address.
func (pb *PutBlock) ProducerAddress() string { return pb.producerAddress }

// ProducerPublicKey return producer public key.
func (pb *PutBlock) ProducerPublicKey() keypair.PublicKey { return pb.SrcPubkey() }

// ByteStream returns the byte representation of put block action.
func (pb *PutBlock) ByteStream() []byte {
	stream := []byte(fmt.Sprintf("%T", pb))
	temp := make([]byte, 4)
	enc.MachineEndian.PutUint32(temp, pb.version)
	stream = append(stream, temp...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, pb.nonce)
	stream = append(stream, temp...)
	stream = append(stream, pb.subChainAddress...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, pb.height)
	stream = append(stream, temp...)
	stream = append(stream, pb.srcAddr...)
	stream = append(stream, pb.srcPubkey[:]...)
	temp = make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, pb.gasLimit)
	stream = append(stream, temp...)
	if pb.gasPrice != nil && len(pb.gasPrice.Bytes()) > 0 {
		stream = append(stream, pb.gasPrice.Bytes()...)
	}
	keys := make([]string, 0, len(pb.roots))
	for k := range pb.roots {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := pb.roots[k]
		stream = append(stream, k...)
		stream = append(stream, v[:]...)
	}
	return stream
}

// Hash returns the hash of putting a sub-chain block message
func (pb *PutBlock) Hash() hash.Hash32B { return blake2b.Sum256(pb.ByteStream()) }

// IntrinsicGas returns the intrinsic gas of a put block action
func (pb *PutBlock) IntrinsicGas() (uint64, error) {
	return PutBlockIntrinsicGas, nil
}

// Cost returns the total cost of a put block action
func (pb *PutBlock) Cost() (*big.Int, error) {
	intrinsicGas, err := pb.IntrinsicGas()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get intrinsic gas for the start-sub chain action")
	}
	fee := big.NewInt(0).Mul(pb.GasPrice(), big.NewInt(0).SetUint64(intrinsicGas))
	return fee, nil
}
