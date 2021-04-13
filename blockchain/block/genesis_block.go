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

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/version"
)

// constants
const (
	GenesisHashMainnet = "ab7d006c1f7a9345ad05eef1b4f062814a176c25c7558052e18896844ee71edb"
	GenesisHashTestnet = "84b5d7158407b85c07ba1bcac0699dedc86d22bbcb29b6d77a5f696ac35047fd"
)

var (
	loadGenesisHash sync.Once
	_genesisHash    hash.Hash256
)

// GenesisBlock returns the genesis block
func GenesisBlock() *Block {
	return &Block{
		Header: Header{
			version:          version.ProtocolVersion,
			height:           0,
			timestamp:        time.Unix(config.GenesisTimestamp(), 0),
			prevBlockHash:    hash.ZeroHash256,
			txRoot:           hash.ZeroHash256,
			deltaStateDigest: hash.ZeroHash256,
			receiptRoot:      hash.ZeroHash256,
		},
	}
}

// GenesisHash returns the genesis block's hash
func GenesisHash() hash.Hash256 {
	return _genesisHash
}

// LoadGenesisHash is done once to compute and save the genesis block's hash
func LoadGenesisHash() {
	loadGenesisHash.Do(func() {
		_genesisHash = GenesisBlock().HashBlock()
	})
}
