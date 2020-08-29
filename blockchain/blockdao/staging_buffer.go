package blockdao

import (
	"github.com/golang/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	stagingBuffer struct {
		size   uint64
		buffer []*block.Store
	}
)

func newStagingBuffer(size uint64) *stagingBuffer {
	return &stagingBuffer{
		size:   size,
		buffer: make([]*block.Store, size),
	}
}

func newStagingBufferFromBytes(buf []byte) (*stagingBuffer, error) {
	pbInfos := &iotextypes.BlockStores{}
	if err := proto.Unmarshal(buf, pbInfos); err != nil {
		return nil, err
	}

	infos := []*block.Store{}
	for _, v := range pbInfos.BlockStores {
		info := &block.Store{}
		if err := info.FromProto(v); err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}

	return &stagingBuffer{
		size:   uint64(len(infos)),
		buffer: infos,
	}, nil
}

func (s *stagingBuffer) Get(pos uint64) (*block.Store, error) {
	if pos >= s.size {
		return nil, ErrNotSupported
	}
	return s.buffer[pos], nil
}

func (s *stagingBuffer) Put(pos uint64, blk *block.Store) (bool, error) {
	if pos >= s.size {
		return false, ErrNotSupported
	}
	s.buffer[pos] = blk
	return pos == s.size-1, nil
}

func (s *stagingBuffer) Serialize() ([]byte, error) {
	blkInfo := []*iotextypes.BlockStore{}
	for _, v := range s.buffer {
		blkInfo = append(blkInfo, v.ToProto())
	}
	allBlks := &iotextypes.BlockStores{
		BlockStores: blkInfo,
	}
	return proto.Marshal(allBlks)
}
