// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockindex/indexpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// BucketInfo is the bucket information
	BucketInfo struct {
		TypeIndex  uint64
		CreatedAt  time.Time
		UnlockedAt time.Time
		UnstakedAt time.Time
		Delegate   string
		Owner      string
	}

	// BucketType is the bucket type
	BucketType struct {
		Amount      *big.Int
		Duration    time.Duration
		ActivatedAt time.Time
	}

	// Bucket is the bucket information including bucket type and bucket info
	Bucket = staking.VoteBucket
)

func (bt *BucketType) toProto() *indexpb.BucketType {
	return &indexpb.BucketType{
		Amount:      bt.Amount.String(),
		Duration:    uint64(bt.Duration),
		ActivatedAt: timestamppb.New(bt.ActivatedAt),
	}
}

func (bt *BucketType) loadProto(p *indexpb.BucketType) error {
	var ok bool
	bt.Amount, ok = big.NewInt(0).SetString(p.Amount, 10)
	if !ok {
		return errors.New("failed to parse amount")
	}
	bt.Duration = time.Duration(p.Duration)
	bt.ActivatedAt = p.ActivatedAt.AsTime()
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
		TypeIndex: bi.TypeIndex,
		Delegate:  bi.Delegate,
		CreatedAt: timestamppb.New(bi.CreatedAt),
		Owner:     bi.Owner,
	}
	if !bi.UnlockedAt.IsZero() {
		pb.UnlockedAt = timestamppb.New(bi.UnlockedAt)
	}
	time.Unix(0, 0).UTC()
	if !bi.UnstakedAt.IsZero() {
		pb.UnstakedAt = timestamppb.New(bi.UnstakedAt)
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
	bi.TypeIndex = p.TypeIndex
	bi.CreatedAt = p.CreatedAt.AsTime()
	if p.UnlockedAt != nil {
		bi.UnlockedAt = p.UnlockedAt.AsTime()
	} else {
		bi.UnlockedAt = time.Time{}
	}
	if p.UnstakedAt != nil {
		bi.UnstakedAt = p.UnstakedAt.AsTime()
	} else {
		bi.UnstakedAt = time.Time{}
	}
	bi.Delegate = p.Delegate
	bi.Owner = p.Owner
	return nil
}
