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
	// (authenticated) peer may take, so no one peer can monopolise a height.
	// An honest peer only ever relays one block per height (duplicates dedup by
	// hash), so 2 is already generous.
	maxBlocksPerPeerPerHeight = 2
)

// uniQueue is not threadsafe
type uniQueue struct {
	blocks  []*peerBlock
	hashes  map[hash.Hash256]struct{}
	perPeer map[string]int
	// lossy is set once a candidate for this height was refused or evicted for
	// lack of room. Blocks are buffered unvalidated, so the authentic block may
	// be among those lost: a lossy height must not count as received, and if its
	// buffered candidates all fail to commit it has to be requested again.
	lossy bool
}

func newUniQueue() *uniQueue {
	return &uniQueue{
		blocks:  []*peerBlock{},
		hashes:  map[hash.Hash256]struct{}{},
		perPeer: map[string]int{},
	}
}

// enque adds blk and reports whether it was added. It rejects duplicates and a
// peer that already holds its per-peer share. When the height is full, the
// block may only displace a block held by a peer that is over its fair share
// (an honest peer relays one block per height, so a peer holding more than one
// slot is not entitled to the extra one); otherwise it is refused. Either way
// the height is marked lossy so the refused or evicted candidate can be
// recovered by re-requesting the height. A peer's single block is therefore
// never evicted, however many other peers flood the height, and memory stays
// bounded by maxBlocksPerHeight.
func (uq *uniQueue) enque(blk *peerBlock) bool {
	h := blk.block.HashBlock()
	if _, ok := uq.hashes[h]; ok {
		return false
	}
	if uq.perPeer[blk.pid] >= maxBlocksPerPeerPerHeight {
		return false
	}
	if len(uq.blocks) >= maxBlocksPerHeight {
		uq.lossy = true
		if !uq.evictOverShare() {
			return false
		}
	}
	uq.hashes[h] = struct{}{}
	uq.perPeer[blk.pid]++
	uq.blocks = append(uq.blocks, blk)
	return true
}

// evictOverShare drops the oldest block held by a peer that occupies more than
// one slot, and reports whether a slot was freed. The choice depends only on
// queue order, never on map iteration order.
func (uq *uniQueue) evictOverShare() bool {
	for i, b := range uq.blocks {
		if uq.perPeer[b.pid] > 1 {
			delete(uq.hashes, b.block.HashBlock())
			uq.perPeer[b.pid]--
			uq.blocks = append(uq.blocks[:i], uq.blocks[i+1:]...)
			return true
		}
	}
	return false
}

// dequeAll returns the buffered blocks and whether any candidate for the height
// was refused or evicted, and resets the queue.
func (uq *uniQueue) dequeAll() ([]*peerBlock, bool) {
	blks, lossy := uq.blocks, uq.lossy
	if len(uq.blocks) > 0 || uq.lossy {
		uq.blocks = []*peerBlock{}
		uq.hashes = map[hash.Hash256]struct{}{}
		uq.perPeer = map[string]int{}
		uq.lossy = false
	}
	return blks, lossy
}
