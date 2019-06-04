// Copyright (c) 2018 IoTeX
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
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"
)

func initConstruct(t *testing.T) (Protocol, context.Context, factory.WorkingSet, *types.ElectionResult) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.Default
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 123456,
		},
	)

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	committee := mock_committee.NewMockCommittee(ctrl)
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
	require.NoError(err)
	return p, ctx, ws, r
}

func TestInitialize(t *testing.T) {
	require := require.New(t)
	p, ctx, ws, r := initConstruct(t)
	require.NoError(p.Initialize(ctx, ws))
	var sc state.CandidateList
	require.NoError(ws.State(candidatesutil.ConstructKey(1), &sc))
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

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().GetCachedBatch().Return(cb).AnyTimes()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	p, ctx, ws, _ := initConstruct(t)
	require.NoError(p.Initialize(ctx, ws))

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
	// Case 2: right action type,setCandidates error
	var sc state.CandidateList
	require.NoError(ws.State(candidatesutil.ConstructKey(1), &sc))
	act := action.NewPutPollResult(1, 123456, sc)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act).Build()
	selp, err = action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	receipt, err = p.Handle(ctx, selp.Action(), sm)
	require.Error(err)
	require.Nil(receipt)
	// Case 3: all right
	p3, ctx3, ws3, _ := initConstruct(t)
	require.NoError(p3.Initialize(ctx3, ws3))
	var sc3 state.CandidateList
	require.NoError(ws3.State(candidatesutil.ConstructKey(1), &sc3))
	act3 := action.NewPutPollResult(1, 1, sc3)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act3).Build()
	selp3, err := action.Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp3)
	receipt, err = p.Handle(ctx3, selp3.Action(), sm)
	require.NoError(err)
	require.NotNil(receipt)
}

func TestProtocol_Validate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	p, ctx, ws, _ := initConstruct(t)
	require.NoError(p.Initialize(ctx, ws))

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
	p2, ctx2, ws2, _ := initConstruct(t)
	require.NoError(p2.Initialize(ctx2, ws2))
	var sc2 state.CandidateList
	require.NoError(ws2.State(candidatesutil.ConstructKey(1), &sc2))
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
	p3, ctx3, ws3, _ := initConstruct(t)
	require.NoError(p3.Initialize(ctx3, ws3))
	var sc3 state.CandidateList
	require.NoError(ws3.State(candidatesutil.ConstructKey(1), &sc3))
	sc3 = append(sc3, &state.Candidate{"1", big.NewInt(10), "2"})
	sc3 = append(sc3, &state.Candidate{"1", big.NewInt(10), "2"})
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
	p4, ctx4, ws4, _ := initConstruct(t)
	require.NoError(p4.Initialize(ctx4, ws4))
	var sc4 state.CandidateList
	require.NoError(ws4.State(candidatesutil.ConstructKey(1), &sc4))
	sc4 = append(sc4, &state.Candidate{"1", big.NewInt(10), "2"})
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
	p5, ctx5, ws5, _ := initConstruct(t)
	require.NoError(p5.Initialize(ctx5, ws5))
	var sc5 state.CandidateList
	require.NoError(ws5.State(candidatesutil.ConstructKey(1), &sc5))
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
	p6, ctx6, ws6, _ := initConstruct(t)
	require.NoError(p6.Initialize(ctx6, ws6))
	var sc6 state.CandidateList
	require.NoError(ws6.State(candidatesutil.ConstructKey(1), &sc6))
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
