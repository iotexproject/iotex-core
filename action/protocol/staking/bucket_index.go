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
	// BucketIndices defines the array of bucket index for a
	BucketIndices []uint64
)

// Proto converts bucket indices to protobuf
func (bis *BucketIndices) Proto() *stakingpb.BucketIndices {
	bucketIndicesPb := make([]uint64, 0, len(*bis))
	for _, bi := range *bis {
		bucketIndicesPb = append(bucketIndicesPb, bi)
	}
	return &stakingpb.BucketIndices{Indices: bucketIndicesPb}
}

// LoadProto converts protobuf to bucket indices
func (bis *BucketIndices) LoadProto(bucketIndicesPb *stakingpb.BucketIndices) error {
	if bucketIndicesPb == nil {
		return errors.New("bucket indices protobuf cannot be nil")
	}
	*bis = bucketIndicesPb.Indices
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

func (bis *BucketIndices) addBucketIndex(index uint64) {
	*bis = append(*bis, index)
}

func (bis *BucketIndices) deleteBucketIndex(index uint64) {
	oldBis := *bis
	for i, bucketIndex := range oldBis {
		if bucketIndex == index {
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

func putBucketIndex(sm protocol.StateManager, voterAddr address.Address, bucketIndex uint64) error {
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
