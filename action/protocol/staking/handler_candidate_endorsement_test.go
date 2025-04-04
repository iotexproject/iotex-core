// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

type appendAction struct {
	act       func() action.Envelope
	status    iotextypes.ReceiptStatus
	validator func(t *testing.T)
}

func TestProtocol_HandleCandidateEndorsement(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(1), "1", 1, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, true, false, &timeBeforeBlockII, 0},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 30, true, true, nil, 0},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, 1},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, false, false, nil, 0},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, endorsementNotExpireHeight},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, 10},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, 1},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
		{identityset.Address(3), identityset.Address(9), identityset.Address(11), "test3"},
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
		endorse             bool
		newProtocol         bool
		append              *appendAction
		// expect
		err              error
		status           iotextypes.ReceiptStatus
		expectCandidates []expectCandidate
		expectBuckets    []expectBucket
	}{
		{
			"endorse candidate with invalid bucket index",
			[]uint64{0, 1},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			2,
			true,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with invalid bucket owner",
			[]uint64{0, 1},
			[]uint64{0, 1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with invalid bucket amount",
			[]uint64{0, 1},
			[]uint64{0, 1},
			1000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			true,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with self-staked bucket",
			[]uint64{0, 3},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with invalid bucket candidate",
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
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with endorsed bucket",
			[]uint64{0, 7},
			[]uint64{0, 1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with unstaked bucket",
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
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			[]expectCandidate{},
			nil,
		},
		{
			"endorse candidate with expired endorsement",
			[]uint64{0, 8},
			[]uint64{0, 1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{
				{identityset.Address(1), candidateNoSelfStakeBucketIndex, "0", "1542516163985454635820817"},
			},
			[]expectBucket{
				{0, identityset.Address(1), false, 0},
				{1, identityset.Address(1), true, endorsementNotExpireHeight},
			},
		},
		{
			"endorse candidate with valid bucket",
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
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{},
			nil,
		},
		{
			"once endorsed, bucket cannot be unstaked",
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
			true,
			&appendAction{
				func() action.Envelope {
					act := action.NewUnstake(1, []byte{})
					return (&action.EnvelopeBuilder{}).SetNonce(0).SetGasLimit(1000000).
						SetGasPrice(big.NewInt(1000)).SetAction(act).Build()
				},
				iotextypes.ReceiptStatus_ErrInvalidBucketType,
				nil,
			},
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{},
			nil,
		},
		{
			"once endorsed, bucket cannot be change candidate",
			[]uint64{0, 10},
			[]uint64{0, 1, 2},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			true,
			&appendAction{
				func() action.Envelope {
					act := action.NewChangeCandidate("test3", 1, []byte{})
					return (&action.EnvelopeBuilder{}).SetNonce(0).
						SetGasLimit(1000000).SetGasPrice(big.NewInt(1000)).
						SetAction(act).Build()
				},
				iotextypes.ReceiptStatus_ErrInvalidBucketType, //todo fix
				nil,
			},
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{},
			nil,
		},
		{
			"unendorse a valid bucket",
			[]uint64{0, 9},
			[]uint64{0, 1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			1,
			true,
			true,
			&appendAction{
				func() action.Envelope {
					act := action.NewCandidateEndorsementLegacy(1, false)
					return (&action.EnvelopeBuilder{}).SetNonce(0).
						SetGasLimit(1000000).SetGasPrice(big.NewInt(1000)).
						SetAction(act).Build()
				},
				iotextypes.ReceiptStatus_Success,
				func(t *testing.T) {
					csm, err := NewCandidateStateManager(sm)
					require.NoError(err)
					esm := NewEndorsementStateManager(csm.SM())
					bucket, err := csm.getBucket(1)
					require.NoError(err)
					endorsement, err := esm.Get(bucket.Index)
					require.NoError(err)
					require.NotNil(endorsement)
					require.Equal(uint64(1), endorsement.ExpireHeight)
				},
			},
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{},
			nil,
		},
		{
			"unendorse a self-staked bucket",
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
			true,
			&appendAction{
				func() action.Envelope {
					act := action.NewCandidateEndorsementLegacy(1, false)
					return (&action.EnvelopeBuilder{}).SetNonce(0).
						SetGasLimit(1000000).SetGasPrice(big.NewInt(1000)).
						SetAction(act).Build()
				},
				iotextypes.ReceiptStatus_Success,
				func(t *testing.T) {
					csm, err := NewCandidateStateManager(sm)
					require.NoError(err)
					esm := NewEndorsementStateManager(csm.SM())
					bucket, err := csm.getBucket(1)
					require.NoError(err)
					endorsement, err := esm.Get(bucket.Index)
					require.NoError(err)
					require.NotNil(endorsement)
					require.Equal(uint64(1), endorsement.ExpireHeight)
				},
			},
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{},
			nil,
		},
		{
			"endorsement withdraw if bucket is self-staked but without endorsement",
			[]uint64{6},
			[]uint64{1},
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			false,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			[]expectCandidate{},
			nil,
		},
		{
			"endorsement withdraw if bucket is self-staked by endorsement",
			[]uint64{11},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			false,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
			[]expectCandidate{
				{identityset.Address(2), 0, "1200000000000000000000000", "1469480667073232815766914"},
			},
			[]expectBucket{
				{0, identityset.Address(2), true, 17281},
			},
		},
		{
			"endorsement withdraw if bucket is endorsement withdrawing",
			[]uint64{12},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			false,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			nil,
			nil,
		},
		{
			"endorsement withdraw if bucket is endorsement expired",
			[]uint64{13},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			false,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
			nil,
			nil,
		},
		{
			"endorsement withdraw if bucket without endorsement",
			[]uint64{0},
			[]uint64{0, 1},
			1300000,
			identityset.Address(1),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			0,
			false,
			true,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
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
			act := action.NewCandidateEndorsementLegacy(test.bucketID, test.endorse)
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
			var appendIntrinsicGas uint64
			if test.append != nil {
				nonce = nonce + 1
				appendIntrinsicGas, _ = act.IntrinsicGas()
				ctx := protocol.WithActionCtx(ctx, protocol.ActionCtx{
					Caller:       test.caller,
					GasPrice:     test.gasPrice,
					IntrinsicGas: IntrinsicGas,
					Nonce:        nonce,
				})
				r, err = p.Handle(ctx, test.append.act(), sm)
				require.NoError(err)
				if r != nil {
					require.Equal(uint64(test.append.status), r.Status, fmt.Sprintf("except :%d, actual:%d", test.append.status, r.Status))
					if test.append.validator != nil {
						test.append.validator(t)
					}
				} else {
					require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
				}
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
				esm := NewEndorsementStateManager(csm.SM())
				for _, expectBkt := range test.expectBuckets {
					bkt, err := csm.getBucket(expectBkt.id)
					require.NoError(err)
					require.Equal(expectBkt.candidate, bkt.Candidate)
					endorse, err := esm.Get(expectBkt.id)
					if expectBkt.hasEndorsement {
						require.NoError(err)
						require.EqualValues(expectBkt.endorsementExpireHeight, endorse.ExpireHeight)
					} else {
						require.ErrorIs(err, state.ErrStateNotExist)
					}
				}

				// test staker's account
				caller, err := accountutil.LoadAccount(sm, test.caller)
				require.NoError(err)
				actCost, err := elp.Cost()
				actCost.Add(actCost, big.NewInt(0).Mul(test.gasPrice, big.NewInt(0).SetUint64(appendIntrinsicGas)))
				require.NoError(err)
				total := big.NewInt(0)
				require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost))
				require.Equal(nonce+1, caller.PendingNonce())
			}
		})
	}
}

