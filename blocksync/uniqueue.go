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
}

func newUniQueue() *uniQueue {
	return &uniQueue{
		blocks:  []*peerBlock{},
		hashes:  map[hash.Hash256]struct{}{},
		perPeer: map[string]int{},
	}
}

// enque adds blk and reports whether it was added. It rejects duplicates and a
// peer that already holds its per-peer share. When the height is otherwise full,
// a block from an under-represented peer evicts one from the most-represented
// peer instead of being dropped: blocks are buffered before validation, so a set
// of peers flooding unvalidated candidates must never be able to crowd out the
// authentic block relayed by another peer (which would otherwise be lost and the
// height stall until a later sync round). Memory stays bounded by
// maxBlocksPerHeight; only which blocks occupy the slots changes.
func (uq *uniQueue) enque(blk *peerBlock) bool {
	h := blk.block.HashBlock()
	if _, ok := uq.hashes[h]; ok {
		return false
	}
	if uq.perPeer[blk.pid] >= maxBlocksPerPeerPerHeight {
		return false
	}
	if len(uq.blocks) >= maxBlocksPerHeight && !uq.evictFrom(blk.pid) {
		return false
	}
	uq.hashes[h] = struct{}{}
	uq.perPeer[blk.pid]++
	uq.blocks = append(uq.blocks, blk)
	return true
}

// evictFrom makes room for a block delivered by incoming by dropping the oldest
// block of the most-represented peer, but only when that peer holds strictly
// more slots than incoming would. It reports whether a slot was freed. This
// guarantees a block from a new or under-represented peer can always displace an
// over-represented one, so no coalition of peers can permanently exclude an
// honest block while keeping the per-height memory bound.
func (uq *uniQueue) evictFrom(incoming string) bool {
	victim := ""
	for pid, n := range uq.perPeer {
		if n > uq.perPeer[incoming] && (victim == "" || n > uq.perPeer[victim]) {
			victim = pid
		}
	}
	if victim == "" {
		return false
	}
	for i, b := range uq.blocks {
		if b.pid == victim {
			delete(uq.hashes, b.block.HashBlock())
			uq.perPeer[victim]--
			if uq.perPeer[victim] == 0 {
				delete(uq.perPeer, victim)
			}
			uq.blocks = append(uq.blocks[:i], uq.blocks[i+1:]...)
			return true
		}
	}
	return false
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
