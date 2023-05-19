// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"github.com/iotexproject/iotex-address/address"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol/staking/contractstaking/contractstakingpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// bucketInfo is the bucket information
	bucketInfo struct {
		TypeIndex  uint64
		CreatedAt  uint64
		UnlockedAt uint64
		UnstakedAt uint64
		Delegate   address.Address // owner address of the delegate
		Owner      address.Address
	}
)

func (bi *bucketInfo) toProto() *contractstakingpb.BucketInfo {
	pb := &contractstakingpb.BucketInfo{
		TypeIndex:  bi.TypeIndex,
		Delegate:   bi.Delegate.String(),
		CreatedAt:  bi.CreatedAt,
		Owner:      bi.Owner.String(),
		UnlockedAt: bi.UnlockedAt,
		UnstakedAt: bi.UnstakedAt,
	}
	return pb
}

// Serialize serializes the bucket info
func (bi *bucketInfo) Serialize() []byte {
	return byteutil.Must(proto.Marshal(bi.toProto()))
}

// Deserialize deserializes the bucket info
func (bi *bucketInfo) Deserialize(b []byte) error {
	m := contractstakingpb.BucketInfo{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bi.loadProto(&m)
}

func (bi *bucketInfo) loadProto(p *contractstakingpb.BucketInfo) error {
	var err error
	bi.TypeIndex = p.TypeIndex
	bi.CreatedAt = p.CreatedAt
	bi.UnlockedAt = p.UnlockedAt
	bi.UnstakedAt = p.UnstakedAt
	bi.Delegate, err = address.FromString(p.Delegate)
	if err != nil {
		return err
	}
	bi.Owner, err = address.FromString(p.Owner)
	if err != nil {
		return err
	}
	return nil
}
