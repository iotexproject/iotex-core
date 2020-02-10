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
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// CandName is the 12-byte of candidate name
	CandName [12]byte

	// VoteBucket is an alias of proto definition
	VoteBucket struct {
		iotextypes.Bucket
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

func stakingGetBucket(sr protocol.StateReader, name CandName, index uint64) (*VoteBucket, error) {
	var vb VoteBucket
	key := bucketKey(name, index)
	if err := sr.State(key, &vb, protocol.NamespaceOption(factory.StakingNameSpace)); err != nil {
		return nil, err
	}
	return &vb, nil
}

func stakingPutBucket(sm protocol.StateManager, name CandName, index uint64, bucket *VoteBucket) error {
	key := bucketKey(name, index)
	return sm.PutState(key, bucket, protocol.NamespaceOption(factory.StakingNameSpace))
}

func stakingDelBucket(sm protocol.StateManager, name CandName, index uint64) error {
	key := bucketKey(name, index)
	return sm.DelState(key, protocol.NamespaceOption(factory.StakingNameSpace))
}

func bucketKey(name CandName, index uint64) hash.Hash160 {
	key := append(name[:], byteutil.Uint64ToBytesBigEndian(index)...)
	return hash.BytesToHash160(key)
}
