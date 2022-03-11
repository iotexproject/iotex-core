// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action/protocol/poll/blockmetapb"
)

// BlockMeta is a struct to store block metadata
type BlockMeta struct {
	Height   uint64
	Producer string
	MintTime time.Time
}

// NewBlockMeta constructs new blockmeta struct with given fieldss
func NewBlockMeta(height uint64, producer string, mintTime time.Time) *BlockMeta {
	return &BlockMeta{
		Height:   height,
		Producer: producer,
		MintTime: mintTime.UTC(),
	}
}

// Serialize serializes BlockMeta struct to bytes
func (bm *BlockMeta) Serialize() ([]byte, error) {
	pb, err := bm.Proto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

// Proto converts the BlockMeta struct to a protobuf message
func (bm *BlockMeta) Proto() (*blockmetapb.BlockMeta, error) {
	blkTime := timestamppb.New(bm.MintTime)
	return &blockmetapb.BlockMeta{
		BlockHeight:   bm.Height,
		BlockProducer: bm.Producer,
		BlockTime:     blkTime,
	}, nil
}

// Deserialize deserializes bytes to blockMeta
func (bm *BlockMeta) Deserialize(buf []byte) error {
	epochMetapb := &blockmetapb.BlockMeta{}
	if err := proto.Unmarshal(buf, epochMetapb); err != nil {
		return errors.Wrap(err, "failed to unmarshal blocklist")
	}
	return bm.LoadProto(epochMetapb)
}

// LoadProto loads blockMeta from proto
func (bm *BlockMeta) LoadProto(pb *blockmetapb.BlockMeta) error {
	mintTime, err := ptypes.Timestamp(pb.GetBlockTime())
	if err != nil {
		return err
	}
	bm.Height = pb.GetBlockHeight()
	bm.Producer = pb.GetBlockProducer()
	bm.MintTime = mintTime.UTC()
	return nil
}
