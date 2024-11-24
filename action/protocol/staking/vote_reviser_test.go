package staking

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestVoteReviser(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(0), nil).Times(4)
	csm := newCandidateStateManager(sm)
	csr := newCandidateStateReader(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	r.NoError(err)

	tests := []struct {
		cand     address.Address
		owner    address.Address
		amount   *big.Int
		duration uint32
		index    uint64
	}{
		{
			identityset.Address(6),
			identityset.Address(6),
			unit.ConvertIotxToRau(1100000),
			21,
			0,
		},
		{
			identityset.Address(1),
			identityset.Address(1),
			unit.ConvertIotxToRau(1200000),
			21,
			1,
		},
		{
			identityset.Address(2),
			identityset.Address(2),
			unit.ConvertIotxToRau(1200000),
			14,
			2,
		},
		{
			identityset.Address(3),
			identityset.Address(3),
			unit.ConvertIotxToRau(1200000),
			25,
			3,
		},
		{
			identityset.Address(4),
			identityset.Address(4),
			unit.ConvertIotxToRau(1200000),
			31,
			4,
		},
		{
			identityset.Address(5),
			identityset.Address(5),
			unit.ConvertIotxToRau(1199999),
			31,
			5,
		},
		{
			identityset.Address(1),
			identityset.Address(2),
			big.NewInt(2100000000),
			21,
			6,
		},
		{
			identityset.Address(2),
			identityset.Address(3),
			big.NewInt(1400000000),
			14,
			7,
		},
		{
			identityset.Address(3),
			identityset.Address(4),
			big.NewInt(2500000000),
			25,
			8,
		},
		{
			identityset.Address(4),
			identityset.Address(1),
			big.NewInt(3100000000),
			31,
			9,
		},
	}

	// test loading with no candidate in stateDB
	stk, err := NewProtocol(
		HelperCtx{
			DepositGas:    nil,
			BlockInterval: getBlockInterval,
		},
		&BuilderConfig{
			Staking:                  genesis.Default.Staking,
			PersistStakingPatchBlock: math.MaxUint64,
			Revise: ReviseConfig{
				VoteWeight:         genesis.Default.Staking.VoteWeightCalConsts,
				CorrectCandsHeight: genesis.Default.OkhotskBlockHeight,
				ReviseHeights:      []uint64{genesis.Default.HawaiiBlockHeight, genesis.Default.GreenlandBlockHeight},
			},
		},
		nil,
		nil,
		nil,
	)
	r.NotNil(stk)
	r.NoError(err)

	// write a number of buckets into stateDB
	for _, e := range tests {
		vb := NewVoteBucket(e.cand, e.owner, e.amount, e.duration, time.Now(), true)
		index, err := csm.putBucketAndIndex(vb)
		r.NoError(err)
		r.Equal(index, vb.Index)
	}

	// load candidates from stateDB and verify
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	v, err := stk.Start(ctx, sm)
	sm.WriteView(_protocolID, v)
	r.NoError(err)
	_, ok := v.(*ViewData)
	r.True(ok)

	csm, err = NewCandidateStateManager(sm, false)
	r.NoError(err)
	oldCand := testCandidates[3].d.Clone()
	oldCand.Name = "old name"
	r.NoError(csm.Upsert(oldCand))
	r.NoError(csm.Commit(ctx))
	r.NotNil(csm.GetByName(oldCand.Name))
	// load a number of candidates
	updateCands := CandidateList{
		testCandidates[3].d, testCandidates[4].d, testCandidates[2].d,
		testCandidates[0].d, testCandidates[1].d, testCandidates[5].d,
	}
	for _, e := range updateCands {
		r.NoError(csm.Upsert(e))
	}
	r.NoError(csm.Commit(ctx))
	r.NotNil(csm.GetByName(oldCand.Name))
	r.NotNil(csm.GetByName(testCandidates[3].d.Name))

	// test revise
	vr := stk.voteReviser
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	r.False(vr.isCacheExist(genesis.Default.GreenlandBlockHeight))
	r.False(vr.isCacheExist(genesis.Default.HawaiiBlockHeight))
	r.NoError(vr.Revise(featureCtx, csm, genesis.Default.HawaiiBlockHeight))
	r.True(vr.isCacheExist(genesis.Default.HawaiiBlockHeight))
	// simulate first revise attempt failed -- call Revise() again
	r.True(vr.isCacheExist(genesis.Default.HawaiiBlockHeight))
	r.NoError(vr.Revise(featureCtx, csm, genesis.Default.HawaiiBlockHeight))
	sm.EXPECT().Height().DoAndReturn(
		func() (uint64, error) {
			return genesis.Default.HawaiiBlockHeight, nil
		},
	).Times(1)
	r.NoError(csm.Commit(ctx))
	r.NotNil(csm.GetByName(oldCand.Name))
	// verify self-stake and total votes match
	result, ok := vr.result(genesis.Default.HawaiiBlockHeight)
	r.True(ok)
	r.Equal(len(testCandidates), len(result))
	cv := genesis.Default.Staking.VoteWeightCalConsts
	for _, c := range result {
		cand := csm.GetByOwner(c.Owner)
		r.True(c.Equal(cand))
		for _, cand := range testCandidates {
			if address.Equal(cand.d.Owner, c.Owner) {
				r.Equal(0, cand.d.SelfStake.Cmp(c.SelfStake))
			}
		}
		for _, v := range tests {
			if address.Equal(v.cand, c.Owner) && v.index != c.SelfStakeBucketIdx {
				bucket, err := csr.getBucket(v.index)
				r.NoError(err)
				total := CalculateVoteWeight(cv, bucket, false)
				bucket, err = csr.getBucket(c.SelfStakeBucketIdx)
				r.NoError(err)
				total.Add(total, CalculateVoteWeight(cv, bucket, true))
				r.Equal(0, total.Cmp(c.Votes))
				break
			}
		}
	}
	r.NoError(vr.Revise(featureCtx, csm, genesis.Default.OkhotskBlockHeight))
	sm.EXPECT().Height().DoAndReturn(
		func() (uint64, error) {
			return genesis.Default.OkhotskBlockHeight, nil
		},
	).Times(1)
	r.NoError(csm.Commit(ctx))
	r.Nil(csm.GetByName(oldCand.Name))
}

