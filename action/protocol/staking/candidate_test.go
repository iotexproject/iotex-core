// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestSer(t *testing.T) {
	r := require.New(t)

	l := &CandidateList{
		&Candidate{
			Owner:              identityset.Address(1),
			Operator:           identityset.Address(2),
			Reward:             identityset.Address(3),
			Name:               "testname1",
			Votes:              big.NewInt(100),
			SelfStakeBucketIdx: 0,
			SelfStake:          big.NewInt(1100000000),
		},
		&Candidate{
			Owner:              identityset.Address(4),
			Operator:           identityset.Address(5),
			Reward:             identityset.Address(6),
			Identifier:         identityset.Address(7),
			Name:               "testname2",
			Votes:              big.NewInt(20),
			SelfStakeBucketIdx: 1,
			SelfStake:          big.NewInt(2100000000),
		},
		&Candidate{
			Owner:              identityset.Address(7),
			Operator:           identityset.Address(8),
			Reward:             identityset.Address(9),
			Identifier:         identityset.Address(10),
			Name:               "testname3",
			Votes:              big.NewInt(3000),
			SelfStakeBucketIdx: 2,
			SelfStake:          big.NewInt(3100000000),
		},
	}

	ser, err := l.Serialize()
	r.NoError(err)
	l1 := &CandidateList{}
	r.NoError(l1.Deserialize(ser))
	r.Equal(l, l1)

	// empty CandidateList can successfully Serialize/Deserialize
	var m CandidateList
	ser, err = m.Serialize()
	r.NoError(err)
	r.Equal([]byte{}, ser)
	var m1 CandidateList
	r.NoError(m1.Deserialize(ser))
	r.Nil(m1)
}

func TestSerWithExitBlockAndDeleted(t *testing.T) {
	r := require.New(t)

	original := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(2),
		Reward:             identityset.Address(3),
		Name:               "test_candidate",
		Votes:              big.NewInt(100),
		SelfStake:          big.NewInt(1000),
		SelfStakeBucketIdx: 1,
		ExitBlock:          99999,
		Deleted:            true,
	}

	data, err := original.Serialize()
	r.NoError(err)

	deserialized := &Candidate{}
	r.NoError(deserialized.Deserialize(data))

	r.Equal(original.ExitBlock, deserialized.ExitBlock)
	r.Equal(original.Deleted, deserialized.Deleted)
	r.Equal(uint64(99999), deserialized.ExitBlock)
	r.True(deserialized.Deleted)

	r.True(original.Equal(deserialized))
}

func TestClone(t *testing.T) {
	r := require.New(t)

	d := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(2),
		Reward:             identityset.Address(3),
		Name:               "testname1234",
		Votes:              big.NewInt(0),
		SelfStakeBucketIdx: 0,
		SelfStake:          big.NewInt(2100000000),
	}
	r.NoError(d.Validate())
	r.Equal(d.Owner, d.GetIdentifier())
	d2 := d.Clone()
	r.True(d.Equal(d2))
	d.AddVote(big.NewInt(100))
	r.False(d.Equal(d2))
	r.NoError(d.Collision(d2))
	d.Owner = identityset.Address(0)
	r.Equal(action.ErrInvalidCanName, d.Collision(d2))
	d.Name = "noconflict"
	r.Equal(ErrInvalidOperator, d.Collision(d2))
	d.Operator = identityset.Address(0)
	r.Equal(ErrInvalidSelfStkIndex, d.Collision(d2))
	d.SelfStakeBucketIdx++
	r.NoError(d.Collision(d2))
	d.Identifier = identityset.Address(3)
	r.Equal(d.Identifier, d.GetIdentifier())

	c := d.toStateCandidate(false)
	r.Equal(d.GetIdentifier().String(), c.Identity)
	r.Equal(d.Owner.String(), c.Address)
	r.Equal(d.Reward.String(), c.RewardAddress)
	r.Equal(d.Votes, c.Votes)
	r.Equal(d.Name, string(c.CanName))

	c = d.toStateCandidate(true)
	r.Equal("", c.Identity)
	r.Equal(d.Owner.String(), c.Address)
	r.Equal(d.Reward.String(), c.RewardAddress)
	r.Equal(d.Votes, c.Votes)
	r.Equal(d.Name, string(c.CanName))
}

func TestCloneWithExitBlockAndDeleted(t *testing.T) {
	r := require.New(t)

	original := &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(2),
		Reward:             identityset.Address(3),
		Name:               "test_candidate",
		Votes:              big.NewInt(100),
		SelfStake:          big.NewInt(1000),
		SelfStakeBucketIdx: 1,
		ExitBlock:          12345,
		Deleted:            true,
	}

	clone := original.Clone()

	r.True(original.Equal(clone))
	r.Equal(original.ExitBlock, clone.ExitBlock)
	r.Equal(original.Deleted, clone.Deleted)
	r.Equal(uint64(12345), clone.ExitBlock)
	r.True(clone.Deleted)

	clone.ExitBlock = 99999
	clone.Deleted = false
	r.Equal(uint64(12345), original.ExitBlock)
	r.True(original.Deleted)
}

