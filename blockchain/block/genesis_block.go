// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package block

import (
	"time"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/pkg/version"
)

// GenesisBlock returns the genesis block
func GenesisBlock(timestamp time.Time) *Block {
	return &Block{
		Header: Header{
			version:          version.ProtocolVersion,
			height:           0,
			timestamp:        timestamp,
			prevBlockHash:    hash.ZeroHash256,
			txRoot:           hash.ZeroHash256,
			deltaStateDigest: hash.ZeroHash256,
			receiptRoot:      hash.ZeroHash256,
		},
	}
}
