// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
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

// AddrKeyWithPrefix returns address key with prefix
func AddrKeyWithPrefix(addr address.Address, prefix byte) []byte {
	k := addr.Bytes()
	key := make([]byte, len(k)+1)
	key[0] = prefix
	copy(key[1:], k)
	return key
}