var (
	testCandidates = []struct {
		d     *Candidate
		index int
	}{
		{
			&Candidate{
				Owner:              identityset.Address(1),
				Operator:           identityset.Address(7),
				Reward:             identityset.Address(1),
				Identifier:         nil,
				Name:               "test1",
				Votes:              big.NewInt(2),
				SelfStakeBucketIdx: 1,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			2,
		},
		{
			&Candidate{
				Owner:              identityset.Address(2),
				Operator:           identityset.Address(8),
				Reward:             identityset.Address(1),
				Identifier:         nil,
				Name:               "test2",
				Votes:              big.NewInt(3),
				SelfStakeBucketIdx: 2,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			1,
		},
		{
			&Candidate{
				Owner:              identityset.Address(3),
				Operator:           identityset.Address(9),
				Reward:             identityset.Address(1),
				Identifier:         nil,
				Name:               "test3",
				Votes:              big.NewInt(3),
				SelfStakeBucketIdx: 3,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			0,
		},
		{
			&Candidate{
				Owner:              identityset.Address(4),
				Operator:           identityset.Address(10),
				Reward:             identityset.Address(1),
				Identifier:         nil,
				Name:               "test4",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 4,
				SelfStake:          unit.ConvertIotxToRau(1200000),
			},
			3,
		},
		// the below 2 will be filtered out in ActiveCandidates() due to insufficient self-stake
		{
			&Candidate{
				Owner:              identityset.Address(5),
				Operator:           identityset.Address(11),
				Reward:             identityset.Address(2),
				Identifier:         nil,
				Name:               "test5",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 5,
				SelfStake:          unit.ConvertIotxToRau(1199999),
			},
			5,
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(2),
				Identifier:         nil,
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			6,
		},
	}
)

func TestGetPutCandidate(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	csr := newCandidateStateReader(sm)

	// put candidates and get
	for _, e := range testCandidates {
		_, _, err := csr.CandidateByAddress(e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
		require.NoError(csm.putCandidate(e.d))
		d1, _, err := csr.CandidateByAddress(e.d.Owner)
		require.NoError(err)
		require.Equal(e.d, d1)
	}

	// get all candidates
	cc, _, err := csr.CreateCandidateCenter(protocol.FeatureCtx{})
	require.NoError(err)
	all := cc.All()
	require.Equal(len(testCandidates), len(all))
	for _, e := range testCandidates {
		for i := range all {
			if all[i].Name == e.d.Name {
				require.Equal(e.d, all[i])
				break
			}
		}
	}

	// delete buckets and get
	for _, e := range testCandidates {
		require.NoError(csm.delCandidate(e.d.GetIdentifier()))
		_, _, err := csr.CandidateByAddress(e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}

func TestLess(t *testing.T) {
	r := require.New(t)
	pairs := []CandidateList{
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(2),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			&Candidate{
				Owner:              identityset.Address(5),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(6),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(5),
				Reward:             identityset.Address(2),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(5),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test5",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 1,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
		{
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100001),
			},
			&Candidate{
				Owner:              identityset.Address(6),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(6),
				Name:               "test6",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 0,
				SelfStake:          unit.ConvertIotxToRau(1100000),
			},
		},
	}
	for _, pair := range pairs {
		r.True(pair.Less(0, 1))
	}
}

func TestCandidate_DeletedCannotReceiveVotes(t *testing.T) {
	r := require.New(t)

	candidate := &Candidate{
		Owner:              identityset.Address(0),
		Operator:           identityset.Address(0),
		Reward:             identityset.Address(0),
		Name:               "test_candidate",
		Votes:              big.NewInt(100),
		SelfStake:          big.NewInt(1000),
		SelfStakeBucketIdx: 1,
		ExitBlock:          0,
		Deleted:            false,
	}

	r.NoError(candidate.AddVote(big.NewInt(50)))
	r.Equal(uint64(150), candidate.Votes.Uint64())

	candidate.Deleted = true
	initialVotes := candidate.Votes.Uint64()

	r.NoError(candidate.AddVote(big.NewInt(50)))
	r.Equal(initialVotes, candidate.Votes.Uint64())

	r.NoError(candidate.SubVote(big.NewInt(10)))
	r.Equal(initialVotes, candidate.Votes.Uint64())
}

func TestCandidate_DeletedCannotModifySelfStake(t *testing.T) {
	r := require.New(t)

	candidate := &Candidate{
		Owner:              identityset.Address(0),
		Operator:           identityset.Address(0),
		Reward:             identityset.Address(0),
		Name:               "test_candidate",
		Votes:              big.NewInt(100),
		SelfStake:          big.NewInt(1000),
		SelfStakeBucketIdx: 1,
		ExitBlock:          0,
		Deleted:            false,
	}

	r.NoError(candidate.AddSelfStake(big.NewInt(500)))
	r.Equal(uint64(1500), candidate.SelfStake.Uint64())

	candidate.Deleted = true
	initialSelfStake := candidate.SelfStake.Uint64()

	r.NoError(candidate.AddSelfStake(big.NewInt(500)))
	r.Equal(initialSelfStake, candidate.SelfStake.Uint64())

	r.NoError(candidate.SubSelfStake(big.NewInt(100)))
	r.Equal(initialSelfStake, candidate.SelfStake.Uint64())
}
