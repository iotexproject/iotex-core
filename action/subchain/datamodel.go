// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"encoding/gob"
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state"
)

func init() {
	gob.Register(InOperation{})
}

// SubChain represents the state of a sub-chain in the state factory
type SubChain struct {
	ChainID            uint32
	SecurityDeposit    *big.Int
	OperationDeposit   *big.Int
	StartHeight        uint64
	ParentHeightOffset uint64
	OwnerPublicKey     keypair.PublicKey
	CurrentHeight      uint64
}

// Serialize serializes sub-chain state into bytes
func (bs *SubChain) Serialize() ([]byte, error) { return state.GobBasedSerialize(bs) }

// Deserialize deserializes bytes into sub-chain state
func (bs *SubChain) Deserialize(data []byte) error { return state.GobBasedDeserialize(bs, data) }

// MerkleRoot defines a merkle root in block proof.
type MerkleRoot struct {
	Name  string
	Value hash.Hash32B
}

// BlockProof represents the block proof of a sub-chain in the state factory
type BlockProof struct {
	SubChainAddress   string
	Height            uint64
	Roots             []MerkleRoot
	ProducerPublicKey keypair.PublicKey
	ProducerAddress   string
}

// Serialize serialize block proof state into bytes
func (bp *BlockProof) Serialize() ([]byte, error) { return state.GobBasedSerialize(bp) }

// Deserialize deserialize bytes into block proof state
func (bp *BlockProof) Deserialize(data []byte) error { return state.GobBasedDeserialize(bp, data) }

// InOperation represents a record of a sub-chain in operation
type InOperation struct {
	ID   uint32
	Addr []byte
}

// SortInOperation compare two ChainInUse records by their chain IDs. If one of the input's type is not
// InOperation, it will not be comparable and 0 will be returned.
func SortInOperation(x interface{}, y interface{}) int {
	cio1, ok := x.(InOperation)
	if !ok {
		return 0
	}
	cio2, ok := y.(InOperation)
	if !ok {
		return 0
	}
	return int(int64(cio1.ID) - int64(cio2.ID))
}

// StartSubChainReceipt is the receipt to user after executed start sub chain operation.
type StartSubChainReceipt struct {
	SubChainAddress string
}
