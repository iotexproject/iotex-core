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
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestImplicitLog(t *testing.T) {
	require := require.New(t)

	h := hash.BytesToHash256([]byte(HandleUnstake))
	log := &iotextypes.Log{
		ContractAddress: "io1qnpz47hx5q6r3w876axtrn6yz95d70cjl35r53",
		Topics:          [][]byte{h[:], hash.ZeroHash256[:], hash.ZeroHash256[:], hash.ZeroHash256[:]},
		Data:            nil,
		BlkHeight:       1,
		ActHash:         hash.ZeroHash256[:],
		Index:           1,
	}
	_, ok := BucketIndexFromReceiptLog(log)
	require.True(ok)

	// verify implicit log topic does not collide with normal log
	implicitTopis := [][]byte{
		action.InContractTransfer[:],
		action.BucketCreateAmount[:],
		action.BucketDepositAmount[:],
		action.BucketWithdrawAmount[:],
		action.CandidateSelfStake[:],
		action.CandidateRegistrationFee[:],
	}
	for _, v := range implicitTopis {
		log.Topics[0] = v
		_, ok := BucketIndexFromReceiptLog(log)
		require.False(ok)
	}
	log.Topics[0] = h[:]
	_, ok = BucketIndexFromReceiptLog(log)
	require.True(ok)
}

func TestProtocol(t *testing.T) {
	r := require.New(t)

	// make sure the prefix stays constant, they affect the key to store objects to DB
	r.Equal(byte(0), _const)
	r.Equal(byte(1), _bucket)
	r.Equal(byte(2), _voterIndex)
	r.Equal(byte(3), _candIndex)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)
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
	stk, err := NewProtocol(nil, genesis.Default.Staking)
	r.NotNil(stk)
	r.NoError(err)
	buckets, err := getAllBuckets(sm)
	r.NoError(err)
	r.Equal(0, len(buckets))

	// address package also defined protocol address, make sure they match
	r.Equal(hash.BytesToHash160(stk.addr.Bytes()), address.StakingProtocolAddrHash)

	// write a number of candidates and buckets into stateDB
	for _, e := range testCandidates {
		r.NoError(putCandidate(sm, e.d))
	}
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
	csm, err := NewCandidateStateManager(sm)
	r.NoError(err)
	r.Equal(len(testCandidates), csm.Size())
	for _, e := range testCandidates {
		r.True(csm.ContainsOwner(e.d.Owner))
		r.True(csm.ContainsName(e.d.Name))
		r.True(csm.ContainsOperator(e.d.Operator))
		r.Equal(e.d, csm.GetByOwner(e.d.Owner))
	}

	// active list should filter out 2 cands with not enough self-stake
	h, _ := sm.Height()
	cand, err := stk.ActiveCandidates(ctx, sm, h)
	r.NoError(err)
	r.Equal(len(testCandidates)-2, len(cand))
	for i := range cand {
		c := testCandidates[i]
		// index is the order of sorted list
		e := cand[c.index]
		r.Equal(e.Votes, c.d.Votes)
		r.Equal(e.RewardAddress, c.d.Reward.String())
		r.Equal(string(e.CanName), c.d.Name)
		r.True(c.d.SelfStake.Cmp(unit.ConvertIotxToRau(1200000)) >= 0)
	}

	// load buckets from stateDB and verify
	buckets, err = getAllBuckets(sm)
	r.NoError(err)
	r.Equal(len(tests), len(buckets))
	// delete one bucket
	r.NoError(delBucket(sm, 1))
	buckets, err = getAllBuckets(sm)
	r.NoError(err)
	r.Equal(len(tests)-1, len(buckets))
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
