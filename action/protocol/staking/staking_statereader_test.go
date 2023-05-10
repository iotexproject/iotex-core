// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
)

func TestStakingStateReader(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	testCandidates := CandidateList{
		{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(1),
			Reward:             identityset.Address(1),
			Name:               "cand1",
			Votes:              big.NewInt(10),
			SelfStakeBucketIdx: 1,
			SelfStake:          big.NewInt(10),
		},
		{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(2),
			Name:               "cand2",
			Votes:              big.NewInt(10),
			SelfStakeBucketIdx: 1,
			SelfStake:          big.NewInt(10),
		},
	}
	testLiquidBuckets := []*VoteBucket{
		{
			Index:          1,
			Candidate:      identityset.Address(1),
			Owner:          identityset.Address(1),
			StakedAmount:   big.NewInt(100),
			StakedDuration: time.Hour * 24,
			CreateTime:     time.Now(),
			StakeStartTime: time.Now(),
			AutoStake:      true,
		},
		{
			Index:          2,
			Candidate:      identityset.Address(2),
			Owner:          identityset.Address(2),
			StakedAmount:   big.NewInt(100),
			StakedDuration: time.Hour * 24,
			CreateTime:     time.Now(),
			StakeStartTime: time.Now(),
			AutoStake:      true,
		},
	}
	testNativeBuckets := []*VoteBucket{
		{
			Index:          1,
			Candidate:      identityset.Address(1),
			Owner:          identityset.Address(1),
			StakedAmount:   big.NewInt(100),
			StakedDuration: time.Hour * 24,
			CreateTime:     time.Now(),
			StakeStartTime: time.Now(),
			AutoStake:      true,
		},
	}

	sf := mock_factory.NewMockFactory(ctrl)
	sf.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	candCenter, err := NewCandidateCenter(testCandidates)
	r.NoError(err)
	testNativeData := &ViewData{
		candCenter: candCenter,
		bucketPool: &BucketPool{
			total: &totalAmount{
				amount: big.NewInt(100),
				count:  1,
			},
		},
	}
	sf.EXPECT().ReadView(gomock.Any()).Return(testNativeData, nil).Times(1)
	states := make([][]byte, len(testNativeBuckets))
	for i := range states {
		states[i], err = state.Serialize(testNativeBuckets[i])
		r.NoError(err)
	}
	iter := state.NewIterator(states)
	sf.EXPECT().States(gomock.Any(), gomock.Any()).Return(uint64(1), iter, nil).Times(1)

	liquidIndexer := NewMockLiquidStakingIndexer(ctrl)
	liquidIndexer.EXPECT().Buckets().Return(testLiquidBuckets, nil).Times(1)

	stakeSR, err := newCompositeStakingStateReader(liquidIndexer, nil, sf)
	r.NoError(err)
	r.NotNil(stakeSR)

	reg := protocol.NewRegistry()
	rolldposProto := rolldpos.NewProtocol(10, 10, 10)
	rolldposProto.Register(reg)
	ctx := context.Background()
	ctx = protocol.WithRegistry(ctx, reg)

	t.Run("readStateBuckets", func(t *testing.T) {
		req := &iotexapi.ReadStakingDataRequest_VoteBuckets{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
		}
		buckets, height, err := stakeSR.readStateBuckets(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, 3)
		for i := range testNativeBuckets {
			iotexBucket, err := testNativeBuckets[i].toIoTeXTypes()
			r.NoError(err)
			r.Equal(iotexBucket, buckets.Buckets[i])
		}
		for i := range testLiquidBuckets {
			iotexBucket, err := testLiquidBuckets[i].toIoTeXTypes()
			r.NoError(err)
			r.Equal(iotexBucket, buckets.Buckets[i+len(testNativeBuckets)])
		}
	})

	t.Run("readStateBucketsByVoter", func(t *testing.T) {
		req := &iotexapi.ReadStakingDataRequest_VoteBucketsByVoter{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
			VoterAddress: identityset.Address(1).String(),
		}
		buckets, height, err := stakeSR.readStateBucketsByVoter(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, 3)
		for i := range testNativeBuckets {
			iotexBucket, err := testNativeBuckets[i].toIoTeXTypes()
			r.NoError(err)
			r.Equal(iotexBucket, buckets.Buckets[i])
		}
		for i := range testLiquidBuckets {
			iotexBucket, err := testLiquidBuckets[i].toIoTeXTypes()
			r.NoError(err)
			r.Equal(iotexBucket, buckets.Buckets[i+len(testNativeBuckets)])
		}
	})
	t.Run("readStateBucketsByCandidate", func(t *testing.T) {})
	t.Run("readStateBucketByIndices", func(t *testing.T) {})
	t.Run("readStateBucketCount", func(t *testing.T) {})
	t.Run("readStateCandidates", func(t *testing.T) {})
	t.Run("readStateCandidateByName", func(t *testing.T) {})
	t.Run("readStateCandidateByAddress", func(t *testing.T) {})
	t.Run("readStateTotalStakingAmount", func(t *testing.T) {})
}
