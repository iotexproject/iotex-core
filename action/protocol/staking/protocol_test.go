// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestProtocol(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey),
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
	stk := NewProtocol(nil, sm, genesis.Staking{
		VoteWeightCalConsts: genesis.VoteWeightCalConsts{
			DurationLg: 1.2,
			AutoStake:  1.05,
			SelfStake:  1.05,
		},
	})
	r.NotNil(stk)

	ctx := context.Background()
	r.NoError(stk.Start(ctx))
	r.Equal(0, stk.inMemCandidates.Size())
	buckets, err := getAllBuckets(sm)
	r.NoError(err)
	r.Equal(0, len(buckets))

	// write a number of candidates and buckets into stateDB
	for _, e := range testCandidates {
		r.NoError(putCandidate(sm, e.d))
	}
	for _, e := range tests {
		vb := NewVoteBucket(e.cand, e.owner, e.amount, e.duration, time.Now(), true)
		_, err := putBucketAndIndex(sm, vb)
		r.NoError(err)
	}

	// load candidates from stateDB and verify
	r.NoError(stk.Start(ctx))
	r.Equal(len(testCandidates), stk.inMemCandidates.Size())
	for _, e := range testCandidates {
		r.True(stk.inMemCandidates.ContainsOwner(e.d.Owner))
		r.True(stk.inMemCandidates.ContainsName(e.d.Name))
		r.True(stk.inMemCandidates.ContainsOperator(e.d.Operator))
		r.Equal(e.d, stk.inMemCandidates.GetByOwner(e.d.Owner))
	}

	// load buckets from stateDB and verify
	buckets, err = getAllBuckets(sm)
	r.NoError(err)
	r.Equal(len(tests), len(buckets))
	for _, e := range tests {
		for i := range buckets {
			if buckets[i].StakedAmount == e.amount {
				vb := NewVoteBucket(e.cand, e.owner, e.amount, e.duration, time.Now(), true)
				r.Equal(vb, buckets[i])
				break
			}
		}
	}
}
