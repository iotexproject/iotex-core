// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type (
	// Executor is the struct of an executor
	Executor struct {
		Owner     address.Address
		Operator  address.Address
		Reward    address.Address
		Type      ExecutorType
		BucketIdx uint64
		Amount    *big.Int
	}
)

var (
	_ state.State = (*Executor)(nil)
)

// Serialize serializes executor into bytes
func (e *Executor) Serialize() ([]byte, error) {
	pb, err := e.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

// Deserialize deserializes bytes into executor
func (e *Executor) Deserialize(buf []byte) error {
	pb := &stakingpb.Executor{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal candidate")
	}
	return e.fromProto(pb)
}

func (e *Executor) toProto() (*stakingpb.Executor, error) {
	if e.Owner == nil || e.Operator == nil || e.Reward == nil ||
		e.Amount == nil {
		return nil, ErrMissingField
	}

	return &stakingpb.Executor{
		OwnerAddress:    e.Owner.String(),
		OperatorAddress: e.Operator.String(),
		RewardAddress:   e.Reward.String(),
		Type:            stakingpb.ExecutorType(e.Type),
		Amount:          e.Amount.String(),
		BucketIdx:       e.BucketIdx,
	}, nil
}

func (e *Executor) fromProto(pb *stakingpb.Executor) error {
	var err error
	e.Owner, err = address.FromString(pb.GetOwnerAddress())
	if err != nil {
		return err
	}

	e.Operator, err = address.FromString(pb.GetOperatorAddress())
	if err != nil {
		return err
	}

	e.Reward, err = address.FromString(pb.GetRewardAddress())
	if err != nil {
		return err
	}

	var ok bool
	e.Amount, ok = new(big.Int).SetString(pb.GetAmount(), 10)
	if !ok {
		return action.ErrInvalidAmount
	}

	e.BucketIdx = pb.GetBucketIdx()
	e.Type = ExecutorType(pb.GetType())
	return nil
}
