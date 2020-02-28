// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// VoteBucket represents a vote
	VoteBucket struct {
		Candidate        address.Address
		Owner            address.Address
		StakedAmount     *big.Int
		StakedDuration   time.Duration
		CreateTime       time.Time
		StakeStartTime   time.Time
		UnstakeStartTime time.Time
		AutoStake        bool
	}

	// totalBucketCount stores the total bucket count
	totalBucketCount struct {
		count uint64
	}
)

// NewVoteBucket creates a new vote bucket
func NewVoteBucket(cand, owner, amount string, duration uint32, ctime time.Time, autoStake bool) (*VoteBucket, error) {
	vote, ok := big.NewInt(0).SetString(amount, 10)
	if !ok {
		return nil, ErrInvalidAmount
	}

	if vote.Sign() <= 0 {
		return nil, ErrInvalidAmount
	}

	candAddr, err := address.FromString(cand)
	if err != nil {
		return nil, err
	}

	ownerAddr, err := address.FromString(owner)
	if err != nil {
		return nil, err
	}

	bucket := VoteBucket{
		Candidate:        candAddr,
		Owner:            ownerAddr,
		StakedAmount:     vote,
		StakedDuration:   time.Duration(duration) * 24 * time.Hour,
		CreateTime:       ctime.UTC(),
		StakeStartTime:   ctime.UTC(),
		UnstakeStartTime: time.Unix(0, 0).UTC(),
		AutoStake:        autoStake,
	}
	return &bucket, nil
}

// Deserialize deserializes bytes into bucket
func (vb *VoteBucket) Deserialize(buf []byte) error {
	pb := &stakingpb.Bucket{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal bucket")
	}

	vote, ok := big.NewInt(0).SetString(pb.StakedAmount, 10)
	if !ok {
		return ErrInvalidAmount
	}

	if vote.Sign() <= 0 {
		return ErrInvalidAmount
	}

	candAddr, err := address.FromString(pb.GetCandidateAddress())
	if err != nil {
		return err
	}
	ownerAddr, err := address.FromString(pb.Owner)
	if err != nil {
		return err
	}

	createTime, err := ptypes.Timestamp(pb.CreateTime)
	if err != nil {
		return err
	}
	stakeTime, err := ptypes.Timestamp(pb.StakeStartTime)
	if err != nil {
		return err
	}
	unstakeTime, err := ptypes.Timestamp(pb.UnstakeStartTime)
	if err != nil {
		return err
	}

	vb.Candidate = candAddr
	vb.Owner = ownerAddr
	vb.StakedAmount = vote
	vb.StakedDuration = time.Duration(pb.StakedDuration) * 24 * time.Hour
	vb.CreateTime = createTime
	vb.StakeStartTime = stakeTime
	vb.UnstakeStartTime = unstakeTime
	vb.AutoStake = pb.AutoStake
	return nil
}

func (vb *VoteBucket) toProto() *stakingpb.Bucket {
	createTime, _ := ptypes.TimestampProto(vb.CreateTime)
	stakeTime, _ := ptypes.TimestampProto(vb.StakeStartTime)
	unstakeTime, _ := ptypes.TimestampProto(vb.UnstakeStartTime)

	return &stakingpb.Bucket{
		CandidateAddress: vb.Candidate.String(),
		Owner:            vb.Owner.String(),
		StakedAmount:     vb.StakedAmount.String(),
		StakedDuration:   uint32(vb.StakedDuration / 24 / time.Hour),
		CreateTime:       createTime,
		StakeStartTime:   stakeTime,
		UnstakeStartTime: unstakeTime,
		AutoStake:        vb.AutoStake,
	}
}

// Serialize serializes bucket into bytes
func (vb *VoteBucket) Serialize() ([]byte, error) {
	return proto.Marshal(vb.toProto())
}

// Deserialize deserializes bytes into bucket count
func (tc *totalBucketCount) Deserialize(data []byte) error {
	tc.count = byteutil.BytesToUint64BigEndian(data)
	return nil
}

// Serialize serializes bucket count into bytes
func (tc *totalBucketCount) Serialize() ([]byte, error) {
	data := byteutil.Uint64ToBytesBigEndian(tc.count)
	return data, nil
}

func (tc *totalBucketCount) Count() uint64 {
	return tc.count
}

func stakingGetTotalCount(sr protocol.StateReader) (uint64, error) {
	var tc totalBucketCount
	_, err := sr.State(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey))
	return tc.count, err
}

func stakingGetBucket(sr protocol.StateReader, name address.Address, index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	if _, err := sr.State(
		&vb,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name.Bytes(), index))); err != nil {
		return nil, err
	}
	return &vb, nil
}

func stakingPutBucket(sm protocol.StateManager, name address.Address, bucket *VoteBucket) error {
	var tc totalBucketCount
	if _, err := sm.State(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey)); err != nil {
		return err
	}

	if _, err := sm.PutState(
		bucket,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name.Bytes(), tc.Count()))); err != nil {
		return err
	}
	tc.count++
	_, err := sm.PutState(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey))
	return err
}

func stakingDelBucket(sm protocol.StateManager, name address.Address, index uint64) error {
	_, err := sm.DelState(
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name.Bytes(), index)))
	return err
}

func bucketKey(name []byte, index uint64) []byte {
	return append(name, byteutil.Uint64ToBytesBigEndian(index)...)
}
