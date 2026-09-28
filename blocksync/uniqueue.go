// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"github.com/iotexproject/go-pkgs/hash"
)

// maxBlocksPerHeight bounds how many distinct blocks are buffered for a single
// height. Blocks are buffered before any validation, and dedup is by full block
// hash, so without a cap a peer can flood cheaply-mutated variants of one height
// and grow the queue without bound while the tip is stalled.
//
// Honest traffic needs very few: a height carries one block, plus a small number
// of competing proposals across a fork.
const maxBlocksPerHeight = 8

// uniQueue is not threadsafe
type uniQueue struct {
	blocks []*peerBlock
	hashes map[hash.Hash256]struct{}
}

func newUniQueue() *uniQueue {
	return &uniQueue{
		blocks: []*peerBlock{},
		hashes: map[hash.Hash256]struct{}{},
	}
}

// enque adds blk unless it is a duplicate or the height is already full. It
// reports whether the block was added.
func (uq *uniQueue) enque(blk *peerBlock) bool {
	h := blk.block.HashBlock()
	if _, ok := uq.hashes[h]; ok {
		return false
	}
	if len(uq.blocks) >= maxBlocksPerHeight {
		return false
	}
	uq.hashes[h] = struct{}{}
	uq.blocks = append(uq.blocks, blk)
	return true
}

func (uq *uniQueue) dequeAll() []*peerBlock {
	blks := uq.blocks
	if len(uq.blocks) > 0 {
		uq.blocks = []*peerBlock{}
		uq.hashes = map[hash.Hash256]struct{}{}
	}
	return blks
}
