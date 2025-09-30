// Copyright (c) 2020 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
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
	csr := newCandidateStateReader(sm)
	csmTemp := newCandidateStateManager(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
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
	g := genesis.TestDefault()

	// test loading with no candidate in stateDB
	stk, err := NewProtocol(HelperCtx{
		DepositGas:    nil,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil, nil)
	r.NotNil(stk)
	r.NoError(err)
	buckets, _, err := csr.NativeBuckets()
	r.NoError(err)
	r.Equal(0, len(buckets))
	cc, _, err := csr.CreateCandidateCenter()
	r.NoError(err)
	r.Equal(0, len(cc.All()))

	// address package also defined protocol address, make sure they match
	r.Equal(stk.addr.Bytes(), address.StakingProtocolAddrHash[:])
	stkAddr, err := address.FromString(address.StakingProtocolAddr)
	r.NoError(err)
	r.Equal(stk.addr.Bytes(), stkAddr.Bytes())

	// write a number of buckets into stateDB
	for _, e := range tests {
		vb := NewVoteBucket(e.cand, e.owner, e.amount, e.duration, time.Now(), true)
		index, err := csmTemp.putBucketAndIndex(vb)
		r.NoError(err)
		r.Equal(index, vb.Index)
	}

	// load candidates from stateDB and verify
	g.QuebecBlockHeight = 1
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 10})
	ctx = protocol.WithFeatureCtx(ctx)
	v, err := stk.Start(ctx, sm)
	r.NoError(err)
	r.NoError(sm.WriteView(_protocolID, v))
	_, ok := v.(*viewData)
	r.True(ok)

	csm, err := NewCandidateStateManager(sm)
	r.NoError(err)
	// load a number of candidates
	for _, e := range testCandidates {
		r.NoError(csm.Upsert(e.d))
	}
	r.NoError(csm.Commit(ctx))
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
	cc, _, err = csr.CreateCandidateCenter()
	r.NoError(err)
	all := cc.All()
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
	buckets, _, err = csr.NativeBuckets()
	r.NoError(err)
	r.Equal(len(tests), len(buckets))
	// delete one bucket
	r.NoError(csm.delBucket(1))
	buckets, _, err = csr.NativeBuckets()
	require.NoError(t, err)
	r.NoError(csm.delBucket(1))
	buckets, _, err = csr.NativeBuckets()
	require.NoError(t, err)
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
	g := genesis.TestDefault()
	p, err := NewProtocol(HelperCtx{
		DepositGas:    nil,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight:    g.Staking.VoteWeightCalConsts,
			ReviseHeights: []uint64{g.GreenlandBlockHeight}},
	}, nil, nil, nil, nil)
	require.NoError(err)
	ctx := protocol.WithBlockCtx(
		genesis.WithGenesisContext(context.Background(), g),
		protocol.BlockCtx{
			BlockHeight: g.GreenlandBlockHeight - 1,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	require.NoError(sm.WriteView(_protocolID, v))
	csm, err := NewCandidateStateManager(sm)
	require.NoError(err)
	require.NotNil(csm)
	require.NoError(p.CreatePreStates(ctx, sm))
	_, err = sm.State(nil, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
	require.EqualError(errors.Cause(err), state.ErrStateNotExist.Error())
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: g.GreenlandBlockHeight + 1,
		},
	)
	require.NoError(p.CreatePreStates(ctx, sm))
	_, err = sm.State(nil, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
	require.EqualError(errors.Cause(err), state.ErrStateNotExist.Error())
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: g.GreenlandBlockHeight,
		},
	)
	require.NoError(p.CreatePreStates(ctx, sm))
	total := &totalAmount{}
	_, err = sm.State(total, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(_bucketPoolAddrKey))
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
	g := genesis.TestDefault()
	p, err := NewProtocol(HelperCtx{
		DepositGas:    nil,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight:    g.Staking.VoteWeightCalConsts,
			ReviseHeights: []uint64{g.GreenlandBlockHeight},
		},
	}, nil, cbi, nil, nil)
	require.NoError(err)

	rol := rolldpos.NewProtocol(23, 4, 3)
	reg := protocol.NewRegistry()
	reg.Register("rolldpos", rol)

	ctx = protocol.WithRegistry(ctx, reg)
	ctx = protocol.WithBlockCtx(
		genesis.WithGenesisContext(ctx, g),
		protocol.BlockCtx{
			BlockHeight: g.GreenlandBlockHeight,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	_, err = NewCandidateStateManager(sm)
	require.Error(err)

	require.NoError(sm.WriteView(_protocolID, v))
	require.NoError(p.CreatePreStates(ctx, sm))
}

func TestCreatePreStatesMigration(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := testdb.NewMockStateManager(ctrl)
	g := genesis.TestDefault()
	mockView := NewMockContractStakeView(ctrl)
	mockContractStaking := NewMockContractStakingIndexer(ctrl)
	mockContractStaking.EXPECT().ContractAddress().Return(identityset.Address(1)).Times(1)
	mockContractStaking.EXPECT().LoadStakeView(gomock.Any(), gomock.Any()).Return(mockView, nil).Times(1)
	mockContractStaking.EXPECT().StartHeight().Return(uint64(0)).Times(1)
	mockContractStaking.EXPECT().Height().Return(uint64(0), nil).Times(1)
	p, err := NewProtocol(HelperCtx{
		DepositGas:    nil,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight:    g.Staking.VoteWeightCalConsts,
			ReviseHeights: []uint64{g.GreenlandBlockHeight}},
	}, nil, nil, nil, mockContractStaking)
	require.NoError(err)
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: g.XinguBlockHeight,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	require.NoError(sm.WriteView(_protocolID, v))
	mockView.EXPECT().Migrate(gomock.Any(), gomock.Any()).Return(errors.New("migration error")).Times(1)
	require.ErrorContains(p.CreatePreStates(ctx, sm), "migration error")
	mockView.EXPECT().Migrate(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	require.NoError(p.CreatePreStates(ctx, sm))
	require.NoError(p.CreatePreStates(protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: g.XinguBlockHeight - 1,
		},
	), sm))
	require.NoError(p.CreatePreStates(protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight: g.XinguBlockHeight + 1,
		},
	), sm))
}

