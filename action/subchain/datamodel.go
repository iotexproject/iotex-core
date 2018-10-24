// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"math/big"
	"sort"

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

// Serialize serializes sub-chain state into bytes
func (bs *SubChain) Serialize() ([]byte, error) { return state.GobBasedSerialize(bs) }

// Deserialize deserializes bytes into sub-chain state
func (bs *SubChain) Deserialize(data []byte) error { return state.GobBasedDeserialize(bs, data) }

// blockProof represents the block proof of a sub-chain in the state factory
type blockProof struct {
	// TODO add all data fields
	Root              hash.Hash32B
	ProducerPublicKey keypair.PublicKey
}

// Serialize serialize block proof state into bytes
func (bp *blockProof) Serialize() ([]byte, error) { return state.GobBasedSerialize(bp) }

// Deserialize deserialize bytes into block proof state
func (bp *blockProof) Deserialize(data []byte) error { return state.GobBasedDeserialize(bp, data) }

// UsedChainIDs represents the used chain IDs in the state factory, which is sorted
type UsedChainIDs []uint32

// Serialize serializes the used chain IDs into bytes
func (usedChainIDs *UsedChainIDs) Serialize() ([]byte, error) {
	return state.GobBasedSerialize(usedChainIDs)
}

// Deserialize deserializes bytes into the used chain IDs
func (usedChainIDs *UsedChainIDs) Deserialize(data []byte) error {
	if err := state.GobBasedDeserialize(usedChainIDs, data); err != nil {
		return err
	}
	return nil
}

// Exist check if a chain ID is used or not
func (usedChainIDs UsedChainIDs) Exist(chainID uint32) bool {
	idx := sort.Search(len(usedChainIDs), func(i int) bool {
		return usedChainIDs[i] >= chainID
	})
	return idx < len(usedChainIDs) && usedChainIDs[idx] == chainID
}

// Append appends a chain ID into the used chain IDs
func (usedChainIDs UsedChainIDs) Append(chainID uint32) UsedChainIDs {
	usedChainIDs = append(usedChainIDs, chainID)
	sort.Slice(usedChainIDs, func(i, j int) bool {
		return usedChainIDs[i] < usedChainIDs[j]
	})
	return usedChainIDs
}
