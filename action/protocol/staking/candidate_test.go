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

	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestCandidateSerialize(t *testing.T) {
	r := require.New(t)
	d := NewCandidate(identityset.Address(1), identityset.Address(1), identityset.Address(1), "testname1234", 0, big.NewInt(2100000000))

	b, err := d.Serialize()
	r.NoError(err)
	d1 := &Candidate{}
	r.NoError(d1.Deserialize(b))
	r.Equal(d, d1)

	d2 := d.Clone()
	r.Equal(d, d2)
	d.AddVote(big.NewInt(100))
	r.NotEqual(d, d2)
}

var (
	tests = []struct {
		d     *Candidate
		index int
	}{
		{
			&Candidate{
				Owner:              identityset.Address(1),
				Operator:           identityset.Address(11),
				Reward:             identityset.Address(1),
				Name:               "test1",
				Votes:              big.NewInt(2),
				SelfStakeBucketIdx: 1,
				SelfStake:          big.NewInt(1200000),
			},
			2,
		},
		{
			&Candidate{
				Owner:              identityset.Address(2),
				Operator:           identityset.Address(12),
				Reward:             identityset.Address(1),
				Name:               "test2",
				Votes:              big.NewInt(3),
				SelfStakeBucketIdx: 2,
				SelfStake:          big.NewInt(1200000),
			},
			1,
		},
		{
			&Candidate{
				Owner:              identityset.Address(3),
				Operator:           identityset.Address(13),
				Reward:             identityset.Address(1),
				Name:               "test3",
				Votes:              big.NewInt(3),
				SelfStakeBucketIdx: 3,
				SelfStake:          big.NewInt(1200000),
			},
			0,
		},
		{
			&Candidate{
				Owner:              identityset.Address(4),
				Operator:           identityset.Address(14),
				Reward:             identityset.Address(1),
				Name:               "test4",
				Votes:              big.NewInt(1),
				SelfStakeBucketIdx: 4,
				SelfStake:          big.NewInt(1200000),
			},
			3,
		},
	}
)

func TestCandCenter(t *testing.T) {
	r := require.New(t)

	m := NewCandidateCenter()
	for i, v := range tests {
		r.NoError(m.Put(tests[i].d))
		r.True(m.ContainsName(v.d.Name))
		r.Equal(v.d, m.GetByName(v.d.Name))
	}
	r.Equal(len(tests), m.Size())

	// test candidate that does not exist
	noName := identityset.Address(22)
	r.False(m.ContainsOwner(noName))
	m.Delete(noName)
	r.Equal(len(tests), m.Size())

	// test serialize
	d, err := m.Serialize()
	r.NoError(err)
	r.NoError(m.Deserialize(d))
	r.Equal(len(tests), m.Size())
	for _, v := range tests {
		r.True(m.ContainsName(v.d.Name))
		r.True(m.ContainsOwner(v.d.Owner))
		r.True(m.ContainsOperator(v.d.Operator))
		r.Equal(v.d, m.GetByName(v.d.Name))
	}

	// verify the serialization is sorted
	c := CandidateList{tests[0].d}
	r.NoError(c.Deserialize(d))
	r.Equal(len(tests), len(c))
	for _, v := range tests {
		r.Equal(v.d, c[v.index])
	}

	// test delete
	for i, v := range tests {
		m.Delete(v.d.Owner)
		r.False(m.ContainsOwner(v.d.Owner))
		r.False(m.ContainsName(v.d.Name))
		r.False(m.ContainsOperator(v.d.Operator))
		r.Equal(len(tests)-i-1, m.Size())
	}
}

func TestGetPutCandidate(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)

	// put candidates and get
	for _, e := range tests {
		_, err := getCandidate(sm, e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
		require.NoError(putCandidate(sm, e.d.Owner, e.d))
		d1, err := getCandidate(sm, e.d.Owner)
		require.NoError(err)
		require.Equal(e.d, d1)
	}

	// delete buckets and get
	for _, e := range tests {
		require.NoError(delCandidate(sm, e.d.Owner))
		_, err := getCandidate(sm, e.d.Owner)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
