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

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

func TestProtocol_HandleCandidateTransferOwnership(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	v, _, err := CreateBaseView(sm, false)
	require.NoError(err)
	sm.WriteView(_protocolID, v)
	csm, err := NewCandidateStateManager(sm)
	require.NoError(err)
	g := genesis.TestDefault()
	// create protocol
	p, err := NewProtocol(HelperCtx{
		DepositGas:    depositGas,
		BlockInterval: getBlockInterval,
	}, &BuilderConfig{
		Staking:                  g.Staking,
		PersistStakingPatchBlock: math.MaxUint64,
		Revise: ReviseConfig{
			VoteWeight: g.Staking.VoteWeightCalConsts,
		},
	}, nil, nil, nil)
	require.NoError(err)
	initCandidateCfgs := []struct {
		Owner      address.Address
		Operator   address.Address
		Reward     address.Address
		Identifier address.Address
		Name       string
	}{
		{identityset.Address(1), identityset.Address(7), identityset.Address(1), nil, "test1"},
		{identityset.Address(2), identityset.Address(8), identityset.Address(1), identityset.Address(5), "test2"},
		{identityset.Address(3), identityset.Address(9), identityset.Address(11), identityset.Address(6), "test3"},
	}
	for _, candCfg := range initCandidateCfgs {
		selfStakeAmount := big.NewInt(0)
		selfStakeBucketID := uint64(candidateNoSelfStakeBucketIndex)

		cand := &Candidate{
			Owner:              candCfg.Owner,
			Operator:           candCfg.Operator,
			Reward:             candCfg.Reward,
			Name:               candCfg.Name,
			Identifier:         candCfg.Identifier,
			Votes:              big.NewInt(0),
			SelfStakeBucketIdx: selfStakeBucketID,
			SelfStake:          selfStakeAmount,
		}
		require.NoError(csm.Upsert(cand))
	}
	require.NoError(csm.Commit(context.Background()))
	cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	ctx := genesis.WithGenesisContext(context.Background(), cfg)
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	vv, err := p.Start(ctx, sm)
	require.NoError(err)
	cc, ok := vv.(*ViewData)
	require.True(ok)
	require.NoError(sm.WriteView(_protocolID, cc))

	tests := []struct {
		name string
		// params
		initCandidateCfgIds []uint64
		initBalance         int64
		caller              address.Address
		owner               address.Address
		payload             []byte

		nonce       uint64
		gasLimit    uint64
		blkGasLimit uint64
		gasPrice    *big.Int
		// expect
		err         error
		status      iotextypes.ReceiptStatus
		expectOwner address.Address
		expectVoter address.Address
	}{
		{
			"transfer ownership to self",
			[]uint64{1},
			1300000,
			identityset.Address(1),
			identityset.Address(1),
			nil,

			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			errors.New("new owner is the same as the current owner"),
			iotextypes.ReceiptStatus_Success,
			identityset.Address(1),
			identityset.Address(1),
		},
		{
			"caller is not a candidate",
			[]uint64{1},
			1300000,
			identityset.Address(11),
			identityset.Address(6),
			nil,

			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			errors.New("candidate does not exist"),
			iotextypes.ReceiptStatus_Success,
			identityset.Address(1),
			identityset.Address(1),
		},
		{
			"transfer ownership to other exist candidate",
			[]uint64{1},
			1300000,
			identityset.Address(2),
			identityset.Address(1),
			nil,

			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			errors.New("new owner is already a candidate"),
			iotextypes.ReceiptStatus_Success,
			identityset.Address(1),
			identityset.Address(1),
		},
		{
			"transfer to another transfered candidate",
			[]uint64{1},
			1300000,
			identityset.Address(1),
			identityset.Address(5),
			nil,

			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			errors.New("new owner is already a candidate"),
			iotextypes.ReceiptStatus_Success,
			identityset.Address(1),
			identityset.Address(1),
		},
		{
			"transfer to valid address",
			[]uint64{1},
			1300000,
			identityset.Address(1),
			identityset.Address(12),
			nil,

			1,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			nil,
			iotextypes.ReceiptStatus_Success,
			identityset.Address(12),
			identityset.Address(1),
		},
	}
	csm, err = NewCandidateStateManager(sm)
	require.NoError(err)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// init candidates
			candidates := make([]*Candidate, 0)
			for _, cfg := range initCandidateCfgs {
				candidates = append(candidates, &Candidate{
					Owner:      cfg.Owner,
					Operator:   cfg.Operator,
					Reward:     cfg.Reward,
					Identifier: cfg.Identifier,
					Name:       cfg.Name,
				})
			}
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
			act, err := action.NewCandidateTransferOwnership(test.owner.String(), test.payload)
			require.NoError(err)
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
			cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
			cfg.TsunamiBlockHeight = 1
			cfg.UpernavikBlockHeight = 1 // enable candidate owner transfer feature
			ctx = genesis.WithGenesisContext(ctx, cfg)
			ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
			require.NoError(p.Validate(ctx, elp, sm))
			_, _, err = p.handleCandidateTransferOwnership(ctx, act, csm)
			if test.err != nil {
				require.Error(err)
				require.Equal(test.err.Error(), err.Error())
				return
			}
			require.NoError(err)
			candidate := csm.GetByOwner(test.expectOwner)
			require.NotNil(candidate)
			require.Equal(test.expectOwner.String(), candidate.Owner.String())
			require.Equal(test.expectVoter.String(), candidate.Identifier.String())
		})
	}
}
