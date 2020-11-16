package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestVoteReviser(t *testing.T) {
	r := require.New(t)

	// make sure the prefix stays constant, they affect the key to store objects to DB
	r.Equal(byte(0), _const)
	r.Equal(byte(1), _bucket)
	r.Equal(byte(2), _voterIndex)
	r.Equal(byte(3), _candIndex)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := testdb.NewMockStateManager(ctrl)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(StakingNameSpace),
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
			identityset.Address(1),
			identityset.Address(2),
			big.NewInt(2100000000),
			21,
			0,
		},
		{
			identityset.Address(2),
			identityset.Address(3),
			big.NewInt(1400000000),
			14,
			1,
		},
		{
			identityset.Address(3),
			identityset.Address(4),
			big.NewInt(2500000000),
			25,
			2,
		},
		{
			identityset.Address(4),
			identityset.Address(1),
			big.NewInt(3100000000),
			31,
			3,
		},
	}

	// test loading with no candidate in stateDB
	stk, err := NewProtocol(nil, genesis.Default.Staking, nil, genesis.Default.GreenlandBlockHeight)
	r.NotNil(stk)
	r.NoError(err)

	// address package also defined protocol address, make sure they match
	r.Equal(hash.BytesToHash160(stk.addr.Bytes()), address.StakingProtocolAddrHash)

	// write a number of buckets into stateDB
	for _, e := range tests {
		vb := NewVoteBucket(e.cand, e.owner, e.amount, e.duration, time.Now(), true)
		index, err := putBucketAndIndex(sm, vb)
		r.NoError(err)
		r.Equal(index, vb.Index)
	}

	// load candidates from stateDB and verify
	ctx := protocol.WithBlockchainCtx(context.Background(), protocol.BlockchainCtx{
		Genesis: genesis.Default,
	})
	v, err := stk.Start(ctx, sm)
	sm.WriteView(protocolID, v)
	r.NoError(err)
	_, ok := v.(*ViewData)
	r.True(ok)

	csm, err := NewCandidateStateManager(sm, false)
	r.NoError(err)
	// load a number of candidates
	for _, e := range testCandidates {
		r.NoError(csm.Upsert(e.d))
	}
	r.NoError(csm.Commit())

	cands, _, err := getAllCandidates(sm)
	r.NoError(err)
	candm := make(map[string]*Candidate)
	for _, cand := range cands {
		candm[cand.Owner.String()] = cand.Clone()
		candm[cand.Owner.String()].Votes = new(big.Int)
		candm[cand.Owner.String()].SelfStake = new(big.Int)
	}

	buckets, _, err := getAllBuckets(sm)
	r.NoError(err)

	cv := genesis.Default.Staking.VoteWeightCalConsts

	t.Logf("bucketsLen: %d, cv: %+v", len(buckets), cv)
	for _, bucket := range buckets {
		r.Equal(bucket.isUnstaked(), false)

		cand, ok := candm[bucket.Candidate.String()]
		r.Equal(ok, true)

		if cand.SelfStakeBucketIdx == bucket.Index {
			r.NoError(cand.AddVote(calculateVoteWeight(cv, bucket, true)))
			cand.SelfStake = bucket.StakedAmount
		} else {
			r.NoError(cand.AddVote(calculateVoteWeight(cv, bucket, false)))
		}
	}

}