func Test_CreateGenesisStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	selfStake, _ := new(big.Int).SetString("1200000000000000000000000", 10)
	g := genesis.TestDefault()
	cfg := g.Staking

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
			"address length = 16, expecting 41",
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
			"address length = 16, expecting 41",
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
			"address length = 16, expecting 41",
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
			"invalid amount",
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
		genesis.WithGenesisContext(context.Background(), g),
		protocol.BlockCtx{
			BlockHeight: g.GreenlandBlockHeight - 1,
		},
	)
	ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
	for _, test := range testBootstrapCandidates {
		cfg.BootstrapCandidates = test.BootstrapCandidate
		p, err := NewProtocol(HelperCtx{
			DepositGas:    nil,
			BlockInterval: getBlockInterval,
		}, &BuilderConfig{
			Staking:                       cfg,
			PersistStakingPatchBlock:      math.MaxUint64,
			SkipContractStakingViewHeight: math.MaxUint64,
			Revise: ReviseConfig{
				VoteWeight: g.Staking.VoteWeightCalConsts,
			},
		}, nil, nil, nil, nil)
		require.NoError(err)

		v, err := p.Start(ctx, sm)
		require.NoError(err)
		require.NoError(sm.WriteView(_protocolID, v))

		err = p.CreateGenesisStates(ctx, sm)
		if err != nil {
			require.Contains(err.Error(), test.errStr)
		}
	}
}

func TestIsSelfStakeBucket(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	featureCtxPostHF := protocol.FeatureCtx{
		DisableDelegateEndorsement: false,
		EnforceLegacyEndorsement:   true,
	}
	featureCtxPreHF := protocol.FeatureCtx{
		DisableDelegateEndorsement: true,
		EnforceLegacyEndorsement:   true,
	}
	t.Run("normal bucket", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(11), identityset.Address(11), "1200000000000000000000000", 100, true, false, nil, 0},
			{identityset.Address(11), identityset.Address(1), "1200000000000000000000000", 100, true, false, nil, 0},
		}
		candCfgs := []*candidateConfig{}
		sm, _, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[0])
		r.NoError(err)
		r.False(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[0])
		r.NoError(err)
		r.False(selfStake)
	})
	t.Run("self-stake bucket", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, true, nil, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "cand1"},
		}
		sm, _, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)

		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
	})
	t.Run("self-stake bucket unstaked", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, true, &timeBeforeBlockII, 0},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "cand1"},
		}
		sm, _, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)

		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[0])
		r.NoError(err)
		r.False(selfStake)
	})
	t.Run("endorsed bucket but not self-staked", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 100, true, true, nil, 0},
			{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 100, true, false, nil, endorsementNotExpireHeight},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "cand1"},
		}
		sm, _, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)

		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[1])
		r.NoError(err)
		r.False(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[1])
		r.NoError(err)
		r.False(selfStake)
	})
	t.Run("endorsed and self-staked", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 100, true, true, nil, endorsementNotExpireHeight},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "cand1"},
		}
		sm, _, buckets, _ := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)

		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
	})
	t.Run("endorsement withdrawing", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 100, true, true, nil, 10},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "cand1"},
		}
		sm, _, buckets, _ := initTestStateWithHeight(t, ctrl, bucketCfgs, candCfgs, 0)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)

		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
	})
	t.Run("endorsement expired", func(t *testing.T) {
		bucketCfgs := []*bucketConfig{
			{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 100, true, true, nil, 1},
		}
		candCfgs := []*candidateConfig{
			{identityset.Address(1), identityset.Address(11), identityset.Address(21), "cand1"},
		}
		sm, _, buckets, _ := initTestStateWithHeight(t, ctrl, bucketCfgs, candCfgs, 2)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		selfStake, err := isSelfStakeBucket(featureCtxPreHF, csm, buckets[0])
		r.NoError(err)
		r.True(selfStake)
		selfStake, err = isSelfStakeBucket(featureCtxPostHF, csm, buckets[0])
		r.NoError(err)
		r.False(selfStake)
	})
}

