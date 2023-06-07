package staking

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
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
		nil,
		&BuilderConfig{
			Staking:                  genesis.Default.Staking,
			PersistStakingPatchBlock: math.MaxUint64,
		},
		nil,
		nil,
		genesis.Default.OkhotskBlockHeight,
		genesis.Default.HawaiiBlockHeight,
		genesis.Default.GreenlandBlockHeight,
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
	r.False(vr.isCacheExist(genesis.Default.GreenlandBlockHeight))
	r.False(vr.isCacheExist(genesis.Default.HawaiiBlockHeight))
	r.NoError(vr.Revise(csm, genesis.Default.HawaiiBlockHeight))
	r.True(vr.isCacheExist(genesis.Default.HawaiiBlockHeight))
	// simulate first revise attempt failed -- call Revise() again
	r.True(vr.isCacheExist(genesis.Default.HawaiiBlockHeight))
	r.NoError(vr.Revise(csm, genesis.Default.HawaiiBlockHeight))
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
				total := calculateVoteWeight(cv, bucket, false)
				bucket, err = csr.getBucket(c.SelfStakeBucketIdx)
				r.NoError(err)
				total.Add(total, calculateVoteWeight(cv, bucket, true))
				r.Equal(0, total.Cmp(c.Votes))
				break
			}
		}
	}
	r.NoError(vr.Revise(csm, genesis.Default.OkhotskBlockHeight))
	sm.EXPECT().Height().DoAndReturn(
		func() (uint64, error) {
			return genesis.Default.OkhotskBlockHeight, nil
		},
	).Times(1)
	r.NoError(csm.Commit(ctx))
	r.Nil(csm.GetByName(oldCand.Name))
}
