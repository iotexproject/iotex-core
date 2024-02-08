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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

type appendAction struct {
	act       func() action.Action
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
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, true, nil, 0},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(2), identityset.Address(1), "1200000000000000000000000", 30, true, false, nil, 0},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 30, true, true, nil, 0},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, endorsementNotExpireHeight},
		{identityset.Address(1), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, 1},
		{identityset.Address(2), identityset.Address(2), "1200000000000000000000000", 91, true, false, nil, 0},
		{identityset.Address(1), identityset.Address(1), "1200000000000000000000000", 30, false, false, nil, 0},
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
				{0, identityset.Address(1)},
				{1, identityset.Address(1)},
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
				func() action.Action {
					act, err := action.NewChangeCandidate(0, "test3", 1, []byte{}, uint64(1000000), big.NewInt(1000))
					require.NoError(err)
					return act
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
				func() action.Action {
					act := action.NewCandidateEndorsement(0, uint64(1000000), big.NewInt(1000), 1, false)
					return act
				},
				iotextypes.ReceiptStatus_Success,
				func(t *testing.T) {
					csm, err := NewCandidateStateManager(sm, false)
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
				func() action.Action {
					act := action.NewCandidateEndorsement(0, uint64(1000000), big.NewInt(1000), 1, false)
					return act
				},
				iotextypes.ReceiptStatus_Success,
				func(t *testing.T) {
					csm, err := NewCandidateStateManager(sm, false)
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			nonce := test.nonce
			if test.newProtocol {
				sm, p, _, _ = initTestStateFromIds(test.initBucketCfgIds, test.initCandidateCfgIds)
			}
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
			act := action.NewCandidateEndorsement(nonce, test.gasLimit, test.gasPrice, test.bucketID, test.endorse)
			IntrinsicGas, _ := act.IntrinsicGas()
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
			cfg := deepcopy.Copy(genesis.Default).(genesis.Genesis)
			cfg.ToBeEnabledBlockHeight = 1
			ctx = genesis.WithGenesisContext(ctx, genesis.Default)
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			require.Equal(test.err, errors.Cause(p.Validate(ctx, act, sm)))
			if test.err != nil {
				return
			}
			r, err := p.Handle(ctx, act, sm)
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
				ctx = genesis.WithGenesisContext(ctx, cfg)
				ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
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
				csm, err := NewCandidateStateManager(sm, false)
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
					bkt, err := csm.getBucket(expectBkt.id)
					require.NoError(err)
					require.Equal(expectBkt.candidate, bkt.Candidate)
				}

				// test staker's account
				caller, err := accountutil.LoadAccount(sm, test.caller)
				require.NoError(err)
				actCost, err := act.Cost()
				actCost.Add(actCost, big.NewInt(0).Mul(test.gasPrice, big.NewInt(0).SetUint64(appendIntrinsicGas)))
				require.NoError(err)
				total := big.NewInt(0)
				require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost))
				require.Equal(nonce+1, caller.PendingNonce())
			}
		})
	}
}