type testExisting struct {
	name string
	// params
	initBalance int64
	caller      address.Address
	nonce       uint64
	gasLimit    uint64
	blkGasLimit uint64
	gasPrice    *big.Int
	bucketID    uint64
	cand        address.Address
	// expect
	err    error
	status iotextypes.ReceiptStatus
}

func TestProtocol_HandleTransferEndorsement(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, false, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(3), "1200000000000000000000000", 91, false, true, nil, endorsementNotExpireHeight},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
	}
	sm, p, buckets, cands := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)

	for _, test := range []testExisting{
		{
			"can transfer endorsed bucket",
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[0].Index,
			cands[1].Owner, // bucket0 transfer to cands1
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		{
			"cannot transfer self-staked endorsed bucket",
			1300000,
			identityset.Address(3),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[1].Index,
			cands[1].Owner, // bucket1 cannot transfer to cands1
			nil,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
	} {
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act, _ := action.NewTransferStake(test.cand.String(), test.bucketID, nil)
		IntrinsicGas, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
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
			require.EqualValues(test.status, r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}
	}
}

func TestProtocol_HandleWithdrawEndorsement(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, false, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(3), "1200000000000000000000000", 91, false, true, nil, endorsementNotExpireHeight},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
	}
	sm, p, buckets, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)

	for _, test := range []testExisting{
		{
			"cannot withdraw endorsed bucket",
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[0].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake,
		},
		{
			"cannot withdraw self-staked endorsed bucket",
			1300000,
			identityset.Address(3),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[1].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake,
		},
	} {
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act := action.NewWithdrawStake(test.bucketID, nil)
		IntrinsicGas, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
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
			require.EqualValues(test.status, r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}
	}
}

