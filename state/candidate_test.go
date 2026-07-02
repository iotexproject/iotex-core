// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestCandidateEqual(t *testing.T) {
	r := require.New(t)
	testCases := []struct {
		cands []*Candidate
		flag  bool
	}{
		{
			[]*Candidate{
				{
					Address: identityset.Address(29).String(),
					Votes:   big.NewInt(2),
				},
				{
					Address: identityset.Address(29).String(),
					Votes:   big.NewInt(2),
				},
			},
			true,
		},
		{
			[]*Candidate{
				{
					Address: identityset.Address(29).String(),
					Votes:   big.NewInt(2),
				},
				{
					Address: identityset.Address(29).String(),
					Votes:   big.NewInt(1),
				},
			},
			false,
		},
	}
	for _, c := range testCases {
		r.Equal(c.flag, c.cands[0].Equal(c.cands[1]))
	}
}

func TestCandidateClone(t *testing.T) {
	r := require.New(t)
	cand1 := &Candidate{
		Address: identityset.Address(29).String(),
		Votes:   big.NewInt(2),
	}
	r.True(cand1.Equal(cand1.Clone()))
}

// TestCandidateCommissionRate verifies IIP-59's CommissionRate field is
// carried through Equal/Clone and proto roundtrip.
func TestCandidateCommissionRate(t *testing.T) {
	r := require.New(t)
	cand := &Candidate{
		Identity:       identityset.Address(29).String(),
		Address:        identityset.Address(30).String(),
		Votes:          big.NewInt(100),
		RewardAddress:  identityset.Address(31).String(),
		CommissionRate: 1500, // 15%
	}

	// Equal: differing CommissionRate must compare not-equal.
	other := cand.Clone()
	other.CommissionRate = 1000
	r.False(cand.Equal(other), "Equal must reflect CommissionRate")

	// Clone: must carry the field across.
	clone := cand.Clone()
	r.True(cand.Equal(clone))
	r.Equal(uint64(1500), clone.CommissionRate)

	// Proto roundtrip: serialize and deserialize must preserve the field.
	buf, err := cand.Serialize()
	r.NoError(err)
	roundtrip := &Candidate{}
	r.NoError(roundtrip.Deserialize(buf))
	r.Equal(uint64(1500), roundtrip.CommissionRate)
	r.True(cand.Equal(roundtrip))
}

func TestCandidateListSerializeAndDeserialize(t *testing.T) {
	r := require.New(t)
	list1 := CandidateList{
		&Candidate{
			Address: identityset.Address(2).String(),
			Votes:   big.NewInt(2),
		},
	}
	sbytes, err := list1.Serialize()
	r.NoError(err)

	list2 := CandidateList{}
	err = list2.Deserialize(sbytes)
	r.NoError(err)
	r.Equal(list1.Len(), list2.Len())
	for i, c := range list2 {
		r.True(c.Equal(list1[i]))
	}
}

func TestCandidate(t *testing.T) {
	require := require.New(t)

	cand1 := &Candidate{
		Address: identityset.Address(28).String(),
		Votes:   big.NewInt(1),
	}
	cand2 := &Candidate{
		Address: identityset.Address(29).String(),
		Votes:   big.NewInt(2),
	}
	cand3 := &Candidate{
		Address: identityset.Address(30).String(),
		Votes:   big.NewInt(3),
	}

	cand1Addr, err := address.FromString(cand1.Address)
	require.NoError(err)
	cand1Hash := hash.BytesToHash160(cand1Addr.Bytes())

	cand2Addr, err := address.FromString(cand2.Address)
	require.NoError(err)
	cand2Hash := hash.BytesToHash160(cand2Addr.Bytes())

	cand3Addr, err := address.FromString(cand3.Address)
	require.NoError(err)
	cand3Hash := hash.BytesToHash160(cand3Addr.Bytes())

	candidateMap := make(CandidateMap)
	candidateMap[cand1Hash] = cand1
	candidateMap[cand2Hash] = cand2
	candidateMap[cand3Hash] = cand3

	candidateList, err := MapToCandidates(candidateMap)
	require.NoError(err)
	require.Equal(3, len(candidateList))
	sort.Sort(candidateList)

	require.Equal(identityset.Address(30).String(), candidateList[0].Address)
	require.Equal(identityset.Address(29).String(), candidateList[1].Address)
	require.Equal(identityset.Address(28).String(), candidateList[2].Address)

	candidatesBytes, err := candidateList.Serialize()
	require.NoError(err)
	var candidates CandidateList
	require.NoError(candidates.Deserialize(candidatesBytes))
	require.Equal(3, len(candidates))
	require.Equal(identityset.Address(30).String(), candidates[0].Address)
	require.Equal(identityset.Address(29).String(), candidates[1].Address)
	require.Equal(identityset.Address(28).String(), candidates[2].Address)

	candidateMap, err = CandidatesToMap(candidateList)
	require.NoError(err)
	require.Equal(3, len(candidateMap))
	require.Equal(uint64(1), candidateMap[cand1Hash].Votes.Uint64())
	require.Equal(uint64(2), candidateMap[cand2Hash].Votes.Uint64())
	require.Equal(uint64(3), candidateMap[cand3Hash].Votes.Uint64())
}
