// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// BucketIndex is an alias of proto definition
	BucketIndex struct {
		stakingpb.BucketIndex
	}

	// BucketIndices is an alias of proto definition
	BucketIndices struct {
		stakingpb.BucketIndices
	}
)

// NewBucketIndex creates a new bucket index
func NewBucketIndex(index uint64, candName CandName) *BucketIndex {
	return &BucketIndex{
		stakingpb.BucketIndex{
			Index:   index,
			CanName: candName[:],
		},
	}
}

// NewBucketIndices creates a new bucket indices
func NewBucketIndices() *BucketIndices {
	return &BucketIndices{
		stakingpb.BucketIndices{
			Indices: make([]*stakingpb.BucketIndex, 0),
		},
	}
}

// Deserialize deserializes bytes into bucket index
func (bi *BucketIndex) Deserialize(data []byte) error {
	return proto.Unmarshal(data, bi)
}

// Serialize serializes bucket index into bytes
func (bi *BucketIndex) Serialize() ([]byte, error) {
	return proto.Marshal(bi)
}

// Deserialize deserializes bytes into bucket indices
func (bis *BucketIndices) Deserialize(data []byte) error {
	return proto.Unmarshal(data, bis)
}

// Serialize serializes bucket indices into bytes
func (bis *BucketIndices) Serialize() ([]byte, error) {
	return proto.Marshal(bis)
}

func (bis *BucketIndices) addBucketIndex(bucketIndex *BucketIndex) {
	bis.Indices = append(bis.Indices, &stakingpb.BucketIndex{
		Index:   bucketIndex.Index,
		CanName: bucketIndex.CanName,
	})
}

func (bis *BucketIndices) deleteBucketIndex(index uint64) {
	for i, bucketIndex := range bis.Indices {
		if bucketIndex.Index == index {
			bis.Indices = append(bis.Indices[:i], bis.Indices[i+1:]...)
			break
		}
	}
}

func stakingGetBucketIndices(sr protocol.StateReader, voterAddr address.Address) (*BucketIndices, error) {
	var bis BucketIndices
	if _, err := sr.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(voterAddr.Bytes())); err != nil {
		return nil, err
	}
	return &bis, nil
}

func stakingPutBucketIndex(sm protocol.StateManager, voterAddr address.Address, bucketIndex *BucketIndex) error {
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

func stakingDelBucketIndex(sm protocol.StateManager, voterAddr address.Address, index uint64) error {
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
	if len(bis.GetIndices()) == 0 {
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
