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
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state"
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
	t.Parallel()

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
	t.Parallel()

	t.Run("is-delegate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		cfsm := newTestCFSM(t, testAddrs[0], ctrl, delegates, nil, nil)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sDKGGeneration, s)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.height)
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.num)
		assert.Equal(t, uint(1), cfsm.ctx.epoch.numSubEpochs)
		assert.Equal(t, delegates, cfsm.ctx.epoch.delegates)
		assert.Equal(t, eGenerateDKG, (<-cfsm.evtq).Type())
	})
	t.Run("is-not-delegate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i+1].RawAddress
		}
		cfsm := newTestCFSM(t, testAddrs[0], ctrl, delegates, nil, nil)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sEpochStart, s)
		assert.NoError(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(0), cfsm.ctx.epoch.height)
		assert.Equal(t, eRollDelegates, (<-cfsm.evtq).Type())
	})

}

func TestGenerateDKGEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}
	t.Run("no-delay", func(t *testing.T) {
		cfsm := newTestCFSM(t, testAddrs[2], ctrl, delegates, nil, nil)
		s, err := cfsm.handleGenerateDKGEvt(cfsm.newCEvt(eGenerateDKG))
		assert.Equal(t, sRoundStart, s)
		assert.NoError(t, err)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
	})
	t.Run("delay", func(t *testing.T) {
		cfsm := newTestCFSM(t, testAddrs[2], ctrl, delegates, nil, nil)
		cfsm.ctx.cfg.ProposerInterval = 2 * time.Second
		start := time.Now()
		s, err := cfsm.handleGenerateDKGEvt(cfsm.newCEvt(eGenerateDKG))
		assert.Equal(t, sRoundStart, s)
		assert.NoError(t, err)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
		// Allow 1 second delay during the process
		assert.True(t, time.Since(start) > time.Second)
	})
}

func TestStartRoundEvt(t *testing.T) {
	t.Parallel()

	t.Run("is-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		cfsm := newTestCFSM(t, testAddrs[2], ctrl, delegates, nil, nil)
		cfsm.ctx.epoch = epochCtx{
			delegates:    delegates,
			num:          uint64(1),
			height:       uint64(1),
			numSubEpochs: uint(1),
		}
		s, err := cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		require.NoError(t, err)
		require.Equal(t, sInitPropose, s)
		assert.NotNil(t, cfsm.ctx.round.proposer, delegates[2])
		assert.NotNil(t, cfsm.ctx.round.prevotes, s)
		assert.NotNil(t, cfsm.ctx.round.votes, s)
		assert.Equal(t, eInitBlock, (<-cfsm.evtq).Type())
	})
	t.Run("is-not-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i+1].RawAddress
		}
		cfsm := newTestCFSM(t, testAddrs[1], ctrl, delegates, nil, nil)
		cfsm.ctx.epoch = epochCtx{
			delegates:    delegates,
			num:          uint64(1),
			height:       uint64(1),
			numSubEpochs: uint(1),
		}
		s, err := cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		require.NoError(t, err)
		require.Equal(t, sAcceptPropose, s)
		assert.NotNil(t, cfsm.ctx.round.proposer, delegates[2])
		assert.NotNil(t, cfsm.ctx.round.prevotes, s)
		assert.NotNil(t, cfsm.ctx.round.votes, s)
		assert.Equal(t, eProposeBlockTimeout, (<-cfsm.evtq).Type())
	})
}

func TestHandleInitBlockEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}

	cfsm := newTestCFSM(
		t,
		testAddrs[2],
		ctrl,
		delegates,
		nil,
		func(p2p *mock_network.MockOverlay) {
			p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
		},
	)
	cfsm.ctx.epoch = epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	cfsm.ctx.round = roundCtx{
		prevotes: make(map[string]bool),
		votes:    make(map[string]bool),
		proposer: delegates[2],
	}

	s, err := cfsm.handleInitBlockEvt(cfsm.newCEvt(eInitBlock))
	require.NoError(t, err)
	require.Equal(t, sAcceptPropose, s)
	e := <-cfsm.evtq
	require.Equal(t, eProposeBlock, e.Type())
	pbe, ok := e.(*proposeBlkEvt)
	require.True(t, ok)
	require.NotNil(t, pbe.block)
	require.Equal(t, 1, len(pbe.block.Transfers))
	require.Equal(t, 1, len(pbe.block.Votes))

}

func TestHandleProposeBlockEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}

	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		prevotes: make(map[string]bool),
		votes:    make(map[string]bool),
		proposer: delegates[2],
	}

	t.Run("pass-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			nil,
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, delegates[2], cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPrevote, state)
		e := <-cfsm.evtq
		evt, ok := e.(*voteEvt)
		require.True(t, ok)
		assert.Equal(t, ePrevote, evt.Type())
		assert.True(t, evt.decision)
		assert.Equal(t, ePrevoteTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("fail-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any()).Return(errors.New("mock error")).Times(1)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, delegates[2], cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPrevote, state)
		e := <-cfsm.evtq
		evt, ok := e.(*voteEvt)
		require.True(t, ok)
		assert.Equal(t, ePrevote, evt.Type())
		assert.False(t, evt.decision)
		assert.Equal(t, ePrevoteTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("skip-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any()).Times(0)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, delegates[2], cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPrevote, state)
		e := <-cfsm.evtq
		evt, ok := e.(*voteEvt)
		require.True(t, ok)
		assert.Equal(t, ePrevote, evt.Type())
		assert.True(t, evt.decision)
		assert.Equal(t, ePrevoteTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("timeout", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any()).Times(0)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(0)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleProposeBlockEvt(cfsm.newCEvt(eProposeBlockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPrevote, state)
		assert.Equal(t, ePrevoteTimeout, (<-cfsm.evtq).Type())
	})
}

func TestHandlePrevoteEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}

	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		prevotes: make(map[string]bool),
		votes:    make(map[string]bool),
		proposer: delegates[2],
	}

	t.Run("gather-prevotes", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			nil,
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		// First prevote
		state, err := cfsm.handlePrevoteEvt(
			newVoteEvt(ePrevote, blk.HashBlock(), true, delegates[0], cfsm.ctx.clock),
		)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPrevote, state)

		// Second prevote
		state, err = cfsm.handlePrevoteEvt(
			newVoteEvt(ePrevote, blk.HashBlock(), true, delegates[1], cfsm.ctx.clock),
		)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPrevote, state)

		// Third prevote, could move on
		state, err = cfsm.handlePrevoteEvt(
			newVoteEvt(ePrevote, blk.HashBlock(), true, delegates[2], cfsm.ctx.clock),
		)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptVote, state)
		e := <-cfsm.evtq
		evt, ok := e.(*voteEvt)
		require.True(t, ok)
		assert.Equal(t, eVote, evt.Type())
		assert.True(t, evt.decision)
		assert.Equal(t, eVoteTimeout, (<-cfsm.evtq).Type())
	})
	t.Run("timeout", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			nil,
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handlePrevoteEvt(cfsm.newCEvt(ePrevoteTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptVote, state)
		e := <-cfsm.evtq
		evt, ok := e.(*voteEvt)
		require.True(t, ok)
		assert.Equal(t, eVote, evt.Type())
		assert.False(t, evt.decision)
		assert.Equal(t, eVoteTimeout, (<-cfsm.evtq).Type())
	})
}

func TestHandleVoteEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}

	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		prevotes: make(map[string]bool),
		votes:    make(map[string]bool),
		proposer: delegates[2],
	}

	t.Run("gather-votes", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(1)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		// First prevote
		state, err := cfsm.handleVoteEvt(
			newVoteEvt(eVote, blk.HashBlock(), true, delegates[0], cfsm.ctx.clock),
		)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptVote, state)

		// Second prevote
		state, err = cfsm.handleVoteEvt(
			newVoteEvt(eVote, blk.HashBlock(), true, delegates[1], cfsm.ctx.clock),
		)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptVote, state)

		// Third prevote, could move on
		state, err = cfsm.handleVoteEvt(
			newVoteEvt(eVote, blk.HashBlock(), true, delegates[2], cfsm.ctx.clock),
		)
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
	})
	t.Run("timeout", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(0)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).Times(0)
			},
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleVoteEvt(cfsm.newCEvt(eVoteTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
	})
}

func TestHandleFinishEpochEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 4)
	for i := 0; i < 4; i++ {
		delegates[i] = testAddrs[i].RawAddress
	}

	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		prevotes: make(map[string]bool),
		votes:    make(map[string]bool),
		proposer: delegates[2],
	}

	t.Run("not-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			nil,
			nil,
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleFinishEpochEvt(cfsm.newCEvt(eFinishEpoch))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
	})
	t.Run("finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(4), nil).Times(1)
			},
			nil,
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleFinishEpochEvt(cfsm.newCEvt(eFinishEpoch))
		assert.NoError(t, err)
		assert.Equal(t, sEpochStart, state)
		assert.Equal(t, eRollDelegates, (<-cfsm.evtq).Type())
	})
}

func newTestCFSM(
	t *testing.T,
	addr *iotxaddress.Address,
	ctrl *gomock.Controller,
	delegates []string,
	mockChain func(*mock_blockchain.MockBlockchain),
	mockP2P func(*mock_network.MockOverlay),
) *cFSM {
	transfer := action.NewTransfer(1, big.NewInt(100), "src", "dst")
	vote := action.NewVote(2, keypair.ZeroPublicKey, keypair.ZeroPublicKey)
	var prevHash hash.Hash32B
	lastBlk := blockchain.NewBlock(1, 1, prevHash, make([]*action.Transfer, 0), make([]*action.Vote, 0))
	blkToMint := blockchain.NewBlock(1, 2, lastBlk.HashBlock(), []*action.Transfer{transfer}, []*action.Vote{vote})
	ctx := makeTestRollDPoSCtx(
		addr,
		ctrl,
		config.RollDPoS{
			EventChanSize: 2,
			NumDelegates:  uint(len(delegates)),
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().GetBlockByHeight(uint64(1)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().
				MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(blkToMint, nil).
				AnyTimes()
			blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
				{Address: delegates[0]},
				{Address: delegates[1]},
				{Address: delegates[2]},
				{Address: delegates[3]},
			}, true).AnyTimes()
			if mockChain == nil {
				blockchain.EXPECT().TipHeight().Return(uint64(1), nil).AnyTimes()
				blockchain.EXPECT().ValidateBlock(gomock.Any()).Return(nil).AnyTimes()
			} else {
				mockChain(blockchain)
			}
		},
		func(pool *mock_delegate.MockPool) {
			pool.EXPECT().NumDelegatesPerEpoch().Return(uint(4), nil).AnyTimes()
			pool.EXPECT().RollDelegates(gomock.Any()).Return(delegates, nil).AnyTimes()
		},
		func(actPool *mock_actpool.MockActPool) {
			actPool.EXPECT().PickActs().Return([]*action.Transfer{transfer}, []*action.Vote{vote}).AnyTimes()
		},
		func(p2p *mock_network.MockOverlay) {
			if mockP2P == nil {
				p2p.EXPECT().Broadcast(gomock.Any()).Return(nil).AnyTimes()
			} else {
				mockP2P(p2p)
			}
		},
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	return cfsm
}

func newTestAddr() *iotxaddress.Address {
	addr, err := iotxaddress.NewAddress(true, iotxaddress.ChainID)
	if err != nil {
		logger.Panic().Err(err).Msg("error when creating test IoTeX address")
	}
	return addr
}
