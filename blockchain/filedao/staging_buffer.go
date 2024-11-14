// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"sync"

	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
)

type (
	stagingBuffer struct {
		lock   sync.RWMutex
		size   uint64
		start  uint64
		buffer []*block.Store
	}
)

func newStagingBuffer(size, start uint64) *stagingBuffer {
	return &stagingBuffer{
		size:   size,
		start:  start,
		buffer: make([]*block.Store, size),
	}
}

func (s *stagingBuffer) Get(height uint64) *block.Store {
	if height < s.start {
		return nil
	}
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.buffer[s.slot(height)]
}

func (s *stagingBuffer) Put(height uint64, blk *block.Store) (bool, error) {
	if height < s.start {
		return false, ErrNotSupported
	}
	pos := s.slot(height)
	s.lock.Lock()
	defer s.lock.Unlock()
	s.buffer[pos] = blk
	return pos == s.size-1, nil
}

func (s *stagingBuffer) slot(height uint64) uint64 {
	return (height - s.start) % s.size
}

func (s *stagingBuffer) Serialize() ([]byte, error) {
	blkStores := []*iotextypes.BlockStore{}
	// blob sidecar data are stored separately
	s.lock.RLock()
	for _, v := range s.buffer {
		blkStores = append(blkStores, v.ToProtoWithoutSidecar())
	}
	s.lock.RUnlock()
	allBlks := &iotextypes.BlockStores{
		BlockStores: blkStores,
	}
	return proto.Marshal(allBlks)
}
