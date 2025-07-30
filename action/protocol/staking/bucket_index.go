// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type (
	// BucketIndices defines the array of bucket index for a
	BucketIndices []uint64
)

var _ protocol.ContractStorage = (*BucketIndices)(nil)

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

func (bis BucketIndices) storageContractAddress(ns string, key []byte) (address.Address, error) {
	if ns != _stakingNameSpace {
		return nil, errors.Errorf("invalid namespace %s, expected %s", ns, _stakingNameSpace)
	}
	return systemcontracts.SystemContracts[systemcontracts.BucketIndicesContractIndex].Address, nil
}

func (bis BucketIndices) StoreToContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	addr, err := bis.storageContractAddress(ns, key)
	if err != nil {
		return err
	}
	contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(addr.Bytes()), backend)
	if err != nil {
		return errors.Wrapf(err, "failed to create bucket indices storage contract")
	}
	data, err := bis.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize bucket indices")
	}
	if err := contract.Put(key, systemcontracts.GenericValue{PrimaryData: data}); err != nil {
		return errors.Wrapf(err, "failed to put bucket indices to contract")
	}
	return nil
}

func (bis *BucketIndices) LoadFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	addr, err := bis.storageContractAddress(ns, key)
	if err != nil {
		return err
	}
	contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(addr.Bytes()), backend)
	if err != nil {
		return errors.Wrapf(err, "failed to create bucket indices storage contract")
	}
	value, err := contract.Get(key)
	if err != nil {
		return errors.Wrapf(err, "failed to get bucket indices from contract")
	}
	if !value.KeyExists {
		return errors.Wrapf(state.ErrStateNotExist, "bucket indices does not exist in contract")
	}
	return bis.Deserialize(value.Value.PrimaryData)
}

func (bis BucketIndices) DeleteFromContract(ns string, key []byte, backend systemcontracts.ContractBackend) error {
	addr, err := bis.storageContractAddress(ns, key)
	if err != nil {
		return err
	}
	contract, err := systemcontracts.NewGenericStorageContract(common.BytesToAddress(addr.Bytes()), backend)
	if err != nil {
		return errors.Wrapf(err, "failed to create bucket indices storage contract")
	}
	if err := contract.Remove(key); err != nil {
		return errors.Wrapf(err, "failed to delete bucket indices from contract")
	}
	return nil
}

func (bis BucketIndices) ListFromContract(_ string, _ systemcontracts.ContractBackend) ([][]byte, []any, error) {
	return nil, nil, errors.New("not implemented")
}

func (bis BucketIndices) BatchFromContract(ns string, keys [][]byte, backend systemcontracts.ContractBackend) ([]any, error) {
	return nil, errors.New("not implemented")
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

// AddrKeyWithPrefix returns address key with prefix
func AddrKeyWithPrefix(addr address.Address, prefix byte) []byte {
	k := addr.Bytes()
	key := make([]byte, len(k)+1)
	key[0] = prefix
	copy(key[1:], k)
	return key
}
