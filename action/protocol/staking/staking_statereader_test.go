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
	"golang.org/x/exp/slices"

	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
)

func TestStakingStateReader(t *testing.T) {
	r := require.New(t)
	contractAddress := "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he"
	testCandidates := CandidateList{
		{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(1),
			Reward:             identityset.Address(1),
			Name:               "cand1",
			Votes:              big.NewInt(10),
			SelfStakeBucketIdx: 0,
			SelfStake:          big.NewInt(10),
		},
		{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(2),
			Name:               "cand2",
			Votes:              big.NewInt(0),
			SelfStakeBucketIdx: 0,
			SelfStake:          big.NewInt(0),
		},
	}
	testLiquidBuckets := []*VoteBucket{
		{
			Index:           1,
			Candidate:       identityset.Address(1),
			Owner:           identityset.Address(1),
			StakedAmount:    big.NewInt(100),
			StakedDuration:  time.Hour * 24,
			CreateTime:      time.Now(),
			StakeStartTime:  time.Now(),
			AutoStake:       true,
			ContractAddress: contractAddress,
		},
		{
			Index:           2,
			Candidate:       identityset.Address(2),
			Owner:           identityset.Address(2),
			StakedAmount:    big.NewInt(100),
			StakedDuration:  time.Hour * 24,
			CreateTime:      time.Now(),
			StakeStartTime:  time.Now(),
			AutoStake:       true,
			ContractAddress: contractAddress,
		},
	}
	testNativeBuckets := []*VoteBucket{
		{
			Index:          1,
			Candidate:      identityset.Address(1),
			Owner:          identityset.Address(1),
			StakedAmount:   big.NewInt(10),
			StakedDuration: time.Hour * 24,
			CreateTime:     time.Now(),
			StakeStartTime: time.Now(),
			AutoStake:      true,
		},
	}
	testNativeTotalAmount := &totalAmount{
		amount: big.NewInt(0),
	}
	for _, b := range testNativeBuckets {
		testNativeTotalAmount.amount.Add(testNativeTotalAmount.amount, b.StakedAmount)
		testNativeTotalAmount.count++
	}
	var err error
	states := make([][]byte, len(testNativeBuckets))
	for i := range states {
		states[i], err = state.Serialize(testNativeBuckets[i])
		r.NoError(err)
	}
	prepare := func(t *testing.T) (*mock_factory.MockFactory, *MockLiquidStakingIndexer, ReadState, context.Context, *require.Assertions) {
		r := require.New(t)
		ctrl := gomock.NewController(t)
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
		states := make([][]byte, len(testNativeBuckets))
		for i := range states {
			states[i], err = state.Serialize(testNativeBuckets[i])
			r.NoError(err)
		}
		sf.EXPECT().ReadView(gomock.Any()).Return(testNativeData, nil).Times(1)

		liquidIndexer := NewMockLiquidStakingIndexer(ctrl)
		liquidIndexer.EXPECT().Buckets().Return(testLiquidBuckets, nil).AnyTimes()

		stakeSR, err := newCompositeStakingStateReader(liquidIndexer, nil, sf)
		r.NoError(err)
		r.NotNil(stakeSR)

		reg := protocol.NewRegistry()
		rolldposProto := rolldpos.NewProtocol(10, 10, 10)
		rolldposProto.Register(reg)
		ctx := context.Background()
		ctx = protocol.WithRegistry(ctx, reg)

		return sf, liquidIndexer, stakeSR, ctx, r
	}
	addLiquidVotes := func(expectCand *Candidate) {
		for _, b := range testLiquidBuckets {
			if b.Candidate.String() == expectCand.Owner.String() {
				expectCand.Votes.Add(expectCand.Votes, b.StakedAmount)
				break
			}
		}
	}
	t.Run("readStateBuckets", func(t *testing.T) {
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().States(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 ...protocol.StateOption) (uint64, state.Iterator, error) {
			iter := state.NewIterator(states)
			return uint64(1), iter, nil
		}).Times(1)

		req := &iotexapi.ReadStakingDataRequest_VoteBuckets{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
		}
		buckets, height, err := stakeSR.readStateBuckets(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, len(testNativeBuckets)+len(testLiquidBuckets))
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
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&BucketIndices{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*BucketIndices)
			*arg0R = []uint64{0}
			return uint64(1), nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&VoteBucket{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*VoteBucket)
			cfg := &protocol.StateConfig{}
			if err := arg1[1](cfg); err != nil {
				return uint64(0), err
			}
			idx := byteutil.BytesToUint64BigEndian(cfg.Key[1:])
			*arg0R = *testNativeBuckets[idx]
			return uint64(1), nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalBucketCount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalBucketCount)
			*arg0R = totalBucketCount{count: 1}
			return uint64(1), nil
		}).Times(1)

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
		r.Len(buckets.Buckets, 2)
		iotexBucket, err := testNativeBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[0])
		iotexBucket, err = testLiquidBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[1])
	})
	t.Run("readStateBucketsByCandidate", func(t *testing.T) {
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&VoteBucket{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*VoteBucket)
			cfg := &protocol.StateConfig{}
			if err := arg1[1](cfg); err != nil {
				return uint64(0), err
			}
			idx := byteutil.BytesToUint64BigEndian(cfg.Key[1:])
			*arg0R = *testNativeBuckets[idx]
			return uint64(1), nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&BucketIndices{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*BucketIndices)
			*arg0R = []uint64{0}
			return uint64(1), nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalBucketCount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalBucketCount)
			*arg0R = totalBucketCount{count: 1}
			return uint64(1), nil
		}).Times(1)
		req := &iotexapi.ReadStakingDataRequest_VoteBucketsByCandidate{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
			CandName: "cand1",
		}
		buckets, height, err := stakeSR.readStateBucketsByCandidate(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, 2)
		iotexBucket, err := testNativeBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[0])
		iotexBucket, err = testLiquidBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[1])
	})
	t.Run("readStateBucketByIndices", func(t *testing.T) {
		sf, liquidIndexer, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&VoteBucket{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*VoteBucket)
			cfg := &protocol.StateConfig{}
			if err := arg1[1](cfg); err != nil {
				return uint64(0), err
			}
			idx := byteutil.BytesToUint64BigEndian(cfg.Key[1:])
			*arg0R = *testNativeBuckets[idx]
			return uint64(1), nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalBucketCount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalBucketCount)
			*arg0R = totalBucketCount{count: 1}
			return uint64(1), nil
		}).Times(1)
		liquidIndexer.EXPECT().BucketsByIndices(gomock.Any()).DoAndReturn(func(arg0 []uint64) ([]*VoteBucket, error) {
			buckets := []*VoteBucket{}
			for i := range arg0 {
				buckets = append(buckets, testLiquidBuckets[arg0[i]])
			}
			return buckets, nil
		}).Times(1)
		req := &iotexapi.ReadStakingDataRequest_VoteBucketsByIndexes{
			Index: []uint64{0},
		}
		buckets, height, err := stakeSR.readStateBucketByIndices(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, 2)
		iotexBucket, err := testNativeBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[0])
		iotexBucket, err = testLiquidBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[1])
	})
	t.Run("readStateBucketCount", func(t *testing.T) {
		sf, liquidIndexer, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalAmount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalAmount)
			*arg0R = *testNativeTotalAmount
			return uint64(1), nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalBucketCount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalBucketCount)
			*arg0R = totalBucketCount{count: 1}
			return uint64(1), nil
		}).Times(1)
		liquidIndexer.EXPECT().TotalBucketCount().Return(uint64(len(testLiquidBuckets))).Times(1)
		cfg := genesis.Default
		cfg.GreenlandBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, cfg)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		req := &iotexapi.ReadStakingDataRequest_BucketsCount{}
		bucketCount, height, err := stakeSR.readStateBucketCount(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.EqualValues(3, bucketCount.Total)
		r.EqualValues(3, bucketCount.Active)
	})
	t.Run("readStateCandidates", func(t *testing.T) {
		_, liquidIndexer, stakeSR, ctx, r := prepare(t)
		liquidIndexer.EXPECT().CandidateVotes(gomock.Any()).DoAndReturn(func(ownerAddr string) *big.Int {
			for _, b := range testLiquidBuckets {
				if b.Owner.String() == ownerAddr {
					return b.StakedAmount
				}
			}
			return big.NewInt(0)
		}).MinTimes(1)
		req := &iotexapi.ReadStakingDataRequest_Candidates{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
		}
		candidates, height, err := stakeSR.readStateCandidates(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(candidates.Candidates, 2)
		for i := range candidates.Candidates {
			idx := slices.IndexFunc(testCandidates, func(c *Candidate) bool {
				return c.Owner.String() == candidates.Candidates[i].OwnerAddress
			})
			r.True(idx >= 0)
			expectCand := *testCandidates[idx]
			addLiquidVotes(&expectCand)
			r.EqualValues(expectCand.toIoTeXTypes(), candidates.Candidates[i])
		}
	})
	t.Run("readStateCandidateByName", func(t *testing.T) {
		_, liquidIndexer, stakeSR, ctx, r := prepare(t)
		liquidIndexer.EXPECT().CandidateVotes(gomock.Any()).DoAndReturn(func(ownerAddr string) *big.Int {
			for _, b := range testLiquidBuckets {
				if b.Owner.String() == ownerAddr {
					return b.StakedAmount
				}
			}
			return big.NewInt(0)
		}).MinTimes(1)
		req := &iotexapi.ReadStakingDataRequest_CandidateByName{
			CandName: "cand1",
		}
		candidate, _, err := stakeSR.readStateCandidateByName(ctx, req)
		r.NoError(err)
		idx := slices.IndexFunc(testCandidates, func(c *Candidate) bool {
			return c.Owner.String() == candidate.OwnerAddress
		})
		r.True(idx >= 0)
		expectCand := *testCandidates[idx]
		addLiquidVotes(&expectCand)
		r.EqualValues(expectCand.toIoTeXTypes(), candidate)
	})
	t.Run("readStateCandidateByAddress", func(t *testing.T) {
		_, liquidIndexer, stakeSR, ctx, r := prepare(t)
		liquidIndexer.EXPECT().CandidateVotes(gomock.Any()).DoAndReturn(func(ownerAddr string) *big.Int {
			for _, b := range testLiquidBuckets {
				if b.Owner.String() == ownerAddr {
					return b.StakedAmount
				}
			}
			return big.NewInt(0)
		}).MinTimes(1)
		req := &iotexapi.ReadStakingDataRequest_CandidateByAddress{
			OwnerAddr: identityset.Address(1).String(),
		}
		candidate, _, err := stakeSR.readStateCandidateByAddress(ctx, req)
		r.NoError(err)
		idx := slices.IndexFunc(testCandidates, func(c *Candidate) bool {
			return c.Owner.String() == candidate.OwnerAddress
		})
		r.True(idx >= 0)
		expectCand := *testCandidates[idx]
		addLiquidVotes(&expectCand)
		r.EqualValues(expectCand.toIoTeXTypes(), candidate)
	})
	t.Run("readStateTotalStakingAmount", func(t *testing.T) {
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalAmount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalAmount)
			*arg0R = *testNativeTotalAmount
			return uint64(1), nil
		}).Times(1)
		cfg := genesis.Default
		cfg.GreenlandBlockHeight = 0
		ctx = genesis.WithGenesisContext(ctx, cfg)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		req := &iotexapi.ReadStakingDataRequest_TotalStakingAmount{}
		total, height, err := stakeSR.readStateTotalStakingAmount(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.EqualValues("210", total.Balance)
	})
}