func TestVoteRevise_CorrectEndorsement(t *testing.T) {
	r := require.New(t)
	t.Run("correct endorse height", func(t *testing.T) {
		revise := NewVoteReviser(ReviseConfig{}, nil)
		r.False(revise.shouldReviseSelfStakeBuckets(1))

		revise = NewVoteReviser(ReviseConfig{
			SelfStakeBucketReviseHeight: 1,
		}, nil)
		r.True(revise.shouldReviseSelfStakeBuckets(1))
		r.False(revise.shouldReviseSelfStakeBuckets(2))
	})
	t.Run("correct endorsement", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		revise := NewVoteReviser(ReviseConfig{
			SelfStakeBucketReviseHeight: 2,
		}, nil)
		g := deepcopy.Copy(genesis.Default).(genesis.Genesis)
		ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 2})
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
		sm.EXPECT().Height().Return(uint64(2), nil).AnyTimes()
		_, err := sm.PutState(
			&totalBucketCount{count: 0},
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(TotalBucketKey),
		)
		r.NoError(err)
		_, err = sm.PutState(&totalAmount{
			amount: big.NewInt(0),
			count:  0,
		}, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		r.NoError(err)
		view, _, err := CreateBaseView(sm, true)
		r.NoError(err)
		sm.WriteView(_protocolID, view)
		csm, err := NewCandidateStateManager(sm, false)
		esm := NewEndorsementStateManager(sm)
		// prepare endorsements
		r.NoError(esm.Put(0, &Endorsement{ExpireHeight: endorsementNotExpireHeight}))
		r.NoError(esm.Put(1, &Endorsement{ExpireHeight: 1}))
		r.NoError(esm.Put(2, &Endorsement{ExpireHeight: 2}))
		r.NoError(esm.Put(3, &Endorsement{ExpireHeight: 3}))
		buckets := make([]*VoteBucket, 4)
		buckets[0] = NewVoteBucket(identityset.Address(1), identityset.Address(2), unit.ConvertIotxToRau(1200000), 91, time.Now(), true)
		buckets[0].Index, err = csm.putBucketAndIndex(buckets[0])
		r.NoError(err)
		buckets[1] = NewVoteBucket(identityset.Address(2), identityset.Address(3), unit.ConvertIotxToRau(1200000), 91, time.Now(), true)
		buckets[1].Index, err = csm.putBucketAndIndex(buckets[1])
		r.NoError(err)
		buckets[2] = NewVoteBucket(identityset.Address(3), identityset.Address(4), unit.ConvertIotxToRau(1200000), 91, time.Now(), true)
		buckets[2].Index, err = csm.putBucketAndIndex(buckets[2])
		r.NoError(err)
		buckets[3] = NewVoteBucket(identityset.Address(4), identityset.Address(5), unit.ConvertIotxToRau(1200000), 91, time.Now(), true)
		buckets[3].Index, err = csm.putBucketAndIndex(buckets[3])
		buckets[3].Index, err = csm.putBucketAndIndex(buckets[3])
		r.NoError(err)
		r.NoError(csm.Upsert(&Candidate{Name: "cand1", SelfStakeBucketIdx: 0, SelfStake: unit.ConvertIotxToRau(1200000), Votes: CalculateVoteWeight(revise.cfg.VoteWeight, buckets[0], true), Owner: identityset.Address(1), Operator: identityset.Address(12), Reward: identityset.Address(12), Identifier: identityset.Address(1)}))
		r.NoError(csm.Upsert(&Candidate{Name: "cand2", SelfStakeBucketIdx: 1, SelfStake: unit.ConvertIotxToRau(1200000), Votes: CalculateVoteWeight(revise.cfg.VoteWeight, buckets[1], true), Owner: identityset.Address(2), Operator: identityset.Address(13), Reward: identityset.Address(12), Identifier: identityset.Address(2)}))
		r.NoError(csm.Upsert(&Candidate{Name: "cand3", SelfStakeBucketIdx: 2, SelfStake: unit.ConvertIotxToRau(1200000), Votes: CalculateVoteWeight(revise.cfg.VoteWeight, buckets[2], true), Owner: identityset.Address(3), Operator: identityset.Address(14), Reward: identityset.Address(12), Identifier: identityset.Address(3)}))
		r.NoError(csm.Upsert(&Candidate{Name: "cand4", SelfStakeBucketIdx: 3, SelfStake: unit.ConvertIotxToRau(1200000), Votes: CalculateVoteWeight(revise.cfg.VoteWeight, buckets[3], true), Owner: identityset.Address(4), Operator: identityset.Address(15), Reward: identityset.Address(12), Identifier: identityset.Address(4)}))
		// correct endorsement
		r.True(revise.NeedRevise(2))
		r.NoError(revise.Revise(protocol.MustGetFeatureCtx(ctx), csm, 2))
		// verify
		t.Run("keep endorsed", func(t *testing.T) {
			endorse, err := esm.Get(0)
			r.NoError(err)
			r.Equal(uint64(endorsementNotExpireHeight), endorse.ExpireHeight)
		})
		t.Run("keep unendorsing", func(t *testing.T) {
			endorse, err := esm.Get(3)
			r.NoError(err)
			r.Equal(uint64(3), endorse.ExpireHeight)
		})
		t.Run("delete expired", func(t *testing.T) {
			_, err := esm.Get(1)
			r.ErrorIs(err, state.ErrStateNotExist)
			_, err = esm.Get(2)
			r.ErrorIs(err, state.ErrStateNotExist)
		})
	})
	t.Run("correct candidate selfstake", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		g := deepcopy.Copy(genesis.Default).(genesis.Genesis)
		g.TsunamiBlockHeight = 2
		g.ToBeEnabledBlockHeight = 2
		ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 2})
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
		sm.EXPECT().Height().Return(uint64(2), nil).AnyTimes()
		_, err := sm.PutState(
			&totalBucketCount{count: 0},
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(TotalBucketKey),
		)
		r.NoError(err)
		_, err = sm.PutState(&totalAmount{
			amount: big.NewInt(0),
			count:  0,
		}, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		r.NoError(err)
		view, _, err := CreateBaseView(sm, true)
		r.NoError(err)
		sm.WriteView(_protocolID, view)
		t.Run("not change selfstaked candidate", func(t *testing.T) {
			revise := NewVoteReviser(ReviseConfig{
				VoteWeight:                  g.VoteWeightCalConsts,
				SelfStakeBucketReviseHeight: 2,
			}, nil)
			bkt := &VoteBucket{
				Index:          0,
				Candidate:      identityset.Address(1),
				Owner:          identityset.Address(1),
				StakedAmount:   unit.ConvertIotxToRau(1200000),
				StakedDuration: time.Hour * 24,
				CreateTime:     time.Now(),
				AutoStake:      true,
			}
			csm, err := NewCandidateStateManager(sm, true)
			r.NoError(err)
			bktIdx, err := csm.putBucketAndIndex(bkt)
			r.NoError(err)
			bkt.Index = bktIdx
			cand := &Candidate{
				Owner:              identityset.Address(1),
				Operator:           identityset.Address(11),
				Reward:             identityset.Address(1),
				Identifier:         identityset.Address(1),
				Name:               "test1",
				Votes:              CalculateVoteWeight(g.VoteWeightCalConsts, bkt, true),
				SelfStakeBucketIdx: bktIdx,
				SelfStake:          bkt.StakedAmount,
			}
			r.NoError(csm.Upsert(cand))
			r.NoError(csm.Commit(ctx))
			r.NoError(revise.Revise(protocol.MustGetFeatureCtx(ctx), csm, 2))
			candPost := csm.GetByIdentifier(cand.GetIdentifier())
			r.Equal(cand, candPost)
		})
		t.Run("endorsed candidate", func(t *testing.T) {
			revise := NewVoteReviser(ReviseConfig{
				VoteWeight:                  g.VoteWeightCalConsts,
				SelfStakeBucketReviseHeight: 2,
			}, nil)
			bkt := &VoteBucket{
				Candidate:      identityset.Address(3),
				Owner:          identityset.Address(4),
				StakedAmount:   unit.ConvertIotxToRau(1200000),
				StakedDuration: time.Hour * 24,
				CreateTime:     time.Now(),
				AutoStake:      true,
			}
			csm, err := NewCandidateStateManager(sm, true)
			r.NoError(err)
			bktIdx, err := csm.putBucketAndIndex(bkt)
			r.NoError(err)
			bkt.Index = bktIdx
			esm := NewEndorsementStateManager(sm)
			r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: endorsementNotExpireHeight}))
			cand := &Candidate{
				Owner:              identityset.Address(3),
				Operator:           identityset.Address(13),
				Reward:             identityset.Address(3),
				Identifier:         identityset.Address(3),
				Name:               "test3",
				Votes:              CalculateVoteWeight(g.VoteWeightCalConsts, bkt, true),
				SelfStakeBucketIdx: bktIdx,
				SelfStake:          bkt.StakedAmount,
			}
			r.NoError(csm.Upsert(cand))
			r.NoError(csm.Commit(ctx))
			r.NoError(revise.Revise(protocol.MustGetFeatureCtx(ctx), csm, 2))
			candPost := csm.GetByIdentifier(cand.GetIdentifier())
			r.Equal(cand, candPost)
		})
		t.Run("unendorsing candidate", func(t *testing.T) {
			revise := NewVoteReviser(ReviseConfig{
				VoteWeight:                  g.VoteWeightCalConsts,
				SelfStakeBucketReviseHeight: 2,
			}, nil)
			bkt := &VoteBucket{
				Candidate:      identityset.Address(4),
				Owner:          identityset.Address(5),
				StakedAmount:   unit.ConvertIotxToRau(1200000),
				StakedDuration: time.Hour * 24,
				CreateTime:     time.Now(),
				AutoStake:      true,
			}
			csm, err := NewCandidateStateManager(sm, true)
			r.NoError(err)
			bktIdx, err := csm.putBucketAndIndex(bkt)
			r.NoError(err)
			bkt.Index = bktIdx
			esm := NewEndorsementStateManager(sm)
			r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: 3}))
			cand := &Candidate{
				Owner:              identityset.Address(4),
				Operator:           identityset.Address(14),
				Reward:             identityset.Address(4),
				Identifier:         identityset.Address(4),
				Name:               "test4",
				Votes:              CalculateVoteWeight(g.VoteWeightCalConsts, bkt, true),
				SelfStakeBucketIdx: bktIdx,
				SelfStake:          bkt.StakedAmount,
			}
			r.NoError(csm.Upsert(cand))
			r.NoError(csm.Commit(ctx))
			r.NoError(revise.Revise(protocol.MustGetFeatureCtx(ctx), csm, 2))
			candPost := csm.GetByIdentifier(cand.GetIdentifier())
			r.Equal(cand, candPost)
		})
		t.Run("endorsement expired candidate", func(t *testing.T) {
			revise := NewVoteReviser(ReviseConfig{
				VoteWeight:                  g.VoteWeightCalConsts,
				SelfStakeBucketReviseHeight: 2,
			}, nil)
			bkt := &VoteBucket{
				Candidate:      identityset.Address(5),
				Owner:          identityset.Address(6),
				StakedAmount:   unit.ConvertIotxToRau(1200000),
				StakedDuration: time.Hour * 24 * 91,
				CreateTime:     time.Now(),
				AutoStake:      true,
			}
			csm, err := NewCandidateStateManager(sm, true)
			r.NoError(err)
			bktIdx, err := csm.putBucketAndIndex(bkt)
			r.NoError(err)
			bkt.Index = bktIdx
			esm := NewEndorsementStateManager(sm)
			r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: 2}))
			cand := &Candidate{
				Owner:              identityset.Address(5),
				Operator:           identityset.Address(15),
				Reward:             identityset.Address(5),
				Identifier:         identityset.Address(5),
				Name:               "test5",
				Votes:              CalculateVoteWeight(g.VoteWeightCalConsts, bkt, true),
				SelfStakeBucketIdx: bktIdx,
				SelfStake:          bkt.StakedAmount,
			}
			r.NoError(csm.Upsert(cand))
			r.NoError(csm.Commit(ctx))
			r.NoError(revise.Revise(protocol.MustGetFeatureCtx(ctx), csm, 2))
			candPost := csm.GetByIdentifier(cand.GetIdentifier())
			cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
			cand.SelfStake = big.NewInt(0)
			cand.Votes = CalculateVoteWeight(g.VoteWeightCalConsts, bkt, false)
			r.Equal(cand, candPost)
		})
	})
}

