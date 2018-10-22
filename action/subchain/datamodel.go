// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state"
)

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

// Serialize serialize sub-chain state into bytes
func (bs *SubChain) Serialize() ([]byte, error) { return state.GobBasedSerialize(bs) }

// Deserialize deserialize bytes into sub-chain state
func (bs *SubChain) Deserialize(data []byte) error { return state.GobBasedDeserialize(bs, data) }

// blockProof represents the block proof of a sub-chain in the state factory
type blockProof struct {
	Hash              hash.Hash32B
	ActionRoot        hash.Hash32B
	StateRoot         hash.Hash32B
	ProducerPublicKey keypair.PublicKey
	// ConfirmationHeight refers to the root chain block height where the sub-chain block gets confirmed
	ConfirmationHeight uint64
}

// Serialize serialize block proof state into bytes
func (bp *blockProof) Serialize() ([]byte, error) { return state.GobBasedSerialize(bp) }

// Deserialize deserialize bytes into block proof state
func (bp *blockProof) Deserialize(data []byte) error { return state.GobBasedDeserialize(bp, data) }
