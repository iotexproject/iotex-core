// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// subChain represents the state of a sub-chain in the state factory
type subChain struct {
	chainID            uint32
	securityDeposit    big.Int
	operationDeposit   big.Int
	startHeight        uint64
	parentHeightOffset uint64
	ownerPublicKey     keypair.PublicKey
	blocks             map[uint64]*blockProof
}

// blockProof represents the block proof of a sub-chain in the state factory
type blockProof struct {
	hash              hash.Hash32B
	actionRoot        hash.Hash32B
	stateRoot         hash.Hash32B
	producerPublicKey keypair.PublicKey
	// confirmationHeight refers to the root chain block height where the sub-chain block gets confirmed
	confirmationHeight uint64
}
