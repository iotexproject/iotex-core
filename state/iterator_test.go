// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestIterator(t *testing.T) {
	r := require.New(t)

	cands := CandidateList{
		&Candidate{
			Address: identityset.Address(4).String(),
			Votes:   big.NewInt(4),
		},
		&Candidate{
			Address: identityset.Address(3).String(),
			Votes:   big.NewInt(3),
		},
		&Candidate{
			Address: identityset.Address(2).String(),
			Votes:   big.NewInt(2),
		},
		&Candidate{
			Address: identityset.Address(1).String(),
			Votes:   big.NewInt(1),
		},
	}

	keys := make([][]byte, len(cands))
	states := make([][]byte, len(cands))
	for i, cand := range cands {
		keys[i] = []byte(cand.Address)
		bytes, err := cand.Serialize()
		r.NoError(err)
		states[i] = bytes
	}

	_, err := NewIterator(nil, states)
	r.Equal(err, ErrInConsistentLength)
	_, err = NewIterator(keys, nil)
	r.Equal(err, ErrInConsistentLength)

	iter, err := NewIterator(keys, states)
	r.NoError(err)
	r.Equal(iter.Size(), len(states))

	for _, cand := range cands {
		c := &Candidate{}
		_, err := iter.Next(c)
		r.NoError(err)

		r.True(c.Equal(cand))
	}

	var noneExistCand Candidate
	_, err = iter.Next(&noneExistCand)
	r.Equal(err, ErrOutOfBoundary)
}
