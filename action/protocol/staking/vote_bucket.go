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
func NewVoteBucket(cand, owner address.Address, amount *big.Int, duration uint32, ctime time.Time, autoStake bool) *VoteBucket {
	return &VoteBucket{
		Candidate:        cand,
		Owner:            owner,
		StakedAmount:     amount,
		StakedDuration:   time.Duration(duration) * 24 * time.Hour,
		CreateTime:       ctime.UTC(),
		StakeStartTime:   ctime.UTC(),
		UnstakeStartTime: time.Unix(0, 0).UTC(),
		AutoStake:        autoStake,
	}
}

// Deserialize deserializes bytes into bucket
func (vb *VoteBucket) Deserialize(buf []byte) error {
	pb := &stakingpb.Bucket{}
	if err := proto.Unmarshal(buf, pb); err != nil {
		return errors.Wrap(err, "failed to unmarshal bucket")
	}

	return vb.fromProto(pb)
}

func (vb *VoteBucket) fromProto(pb *stakingpb.Bucket) error {
	vote, ok := big.NewInt(0).SetString(pb.GetStakedAmount(), 10)
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
	ownerAddr, err := address.FromString(pb.GetOwner())
	if err != nil {
		return err
	}

	createTime, err := ptypes.Timestamp(pb.GetCreateTime())
	if err != nil {
		return err
	}
	stakeTime, err := ptypes.Timestamp(pb.GetStakeStartTime())
	if err != nil {
		return err
	}
	unstakeTime, err := ptypes.Timestamp(pb.GetUnstakeStartTime())
	if err != nil {
		return err
	}

	vb.Candidate = candAddr
	vb.Owner = ownerAddr
	vb.StakedAmount = vote
	vb.StakedDuration = time.Duration(pb.GetStakedDuration()) * 24 * time.Hour
	vb.CreateTime = createTime
	vb.StakeStartTime = stakeTime
	vb.UnstakeStartTime = unstakeTime
	vb.AutoStake = pb.GetAutoStake()
	return nil
}

func (vb *VoteBucket) toProto() (*stakingpb.Bucket, error) {
	if vb.Candidate == nil || vb.Owner == nil || vb.StakedAmount == nil {
		return nil, ErrMissingField
	}
	createTime, err := ptypes.TimestampProto(vb.CreateTime)
	if err != nil {
		return nil, err
	}
	stakeTime, err := ptypes.TimestampProto(vb.StakeStartTime)
	if err != nil {
		return nil, err
	}
	unstakeTime, err := ptypes.TimestampProto(vb.UnstakeStartTime)
	if err != nil {
		return nil, err
	}

	return &stakingpb.Bucket{
		CandidateAddress: vb.Candidate.String(),
		Owner:            vb.Owner.String(),
		StakedAmount:     vb.StakedAmount.String(),
		StakedDuration:   uint32(vb.StakedDuration / 24 / time.Hour),
		CreateTime:       createTime,
		StakeStartTime:   stakeTime,
		UnstakeStartTime: unstakeTime,
		AutoStake:        vb.AutoStake,
	}, nil
}

// Serialize serializes bucket into bytes
func (vb *VoteBucket) Serialize() ([]byte, error) {
	pb, err := vb.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
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

func getTotalBucketCount(sr protocol.StateReader) (uint64, error) {
	var tc totalBucketCount
	_, err := sr.State(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey))
	return tc.count, err
}

func getBucket(sr protocol.StateReader, name address.Address, index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	if _, err := sr.State(
		&vb,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name.Bytes(), index))); err != nil {
		return nil, err
	}
	return &vb, nil
}

func putBucket(sm protocol.StateManager, name address.Address, bucket *VoteBucket) (uint64, error) {
	var tc totalBucketCount
	if _, err := sm.State(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey)); err != nil {
		return 0, err
	}

	index := tc.Count()
	if _, err := sm.PutState(
		bucket,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name.Bytes(), index))); err != nil {
		return 0, err
	}
	tc.count++
	_, err := sm.PutState(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey))
	return index, err
}

func delBucket(sm protocol.StateManager, name address.Address, index uint64) error {
	_, err := sm.DelState(
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name.Bytes(), index)))
	return err
}

func bucketKey(name []byte, index uint64) []byte {
	return append(name, byteutil.Uint64ToBytesBigEndian(index)...)
}
