// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type (
	// ContractStakingBucketType defines the type of contract staking bucket
	ContractStakingBucketType struct {
		Amount      *big.Int
		Duration    uint64 // block numbers
		ActivatedAt uint64 // block height
	}
)

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

// Clone clones the bucket type
func (bt *ContractStakingBucketType) Clone() *ContractStakingBucketType {
	return &ContractStakingBucketType{
		Amount:      big.NewInt(0).Set(bt.Amount),
		Duration:    bt.Duration,
		ActivatedAt: bt.ActivatedAt,
	}
}

func (bt *ContractStakingBucketType) toProto() *stakingpb.BucketType {
	return &stakingpb.BucketType{
		Amount:      bt.Amount.String(),
		Duration:    bt.Duration,
		ActivatedAt: bt.ActivatedAt,
	}
}

func (bt *ContractStakingBucketType) loadProto(p *stakingpb.BucketType) error {
	amount, ok := big.NewInt(0).SetString(p.Amount, 10)
	if !ok {
		return errors.New("failed to parse amount")
	}
	bt.Amount = amount
	bt.Duration = p.Duration
	bt.ActivatedAt = p.ActivatedAt
	return nil
}
