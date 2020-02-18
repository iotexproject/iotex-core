// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"errors"
	"math/big"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// VoteBucket is an alias of proto definition
	VoteBucket struct {
		iotextypes.Bucket
	}

	// totalBucketCount stores the total bucket count
	totalBucketCount struct {
		count uint64
	}
)

// NewVoteBucket creates a new vote bucket
func NewVoteBucket(name, owner, amount string, duration uint32, ctime time.Time, nonDecay bool) (*VoteBucket, error) {
	if len(name) == 0 {
		return nil, errors.New("empty candidate name")
	}

	if _, ok := big.NewInt(0).SetString(amount, 10); !ok {
		return nil, errors.New("failed to cast amount")
	}

	if _, err := address.FromString(owner); err != nil {
		return nil, err
	}

	createTime, err := ptypes.TimestampProto(ctime)
	if err != nil {
		return nil, err
	}
	unstakeTime, _ := ptypes.TimestampProto(time.Unix(0, 0))

	bucket := VoteBucket{
		iotextypes.Bucket{
			CandidateName:    name,
			StakedAmount:     amount,
			StakedDuration:   duration,
			CreateTime:       createTime,
			StakeStartTime:   createTime,
			UnstakeStartTime: unstakeTime,
			NonDecay:         nonDecay,
			Owner:            owner,
		},
	}
	return &bucket, nil
}

// Deserialize deserializes bytes into bucket
func (vb *VoteBucket) Deserialize(data []byte) error {
	return proto.Unmarshal(data, vb)
}

// Serialize serializes bucket into bytes
func (vb *VoteBucket) Serialize() ([]byte, error) {
	return proto.Marshal(vb)
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

func stakingGetBucket(sr protocol.StateReader, name CandName, index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	if _, err := sr.State(
		&vb,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name, index))); err != nil {
		return nil, err
	}
	return &vb, nil
}

func stakingPutBucket(sm protocol.StateManager, name CandName, bucket *VoteBucket) error {
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
		protocol.KeyOption(bucketKey(name, tc.Count()))); err != nil {
		return err
	}
	tc.count++
	_, err := sm.PutState(
		&tc,
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey))
	return err
}

func stakingDelBucket(sm protocol.StateManager, name CandName, index uint64) error {
	_, err := sm.DelState(
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(bucketKey(name, index)))
	return err
}

func bucketKey(name CandName, index uint64) []byte {
	return append(name[:], byteutil.Uint64ToBytesBigEndian(index)...)
}