func TestVoteRevise_CorrectSelfStake(t *testing.T) {
	r := require.New(t)
	t.Run("correct height", func(t *testing.T) {
		revise := NewVoteReviser(ReviseConfig{}, nil)
		r.False(revise.shouldCorrectCandSelfStake(1))

		revise = NewVoteReviser(ReviseConfig{
			CorrectCandSelfStakeHeight: 1,
		}, nil)
		r.True(revise.shouldCorrectCandSelfStake(1))
		r.False(revise.shouldCorrectCandSelfStake(2))
	})
	t.Run("correct candidate selfstake", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		g := deepcopy.Copy(genesis.Default).(genesis.Genesis)
		ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 2})
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
		sm.EXPECT().Height().Return(uint64(2), nil).AnyTimes()
		_, err := sm.PutState(
			&totalBucketCount{count: 0},
			protocol.NamespaceOption(_stakingNameSpace),
			protocol.KeyOption(TotalBucketKey),
		)
		r.NoError(err)
		_, err = sm.PutState(&totalAmount{
			amount: big.NewInt(0),
			count:  0,
		}, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
		r.NoError(err)
		view, _, err := CreateBaseView(sm, true)
		r.NoError(err)
		sm.WriteView(_protocolID, view)
		revise := NewVoteReviser(ReviseConfig{
			VoteWeight:                 g.VoteWeightCalConsts,
			CorrectCandSelfStakeHeight: 2,
		}, nil)
		bkt := &VoteBucket{
			Index:            0,
			Candidate:        identityset.Address(1),
			Owner:            identityset.Address(1),
			StakedAmount:     unit.ConvertIotxToRau(1200000),
			StakedDuration:   time.Hour * 24,
			CreateTime:       time.Now(),
			AutoStake:        true,
			UnstakeStartTime: time.Now(),
		}
		csm, err := NewCandidateStateManager(sm, true)
		r.NoError(err)
		bktIdx, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		bkt.Index = bktIdx
		cand := &Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(11),
			Reward:             identityset.Address(1),
			Identifier:         identityset.Address(1),
			Name:               "test1",
			Votes:              CalculateVoteWeight(g.VoteWeightCalConsts, bkt, true),
			SelfStakeBucketIdx: bktIdx,
			SelfStake:          bkt.StakedAmount,
		}
		cand2 := &Candidate{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(12),
			Reward:             identityset.Address(2),
			Identifier:         identityset.Address(2),
			Name:               "test2",
			Votes:              CalculateVoteWeight(g.VoteWeightCalConsts, bkt, true),
			SelfStakeBucketIdx: 100000,
			SelfStake:          bkt.StakedAmount,
		}
		r.NoError(csm.Upsert(cand))
		r.NoError(csm.Upsert(cand2))
		r.NoError(csm.Commit(ctx))
		r.True(revise.NeedRevise(protocol.MustGetBlockCtx(ctx).BlockHeight))
		r.NoError(revise.Revise(protocol.MustGetFeatureCtx(ctx), csm, 2))
		candPost := csm.GetByIdentifier(cand.GetIdentifier())
		// revise unstaked but not cleaned up
		r.Equal(&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(11),
			Reward:             identityset.Address(1),
			Identifier:         identityset.Address(1),
			Name:               "test1",
			Votes:              big.NewInt(0),
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
			SelfStake:          big.NewInt(0),
		}, candPost)
		// revise withdrawn
		candPost = csm.GetByIdentifier(cand2.GetIdentifier())
		r.Equal(&Candidate{
			Owner:              identityset.Address(2),
			Operator:           identityset.Address(12),
			Reward:             identityset.Address(2),
			Identifier:         identityset.Address(2),
			Name:               "test2",
			Votes:              big.NewInt(0),
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
			SelfStake:          big.NewInt(0),
		}, candPost)
	})
}
