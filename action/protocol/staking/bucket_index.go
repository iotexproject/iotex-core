// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
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

func getBucketIndices(sr protocol.StateReader, key []byte) (*BucketIndices, uint64, error) {
	var bis BucketIndices
	height, err := sr.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key))
	if err != nil {
		return nil, height, err
	}
	return &bis, height, nil
}

func putBucketIndex(sm protocol.StateManager, key []byte, index uint64) error {
	var bis BucketIndices
	if _, err := sm.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	bis.addBucketIndex(index)
	_, err := sm.PutState(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key))
	return err
}

func delBucketIndex(sm protocol.StateManager, key []byte, index uint64) error {
	var bis BucketIndices
	if _, err := sm.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key)); err != nil {
		return err
	}
	bis.deleteBucketIndex(index)

	var err error
	if len(bis) == 0 {
		_, err = sm.DelState(
			protocol.NamespaceOption(StakingNameSpace),
			protocol.KeyOption(key))
	} else {
		_, err = sm.PutState(
			&bis,
			protocol.NamespaceOption(StakingNameSpace),
			protocol.KeyOption(key))
	}
	return err
}

func getVoterBucketIndices(sr protocol.StateReader, addr address.Address) (*BucketIndices, uint64, error) {
	return getBucketIndices(sr, addrKeyWithPrefix(addr, _voterIndex))
}

func putVoterBucketIndex(sm protocol.StateManager, addr address.Address, index uint64) error {
	return putBucketIndex(sm, addrKeyWithPrefix(addr, _voterIndex), index)
}

func delVoterBucketIndex(sm protocol.StateManager, addr address.Address, index uint64) error {
	return delBucketIndex(sm, addrKeyWithPrefix(addr, _voterIndex), index)
}

func getCandBucketIndices(sr protocol.StateReader, addr address.Address) (*BucketIndices, uint64, error) {
	return getBucketIndices(sr, addrKeyWithPrefix(addr, _candIndex))
}

func putCandBucketIndex(sm protocol.StateManager, addr address.Address, index uint64) error {
	return putBucketIndex(sm, addrKeyWithPrefix(addr, _candIndex), index)
}

func delCandBucketIndex(sm protocol.StateManager, addr address.Address, index uint64) error {
	return delBucketIndex(sm, addrKeyWithPrefix(addr, _candIndex), index)
}

func addrKeyWithPrefix(addr address.Address, prefix byte) []byte {
	k := addr.Bytes()
	key := make([]byte, len(k)+1)
	key[0] = prefix
	copy(key[1:], k)
	return key
}
