// Copyright (c) 2024 IoTeX Foundation
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
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

var (
	timeBeforeBlockI  = time.Unix(1502044560, 0)
	timeBeforeBlockII = time.Unix(1602044560, 0)
	timeBlock         = time.Unix(1702044560, 0)
)

type (
	bucketConfig struct {
		Candidate       address.Address
		Owner           address.Address
		StakedAmountStr string
		StakedDuration  uint32
		AutoStake       bool
		SelfStake       bool
		UnstakeTime     *time.Time
		EndorseExpire   uint64
	}
	candidateConfig struct {
		Owner    address.Address
		Operator address.Address
		Reward   address.Address
		Name     string
	}
	expectCandidate struct {
		owner                  address.Address
		candSelfStakeIndex     uint64
		candSelfStakeAmountStr string
		candVoteStr            string
	}
	expectBucket struct {
		id                      uint64
		candidate               address.Address
		hasEndorsement          bool
		endorsementExpireHeight uint64
	}
)

func initTestState(t *testing.T, ctrl *gomock.Controller, bucketCfgs []*bucketConfig, candidateCfgs []*candidateConfig) (protocol.StateManager, *Protocol, []*VoteBucket, []*Candidate) {
	return initTestStateWithHeight(t, ctrl, bucketCfgs, candidateCfgs, 0)
}

func initTestStateWithHeight(t *testing.T, ctrl *gomock.Controller, bucketCfgs []*bucketConfig, candidateCfgs []*candidateConfig, height uint64) (protocol.StateManager, *Protocol, []*VoteBucket, []*Candidate) {
	require := require.New(t)
	sm := testdb.NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(height, nil).AnyTimes()
	csm := newCandidateStateManager(sm)
	esm := NewEndorsementStateManager(sm)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)

	g := genesis.TestDefault()
	// create protocol
	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                       g.Staking,
		PersistStakingPatchBlock:      math.MaxUint64,
		SkipContractStakingViewHeight: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil, nil)
	require.NoError(err)

	// set up bucket
	buckets := []*VoteBucket{}
	candVotesMap := make(map[string]*big.Int)
	selfStakeMap := make(map[string]uint64)
	for _, bktCfg := range bucketCfgs {
		amount, _ := big.NewInt(0).SetString(bktCfg.StakedAmountStr, 10)
		// bkt := NewVoteBucket(bktCfg.Candidate, bktCfg.Owner, amount, bktCfg.StakedDuration, time.Now(), bktCfg.AutoStake)
		bkt := &VoteBucket{
			Candidate:        bktCfg.Candidate,
			Owner:            bktCfg.Owner,
			StakedAmount:     amount,
			StakedDuration:   time.Duration(bktCfg.StakedDuration) * 24 * time.Hour,
			CreateTime:       timeBeforeBlockI,
			StakeStartTime:   timeBeforeBlockI,
			UnstakeStartTime: time.Unix(0, 0).UTC(),
			AutoStake:        bktCfg.AutoStake,
		}
		if bktCfg.UnstakeTime != nil {
			bkt.UnstakeStartTime = bktCfg.UnstakeTime.UTC()
		}
		_, err = csm.putBucketAndIndex(bkt)
		require.NoError(err)
		buckets = append(buckets, bkt)
		if _, ok := candVotesMap[bkt.Candidate.String()]; !ok {
			candVotesMap[bkt.Candidate.String()] = big.NewInt(0)
		}
		candVotesMap[bkt.Candidate.String()].Add(candVotesMap[bkt.Candidate.String()], p.calculateVoteWeight(bkt, bktCfg.SelfStake))
		if bktCfg.SelfStake {
			selfStakeMap[bkt.Candidate.String()] = bkt.Index
		}
		if bktCfg.EndorseExpire != 0 {
			require.NoError(esm.Put(bkt.Index, &Endorsement{ExpireHeight: bktCfg.EndorseExpire}))
		}
	}

	// set up candidate
	candidates := []*Candidate{}
	for _, candCfg := range candidateCfgs {
		selfStakeAmount := big.NewInt(0)
		selfStakeBucketID := uint64(candidateNoSelfStakeBucketIndex)
		if _, ok := selfStakeMap[candCfg.Owner.String()]; ok {
			selfStakeAmount = selfStakeAmount.SetBytes(buckets[selfStakeMap[candCfg.Owner.String()]].StakedAmount.Bytes())
			selfStakeBucketID = selfStakeMap[candCfg.Owner.String()]
		}
		votes := big.NewInt(0)
		if candVotesMap[candCfg.Owner.String()] != nil {
			votes = votes.Add(votes, candVotesMap[candCfg.Owner.String()])
		}
		cand := &Candidate{
			Owner:              candCfg.Owner,
			Operator:           candCfg.Operator,
			Reward:             candCfg.Reward,
			Name:               candCfg.Name,
			Votes:              votes,
			SelfStakeBucketIdx: selfStakeBucketID,
			SelfStake:          selfStakeAmount,
		}
		require.NoError(csm.putCandidate(cand))
		candidates = append(candidates, cand)
	}
	cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	ctx := genesis.WithGenesisContext(context.Background(), cfg)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(protocol.WithBlockCtx(ctx, protocol.BlockCtx{}))
	v, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := v.(*viewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))

	return sm, p, buckets, candidates
}

