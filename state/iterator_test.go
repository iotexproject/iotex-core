// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided   'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to   the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed   by Apache
// License 2.0 that can be found in the LICENSE file.

package state

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
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

	states := make([][]byte, len(cands))
	for i, cand := range cands {
		bytes, err := cand.Serialize()
		r.NoError(err)
		states[i] = bytes
	}

	iter := NewIterator(states)
	r.Equal(iter.Size(), len(states))

	for _, cand := range cands {
		c := &Candidate{}
		err := iter.Next(c)
		r.NoError(err)

		r.True(c.Equal(cand))
	}

	var noneExistCand Candidate
	r.Equal(iter.Next(&noneExistCand), ErrOutOfBoundary)
}