func TestProtocol_HandleRestakeEndorsement(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, false, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(3), "1200000000000000000000000", 91, false, true, nil, endorsementNotExpireHeight},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
	}
	sm, p, buckets, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)

	for _, test := range []testExisting{
		{
			"can restake endorsed bucket",
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[0].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		{
			"can restake self-staked endorsed bucket",
			1300000,
			identityset.Address(3),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[1].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	} {
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act := action.NewRestake(test.bucketID, 3, false, nil)
		IntrinsicGas, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
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
			require.EqualValues(test.status, r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}
	}
}

func TestProtocol_HandleDepositEndorsement(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(3), "1200000000000000000000000", 91, true, true, nil, endorsementNotExpireHeight},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
	}
	sm, p, buckets, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)

	for _, test := range []testExisting{
		{
			"can deposit to endorsed bucket",
			1300000,
			identityset.Address(2),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[0].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		{
			"can deposit to self-staked endorsed bucket",
			1300000,
			identityset.Address(3),
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[1].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	} {
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act, _ := action.NewDepositToStake(test.bucketID, "300000", nil)
		IntrinsicGas, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
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
			require.EqualValues(test.status, r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}
	}
}

func TestProtocol_HandleConsignmentEndorsement(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	initBucketCfgs := []*bucketConfig{
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, false, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(3), "1200000000000000000000000", 91, false, true, nil, endorsementNotExpireHeight},
	}
	initCandidateCfgs := []*candidateConfig{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), "test2"},
	}
	sm, p, buckets, _ := initTestState(t, ctrl, initBucketCfgs, initCandidateCfgs)

	for _, test := range []testExisting{
		{
			"can consign endorsed bucket",
			1300000,
			identityset.Address(7), // bucket0 transfer to caller
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[0].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		{
			"cannot consign self-staked endorsed bucket",
			1300000,
			identityset.Address(8), // bucket1 cannot transfer to caller
			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			buckets[1].Index,
			nil,
			nil,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
	} {
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		var sk string
		if test.bucketID == buckets[0].Index {
			// bucket0 owned by privatekey 2
			sk = identityset.PrivateKey(2).HexString()
		} else if test.bucketID == buckets[1].Index {
			// bucket1 owned by privatekey 3
			sk = identityset.PrivateKey(3).HexString()
		}
		consign := newconsignment(require, test.bucketID, test.nonce, sk, test.caller.String(), "Ethereum", _reclaim, false)
		act, _ := action.NewTransferStake(test.caller.String(), test.bucketID, consign)
		IntrinsicGas, _ := act.IntrinsicGas()
		elp := builder.SetNonce(test.nonce).SetGasLimit(test.gasLimit).
			SetGasPrice(test.gasPrice).SetAction(act).Build()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: timeBlock,
			GasLimit:       test.blkGasLimit,
		})
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
		cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
		cfg.GreenlandBlockHeight = 1
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
			require.EqualValues(test.status, r.Status)
		} else {
			require.Equal(test.status, iotextypes.ReceiptStatus_Failure)
		}
	}
}
