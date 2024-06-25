package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestEndorsementStateManager_Put(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	esm := NewEndorsementStateManager(sm)

	// insert endorsement
	bucketIndex := uint64(123)
	endorse := &Endorsement{
		ExpireHeight: 456,
	}
	err := esm.Put(bucketIndex, endorse)
	r.NoError(err)
	expect := Endorsement{}
	_, err = esm.State(&expect, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(endorsementKey(bucketIndex)))
	r.NoError(err)
	r.EqualValues(expect.ExpireHeight, 456)

	// update endorsement
	endorse.ExpireHeight = 789
	err = esm.Put(bucketIndex, endorse)
	r.NoError(err)
	_, err = esm.State(&expect, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(endorsementKey(bucketIndex)))
	r.NoError(err)
	r.EqualValues(expect.ExpireHeight, 789)
}

func TestEndorsementStateReader_Get(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	esm := NewEndorsementStateManager(sm)
	esr := NewEndorsementStateReader(sm)

	// get non-exist endorsement
	bucketIndex := uint64(123)
	_, err := esr.Get(bucketIndex)
	r.ErrorIs(err, state.ErrStateNotExist)
	_, err = esm.Get(bucketIndex)
	r.ErrorIs(err, state.ErrStateNotExist)

	// get exist endorsement
	endorse := &Endorsement{
		ExpireHeight: 456,
	}
	err = esm.Put(bucketIndex, endorse)
	r.NoError(err)
	expect, err := esr.Get(bucketIndex)
	r.NoError(err)
	r.EqualValues(expect.ExpireHeight, 456)
	expect, err = esm.Get(bucketIndex)
	r.NoError(err)
	r.EqualValues(expect.ExpireHeight, 456)
}

func TestEndorsementStateReader_Delete(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	esm := NewEndorsementStateManager(sm)
	esr := NewEndorsementStateReader(sm)

	t.Run("delete non-exist endorsement", func(t *testing.T) {
		bucketIndex := uint64(123)
		err := esm.Delete(bucketIndex)
		r.ErrorContains(err, "bucket not exist in DB")
		endorse := &Endorsement{
			ExpireHeight: 456,
		}
		err = esm.Put(bucketIndex, endorse)
		r.NoError(err)
		err = esm.Delete(234)
		r.NoError(err)
	})
	t.Run("delete exist endorsement", func(t *testing.T) {
		bucketIndex := uint64(123)
		endorse := &Endorsement{
			ExpireHeight: 456,
		}
		err := esm.Put(bucketIndex, endorse)
		r.NoError(err)
		e, err := esm.Get(bucketIndex)
		r.NoError(err)
		r.Equal(e, endorse)
		err = esm.Delete(bucketIndex)
		r.NoError(err)
		_, err = esr.Get(bucketIndex)
		r.ErrorIs(err, state.ErrStateNotExist)
	})
}

