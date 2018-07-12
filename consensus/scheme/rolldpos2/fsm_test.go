// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_network"
	"github.com/iotexproject/iotex-core/test/util"
)

var testAddrs = []*iotxaddress.Address{
	newTestAddr(),
	newTestAddr(),
	newTestAddr(),
	newTestAddr(),
	newTestAddr(),
}

func TestBackdoorEvt(t *testing.T) {
	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		nil,
		config.RollDPoS{
			EventChanSize: 1,
		},
		func(_ *mock_blockchain.MockBlockchain) {},
		func(_ *mock_delegate.MockPool) {},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	require.Equal(t, sEpochStart, cfsm.currentState())

	cfsm.Start(context.Background())
	defer cfsm.Stop(context.Background())

	for _, state := range consensusStates {
		cfsm.produce(cfsm.newBackdoorEvt(state), 0)
		util.WaitUntil(10*time.Millisecond, 100*time.Millisecond, func() (bool, error) {
			return state == cfsm.currentState(), nil
		})
	}

}

func TestRollDelegatesEvt(t *testing.T) {
	t.Run("is-delegate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		ctx, cfsm := newTestCFSM(t, testAddrs[0], ctrl, delegates, nil)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sDKGGeneration, s)
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), ctx.epoch.height)
		assert.Equal(t, uint64(1), ctx.epoch.num)
		assert.Equal(t, uint(1), ctx.epoch.numSubEpochs)
		assert.Equal(t, delegates, ctx.epoch.delegates)
		assert.Equal(t, eGenerateDKG, (<-cfsm.evtq).Type())
	})
	t.Run("is-not-delegate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i+1].RawAddress
		}
		ctx, cfsm := newTestCFSM(t, testAddrs[0], ctrl, delegates, nil)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sEpochStart, s)
		assert.Nil(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(0), ctx.epoch.height)
		assert.Equal(t, eRollDelegates, (<-cfsm.evtq).Type())
	})

}

func TestGenerateDKGEvt(t *testing.T) {
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	require.NoError(t, err)
	ctx := makeTestRollDPoSCtx(
		addr,
		nil,
		config.RollDPoS{
			EventChanSize: 1,
		},
		func(_ *mock_blockchain.MockBlockchain) {},
		func(_ *mock_delegate.MockPool) {},
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)

	s, err := cfsm.handleGenerateDKGEvt(cfsm.newCEvt(eGenerateDKG))
	assert.Equal(t, sRoundStart, s)
	assert.Nil(t, err)
	assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
}

func TestStartRoundEvt(t *testing.T) {
	t.Run("is-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		ctx, cfsm := newTestCFSM(t, testAddrs[2], ctrl, delegates, nil)
		ctx.epoch = epochCtx{
			delegates:    delegates,
			num:          uint64(1),
			height:       uint64(1),
			numSubEpochs: uint(1),
		}
		s, err := cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		require.Nil(t, err)
		require.Equal(t, sInitPropose, s)
		assert.NotNil(t, ctx.round.proposer, delegates[2])
		assert.NotNil(t, ctx.round.prevotes, s)
		assert.NotNil(t, ctx.round.votes, s)
		assert.Equal(t, eInitBlock, (<-cfsm.evtq).Type())
	})
	t.Run("is-not-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i+1].RawAddress
		}
		ctx, cfsm := newTestCFSM(t, testAddrs[1], ctrl, delegates, nil)
		ctx.epoch = epochCtx{
			delegates:    delegates,
			num:          uint64(1),
			height:       uint64(1),
			numSubEpochs: uint(1),
		}
		s, err := cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		require.Nil(t, err)
		require.Equal(t, sAcceptPropose, s)
		assert.NotNil(t, ctx.round.proposer, delegates[2])
		assert.NotNil(t, ctx.round.prevotes, s)
		assert.NotNil(t, ctx.round.votes, s)
		assert.Equal(t, eProposeBlockTimeout, (<-cfsm.evtq).Type())
	})
}

func TestHandleInitBlockEvt(t *testing.T) {
	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx, cfsm := newTestCFSM(
		t,
		testAddrs[2],
		ctrl,
		delegates,
		func(p2p *mock_network.MockOverlay) {
			p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
		},
	)
	ctx.epoch = epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	ctx.round = roundCtx{
		prevotes: make(map[string]*hash.Hash32B),
		votes:    make(map[string]*hash.Hash32B),
		proposer: delegates[2],
	}

	s, err := cfsm.handleInitBlockEvt(cfsm.newCEvt(eInitBlock))
	require.Nil(t, err)
	require.Equal(t, sAcceptPropose, s)
	e := <-cfsm.evtq
	require.Equal(t, eProposeBlock, e.Type())
	pbe, ok := e.(*proposeBlkEvt)
	require.True(t, ok)
	require.NotNil(t, pbe.blk)
	require.Equal(t, 1, len(pbe.blk.Transfers))
	require.Equal(t, 1, len(pbe.blk.Votes))
}

func newTestCFSM(
	t *testing.T,
	addr *iotxaddress.Address,
	ctrl *gomock.Controller,
	candidates []string,
	mockP2P func(overlay *mock_network.MockOverlay),
) (*rollDPoSCtx, *cFSM) {
	transfer := action.NewTransfer(1, big.NewInt(100), "src", "dst")
	vote := action.NewVote(2, []byte("src"), []byte("dst"))
	var prevHash hash.Hash32B
	blk := blockchain.NewBlock(1, 2, prevHash, []*action.Transfer{transfer}, []*action.Vote{vote})
	if mockP2P == nil {
		mockP2P = func(p2p *mock_network.MockOverlay) {
			p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).AnyTimes()
		}
	}
	ctx := makeTestRollDPoSCtx(
		addr,
		ctrl,
		config.RollDPoS{
			EventChanSize: 2,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().TipHeight().Return(uint64(1), nil).AnyTimes()
			blockchain.EXPECT().
				MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(blk, nil).
				AnyTimes()
		},
		func(pool *mock_delegate.MockPool) {
			pool.EXPECT().NumDelegatesPerEpoch().Return(uint(4), nil).AnyTimes()
			pool.EXPECT().RollDelegates(gomock.Any()).Return(candidates, nil).AnyTimes()
		},
		func(actPool *mock_actpool.MockActPool) {
			actPool.EXPECT().PickActs().Return([]*action.Transfer{transfer}, []*action.Vote{vote}).AnyTimes()
		},
		mockP2P,
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	return ctx, cfsm
}

func newTestAddr() *iotxaddress.Address {
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		logger.Panic().Err(err).Msg("error when creating test IoTeX address")
	}
	return addr
}
