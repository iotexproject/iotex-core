// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"slices"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_factory"
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
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
			SelfStake:          big.NewInt(10),
		},
		{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(2),
			Name:               "cand2",
			Votes:              big.NewInt(0),
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
			SelfStake:          big.NewInt(0),
		},
	}
	testContractBuckets := []*VoteBucket{
		{
			Index:                   1,
			Candidate:               identityset.Address(1),
			Owner:                   identityset.Address(1),
			StakedAmount:            big.NewInt(100),
			StakedDuration:          time.Hour * 24,
			CreateTime:              time.Now(),
			StakeStartTime:          time.Now(),
			AutoStake:               true,
			ContractAddress:         contractAddress,
			UnstakeStartBlockHeight: maxBlockNumber,
		},
		{
			Index:                   2,
			Candidate:               identityset.Address(2),
			Owner:                   identityset.Address(2),
			StakedAmount:            big.NewInt(100),
			StakedDuration:          time.Hour * 24,
			CreateTime:              time.Now(),
			StakeStartTime:          time.Now(),
			AutoStake:               true,
			ContractAddress:         contractAddress,
			UnstakeStartBlockHeight: maxBlockNumber,
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
	keys := make([][]byte, len(testNativeBuckets))
	states := make([][]byte, len(testNativeBuckets))
	for i := range states {
		keys[i] = byteutil.Uint64ToBytesBigEndian(uint64(i))
		states[i], err = state.Serialize(testNativeBuckets[i])
		r.NoError(err)
	}
	prepare := func(t *testing.T) (*mock_factory.MockFactory, *MockContractStakingIndexerWithBucketType, ReadState, context.Context, *require.Assertions) {
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

		contractIndexer := NewMockContractStakingIndexerWithBucketType(ctrl)
		contractIndexer.EXPECT().Buckets(gomock.Any()).Return(testContractBuckets, nil).AnyTimes()
		contractIndexer.EXPECT().BucketsByCandidate(gomock.Any(), gomock.Any()).DoAndReturn(func(ownerAddr address.Address, height uint64) ([]*VoteBucket, error) {
			buckets := []*VoteBucket{}
			for i := range testContractBuckets {
				if testContractBuckets[i].Owner.String() == ownerAddr.String() {
					buckets = append(buckets, testContractBuckets[i])
				}
			}
			return buckets, nil
		}).AnyTimes()
		stakeSR, err := newCompositeStakingStateReader(nil, sf, func(v *VoteBucket, selfStake bool) *big.Int {
			return v.StakedAmount
		}, contractIndexer)
		r.NoError(err)
		r.NotNil(stakeSR)

		reg := protocol.NewRegistry()
		rolldposProto := rolldpos.NewProtocol(10, 10, 10)
		rolldposProto.Register(reg)
		g := genesis.TestDefault()
		g.QuebecBlockHeight = 1
		ctx := genesis.WithGenesisContext(context.Background(), g)
		ctx = protocol.WithRegistry(ctx, reg)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1000})
		ctx = protocol.WithFeatureCtx(ctx)

		return sf, contractIndexer, stakeSR, ctx, r
	}
	addContractVotes := func(expectCand *Candidate) {
		for _, b := range testContractBuckets {
			if b.Candidate.String() == expectCand.Owner.String() {
				expectCand.Votes.Add(expectCand.Votes, b.StakedAmount)
				break
			}
		}
	}
	t.Run("readStateBuckets", func(t *testing.T) {
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().States(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 ...protocol.StateOption) (uint64, state.Iterator, error) {
			iter, err := state.NewIterator(keys, states)
			r.NoError(err)
			return uint64(1), iter, nil
		}).Times(1)
		sf.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), state.ErrStateNotExist).Times(1)

		req := &iotexapi.ReadStakingDataRequest_VoteBuckets{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
		}
		buckets, height, err := stakeSR.readStateBuckets(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, len(testNativeBuckets)+len(testContractBuckets))
		for i := range testNativeBuckets {
			iotexBucket, err := testNativeBuckets[i].toIoTeXTypes()
			r.NoError(err)
			r.Equal(iotexBucket, buckets.Buckets[i])
		}
		for i := range testContractBuckets {
			iotexBucket, err := testContractBuckets[i].toIoTeXTypes()
			r.NoError(err)
			r.Equal(iotexBucket, buckets.Buckets[i+len(testNativeBuckets)])
		}
	})
	t.Run("readStateBucketsWithEndorsement", func(t *testing.T) {
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().States(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 ...protocol.StateOption) (uint64, state.Iterator, error) {
			iter, err := state.NewIterator(keys, states)
			r.NoError(err)
			return uint64(1), iter, nil
		}).Times(1)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&Endorsement{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*Endorsement)
			*arg0R = Endorsement{ExpireHeight: 100}
			return uint64(1), nil
		}).AnyTimes()

		req := &iotexapi.ReadStakingDataRequest_VoteBuckets{
			Pagination: &iotexapi.PaginationParam{
				Offset: 0,
				Limit:  100,
			},
		}
		buckets, height, err := stakeSR.readStateBuckets(ctx, req)
		r.NoError(err)
		r.EqualValues(1, height)
		r.Len(buckets.Buckets, len(testNativeBuckets)+len(testContractBuckets))
		iotexBuckets, err := toIoTeXTypesVoteBucketList(sf, testNativeBuckets)
		r.NoError(err)
		for i := range testNativeBuckets {
			r.Equal(iotexBuckets.Buckets[i], buckets.Buckets[i])
		}
		iotexBuckets, err = toIoTeXTypesVoteBucketList(sf, testContractBuckets)
		r.NoError(err)
		for i := range testContractBuckets {
			r.Equal(iotexBuckets.Buckets[i], buckets.Buckets[i+len(testNativeBuckets)])
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
		sf.EXPECT().State(gomock.AssignableToTypeOf(&Endorsement{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			return uint64(0), state.ErrStateNotExist
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
		iotexBucket, err = testContractBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[1])
	})
	t.Run("readStateBucketsByCandidate", func(t *testing.T) {
		sf, contractIndexer, stakeSR, ctx, r := prepare(t)
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
		sf.EXPECT().State(gomock.AssignableToTypeOf(&Endorsement{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			return uint64(0), state.ErrStateNotExist
		}).Times(1)
		contractIndexer.EXPECT().BucketsByCandidate(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 address.Address, arg1 uint64) ([]*VoteBucket, error) {
			buckets := []*VoteBucket{}
			for i := range testContractBuckets {
				if testContractBuckets[i].Candidate.String() == arg0.String() {
					buckets = append(buckets, testContractBuckets[i])
				}
			}
			return buckets, nil
		}).MaxTimes(2)
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
		iotexBucket, err = testContractBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[1])
	})
	t.Run("readStateBucketByIndices", func(t *testing.T) {
		sf, contractIndexer, stakeSR, ctx, r := prepare(t)
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
		sf.EXPECT().State(gomock.AssignableToTypeOf(&Endorsement{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			return uint64(0), state.ErrStateNotExist
		}).Times(1)
		contractIndexer.EXPECT().BucketsByIndices(gomock.Any(), gomock.Any()).DoAndReturn(func(arg0 []uint64, arg1 uint64) ([]*VoteBucket, error) {
			buckets := []*VoteBucket{}
			for i := range arg0 {
				buckets = append(buckets, testContractBuckets[arg0[i]])
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
		iotexBucket, err = testContractBuckets[0].toIoTeXTypes()
		r.NoError(err)
		r.Equal(iotexBucket, buckets.Buckets[1])
	})
	t.Run("readStateBucketCount", func(t *testing.T) {
		sf, contractIndexer, stakeSR, ctx, r := prepare(t)
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
		contractIndexer.EXPECT().TotalBucketCount(gomock.Any()).Return(uint64(len(testContractBuckets)), nil).Times(1)
		cfg := genesis.TestDefault()
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
		_, _, stakeSR, ctx, r := prepare(t)
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
			addContractVotes(&expectCand)
			r.EqualValues(expectCand.toIoTeXTypes(), candidates.Candidates[i])
		}
	})
	t.Run("readStateCandidateByName", func(t *testing.T) {
		_, _, stakeSR, ctx, r := prepare(t)
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
		addContractVotes(&expectCand)
		r.EqualValues(expectCand.toIoTeXTypes(), candidate)
	})
	t.Run("readStateCandidateByAddress", func(t *testing.T) {
		_, _, stakeSR, ctx, r := prepare(t)
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
		addContractVotes(&expectCand)
		r.EqualValues(expectCand.toIoTeXTypes(), candidate)
	})
	t.Run("readStateTotalStakingAmount", func(t *testing.T) {
		sf, _, stakeSR, ctx, r := prepare(t)
		sf.EXPECT().State(gomock.AssignableToTypeOf(&totalAmount{}), gomock.Any()).DoAndReturn(func(arg0 any, arg1 ...protocol.StateOption) (uint64, error) {
			arg0R := arg0.(*totalAmount)
			*arg0R = *testNativeTotalAmount
			return uint64(1), nil
		}).Times(1)
		cfg := genesis.TestDefault()
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