func TestEndorsementStateReader_List(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(2), nil).AnyTimes()
	esm := NewEndorsementStateManager(sm)
	esr := NewEndorsementStateReader(sm)

	t.Run("list non-exist endorsement", func(t *testing.T) {
		bucketIndices, endorsements, err := esr.List()
		r.NoError(err)
		r.Len(bucketIndices, 0)
		r.Len(endorsements, 0)
	})
	t.Run("list exist endorsement", func(t *testing.T) {
		endorses := []*Endorsement{
			{ExpireHeight: endorsementNotExpireHeight},
			{ExpireHeight: 456},
			{ExpireHeight: 789},
			{ExpireHeight: endorsementNotExpireHeight},
		}
		for i, endorse := range endorses {
			r.NoError(esm.Put(uint64(i), endorse))
		}
		bucketIndices, endorsements, err := esr.List()
		r.NoError(err)
		r.ElementsMatch(bucketIndices, []uint64{0, 1, 2, 3})
		r.ElementsMatch(endorses, endorsements)
		// delete one endorsement
		r.NoError(esm.Delete(1))
		bucketIndices, endorsements, err = esr.List()
		r.NoError(err)
		r.ElementsMatch(bucketIndices, []uint64{0, 2, 3})
		r.ElementsMatch([]*Endorsement{endorses[0], endorses[2], endorses[3]}, endorsements)
	})
	t.Run("hybrid states", func(t *testing.T) {
		// put endorsement
		endorses := []*Endorsement{
			{ExpireHeight: endorsementNotExpireHeight},
			{ExpireHeight: 456},
			{ExpireHeight: 789},
			{ExpireHeight: endorsementNotExpireHeight},
		}
		for i, endorse := range endorses {
			r.NoError(esm.Put(uint64(i), endorse))
		}
		// put vote bucket
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
		r.NoError(err)
		index, err := csm.putBucketAndIndex(&VoteBucket{
			Index:          1,
			Candidate:      identityset.Address(1),
			Owner:          identityset.Address(2),
			StakedAmount:   unit.ConvertIotxToRau(1200000),
			StakedDuration: time.Hour,
		})
		r.NoError(err)
		// put candidate
		r.NoError(csm.Upsert(&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(3),
			Identifier:         identityset.Address(1),
			Name:               "cand1",
			Votes:              unit.ConvertIotxToRau(1200000),
			SelfStakeBucketIdx: index,
			SelfStake:          unit.ConvertIotxToRau(1200000),
		}))
		// list
		bucketIndices, endorsements, err := esr.List()
		r.NoError(err)
		r.ElementsMatch([]uint64{0, 1, 2, 3}, bucketIndices)
		r.ElementsMatch([]*Endorsement{endorses[0], endorses[1], endorses[2], endorses[3]}, endorsements)
	})
}

func TestEndorsementStateReader_Status(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(2), nil).AnyTimes()
	esm := NewEndorsementStateManager(sm)
	esr := NewEndorsementStateReader(sm)

	t.Run("legacy status", func(t *testing.T) {
		g := deepcopy.Copy(genesis.Default).(genesis.Genesis)
		g.TsunamiBlockHeight = 1 // enable endorsement protocol at block 1
		ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 2})
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		r.NoError(esm.Put(1, &Endorsement{ExpireHeight: endorsementNotExpireHeight}))
		r.NoError(esm.Put(2, &Endorsement{ExpireHeight: 2}))
		r.NoError(esm.Put(3, &Endorsement{ExpireHeight: 3}))
		status, err := esr.Status(protocol.MustGetFeatureCtx(ctx), 1, 2)
		r.NoError(err)
		r.Equal(Endorsed, status)
		status, err = esr.Status(protocol.MustGetFeatureCtx(ctx), 2, 2)
		r.NoError(err)
		r.Equal(EndorseExpired, status)
		status, err = esr.Status(protocol.MustGetFeatureCtx(ctx), 3, 2)
		r.NoError(err)
		r.Equal(UnEndorsing, status)
	})
	t.Run("status after Upernavik", func(t *testing.T) {
		g := deepcopy.Copy(genesis.Default).(genesis.Genesis)
		g.TsunamiBlockHeight = 1     // enable endorsement protocol at block 1
		g.ToBeEnabledBlockHeight = 2 // enable endorsement protocol improvement at block 2
		ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 2})
		ctx = genesis.WithGenesisContext(ctx, g)
		ctx = protocol.WithFeatureCtx(ctx)
		r.NoError(esm.Put(1, &Endorsement{ExpireHeight: endorsementNotExpireHeight}))
		r.NoError(esm.Put(2, &Endorsement{ExpireHeight: 2}))
		r.NoError(esm.Put(3, &Endorsement{ExpireHeight: 3}))
		status, err := esr.Status(protocol.MustGetFeatureCtx(ctx), 1, 2)
		r.NoError(err)
		r.Equal(Endorsed, status)
		status, err = esr.Status(protocol.MustGetFeatureCtx(ctx), 2, 2)
		r.NoError(err)
		r.Equal(UnEndorsing, status)
		status, err = esr.Status(protocol.MustGetFeatureCtx(ctx), 3, 2)
		r.NoError(err)
		r.Equal(Endorsed, status)
	})
}
