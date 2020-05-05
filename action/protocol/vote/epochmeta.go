// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package vote

import (
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol/vote/epochmetapb"
)

type EpochMeta struct {
	EpochNumber uint64
	BlockMetas []*BlockMeta
}

func NewEpochMeta(epochNumber uint64) *EpochMeta {
	return &EpochMeta{
		EpochNumber: epochNumber,
		BlockMetas: make([]*BlockMeta, 0),
	}
}

func (em *EpochMeta) AddBlockMeta(height uint64, producer string, mintTime time.Time) {
	elem := &BlockMeta{
		Height: height,
		Producer: producer,
		MintTime: mintTime.UTC(),
	}
	em.BlockMetas = append(em.BlockMetas, elem)
	return
}

// Serialize serializes EpochMeta struct to bytes
func (em *EpochMeta) Serialize() ([]byte, error) {
	pb, err := em.Proto()
	if err != nil {
		return nil, err
	} 
	return proto.Marshal(pb)
}

// Proto converts the EpochMeta struct to a protobuf message
func (em *EpochMeta) Proto() (*epochmetapb.EpochMeta, error) {
	blockMetasPb := make([]*epochmetapb.BlockMeta, 0, len(em.BlockMetas))
	for _, bm := range em.BlockMetas {
		bmProto, err := bm.Proto()
		if err != nil {
			return nil, err
		}
		blockMetasPb = append(blockMetasPb, bmProto)
	}
	return &epochmetapb.EpochMeta{
		BlockMetas:    blockMetasPb,
		EpochNumber:   em.EpochNumber,
	}, nil
}

// Deserialize deserializes bytes to epochMeta 
func (em *EpochMeta) Deserialize(buf []byte) error {
	epochMetapb := &epochmetapb.EpochMeta{}
	if err := proto.Unmarshal(buf, epochMetapb); err != nil {
		return errors.Wrap(err, "failed to unmarshal blacklist")
	}
	return em.LoadProto(epochMetapb)
}

// LoadProto loads epochMeta from proto
func (em *EpochMeta) LoadProto(epochMetapb *epochmetapb.EpochMeta) error {
	blockMetas := make([]*BlockMeta, 0)
	blockMetasPb := epochMetapb.BlockMetas
	for _, elemPb := range blockMetasPb {
		bm := &BlockMeta{}
		if err := bm.LoadProto(elemPb); err != nil {
			return err
		}
		blockMetas = append(blockMetas, bm)
	}
	em.EpochNumber = epochMetapb.EpochNumber
	em.BlockMetas = blockMetas

	return nil
}

type BlockMeta struct {
	Height 		uint64
	Producer 	string 	
	MintTime 	time.Time
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
func (bm *BlockMeta) Proto() (*epochmetapb.BlockMeta, error) {
	blkTime, err := ptypes.TimestampProto(bm.MintTime)
	if err != nil {
		return nil, err
	}
	return &epochmetapb.BlockMeta{
		BlockHeight:	bm.Height,
		BlockProducer:	bm.Producer,
		BlockTime:		blkTime,
	}, nil
}

// Deserialize deserializes bytes to blockMeta 
func (bm *BlockMeta) Deserialize(buf []byte) error {
	epochMetapb := &epochmetapb.BlockMeta{}
	if err := proto.Unmarshal(buf, epochMetapb); err != nil {
		return errors.Wrap(err, "failed to unmarshal blacklist")
	}
	return bm.LoadProto(epochMetapb)
}

// LoadProto loads blockMeta from proto
func (bm *BlockMeta) LoadProto(pb *epochmetapb.BlockMeta) error {
	mintTime, err := ptypes.Timestamp(pb.GetBlockTime())
	if err != nil {
		return err
	}
	bm.Height = pb.GetBlockHeight()
	bm.Producer = pb.GetBlockProducer()
	bm.MintTime = mintTime.UTC()
	return nil
}