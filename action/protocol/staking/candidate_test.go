// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
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
			Name:               "testname2",
			Votes:              big.NewInt(20),
			SelfStakeBucketIdx: 1,
			SelfStake:          big.NewInt(2100000000),
		},
		&Candidate{
			Owner:              identityset.Address(7),
			Operator:           identityset.Address(8),
			Reward:             identityset.Address(9),
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

	d2 := d.Clone()
	r.True(d.Equal(d2))
	d.AddVote(big.NewInt(100))
	r.False(d.Equal(d2))
	r.NoError(d.Collision(d2))
	d.Owner = identityset.Address(0)
	r.Equal(ErrInvalidCanName, d.Collision(d2))
	d.Name = "noconflict"
	r.Equal(ErrInvalidOperator, d.Collision(d2))
	d.Operator = identityset.Address(0)
	r.Equal(ErrInvalidSelfStkIndex, d.Collision(d2))
	d.SelfStakeBucketIdx++
	r.NoError(d.Collision(d2))

	c := d.toStateCandidate()
	r.Equal(d.Owner.String(), c.Address)
	r.Equal(d.Reward.String(), c.RewardAddress)
	r.Equal(d.Votes, c.Votes)
	r.Equal(d.Name, string(c.CanName))
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
		_, _, err := csr.getCandidate(e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
		require.NoError(csm.putCandidate(e.d))
		d1, _, err := csr.getCandidate(e.d.Owner)
		require.NoError(err)
		require.Equal(e.d, d1)
	}

	// get all candidates
	all, _, err := csr.getAllCandidates()
	require.NoError(err)
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
		require.NoError(csm.delCandidate(e.d.Owner))
		_, _, err := csr.getCandidate(e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
