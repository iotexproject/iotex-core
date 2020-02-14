// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

const (
	stateDBPath = "stateDB.test"
)

func TestBucket(t *testing.T) {
	require := require.New(t)

	vb, err := NewVoteBucket("", "d390*jk jh{}", "a2100000000", 21, time.Now(), true)
	require.Error(err)
	vb, err = NewVoteBucket("testname1234", "d390*jk jh{}", "a2100000000", 21, time.Now(), true)
	require.Equal(ErrInvalidAmount, errors.Cause(err))
	vb, err = NewVoteBucket("testname1234", "d390*jk jh{}", "-2100000000", 21, time.Now(), true)
	require.Equal(ErrInvalidAmount, errors.Cause(err))
	vb, err = NewVoteBucket("testname1234", "d390*jk jh{}", "2100000000", 21, time.Now(), true)
	require.Error(err)
	vb, err = NewVoteBucket("testname1234", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "2100000000", 21, time.Now(), true)
	require.NoError(err)

	data, err := vb.Serialize()
	require.NoError(err)
	vb1 := VoteBucket{}
	require.NoError(vb1.Deserialize(data))
	require.Equal(vb.CandidateName, vb1.CandidateName)
	require.Equal(vb.StakedAmount, vb1.StakedAmount)
	require.Equal(vb.StakedDuration, vb1.StakedDuration)
	require.Equal(vb.CreateTime.Seconds, vb1.CreateTime.Seconds)
	require.Equal(vb.CreateTime.Nanos, vb1.CreateTime.Nanos)
	require.Equal(vb.StakeStartTime.Seconds, vb1.StakeStartTime.Seconds)
	require.Equal(vb.StakeStartTime.Nanos, vb1.StakeStartTime.Nanos)
	require.Equal(vb.UnstakeStartTime.Seconds, vb1.UnstakeStartTime.Seconds)
	require.Equal(vb.UnstakeStartTime.Nanos, vb1.UnstakeStartTime.Nanos)
	require.Equal(vb.AutoStake, vb1.AutoStake)
	require.Equal(vb.Owner, vb1.Owner)
}

func createKey(opts ...protocol.StateOption) (hash.Hash256, error) {
	cfg, err := protocol.CreateStateConfig(opts...)
	if err != nil {
		return hash.ZeroHash256, err
	}
	return hash.Hash256b(append([]byte(cfg.Namespace), cfg.Key...)), nil
}

func newMockStateManager(ctrl *gomock.Controller) protocol.StateManager {
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	kv := map[hash.Hash256][]byte{}
	sm.EXPECT().State(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(s interface{}, opts ...protocol.StateOption) (uint64, error) {
			key, err := createKey(opts...)
			if err != nil {
				return 0, err
			}
			value, ok := kv[key]
			if !ok {
				return 0, state.ErrStateNotExist
			}
			ss, ok := s.(state.Deserializer)
			if !ok {
				return 0, errors.New("state is not a deserializer")
			}
			return 0, ss.Deserialize(value)
		},
	).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(s interface{}, opts ...protocol.StateOption) (uint64, error) {
			key, err := createKey(opts...)
			if err != nil {
				return 0, err
			}
			ss, ok := s.(state.Serializer)
			if !ok {
				return 0, errors.New("state is not a serializer")
			}
			value, err := ss.Serialize()
			if err != nil {
				return 0, err
			}
			kv[key] = value
			return 0, nil
		},
	).AnyTimes()
	sm.EXPECT().DelState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(opts ...protocol.StateOption) (uint64, error) {
			key, err := createKey(opts...)
			if err != nil {
				return 0, err
			}
			if _, ok := kv[key]; !ok {
				return 0, state.ErrStateNotExist
			}
			delete(kv, key)

			return 0, nil
		},
	).AnyTimes()

	return sm
}

func TestGetPutStaking(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)
	sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey),
	)
	tests := []struct {
		name  CandName
		index uint64
	}{
		{
			CandName{1, 2, 3, 4},
			0,
		},
		{
			CandName{1, 2, 3, 4},
			1,
		},
		{
			CandName{2, 3, 4, 5},
			2,
		},
		{
			CandName{2, 3, 4, 5},
			3,
		},
	}

	// put buckets and get
	for _, e := range tests {
		_, err := stakingGetBucket(sm, e.name, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		vb, err := NewVoteBucket("testnameof12", "io14s0vgnj0pjnazu4hsqlksdk7slah9vcfscn9ks", "2100000000", 21, time.Now(), true)
		require.NoError(err)

		count, err := stakingGetTotalCount(sm)
		require.NoError(err)
		require.Equal(e.index, count)
		require.NoError(stakingPutBucket(sm, e.name, vb))
		count, err = stakingGetTotalCount(sm)
		require.NoError(err)
		require.Equal(e.index+1, count)
		vb1, err := stakingGetBucket(sm, e.name, e.index)
		require.NoError(err)
		require.Equal(vb.CandidateName, vb1.CandidateName)
		require.Equal(vb.StakedAmount, vb1.StakedAmount)
		require.Equal(vb.StakedDuration, vb1.StakedDuration)
		require.Equal(vb.CreateTime.Seconds, vb1.CreateTime.Seconds)
		require.Equal(vb.CreateTime.Nanos, vb1.CreateTime.Nanos)
		require.Equal(vb.StakeStartTime.Seconds, vb1.StakeStartTime.Seconds)
		require.Equal(vb.StakeStartTime.Nanos, vb1.StakeStartTime.Nanos)
		require.Equal(vb.UnstakeStartTime.Seconds, vb1.UnstakeStartTime.Seconds)
		require.Equal(vb.UnstakeStartTime.Nanos, vb1.UnstakeStartTime.Nanos)
		require.Equal(vb.AutoStake, vb1.AutoStake)
		require.Equal(vb.Owner, vb1.Owner)
	}

	// delete buckets and get
	for _, e := range tests {
		require.NoError(stakingDelBucket(sm, e.name, e.index))
	}

	for _, e := range tests {
		_, err := stakingGetBucket(sm, e.name, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