func TestProtocol_HandleCandidateSelfStake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	// NOT change existed items in initBucketCfgs and initCandidateCfgs
	// only append new items to the end of the list if needed
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "1", 1, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, false, &timeBeforeBlockII, 0},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 30, true, true, nil, 0},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 30, true, true, nil, 0},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, 1},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, 0},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, endorsementNotExpireHeight},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
	}
	initTestStateFromIds := func(bucketCfgIdx, candCfgIds []uint64) (protocol.StateManager, *Protocol, []*VoteBucket, []*Candidate) {
		bucketCfgs := []*bucketConfig{}
		for _, idx := range bucketCfgIdx {
			bucketCfgs = append(bucketCfgs, initBucketCfgs[idx])
		}
		candCfgs := []*candidateConfig{}
		for _, idx := range candCfgIds {
			candCfgs = append(candCfgs, initCandidateCfgs[idx])
		}
		return initTestState(t, ctrl, bucketCfgs, candCfgs)
	}
	sm, p, _, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)

	tests := []struct {
		name string
		// params
		initBucketCfgIds    []uint64
		initCandidateCfgIds []uint64
		initBalance         int64
		caller              address.Address
		nonce               uint64
		gasLimit            uint64
		blkGasLimit         uint64
		gasPrice            *big.Int
		bucketID            uint64
		newProtocol         bool
		// expect
		err              error
		status           iotextypes.ReceiptStatus
		expectCandidates []expectCandidate
		expectBuckets    []expectBucket
	}{
		{
			"selfstake for unstaked candidate",
			[]uint64{0, 1},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{
				{identityset.Address(1), 1, "1200000000000000000000000", "1469480667073232815766915"},
			},
			nil,
		},
		{
			"selfstake bucket amount is unsufficient",
			[]uint64{0, 1},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
			nil,
			nil,
		},
		{
			"selfstake bucket is unstaked",
			[]uint64{0, 2},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			nil,
			nil,
		},
		{
			"bucket is already selfstaked",
			[]uint64{0, 10},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			nil,
			nil,
		},
		{
			"other candidate's bucket is unauthorized",
			[]uint64{0, 4},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
			nil,
			nil,
		},
		{
			"bucket has been voted to other candidate",
			[]uint64{0, 5, 6},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			nil,
			nil,
		},
		{
			"bucket is endorsed to candidate",
			[]uint64{0, 7},
			[]uint64{0},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{
				{identityset.Address(1), 1, "1200000000000000000000000", "1635067133824581908640995"},
			},
			[]expectBucket{
				{1, identityset.Address(1), false, 0},
			},
		},
		{
			"bucket endorsement is expired",
			[]uint64{0, 8},
			[]uint64{0},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
			nil,
			nil,
		},
		{
			"candidate has already been selfstaked",
			[]uint64{3, 9},
			[]uint64{1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{
				{identityset.Address(2), 1, "1200000000000000000000000", "3104547800897814724407908"},
			},
			[]expectBucket{
				{1, identityset.Address(2), false, 0},
			},
		},
		{
			"bucket is already selfstaked by endorsement",
			[]uint64{0, 11},
			[]uint64{0, 1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			nil,
			nil,
		},
		{
			"bucket has no endorsement",
			[]uint64{0, 5},
			[]uint64{0, 1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
			nil,
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nonce := test.nonce
			if test.newProtocol {
				sm, p, _, _ = initTestStateFromIds(test.initBucketCfgIds, test.initCandidateCfgIds)
			}
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
			act := action.NewCandidateActivate(test.bucketID)
			IntrinsicGas, _ := act.IntrinsicGas()
			elp := builder.SetNonce(nonce).SetGasLimit(test.gasLimit).
				SetGasPrice(test.gasPrice).SetAction(act).Build()
			ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
				Caller:       test.caller,
				GasPrice:     test.gasPrice,
				IntrinsicGas: IntrinsicGas,
				Nonce:        nonce,
			})
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:    1,
				BlockTimeStamp: timeBlock,
				GasLimit:       test.blkGasLimit,
			})
			ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
			cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
			cfg.TsunamiBlockHeight = 1
			ctx = genesis.WithGenesisContext(ctx, cfg)
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			require.Equal(test.err, errors.Cause(p.Validate(ctx, elp, sm)))
			if test.err != nil {
				return
			}
			r, err := p.Handle(ctx, elp, sm)
			require.NoError(err)
			if r != nil {
				require.Equal(uint64(test.status), r.Status)
			} else {
				require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
			}

			if test.err == nil && test.status == iotextypes.ReceiptStatus_Success {
				// check candidate
				csm, err := NewCandidateStateManager(sm)
				require.NoError(err)
				for _, expectCand := range test.expectCandidates {
					candidate := csm.GetByOwner(expectCand.owner)
					require.NotNil(candidate)
					require.Equal(expectCand.candSelfStakeIndex, candidate.SelfStakeBucketIdx)
					require.Equal(expectCand.candSelfStakeAmountStr, candidate.SelfStake.String())
					require.Equal(expectCand.candVoteStr, candidate.Votes.String())
				}
				// check buckets
				for _, expectBkt := range test.expectBuckets {
					bkt, err := csm.NativeBucket(expectBkt.id)
					require.NoError(err)
					require.Equal(expectBkt.candidate, bkt.Candidate)
				}

				// test staker's account
				caller, err := accountutil.LoadAccount(sm, test.caller)
				require.NoError(err)
				actCost, err := elp.Cost()
				require.NoError(err)
				total := big.NewInt(0)
				require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost))
				require.Equal(nonce+1, caller.PendingNonce())
			}
		})
	}
}

