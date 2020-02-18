// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
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

func stakingGetBucketIndices(sr protocol.StateReader, voterAddr string) (*BucketIndices, error) {
	addrHash, err := addrToHash(voterAddr)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get address hash from voter's address")
	}
	var bis BucketIndices
	if _, err := sr.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.LegacyKeyOption(addrHash)); err != nil {
		return nil, err
	}
	return &bis, nil
}

func stakingPutBucketIndex(sm protocol.StateManager, voterAddr string, bucketIndex *BucketIndex) error {
	addrHash, err := addrToHash(voterAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get address hash from voter's address")
	}
	var bis BucketIndices
	if _, err := sm.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.LegacyKeyOption(addrHash)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return err
	}
	bis.addBucketIndex(bucketIndex)
	_, err = sm.PutState(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.LegacyKeyOption(addrHash))
	return err
}

func stakingDelBucketIndex(sm protocol.StateManager, voterAddr string, index uint64) error {
	addrHash, err := addrToHash(voterAddr)
	if err != nil {
		return errors.Wrap(err, "failed to get address hash from voter's address")
	}
	var bis BucketIndices
	if _, err := sm.State(
		&bis,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.LegacyKeyOption(addrHash)); err != nil {
		return err
	}
	bis.deleteBucketIndex(index)
	if len(bis.GetIndices()) == 0 {
		_, err = sm.DelState(
			protocol.NamespaceOption(factory.StakingNameSpace),
			protocol.LegacyKeyOption(addrHash))
	} else {
		_, err = sm.PutState(
			&bis,
			protocol.NamespaceOption(factory.StakingNameSpace),
			protocol.LegacyKeyOption(addrHash))
	}
	return err
}

func addrToHash(encodedAddr string) (hash.Hash160, error) {
	addr, err := address.FromString(encodedAddr)
	if err != nil {
		return hash.Hash160{}, errors.Wrap(err, "failed to get address public key hash from encoded address")
	}
	return hash.BytesToHash160(addr.Bytes()), nil
}
