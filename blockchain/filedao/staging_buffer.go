// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	stagingBuffer struct {
		evmNetworkID uint32
		size         uint64
		buffer       []*block.Store
	}
)

func newStagingBuffer(size uint64, evmNetworkID uint32) *stagingBuffer {
	return &stagingBuffer{
		evmNetworkID: evmNetworkID,
		size:         size,
		buffer:       make([]*block.Store, size),
	}
}

func (s *stagingBuffer) Get(pos uint64) (*block.Store, error) {
	if pos >= s.size {
		return nil, ErrNotSupported
	}
	return s.buffer[pos], nil
}

func (s *stagingBuffer) Put(pos uint64, blkBytes []byte) (bool, error) {
	if pos >= s.size {
		return false, ErrNotSupported
	}
	deser := (&block.Deserializer{}).SetEvmNetworkID(s.evmNetworkID)
	blk, err := deser.DeserializeBlockStore(blkBytes)
	if err != nil {
		return false, err
	}
	s.buffer[pos] = blk
	return pos == s.size-1, nil
}

func (s *stagingBuffer) Serialize() ([]byte, error) {
	blkStores := []*iotextypes.BlockStore{}
	for _, v := range s.buffer {
		blkStores = append(blkStores, v.ToProto())
	}
	allBlks := &iotextypes.BlockStores{
		BlockStores: blkStores,
	}
	return proto.Marshal(allBlks)
}
