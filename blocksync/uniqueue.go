// Copyright (c) 2021 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"github.com/iotexproject/go-pkgs/hash"
)

// uniQueue is not threadsafe
type uniQueue struct {
	blocks []*peerBlock
	hashes map[hash.Hash256]struct{}
}

func newUniQueue() *uniQueue {
	return &uniQueue{
		blocks: []*peerBlock{},
		hashes: map[hash.Hash256]struct{},
	}
}

func (uq *uniQueue) enque(blk *peerBlock) {
	h := blk.block.HashBlock()
	if _, ok := uq.hashes[h]; ok {
		return
	}
	uq.hashes[h] = true
	uq.blocks = append(uq.blocks, blk)
}

func (uq *uniQueue) dequeAll() []*peerBlock {
	if len(uq.blocks) == 0 {
		return nil
	}
	blks := uq.blocks
	uq.blocks = []*peerBlock{}
	uq.hashes = map[hash.Hash256]struct{}

	return blks
}
