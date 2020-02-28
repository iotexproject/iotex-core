// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// BucketIndex is an alias of proto definition
	BucketIndex struct {
		Index       uint64
		CandAddress address.Address
	}

	// BucketIndices is an alias of proto definition
	BucketIndices []*BucketIndex
)

// NewBucketIndex creates a new bucket index
func NewBucketIndex(index uint64, candAddress string) (*BucketIndex, error) {
	addr, err := address.FromString(candAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to derive address from string")
	}
	return &BucketIndex{
		Index:       index,
		CandAddress: addr,
	}, nil
}

// Proto converts bucket index to protobuf
func (bi *BucketIndex) Proto() *stakingpb.BucketIndex {
	return &stakingpb.BucketIndex{
		Index:       bi.Index,
		CandAddress: bi.CandAddress.String(),
	}
}

// LoadProto converts proto to bucket index
func (bi *BucketIndex) LoadProto(bucketIndexPb *stakingpb.BucketIndex) error {
	if bucketIndexPb == nil {
		return errors.New("bucket index protobuf cannot be nil")
	}
	addr, err := address.FromString(bucketIndexPb.CandAddress)
	if err != nil {
		return errors.Wrap(err, "failed to derive address from string")
	}
	*bi = BucketIndex{
		Index:       bucketIndexPb.Index,
		CandAddress: addr,
	}
	return nil
}

// Deserialize deserializes bytes into bucket index
func (bi *BucketIndex) Deserialize(data []byte) error {
	bucketIndexPb := &stakingpb.BucketIndex{}
	if err := proto.Unmarshal(data, bucketIndexPb); err != nil {
		return errors.Wrap(err, "failed to unmarshal bucket index")
	}
	return bi.LoadProto(bucketIndexPb)
}

// Serialize serializes bucket index into bytes
func (bi *BucketIndex) Serialize() ([]byte, error) {
	return proto.Marshal(bi.Proto())
}

// Proto converts bucket indices to protobuf
func (bis *BucketIndices) Proto() *stakingpb.BucketIndices {
	bucketIndicesPb := make([]*stakingpb.BucketIndex, 0, len(*bis))
	for _, bi := range *bis {
		bucketIndicesPb = append(bucketIndicesPb, bi.Proto())
	}
	return &stakingpb.BucketIndices{Indices: bucketIndicesPb}
}

// LoadProto converts protobuf to bucket indices
func (bis *BucketIndices) LoadProto(bucketIndicesPb *stakingpb.BucketIndices) error {
	bucketIndices := make(BucketIndices, 0)
	bisPb := bucketIndicesPb.Indices
	for _, biPb := range bisPb {
		bi := &BucketIndex{}
		if err := bi.LoadProto(biPb); err != nil {
			return errors.Wrap(err, "failed to load proto to bucket index")
		}
		bucketIndices = append(bucketIndices, bi)
	}
	*bis = bucketIndices
	return nil
}

// Deserialize deserializes bytes into bucket indices
func (bis *BucketIndices) Deserialize(data []byte) error {
	bucketIndicesPb := &stakingpb.BucketIndices{}
	if err := proto.Unmarshal(data, bucketIndicesPb); err != nil {
		return errors.Wrap(err, "failed to unmarshal bucket indices")
	}
	return bis.LoadProto(bucketIndicesPb)
}

// Serialize serializes bucket indices into bytes
func (bis *BucketIndices) Serialize() ([]byte, error) {
	return proto.Marshal(bis.Proto())
}

func (bis *BucketIndices) addBucketIndex(bucketIndex *BucketIndex) {
	*bis = append(*bis, bucketIndex)
}

func (bis *BucketIndices) deleteBucketIndex(index uint64) {
	oldBis := *bis
	for i, bucketIndex := range oldBis {
		if bucketIndex.Index == index {
			*bis = append(oldBis[:i], oldBis[i+1:]...)
			break
		}
	}
}

func getBucketIndices(sr protocol.StateReader, voterAddr address.Address) (*BucketIndices, error) {
	var bis BucketIndices
	if _, err := sr.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(voterAddr.Bytes())); err != nil {
		return nil, err
	}
	return &bis, nil
}

func putBucketIndex(sm protocol.StateManager, voterAddr address.Address, bucketIndex *BucketIndex) error {
	var bis BucketIndices
	if _, err := sm.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(voterAddr.Bytes())); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	bis.addBucketIndex(bucketIndex)
	_, err := sm.PutState(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(voterAddr.Bytes()))
	return err
}

func delBucketIndex(sm protocol.StateManager, voterAddr address.Address, index uint64) error {
	key := voterAddr.Bytes()
	var bis BucketIndices
	if _, err := sm.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(key)); err != nil {
		return err
	}
	bis.deleteBucketIndex(index)

	var err error
	if len(bis) == 0 {
		_, err = sm.DelState(
			protocol.NamespaceOption(factory.StakingNameSpace),
			protocol.KeyOption(key))
	} else {
		_, err = sm.PutState(
			&bis,
			protocol.NamespaceOption(factory.StakingNameSpace),
			protocol.KeyOption(key))
	}
	return err
}