func TestSlashCandidate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	owner := identityset.Address(1)
	operator := identityset.Address(2)
	reward := identityset.Address(3)
	selfStake := big.NewInt(1000)
	bucket := NewVoteBucket(owner, owner, new(big.Int).Set(selfStake), 10, time.Now(), true)
	bucketIdx := uint64(0)
	bucket.Index = bucketIdx

	cand := &Candidate{
		Owner:              owner,
		Operator:           operator,
		Reward:             reward,
		Name:               "cand1",
		Votes:              big.NewInt(1000),
		SelfStakeBucketIdx: bucketIdx,
		SelfStake:          new(big.Int).Set(selfStake),
	}
	cc, err := NewCandidateCenter(CandidateList{cand})
	require.NoError(err)
	require.NoError(sm.WriteView(_protocolID, &viewData{
		candCenter: cc,
		bucketPool: &BucketPool{
			enableSMStorage: true,
			total: &totalAmount{
				amount: big.NewInt(0),
			},
		},
	}))
	csm, err := NewCandidateStateManager(sm)
	require.NoError(err)

	p := &Protocol{
		config: Configuration{
			RegistrationConsts: RegistrationConsts{
				MinSelfStake: big.NewInt(1000),
			},
			MinSelfStakeToBeActive: big.NewInt(590),
		},
	}
	ctx := context.Background()
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: 100,
	})
	ctx = protocol.WithFeatureCtx(ctx)

	t.Run("nil amount", func(t *testing.T) {
		err := p.SlashCandidate(ctx, sm, owner, nil)
		require.ErrorContains(err, "nil or non-positive amount")
	})

	t.Run("zero amount", func(t *testing.T) {
		err := p.SlashCandidate(ctx, sm, owner, big.NewInt(0))
		require.ErrorContains(err, "nil or non-positive amount")
	})

	t.Run("candidate not exist", func(t *testing.T) {
		err := p.SlashCandidate(ctx, sm, identityset.Address(9), big.NewInt(1))
		require.ErrorContains(err, "does not exist")
	})

	t.Run("bucket not exist", func(t *testing.T) {
		err := p.SlashCandidate(ctx, sm, owner, big.NewInt(1))
		require.ErrorContains(err, "failed to fetch bucket")
	})

	_, err = csm.putBucket(bucket)
	require.NoError(err)
	require.NoError(csm.DebitBucketPool(bucket.StakedAmount, true))
	cl, err := p.ActiveCandidates(ctx, sm, 0)
	require.NoError(err)
	require.Equal(1, len(cl))

	t.Run("amount greater than staked", func(t *testing.T) {
		err := p.SlashCandidate(ctx, sm, owner, big.NewInt(2000))
		require.ErrorContains(err, "is greater than staked amount")
	})

	t.Run("success", func(t *testing.T) {
		amount := big.NewInt(400)
		remaining := bucket.StakedAmount.Sub(bucket.StakedAmount, amount)
		require.NoError(p.SlashCandidate(ctx, sm, owner, amount))
		cl, err = p.ActiveCandidates(ctx, sm, 0)
		require.NoError(err)
		require.Equal(0, len(cl))
		ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: genesis.Default.XinguBlockHeight,
		}))
		cl, err = p.ActiveCandidates(
			ctx,
			sm,
			0,
		)
		require.NoError(err)
		require.Equal(1, len(cl))
		bucket, err := csm.NativeBucket(bucketIdx)
		require.NoError(err)
		require.Equal(remaining.String(), bucket.StakedAmount.String())
		cand := csm.GetByIdentifier(owner)
		require.Equal(remaining.String(), cand.SelfStake.String())
		require.NoError(p.SlashCandidate(ctx, sm, owner, big.NewInt(11)))
		cl, err = p.ActiveCandidates(
			ctx,
			sm,
			0,
		)
		require.NoError(err)
		require.Equal(0, len(cl))
		require.NoError(cand.AddSelfStake(big.NewInt(21)))
		require.NoError(csm.Upsert(cand))
		cl, err = p.ActiveCandidates(
			ctx,
			sm,
			0,
		)
		require.NoError(err)
		require.Equal(1, len(cl))
	})
}