func TestProtocol_HandleCandidateDeactivate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name        string
		exitBlock   uint64
		deleted     bool
		blkHeight   uint64
		setupFunc   func(*Protocol, CandidateStateManager, *Candidate) error
		expectedErr error
		verifyFunc  func(*require.Assertions, CandidateStateManager, address.Address)
	}{
		{
			name:      "request exit - success",
			exitBlock: 0,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				return csm.requestDeactivation(cand.Owner)
			},
			expectedErr: nil,
			verifyFunc: func(r *require.Assertions, csm CandidateStateManager, id address.Address) {
				r.Equal(candidateExitRequested, csm.GetByIdentifier(id).DeactivatedAt)
			},
		},
		{
			name:      "request exit - already requested",
			exitBlock: math.MaxUint64,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				return csm.requestDeactivation(cand.Owner)
			},
			expectedErr: ErrExitAlreadyRequested,
			verifyFunc:  nil,
		},
		{
			name:      "confirm exit - success",
			exitBlock: 90,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				bucket, err := csm.NativeBucket(cand.SelfStakeBucketIdx)
				if err != nil {
					return err
				}
				// Use a vote calculation function that returns self-stake votes and 0 for non-self-stake
				// This matches the original behavior: only subtract self-stake, don't add back non-self-stake
				return csm.deactivate(cand, bucket, 100, func(b *VoteBucket, selfStake bool) *big.Int {
					if selfStake {
						return p.calculateVoteWeight(b, true)
					}
					return big.NewInt(0)
				})
			},
			expectedErr: nil,
			verifyFunc: func(r *require.Assertions, csm CandidateStateManager, id address.Address) {
				cand := csm.GetByIdentifier(id)
				r.Equal(uint64(0), cand.Votes.Uint64())
				r.Equal(uint64(0), cand.SelfStake.Uint64())
			},
		},
		{
			name:      "confirm exit - not ready",
			exitBlock: 110,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				bucket, err := csm.NativeBucket(cand.SelfStakeBucketIdx)
				if err != nil {
					return err
				}
				return csm.deactivate(cand, bucket, 100, p.calculateVoteWeight)
			},
			expectedErr: ErrExitNotReady,
			verifyFunc:  nil,
		},
		{
			name:      "confirm exit - not scheduled",
			exitBlock: math.MaxUint64,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				bucket, err := csm.NativeBucket(cand.SelfStakeBucketIdx)
				if err != nil {
					return err
				}
				return csm.deactivate(cand, bucket, 100, p.calculateVoteWeight)
			},
			expectedErr: ErrExitNotScheduled,
			verifyFunc:  nil,
		},
		{
			name:      "confirm exit - success with votes",
			exitBlock: 90,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				// Set up candidate with extra votes (in addition to self-stake bucket votes)
				cand.Votes = big.NewInt(1000)
				// Note: SelfStake and SelfStakeBucketIdx are already set by initTestState
				bucket, err := csm.NativeBucket(cand.SelfStakeBucketIdx)
				if err != nil {
					return err
				}
				return csm.deactivate(cand, bucket, 100, p.calculateVoteWeight)
			},
			expectedErr: nil,
			verifyFunc: func(r *require.Assertions, csm CandidateStateManager, id address.Address) {
				cand := csm.GetByIdentifier(id)
				// Votes should be: initialVotes - selfStakeVoteWeight + nonSelfStakeVoteWeight
				// Since bucket duration (30 days) < 91 days, both selfStake=true and false
				// return the same vote weight. So: 1000 - W + W = 1000
				r.Equal(uint64(1000), cand.Votes.Uint64())
				r.Equal(uint64(0), cand.SelfStake.Uint64())
				r.Equal(uint64(candidateNoSelfStakeBucketIndex), cand.SelfStakeBucketIdx)
			},
		},
		{
			name:      "confirm exit - no self-stake bucket",
			exitBlock: 90,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				// Test with nil bucket to simulate candidate without self-stake bucket
				return csm.deactivate(cand, nil, 100, p.calculateVoteWeight)
			},
			expectedErr: errors.Wrapf(ErrNoSelfStakeBucket, "invalid bucket"),
			verifyFunc:  nil,
		},
		{
			name:      "confirm exit - bucket index mismatch",
			exitBlock: 90,
			deleted:   false,
			blkHeight: 100,
			setupFunc: func(p *Protocol, csm CandidateStateManager, cand *Candidate) error {
				// Create a mismatched bucket index to test validation
				bucket, err := csm.NativeBucket(cand.SelfStakeBucketIdx)
				if err != nil {
					return err
				}
				// Manually set wrong bucket index to trigger validation error
				cand.SelfStakeBucketIdx = 999
				return csm.deactivate(cand, bucket, 100, p.calculateVoteWeight)
			},
			expectedErr: errors.New("self-stake bucket index mismatch"),
			verifyFunc:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			sm, p, _, candidates := initTestState(t, ctrl, []*bucketConfig{
				{identityset.Address(1), identityset.Address(1), "200", 30, true, true, nil, 0},
			}, []*candidateConfig{
				{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
			})
			candidate := candidates[0]
			candidate.DeactivatedAt = tt.exitBlock
			csm, err := NewCandidateStateManager(sm)
			r.NoError(err)
			r.NoError(csm.Upsert(candidate))

			err = tt.setupFunc(p, csm, candidate)

			if tt.expectedErr != nil {
				r.Error(err)
				r.Contains(err.Error(), tt.expectedErr.Error())
			} else {
				r.NoError(err)
			}

			if tt.verifyFunc != nil {
				tt.verifyFunc(r, csm, candidate.GetIdentifier())
			}
		})
	}
}
