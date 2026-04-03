// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/version"
)

var (
	_loadGenesisHash sync.Once
	_genesisHash     atomic.Value // stores hash.Hash256
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
	if v := _genesisHash.Load(); v != nil {
		return v.(hash.Hash256)
	}
	return hash.ZeroHash256
}

// LoadGenesisHash computes and saves the genesis block's hash from the genesis config.
// It uses sync.Once to ensure it's only set once in production (via server/main.go).
// For testing with multiple genesis configs, use SetGenesisHash directly.
func LoadGenesisHash(g *genesis.Genesis) {
	_loadGenesisHash.Do(func() {
		_genesisHash.Store(g.Hash())
	})
}

// SetGenesisHash forces the genesis hash to a specific value.
// IMPORTANT: Must be called during single-threaded init only, before any goroutines
// call GenesisHash(). Used by tests and minicluster that create custom genesis configs
// after startup.
func SetGenesisHash(g *genesis.Genesis) {
	_genesisHash.Store(g.Hash())
}
