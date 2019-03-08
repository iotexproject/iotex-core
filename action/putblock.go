// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"sort"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

// PutBlockIntrinsicGas is the instrinsic gas for put block action.
const PutBlockIntrinsicGas = uint64(1000)

var _ hasDestination = (*PutBlock)(nil)

// PutBlock represents put a sub-chain block message.
type PutBlock struct {
	AbstractAction

	subChainAddress string
	height          uint64
	roots           map[string]hash.Hash256
}

// NewPutBlock instantiates a putting sub-chain block action struct.
func NewPutBlock(
	nonce uint64,
	subChainAddress string,
	height uint64,
	roots map[string]hash.Hash256,
	gasLimit uint64,
	gasPrice *big.Int,
) *PutBlock {
	return &PutBlock{
		AbstractAction: AbstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		subChainAddress: subChainAddress,
		height:          height,
		roots:           roots,
	}
}

// LoadProto converts a proto message into put block action.
func (pb *PutBlock) LoadProto(putBlockPb *iotextypes.PutBlock) error {
	if putBlockPb == nil {
		return errors.New("empty action proto to load")
	}
	if pb == nil {
		return errors.New("nil action to load proto")
	}
	*pb = PutBlock{}

	pb.subChainAddress = putBlockPb.SubChainAddress
	pb.height = putBlockPb.Height

	pb.roots = make(map[string]hash.Hash256)
	for _, r := range putBlockPb.Roots {
		pb.roots[r.Name] = hash.BytesToHash256(r.Value)
	}
	return nil
}

// Proto converts put sub-chain block action into a proto message.
func (pb *PutBlock) Proto() *iotextypes.PutBlock {
	act := &iotextypes.PutBlock{
		SubChainAddress: pb.subChainAddress,
		Height:          pb.height,
	}

	keys := make([]string, 0, len(pb.roots))
	for k := range pb.roots {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	act.Roots = make([]*iotextypes.MerkleRoot, 0, len(pb.roots))
	for _, k := range keys {
		v := pb.roots[k]
		nv := make([]byte, len(v))
		copy(nv, v[:])

		act.Roots = append(act.Roots, &iotextypes.MerkleRoot{
			Name:  k,
			Value: nv,
		})
	}

	return act
}

// SubChainAddress returns sub chain address.
func (pb *PutBlock) SubChainAddress() string { return pb.subChainAddress }

// Destination returns sub chain address.
func (pb *PutBlock) Destination() string { return pb.SubChainAddress() }

// Height returns put block height.
func (pb *PutBlock) Height() uint64 { return pb.height }

// Roots return merkel roots put in.
func (pb *PutBlock) Roots() map[string]hash.Hash256 { return pb.roots }

// ProducerPublicKey return producer public key.
func (pb *PutBlock) ProducerPublicKey() keypair.PublicKey { return pb.SrcPubkey() }

// ByteStream returns the byte representation of put block action.
func (pb *PutBlock) ByteStream() []byte {
	return byteutil.Must(proto.Marshal(pb.Proto()))
}

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
