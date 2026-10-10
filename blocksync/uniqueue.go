// Copyright (c) 2021 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"github.com/iotexproject/go-pkgs/hash"
)

const (
	// maxBlocksPerHeight bounds how many distinct blocks are buffered for a
	// single height. Blocks are buffered before any validation, and dedup is by
	// full block hash, so without a cap a peer can flood cheaply-mutated
	// variants of one height and grow the queue without bound while the tip is
	// stalled.
	//
	// Honest traffic needs very few: a height carries one block, plus a small
	// number of competing proposals across a fork.
	maxBlocksPerHeight = 8

	// maxBlocksPerPeerPerHeight bounds how many of those slots a single
	// (authenticated) peer may take. Without it one peer can fill the whole
	// height with unvalidated junk and crowd out the authentic block that
	// arrives later: commitBlocks would then reject every buffered candidate
	// and stall until the next sync round. An honest peer only ever relays one
	// block per height (duplicates dedup by hash), so 2 is already generous and
	// still leaves room for other peers' blocks.
	maxBlocksPerPeerPerHeight = 2
)

// uniQueue is not threadsafe
type uniQueue struct {
	blocks  []*peerBlock
	hashes  map[hash.Hash256]struct{}
	perPeer map[string]int
}

func newUniQueue() *uniQueue {
	return &uniQueue{
		blocks:  []*peerBlock{},
		hashes:  map[hash.Hash256]struct{}{},
		perPeer: map[string]int{},
	}
}

// enque adds blk unless it is a duplicate, the height is already full, or the
// delivering peer already holds its per-peer share of this height. It reports
// whether the block was added.
func (uq *uniQueue) enque(blk *peerBlock) bool {
	h := blk.block.HashBlock()
	if _, ok := uq.hashes[h]; ok {
		return false
	}
	if len(uq.blocks) >= maxBlocksPerHeight {
		return false
	}
	if uq.perPeer[blk.pid] >= maxBlocksPerPeerPerHeight {
		return false
	}
	uq.hashes[h] = struct{}{}
	uq.perPeer[blk.pid]++
	uq.blocks = append(uq.blocks, blk)
	return true
}

func (uq *uniQueue) dequeAll() []*peerBlock {
	blks := uq.blocks
	if len(uq.blocks) > 0 {
		uq.blocks = []*peerBlock{}
		uq.hashes = map[hash.Hash256]struct{}{}
		uq.perPeer = map[string]int{}
	}
	return blks
}
