// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"

	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
)

// PutBlock represents put a sub-chain block message
type PutBlock struct {
	abstractAction
	chainID uint32
	height  uint64
	roots   map[string]hash.Hash32B
}

// NewStartSubChain instantiates a start sub-chain action struct
func NewPutBlock(
	nonce uint64,
	chainID uint32,
	producerAddress string,
	height uint64,
	roots map[string]hash.Hash32B,
	gasLimit uint64,
	gasPrice *big.Int,
) *PutBlock {
	return &PutBlock{
		abstractAction: abstractAction{
			version:  version.ProtocolVersion,
			nonce:    nonce,
			srcAddr:  producerAddress,
			gasLimit: gasLimit,
			gasPrice: gasPrice,
		},
		chainID: chainID,
		height:  height,
		roots:   roots,
	}
}

// NewPutBlockFromProto converts a proto message into start sub-chain action
func NewPutBlockFromProto(actPb *iproto.ActionPb) *PutBlock {
	if actPb == nil {
		return nil
	}
	putBlockPb := actPb.GetPutBlock()
	if putBlockPb == nil {
		return nil
	}
	pb := PutBlock{
		abstractAction: abstractAction{
			version:   actPb.Version,
			nonce:     actPb.Nonce,
			srcAddr:   putBlockPb.ProducerAddress,
			gasLimit:  actPb.GasLimit,
			gasPrice:  big.NewInt(0),
			signature: actPb.Signature,
		},
		chainID: putBlockPb.ChainID,
		height:  putBlockPb.Height,
	}
	pb.roots = make(map[string]hash.Hash32B)
	for k, v := range putBlockPb.Roots {
		pb.roots[k] = byteutil.BytesTo32B(v)
	}
	copy(pb.srcPubkey[:], putBlockPb.ProducerPublicKey)
	return &pb
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

	return act
}

func (pb *PutBlock) ChainID() uint32 { return pb.chainID }

func (pb *PutBlock) Height() uint64 { return pb.height }

func (pb *PutBlock) Roots() map[string]hash.Hash32B { return pb.roots }

func (pb *PutBlock) ByteStream() []byte {
	stream := []byte(reflect.TypeOF(pb).String())
	// TODO
	return stream
}

// Hash returns the hash of putting a sub-chain block message
func (pb *PutBlock) Hash() hash.Hash32B { return blake2b.Sum256(pb.ByteStream()) }
