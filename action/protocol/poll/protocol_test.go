// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"
)

func initConstruct(ctrl *gomock.Controller) (Protocol, context.Context, protocol.StateManager, *types.ElectionResult, error) {
	cfg := config.Default
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 123456,
		},
	)

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	committee := mock_committee.NewMockCommittee(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			val, err := cb.Get("state", addrHash[:])
			if err != nil {
				return state.ErrStateNotExist
			}
			return state.Deserialize(account, val)
		}).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(addrHash hash.Hash160, account interface{}) error {
			ss, err := state.Serialize(account)
			if err != nil {
				return err
			}
			cb.Put("state", addrHash[:], ss, "failed to put state")
			return nil
		}).AnyTimes()
	sm.EXPECT().GetCachedBatch().Return(cb).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	r := types.NewElectionResultForTest(time.Now())
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(r, nil).AnyTimes()
	committee.EXPECT().HeightByTime(gomock.Any()).Return(uint64(123456), nil).AnyTimes()
	p, err := NewGovernanceChainCommitteeProtocol(
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		func(uint64) uint64 { return 1 },
		func(uint64) uint64 { return 1 },
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Chain.PollInitialCandidatesInterval,
	)
	return p, ctx, sm, r, err
}

func TestInitialize(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	p, ctx, sm, r, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p.Initialize(ctx, sm))
	var sc state.CandidateList
	require.NoError(sm.State(candidatesutil.ConstructKey(1), &sc))
	candidates, err := state.CandidatesToMap(sc)
	require.NoError(err)
	require.Equal(2, len(candidates))
	for _, d := range r.Delegates() {
		operator := string(d.OperatorAddress())
		addr, err := address.FromString(operator)
		require.NoError(err)
		c, ok := candidates[hash.BytesToHash160(addr.Bytes())]
		require.True(ok)
		require.Equal(addr.String(), c.Address)
	}
}

func TestHandle(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, ctx, sm, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p.Initialize(ctx, sm))

	// wrong action
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)
	tsf, err := action.NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()
	selp, err := action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	// Case 1: wrong action type
	receipt, err := p.Handle(ctx, selp.Action(), nil)
	require.NoError(err)
	require.Nil(receipt)
	// Case 2: all right
	p2, ctx2, sm2, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p2.Initialize(ctx2, sm2))
	var sc2 state.CandidateList
	require.NoError(sm2.State(candidatesutil.ConstructKey(1), &sc2))
	act2 := action.NewPutPollResult(1, 1, sc2)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act2).Build()
	selp2, err := action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp2)
	receipt, err = p.Handle(ctx2, selp2.Action(), sm2)
	require.NoError(err)
	require.NotNil(receipt)
}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, ctx, sm, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p.Initialize(ctx, sm))

	// wrong action
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)
	tsf, err := action.NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()
	selp, err := action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	// Case 1: wrong action type
	require.NoError(p.Validate(ctx, selp.Action()))
	// Case 2: Only producer could create this protocol
	p2, ctx2, sm2, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p2.Initialize(ctx2, sm2))
	var sc2 state.CandidateList
	require.NoError(sm2.State(candidatesutil.ConstructKey(1), &sc2))
	act2 := action.NewPutPollResult(1, 1, sc2)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act2).Build()
	selp2, err := action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp2)
	caller, err := address.FromBytes(selp.SrcPubkey().Hash())
	require.NoError(err)
	ctx2 = protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
			BlockHeight:  1,
			ProducerAddr: recipientAddr.String(),
			Caller:       caller,
		},
	)
	err = p.Validate(ctx2, selp2.Action())
	require.True(strings.Contains(err.Error(), "Only producer could create this protocol"))
	// Case 3: duplicate candidate
	p3, ctx3, sm3, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p3.Initialize(ctx3, sm3))
	var sc3 state.CandidateList
	require.NoError(sm3.State(candidatesutil.ConstructKey(1), &sc3))
	sc3 = append(sc3, &state.Candidate{"1", big.NewInt(10), "2", nil})
	sc3 = append(sc3, &state.Candidate{"1", big.NewInt(10), "2", nil})
	act3 := action.NewPutPollResult(1, 1, sc3)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act3).Build()
	selp3, err := action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp3)
	ctx3 = protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
			BlockHeight:  1,
			ProducerAddr: identityset.Address(27).String(),
			Caller:       caller,
		},
	)
	err = p.Validate(ctx3, selp3.Action())
	require.True(strings.Contains(err.Error(), "duplicate candidate"))

	// Case 4: delegate's length is not equal
	p4, ctx4, sm4, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p4.Initialize(ctx4, sm4))
	var sc4 state.CandidateList
	require.NoError(sm4.State(candidatesutil.ConstructKey(1), &sc4))
	sc4 = append(sc4, &state.Candidate{"1", big.NewInt(10), "2", nil})
	act4 := action.NewPutPollResult(1, 1, sc4)
	bd4 := &action.EnvelopeBuilder{}
	elp4 := bd4.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act4).Build()
	selp4, err := action.Sign(elp4, senderKey)
	require.NoError(err)
	require.NotNil(selp4)
	ctx4 = protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
			BlockHeight:  1,
			ProducerAddr: identityset.Address(27).String(),
			Caller:       caller,
		},
	)
	err = p4.Validate(ctx4, selp4.Action())
	require.True(strings.Contains(err.Error(), "the proposed delegate list length"))
	// Case 5: candidate's vote is not equal
	p5, ctx5, sm5, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p5.Initialize(ctx5, sm5))
	var sc5 state.CandidateList
	require.NoError(sm5.State(candidatesutil.ConstructKey(1), &sc5))
	sc5[0].Votes = big.NewInt(10)
	act5 := action.NewPutPollResult(1, 1, sc5)
	bd5 := &action.EnvelopeBuilder{}
	elp5 := bd5.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act5).Build()
	selp5, err := action.Sign(elp5, senderKey)
	require.NoError(err)
	require.NotNil(selp5)
	ctx5 = protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
			BlockHeight:  1,
			ProducerAddr: identityset.Address(27).String(),
			Caller:       caller,
		},
	)
	err = p5.Validate(ctx5, selp5.Action())
	require.True(strings.Contains(err.Error(), "delegates are not as expected"))
	// Case 6: all good
	p6, ctx6, sm6, _, err := initConstruct(ctrl)
	require.NoError(err)
	require.NoError(p6.Initialize(ctx6, sm6))
	var sc6 state.CandidateList
	require.NoError(sm6.State(candidatesutil.ConstructKey(1), &sc6))
	act6 := action.NewPutPollResult(1, 1, sc6)
	bd6 := &action.EnvelopeBuilder{}
	elp6 := bd6.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act6).Build()
	selp6, err := action.Sign(elp6, senderKey)
	require.NoError(err)
	require.NotNil(selp6)
	caller6, err := address.FromBytes(selp6.SrcPubkey().Hash())
	require.NoError(err)
	ctx6 = protocol.WithValidateActionsCtx(
		context.Background(),
		protocol.ValidateActionsCtx{
			BlockHeight:  1,
			ProducerAddr: identityset.Address(27).String(),
			Caller:       caller6,
		},
	)
	err = p6.Validate(ctx6, selp6.Action())
	require.NoError(err)
}
