// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	maxBlockNumber = math.MaxUint64
)

type (
	// BucketInfo is the bucket information
	BucketInfo struct {
		TypeIndex  uint64
		CreatedAt  uint64
		UnlockedAt uint64
		UnstakedAt uint64
		Delegate   address.Address // owner address of the delegate
		Owner      address.Address
	}

	// BucketType is the bucket type
	BucketType struct {
		Amount      *big.Int
		Duration    uint64
		ActivatedAt uint64
	}

	// Bucket is the bucket information including bucket type and bucket info
	Bucket struct {
		Index            uint64
		Candidate        address.Address
		Owner            address.Address
		StakedAmount     *big.Int
		StakedDuration   uint64
		CreateTime       uint64
		StakeStartTime   uint64
		UnstakeStartTime uint64
		AutoStake        bool
	}
)

func (bt *BucketType) toProto() *indexpb.BucketType {
	return &indexpb.BucketType{
		Amount:      bt.Amount.String(),
		Duration:    bt.Duration,
		ActivatedAt: bt.ActivatedAt,
	}
}

func (bt *BucketType) loadProto(p *indexpb.BucketType) error {
	var ok bool
	bt.Amount, ok = big.NewInt(0).SetString(p.Amount, 10)
	if !ok {
		return errors.New("failed to parse amount")
	}
	bt.Duration = p.Duration
	bt.ActivatedAt = p.ActivatedAt
	return nil
}

func (bt *BucketType) serialize() []byte {
	return byteutil.Must(proto.Marshal(bt.toProto()))
}

func (bt *BucketType) deserialize(b []byte) error {
	m := indexpb.BucketType{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bt.loadProto(&m)
}

func (bi *BucketInfo) toProto() *indexpb.BucketInfo {
	pb := &indexpb.BucketInfo{
		TypeIndex:  bi.TypeIndex,
		Delegate:   bi.Delegate.String(),
		CreatedAt:  bi.CreatedAt,
		Owner:      bi.Owner.String(),
		UnlockedAt: bi.UnlockedAt,
		UnstakedAt: bi.UnstakedAt,
	}
	return pb
}

func (bi *BucketInfo) serialize() []byte {
	return byteutil.Must(proto.Marshal(bi.toProto()))
}

func (bi *BucketInfo) deserialize(b []byte) error {
	m := indexpb.BucketInfo{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bi.loadProto(&m)
}

func (bi *BucketInfo) loadProto(p *indexpb.BucketInfo) error {
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
