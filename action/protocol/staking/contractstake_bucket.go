// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// ContractStakingBucketInfo is the bucket information
	ContractStakingBucketInfo struct {
		TypeIndex  uint64
		CreatedAt  uint64
		UnlockedAt uint64
		UnstakedAt uint64
		Delegate   address.Address // owner address of the delegate
		Owner      address.Address
	}

	// ContractStakingBucketType defines the type of contract staking bucket
	ContractStakingBucketType struct {
		Amount      *big.Int
		Duration    uint64 // block numbers
		ActivatedAt uint64 // block height
	}
)

func (bt *ContractStakingBucketType) toProto() *stakingpb.BucketType {
	return &stakingpb.BucketType{
		Amount:      bt.Amount.String(),
		Duration:    bt.Duration,
		ActivatedAt: bt.ActivatedAt,
	}
}

func (bt *ContractStakingBucketType) loadProto(p *stakingpb.BucketType) error {
	var ok bool
	bt.Amount, ok = big.NewInt(0).SetString(p.Amount, 10)
	if !ok {
		return errors.New("failed to parse amount")
	}
	bt.Duration = p.Duration
	bt.ActivatedAt = p.ActivatedAt
	return nil
}

// Serialize serializes the bucket type
func (bt *ContractStakingBucketType) Serialize() []byte {
	return byteutil.Must(proto.Marshal(bt.toProto()))
}

// Deserialize deserializes the bucket type
func (bt *ContractStakingBucketType) Deserialize(b []byte) error {
	m := stakingpb.BucketType{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bt.loadProto(&m)
}

func (bi *ContractStakingBucketInfo) toProto() *stakingpb.BucketInfo {
	pb := &stakingpb.BucketInfo{
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
func (bi *ContractStakingBucketInfo) Serialize() []byte {
	return byteutil.Must(proto.Marshal(bi.toProto()))
}

// Deserialize deserializes the bucket info
func (bi *ContractStakingBucketInfo) Deserialize(b []byte) error {
	m := stakingpb.BucketInfo{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	return bi.loadProto(&m)
}

func (bi *ContractStakingBucketInfo) loadProto(p *stakingpb.BucketInfo) error {
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
