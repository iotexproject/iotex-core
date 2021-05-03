// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"sync"
	"time"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/version"
)

var (
	loadGenesisHash sync.Once
	genesisHash     hash.Hash256
)

// GenesisBlock returns the genesis block
func GenesisBlock() *Block {
	return &Block{
		Header: Header{
			version:          version.ProtocolVersion,
			height:           0,
			timestamp:        time.Unix(genesis.Timestamp(), 0),
			prevBlockHash:    hash.ZeroHash256,
			txRoot:           hash.ZeroHash256,
			deltaStateDigest: hash.ZeroHash256,
			receiptRoot:      hash.ZeroHash256,
		},
	}
}

// GenesisHash returns the genesis block's hash
func GenesisHash() hash.Hash256 {
	return genesisHash
}

// LoadGenesisHash is done once to compute and save the genesis block's hash
func LoadGenesisHash() {
	loadGenesisHash.Do(func() {
		genesisHash = GenesisBlock().HashBlock()
	})
}
