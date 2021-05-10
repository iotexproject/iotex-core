// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

// uniQueue is not threadsafe
type uniQueue struct {
	blocks []*block.Block
	hashes map[hash.Hash256]bool
}

func newUniQueue() *uniQueue {
	return &uniQueue{
		blocks: []*block.Block{},
		hashes: map[hash.Hash256]bool{},
	}
}

func (uq *uniQueue) enque(blk *block.Block) {
	h := blk.HashBlock()
	if _, ok := uq.hashes[h]; ok {
		return
	}
	uq.hashes[h] = true
	uq.blocks = append(uq.blocks, blk)
}

func (uq *uniQueue) dequeAll() []*block.Block {
	if len(uq.blocks) == 0 {
		return nil
	}
	blks := uq.blocks
	uq.blocks = []*block.Block{}
	uq.hashes = map[hash.Hash256]bool{}

	return blks
}
