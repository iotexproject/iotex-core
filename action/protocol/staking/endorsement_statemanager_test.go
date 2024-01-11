package staking

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
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
