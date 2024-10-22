// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"github.com/iotexproject/iotex-address/address"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockindex/contractstaking/contractstakingpb"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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

// clone clones the bucket info
func (bi *bucketInfo) clone() *bucketInfo {
	delegate := bi.Delegate
	if delegate != nil {
		delegate, _ = address.FromBytes(delegate.Bytes())
	}
	owner := bi.Owner
	if owner != nil {
		owner, _ = address.FromBytes(owner.Bytes())
	}
	return &bucketInfo{
		TypeIndex:  bi.TypeIndex,
		CreatedAt:  bi.CreatedAt,
		UnlockedAt: bi.UnlockedAt,
		UnstakedAt: bi.UnstakedAt,
		Delegate:   delegate,
		Owner:      owner,
	}
}

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

func (bi *bucketInfo) loadProto(p *contractstakingpb.BucketInfo) error {
	delegate, err := address.FromString(p.Delegate)
	if err != nil {
		return err
	}
	owner, err := address.FromString(p.Owner)
	if err != nil {
		return err
	}
	bi.TypeIndex = p.TypeIndex
	bi.CreatedAt = p.CreatedAt
	bi.UnlockedAt = p.UnlockedAt
	bi.UnstakedAt = p.UnstakedAt
	bi.Delegate = delegate
	bi.Owner = owner
	return nil
}
