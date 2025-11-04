// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	memoryDao struct {
		mu                  sync.RWMutex
		tipHeight           uint64
		blocks              map[uint64]*block.Block
		hashesToBlockHeight map[hash.Hash256]uint64
		blockHashes         map[uint64]hash.Hash256
		cap                 uint64
	}
)

func newMemoryDao(cap uint64) *memoryDao {
	return &memoryDao{
		mu:                  sync.RWMutex{},
		blocks:              make(map[uint64]*block.Block, cap),
		hashesToBlockHeight: make(map[hash.Hash256]uint64, cap),
		blockHashes:         make(map[uint64]hash.Hash256, cap),
		cap:                 cap,
	}
}

func (md *memoryDao) TipHeight() uint64 {
	md.mu.RLock()
	defer md.mu.RUnlock()
	return md.tipHeight
}

func (md *memoryDao) PutBlock(blk *block.Block) error {
	md.mu.Lock()
	defer md.mu.Unlock()
	blkHeight := blk.Height()
	if blk.Height() <= md.tipHeight {
		return errors.Wrapf(ErrAlreadyExist, "block height %d, local height %d", blk.Height(), md.tipHeight)
	}
	if blk.Height() > md.tipHeight+1 {
		return errors.Errorf("block height %d is larger than local height %d + 1", blk.Height(), md.tipHeight)
	}
	blkHash := blk.HashBlock()
	if uint64(len(md.blocks)) >= md.cap && blkHeight > md.cap {
		toDelete, ok := md.blockHashes[blkHeight-md.cap]
		if ok {
			delete(md.blocks, blkHeight-md.cap)
			delete(md.blockHashes, blkHeight-md.cap)
			delete(md.hashesToBlockHeight, toDelete)
		}
	}
	md.blocks[blkHeight] = blk
	md.hashesToBlockHeight[blkHash] = blkHeight
	md.blockHashes[blkHeight] = blkHash
	md.tipHeight = blkHeight

	return nil
}

func (md *memoryDao) BlockByHeight(height uint64) (*block.Block, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	blk, ok := md.blocks[height]
	return blk, ok
}

func (md *memoryDao) HeaderByHeight(height uint64) (*block.Header, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	blk, ok := md.blocks[height]
	if !ok {
		return nil, false
	}
	return &blk.Header, true
}

func (md *memoryDao) FooterByHeight(height uint64) (*block.Footer, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	blk, ok := md.blocks[height]
	if !ok {
		return nil, false
	}
	return &blk.Footer, true
}

func (md *memoryDao) BlockHash(height uint64) (hash.Hash256, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	hash, ok := md.blockHashes[height]
	return hash, ok
}

func (md *memoryDao) BlockHeight(h hash.Hash256) (uint64, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	height, ok := md.hashesToBlockHeight[h]
	return height, ok
}

func (md *memoryDao) BlockByHash(h hash.Hash256) (*block.Block, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	height, ok := md.hashesToBlockHeight[h]
	if !ok {
		return nil, false
	}
	blk, ok := md.blocks[height]
	return blk, ok
}

func (md *memoryDao) HeaderByHash(h hash.Hash256) (*block.Header, bool) {
	md.mu.RLock()
	defer md.mu.RUnlock()
	height, ok := md.hashesToBlockHeight[h]
	if !ok {
		return nil, false
	}
	blk, ok := md.blocks[height]
	if !ok {
		return nil, false
	}
	return &blk.Header, true
}
