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

func (csr *candSR) getBucketIndices(key []byte) (*BucketIndices, error) {
	var bis BucketIndices
	if _, err := csr.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key)); err != nil {
		return nil, err
	}
	return &bis, nil
}

func (csm *candSM) putBucketIndex(key []byte, index uint64) error {
	var bis BucketIndices
	if _, err := csm.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	bis.addBucketIndex(index)
	_, err := csm.PutState(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key))
	return err
}

func (csm *candSM) delBucketIndex(key []byte, index uint64) error {
	var bis BucketIndices
	if _, err := csm.State(
		&bis,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(key)); err != nil {
		return err
	}
	bis.deleteBucketIndex(index)

	var err error
	if len(bis) == 0 {
		_, err = csm.DelState(
			protocol.NamespaceOption(StakingNameSpace),
			protocol.KeyOption(key))
	} else {
		_, err = csm.PutState(
			&bis,
			protocol.NamespaceOption(StakingNameSpace),
			protocol.KeyOption(key))
	}
	return err
}

func (csr *candSR) getVoterBucketIndices(addr address.Address) (*BucketIndices, error) {
	return csr.getBucketIndices(addrKeyWithPrefix(addr, _voterIndex))
}

func (csm *candSM) putVoterBucketIndex(addr address.Address, index uint64) error {
	return csm.putBucketIndex(addrKeyWithPrefix(addr, _voterIndex), index)
}

func (csm *candSM) delVoterBucketIndex(addr address.Address, index uint64) error {
	return csm.delBucketIndex(addrKeyWithPrefix(addr, _voterIndex), index)
}

func (csr *candSR) getCandBucketIndices(addr address.Address) (*BucketIndices, error) {
	return csr.getBucketIndices(addrKeyWithPrefix(addr, _candIndex))
}

func (csm *candSM) putCandBucketIndex(addr address.Address, index uint64) error {
	return csm.putBucketIndex(addrKeyWithPrefix(addr, _candIndex), index)
}

func (csm *candSM) delCandBucketIndex(addr address.Address, index uint64) error {
	return csm.delBucketIndex(addrKeyWithPrefix(addr, _candIndex), index)
}

func addrKeyWithPrefix(addr address.Address, prefix byte) []byte {
	k := addr.Bytes()
	key := make([]byte, len(k)+1)
	key[0] = prefix
	copy(key[1:], k)
	return key
}
