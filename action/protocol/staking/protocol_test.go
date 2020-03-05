// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestProtocol(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)

	// test loading with no candidate in stateDB
	stk := NewProtocol(nil, sm, Configuration{
		VoteCal: VoteWeightCalConsts{
			DurationLg: 1.2,
			AutoStake:  1.05,
			SelfStake:  1.05,
		},
	})
	r.NotNil(stk)
	r.NoError(stk.Start(context.Background()))
	r.Equal(0, stk.inMemCandidates.Size())

	// write a number of candidates into stateDB
	for _, e := range testCandidates {
		r.NoError(putCandidate(sm, e.d))
	}

	// load candidates from stateDB and verify
	r.NoError(stk.Start(context.Background()))
	r.Equal(len(testCandidates), stk.inMemCandidates.Size())

	for _, e := range testCandidates {
		r.True(stk.inMemCandidates.ContainsOwner(e.d.Owner))
		r.True(stk.inMemCandidates.ContainsName(e.d.Name))
		r.True(stk.inMemCandidates.ContainsOperator(e.d.Operator))
		r.Equal(e.d, stk.inMemCandidates.GetByOwner(e.d.Owner))
	}
}
