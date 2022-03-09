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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestProtocol(t *testing.T) {
	r := require.New(t)

	// make sure the prefix stays constant, they affect the key to store objects to DB
	r.Equal(byte(0), _const)
	r.Equal(byte(1), _bucket)
	r.Equal(byte(2), _voterIndex)
	r.Equal(byte(3), _candIndex)

	ctrl := gomock.NewController(t)
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
	buckets, _, err := getAllBuckets(sm)
	r.NoError(err)
	r.Equal(0, len(buckets))
	c, _, err := getAllCandidates(sm)
	r.Equal(state.ErrStateNotExist, err)
	r.Equal(0, len(c))

	// address package also defined protocol address, make sure they match
	r.Equal(stk.addr.Bytes(), address.StakingProtocolAddrHash[:])

	// write a number of buckets into stateDB
	for _, e := range tests {
		vb := NewVoteBucket(e.cand, e.owner, e.amount, e.duration, time.Now(), true)
		index, err := putBucketAndIndex(sm, vb)
		r.NoError(err)
		r.Equal(index, vb.Index)
	}

	// load candidates from stateDB and verify
	ctx := genesis.WithGenesisContext(context.Background(), genesis.Default)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
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

	// load all candidates from stateDB and verify
	all, _, err := getAllCandidates(sm)
	r.NoError(err)
	r.Equal(len(testCandidates), len(all))
	for _, e := range testCandidates {
		for i := range all {
			if all[i].Name == e.d.Name {
				r.Equal(e.d, all[i])
				break
			}
		}
	}

	// csm's candidate center should be identical to all candidates in stateDB
	c1, err := all.toStateCandidateList()
	r.NoError(err)
	c2, err := csm.DirtyView().candCenter.All().toStateCandidateList()
	r.NoError(err)
	r.Equal(c1, c2)

	// load buckets from stateDB and verify
	buckets, _, err = getAllBuckets(sm)
	r.NoError(err)
	r.Equal(len(tests), len(buckets))
	// delete one bucket
	r.NoError(delBucket(sm, 1))
	buckets, _, err = getAllBuckets(sm)
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

func TestCreatePreStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	p, err := NewProtocol(nil, genesis.Default.Staking, nil, genesis.Default.GreenlandBlockHeight)
	require.NoError(err)
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), genesis.Default),
		protocol.BlockCtx{
			BlockHeight: genesis.Default.GreenlandBlockHeight - 1,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	require.NoError(sm.WriteView(protocolID, v))
	csm, err := NewCandidateStateManager(sm, false)
	require.NoError(err)
	require.NotNil(csm)
	_, err = NewCandidateStateManager(sm, true)
	require.Error(err)
	require.NoError(p.CreatePreStates(ctx, sm))
	_, err = sm.State(nil, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(bucketPoolAddrKey))
	require.EqualError(errors.Cause(err), state.ErrStateNotExist.Error())
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: genesis.Default.GreenlandBlockHeight + 1,
		},
	)
	require.NoError(p.CreatePreStates(ctx, sm))
	_, err = sm.State(nil, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(bucketPoolAddrKey))
	require.EqualError(errors.Cause(err), state.ErrStateNotExist.Error())
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: genesis.Default.GreenlandBlockHeight,
		},
	)
	require.NoError(p.CreatePreStates(ctx, sm))
	total := &totalAmount{}
	_, err = sm.State(total, protocol.NamespaceOption(StakingNameSpace), protocol.KeyOption(bucketPoolAddrKey))
	require.NoError(err)
}

func Test_CreatePreStatesWithRegisterProtocol(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	testPath, err := testutil.PathOfTempFile("test-bucket")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	store := db.NewBoltDB(cfg)
	cbi, err := NewStakingCandidatesBucketsIndexer(store)
	require.NoError(err)

	ctx := context.Background()
	require.NoError(cbi.Start(ctx))
	p, err := NewProtocol(nil, genesis.Default.Staking, cbi, genesis.Default.GreenlandBlockHeight)
	require.NoError(err)

	rol := rolldpos.NewProtocol(23, 4, 3)
	reg := protocol.NewRegistry()
	reg.Register("rolldpos", rol)

	ctx = protocol.WithRegistry(ctx, reg)
	ctx = protocol.WithBlockCtx(
		genesis.WithGenesisContext(ctx, genesis.Default),
		protocol.BlockCtx{
			BlockHeight: genesis.Default.GreenlandBlockHeight,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	require.NoError(sm.WriteView(protocolID, v))
	_, err = NewCandidateStateManager(sm, true)
	require.Error(err)

	require.NoError(p.CreatePreStates(ctx, sm))
}

func Test_CreateGenesisStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	selfStake, _ := new(big.Int).SetString("1200000000000000000000000", 10)
	cfg := genesis.Default.Staking

	testBootstrapCandidates := []struct {
		BootstrapCandidate []genesis.BootstrapCandidate
		errStr             string
	}{
		{
			[]genesis.BootstrapCandidate{
				{
					OwnerAddress:      "xxxxxxxxxxxxxxxx",
					OperatorAddress:   identityset.Address(23).String(),
					RewardAddress:     identityset.Address(23).String(),
					Name:              "test1",
					SelfStakingTokens: "test123",
				},
			},
			"address prefix io don't match",
		},
		{
			[]genesis.BootstrapCandidate{
				{
					OwnerAddress:      identityset.Address(22).String(),
					OperatorAddress:   "xxxxxxxxxxxxxxxx",
					RewardAddress:     identityset.Address(23).String(),
					Name:              "test1",
					SelfStakingTokens: selfStake.String(),
				},
			},
			"address prefix io don't match",
		},
		{
			[]genesis.BootstrapCandidate{
				{
					OwnerAddress:      identityset.Address(22).String(),
					OperatorAddress:   identityset.Address(23).String(),
					RewardAddress:     "xxxxxxxxxxxxxxxx",
					Name:              "test1",
					SelfStakingTokens: selfStake.String(),
				},
			},
			"address prefix io don't match",
		},
		{
			[]genesis.BootstrapCandidate{
				{
					OwnerAddress:      identityset.Address(22).String(),
					OperatorAddress:   identityset.Address(23).String(),
					RewardAddress:     identityset.Address(23).String(),
					Name:              "test1",
					SelfStakingTokens: "test123",
				},
			},
			"invalid staking amount",
		},
		{
			[]genesis.BootstrapCandidate{
				{
					OwnerAddress:      identityset.Address(22).String(),
					OperatorAddress:   identityset.Address(23).String(),
					RewardAddress:     identityset.Address(23).String(),
					Name:              "test1",
					SelfStakingTokens: selfStake.String(),
				},
				{
					OwnerAddress:      identityset.Address(24).String(),
					OperatorAddress:   identityset.Address(25).String(),
					RewardAddress:     identityset.Address(25).String(),
					Name:              "test2",
					SelfStakingTokens: selfStake.String(),
				},
			},
			"",
		},
	}
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), genesis.Default),
		protocol.BlockCtx{
			BlockHeight: genesis.Default.GreenlandBlockHeight - 1,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	for _, test := range testBootstrapCandidates {
		cfg.BootstrapCandidates = test.BootstrapCandidate
		p, err := NewProtocol(nil, cfg, nil, genesis.Default.GreenlandBlockHeight)
		require.NoError(err)

		v, err := p.Start(ctx, sm)
		require.NoError(err)
		require.NoError(sm.WriteView(protocolID, v))

		err = p.CreateGenesisStates(ctx, sm)
		if err != nil {
			require.Contains(err.Error(), test.errStr)
		}
	}
}
