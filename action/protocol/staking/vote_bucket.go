// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	// VoteBucket represents a vote
	VoteBucket struct {
		Index            uint64
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
	vote, ok := new(big.Int).SetString(pb.GetStakedAmount(), 10)
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

	if err := pb.GetCreateTime().CheckValid(); err != nil {
		return err
	}
	createTime := pb.GetCreateTime().AsTime()
	if err := pb.GetStakeStartTime().CheckValid(); err != nil {
		return err
	}
	stakeTime := pb.GetStakeStartTime().AsTime()
	if err := pb.GetUnstakeStartTime().CheckValid(); err != nil {
		return err
	}
	unstakeTime := pb.GetUnstakeStartTime().AsTime()

	vb.Index = pb.GetIndex()
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
	createTime := timestamppb.New(vb.CreateTime)
	stakeTime := timestamppb.New(vb.StakeStartTime)
	unstakeTime := timestamppb.New(vb.UnstakeStartTime)

	return &stakingpb.Bucket{
		Index:            vb.Index,
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

func (vb *VoteBucket) toIoTeXTypes() (*iotextypes.VoteBucket, error) {
	createTime := timestamppb.New(vb.CreateTime)
	stakeTime := timestamppb.New(vb.StakeStartTime)
	unstakeTime := timestamppb.New(vb.UnstakeStartTime)

	return &iotextypes.VoteBucket{
		Index:            vb.Index,
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

func (vb *VoteBucket) isUnstaked() bool {
	return vb.UnstakeStartTime.After(vb.StakeStartTime)
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
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey))
	return tc.count, err
}

func getBucket(sr protocol.StateReader, index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	var err error
	if _, err = sr.State(
		&vb,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return nil, err
	}
	var tc totalBucketCount
	if _, err := sr.State(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return nil, err
	}
	if errors.Cause(err) == state.ErrStateNotExist && index < tc.Count() {
		return nil, ErrWithdrawnBucket
	}
	return &vb, nil
}

func updateBucket(sm protocol.StateManager, index uint64, bucket *VoteBucket) error {
	if _, err := getBucket(sm, index); err != nil {
		return err
	}

	_, err := sm.PutState(
		bucket,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index)))
	return err
}

func putBucket(sm protocol.StateManager, bucket *VoteBucket) (uint64, error) {
	var tc totalBucketCount
	if _, err := sm.State(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}

	index := tc.Count()
	// Add index inside bucket
	bucket.Index = index
	if _, err := sm.PutState(
		bucket,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index))); err != nil {
		return 0, err
	}
	tc.count++
	_, err := sm.PutState(
		&tc,
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey))
	return index, err
}

func delBucket(sm protocol.StateManager, index uint64) error {
	_, err := sm.DelState(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(bucketKey(index)))
	return err
}

func getAllBuckets(sr protocol.StateReader) ([]*VoteBucket, uint64, error) {
	height, iter, err := sr.States(
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeysOption(func() ([][]byte, error) {
			// TODO (zhi): fix potential racing issue
			count, err := getTotalBucketCount(sr)
			if err != nil {
				return nil, err
			}
			keys := [][]byte{}
			for i := uint64(0); i < count; i++ {
				keys = append(keys, bucketKey(i))
			}
			return keys, nil
		}),
	)
	if err != nil {
		return nil, height, err
	}

	buckets := make([]*VoteBucket, 0, iter.Size())
	for i := 0; i < iter.Size(); i++ {
		vb := &VoteBucket{}
		switch err := iter.Next(vb); errors.Cause(err) {
		case nil:
			buckets = append(buckets, vb)
		case state.ErrNilValue:
		default:
			return nil, height, errors.Wrapf(err, "failed to deserialize bucket")
		}
	}
	return buckets, height, nil
}

func getBucketsWithIndices(sr protocol.StateReader, indices BucketIndices) ([]*VoteBucket, error) {
	buckets := make([]*VoteBucket, 0, len(indices))
	for _, i := range indices {
		b, err := getBucket(sr, i)
		if err != nil && err != ErrWithdrawnBucket {
			return buckets, err
		}
		buckets = append(buckets, b)
	}
	return buckets, nil
}

func bucketKey(index uint64) []byte {
	key := []byte{_bucket}
	return append(key, byteutil.Uint64ToBytesBigEndian(index)...)
}

func calculateVoteWeight(c genesis.VoteWeightCalConsts, v *VoteBucket, selfStake bool) *big.Int {
	remainingTime := v.StakedDuration.Seconds()
	weight := float64(1)
	var m float64
	if v.AutoStake {
		m = c.AutoStake
	}
	if remainingTime > 0 {
		weight += math.Log(math.Ceil(remainingTime/86400)*(1+m)) / math.Log(c.DurationLg) / 100
	}
	if selfStake && v.AutoStake && v.StakedDuration >= time.Duration(91)*24*time.Hour {
		// self-stake extra bonus requires enable auto-stake for at least 3 months
		weight *= c.SelfStake
	}

	amount := new(big.Float).SetInt(v.StakedAmount)
	weightedAmount, _ := amount.Mul(amount, big.NewFloat(weight)).Int(nil)
	return weightedAmount
}
