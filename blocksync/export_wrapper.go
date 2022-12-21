// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

// Package blocksync export_wrapper.go export some private functions/types to integration test
// it's a temporary solution to solve these two problems without any modification of the test code logic
//  1. circular dependency between the config and api package
//  2. integration test moved out of package need to access package private functions/types
//
// it should be deprecated after integration test been refactored such as remove access to private things.
// non integration test code should never access thie file.
package blocksync

type (
	// BlockSyncerWrapper export blockSyncer
	BlockSyncerWrapper = blockSyncer
	// BlockBufferWrapper export blockBuffer
	BlockBufferWrapper struct {
		blockBuffer
	}
	// UniQueueWrapper export uniQueue
	UniQueueWrapper = uniQueue
)

var (
	// NewPeerBlockWrapper export newPeerBlock
	NewPeerBlockWrapper = newPeerBlock
)

// NewBlockBufferWrapper export blockBuffer
func NewBlockBufferWrapper(bufferSize uint64, intervalSize uint64) BlockBufferWrapper {
	return BlockBufferWrapper{
		blockBuffer: blockBuffer{
			blockQueues:  make(map[uint64]*uniQueue),
			bufferSize:   bufferSize,
			intervalSize: intervalSize,
		},
	}
}

// BlockQueuesLen export blockBuffer.blockQueues length
func (b *BlockBufferWrapper) BlockQueuesLen() int { return len(b.blockQueues) }

// SetIntervalSize export blockBuffer.intervalSize
func (b *BlockBufferWrapper) SetIntervalSize(i uint64) { b.intervalSize = i }
