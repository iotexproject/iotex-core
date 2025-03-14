package staking

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
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

func TestEndorsementStateReader_Status(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(2), nil).AnyTimes()
	esm := NewEndorsementStateManager(sm)
	esr := NewEndorsementStateReader(sm)

	t.Run("legacy status", func(t *testing.T) {
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
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
		g := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		g.TsunamiBlockHeight = 1   // enable endorsement protocol at block 1
		g.UpernavikBlockHeight = 2 // enable endorsement protocol improvement at block 2
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
