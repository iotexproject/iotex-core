// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_network"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
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
		func(_ *mock_actpool.MockActPool) {},
		func(_ *mock_network.MockOverlay) {},
		clock.New(),
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	require.Equal(t, sEpochStart, cfsm.currentState())

	cfsm.Start(context.Background())
	defer cfsm.Stop(context.Background())

	for _, state := range consensusStates {
		cfsm.produce(cfsm.newBackdoorEvt(state), 0)
		testutil.WaitUntil(10*time.Millisecond, 100*time.Millisecond, func() (bool, error) {
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
		cfsm := newTestCFSM(t, testAddrs[0], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sDKGGeneration, s)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.height)
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.num)
		assert.Equal(t, uint(1), cfsm.ctx.epoch.numSubEpochs)
		crypto.SortCandidates(delegates, cfsm.ctx.epoch.num)
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
		cfsm := newTestCFSM(t, testAddrs[0], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sEpochStart, s)
		assert.NoError(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(0), cfsm.ctx.epoch.height)
		assert.Equal(t, eRollDelegates, (<-cfsm.evtq).Type())
	})
	t.Run("calcEpochNumAndHeight-error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(mockBlockchain *mock_blockchain.MockBlockchain) {
				mockBlockchain.EXPECT().TipHeight().Return(uint64(0)).Times(1)
				mockBlockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(nil, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sInvalid, s)
		assert.Error(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(0), cfsm.ctx.epoch.height)
		assert.Equal(t, eRollDelegates, (<-cfsm.evtq).Type())
	})
	t.Run("rollingDelegates-error", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates, func(mockBlockchain *mock_blockchain.MockBlockchain) {
				mockBlockchain.EXPECT().TipHeight().Return(uint64(1)).Times(1)
				mockBlockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(nil, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sInvalid, s)
		assert.Error(t, err)
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
		cfsm := newTestCFSM(t, testAddrs[2], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
		s, err := cfsm.handleGenerateDKGEvt(cfsm.newCEvt(eGenerateDKG))
		assert.Equal(t, sRoundStart, s)
		assert.NoError(t, err)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
	})
	t.Run("delay", func(t *testing.T) {
		cfsm := newTestCFSM(t, testAddrs[2], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
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
		cfsm := newTestCFSM(t, testAddrs[2], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
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
		assert.NotNil(t, cfsm.ctx.round.proposalEndorses, s)
		assert.NotNil(t, cfsm.ctx.round.commitEndorses, s)
		assert.Equal(t, eInitBlock, (<-cfsm.evtq).Type())
	})
	t.Run("is-not-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i+1].RawAddress
		}
		cfsm := newTestCFSM(t, testAddrs[1], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
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
		assert.NotNil(t, cfsm.ctx.round.proposalEndorses, s)
		assert.NotNil(t, cfsm.ctx.round.commitEndorses, s)
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
		testAddrs[2],
		ctrl,
		delegates,
		nil,
		func(p2p *mock_network.MockOverlay) {
			p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		},
		clock.New(),
	)
	cfsm.ctx.epoch = epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	cfsm.ctx.round = roundCtx{
		proposalEndorses: make(map[hash.Hash32B]map[string]bool),
		commitEndorses:   make(map[hash.Hash32B]map[string]bool),
		proposer:         delegates[2],
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
		height:           2,
		proposalEndorses: make(map[hash.Hash32B]map[string]bool),
		commitEndorses:   make(map[hash.Hash32B]map[string]bool),
		proposer:         delegates[2],
	}

	t.Run("pass-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			nil,
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()

		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))

		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.True(t, evt.endorse.decision)
		assert.Equal(t, eEndorseProposalTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("pass-validation-time-rotation", func(t *testing.T) {
		clock := clock.NewMock()
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			nil,
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			},
			clock,
		)
		cfsm.ctx.cfg.TimeBasedRotation = true
		cfsm.ctx.cfg.ProposerInterval = 10 * time.Second
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		clock.Add(11 * time.Second)
		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.True(t, evt.endorse.decision)
		assert.Equal(t, eEndorseProposalTimeout, (<-cfsm.evtq).Type())

		clock.Add(10 * time.Second)
		err = blk.SignBlock(testAddrs[3])
		assert.NoError(t, err)
		state, err = cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e = <-cfsm.evtq
		evt, ok = e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.True(t, evt.endorse.decision)
		assert.Equal(t, eEndorseProposalTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("fail-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Return(errors.New("mock error")).Times(1)
			},
			nil,
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
	})

	t.Run("skip-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Times(0)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.True(t, evt.endorse.decision)
		assert.Equal(t, eEndorseProposalTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("invalid-proposer", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			testAddrs[3],
			ctrl,
			delegates,
			nil,
			nil,
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
		state, err = cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eProposeBlockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
	})

	t.Run("invalid-proposer-time-rotation", func(t *testing.T) {
		clock := clock.NewMock()
		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			testAddrs[3],
			ctrl,
			delegates,
			nil,
			nil,
			clock,
		)
		cfsm.ctx.cfg.TimeBasedRotation = true
		cfsm.ctx.cfg.ProposerInterval = 10 * time.Second
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		clock.Add(11 * time.Second)
		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
		state, err = cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eProposeBlockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
	})

	t.Run("timeout", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Times(0)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eProposeBlockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		assert.Equal(t, eEndorseProposalTimeout, (<-cfsm.evtq).Type())
	})
}

func TestHandleProposalEndorseEvt(t *testing.T) {
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
		proposalEndorses: make(map[hash.Hash32B]map[string]bool),
		commitEndorses:   make(map[hash.Hash32B]map[string]bool),
		proposer:         delegates[2],
	}

	t.Run("gather-endorses", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		// First endorse prepare
		eEvt, err := newEndorseEvt(endorseProposal, blk.HashBlock(), true, round.height, testAddrs[0], cfsm.ctx.clock)
		assert.NoError(t, err)
		state, err := cfsm.handleEndorseProposalEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)

		// Second endorse prepare
		eEvt, err = newEndorseEvt(endorseProposal, blk.HashBlock(), true, round.height, testAddrs[1], cfsm.ctx.clock)
		assert.NoError(t, err)
		state, err = cfsm.handleEndorseProposalEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)

		// Third endorse prepare, could move on
		eEvt, err = newEndorseEvt(endorseProposal, blk.HashBlock(), true, round.height, testAddrs[2], cfsm.ctx.clock)
		assert.NoError(t, err)
		state, err = cfsm.handleEndorseProposalEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseCommit, evt.Type())
		assert.True(t, evt.endorse.decision)
		assert.Equal(t, eEndorseCommitTimeout, (<-cfsm.evtq).Type())
	})

	t.Run("timeout", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			},
			nil,
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseProposalTimeout(cfsm.newCEvt(eEndorseProposalTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*timeoutEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseCommitTimeout, evt.Type())
	})
}

func TestHandleCommitEndorseEvt(t *testing.T) {
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
		proposalEndorses: make(map[hash.Hash32B]map[string]bool),
		commitEndorses:   make(map[hash.Hash32B]map[string]bool),
		proposer:         delegates[2],
	}

	t.Run("gather-commits", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		// First endorse prepare
		eEvt, err := newEndorseEvt(endorseCommit, blk.HashBlock(), true, round.height, testAddrs[0], cfsm.ctx.clock)
		assert.NoError(t, err)
		state, err := cfsm.handleEndorseCommitEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)

		// Second endorse prepare
		eEvt, err = newEndorseEvt(endorseCommit, blk.HashBlock(), true, round.height, testAddrs[1], cfsm.ctx.clock)
		assert.NoError(t, err)
		state, err = cfsm.handleEndorseCommitEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)

		// Third endorse prepare, could move on
		eEvt, err = newEndorseEvt(endorseCommit, blk.HashBlock(), true, round.height, testAddrs[2], cfsm.ctx.clock)
		assert.NoError(t, err)
		state, err = cfsm.handleEndorseCommitEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
	})
	t.Run("timeout-blocking", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(0)
				chain.EXPECT().
					MintNewDummyBlock().
					Return(blockchain.NewBlock(0, 0, hash.ZeroHash32B, clock.New(), nil, nil, nil)).Times(0)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(0)
			},
			clock.New(),
		)
		cfsm.ctx.cfg.EnableDummyBlock = false
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseCommitTimeout(cfsm.newCEvt(eEndorseCommitTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
	})
	t.Run("timeout-dummy-block", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
				chain.EXPECT().
					MintNewDummyBlock().
					Return(blockchain.NewBlock(0, 0, hash.ZeroHash32B, clock.New(), nil, nil, nil)).Times(1)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			},
			func(p2p *mock_network.MockOverlay) {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).Times(1)
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseCommitTimeout(cfsm.newCEvt(eEndorseCommitTimeout))
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
		proposalEndorses: make(map[hash.Hash32B]map[string]bool),
		commitEndorses:   make(map[hash.Hash32B]map[string]bool),
		proposer:         delegates[2],
	}

	t.Run("not-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(1)).Times(2)
			},
			nil,
			clock.New(),
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
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(4)).Times(1)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			},
			nil,
			clock.New(),
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
	proposer *iotxaddress.Address,
	ctrl *gomock.Controller,
	delegates []string,
	mockChain func(*mock_blockchain.MockBlockchain),
	mockP2P func(*mock_network.MockOverlay),
	clock clock.Clock,
) *cFSM {
	transfer, err := action.NewTransfer(1, big.NewInt(100), "src", "dst", []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	selfPubKey := testaddress.Addrinfo["producer"].PublicKey
	require.NoError(t, err)
	address, err := iotxaddress.GetAddressByPubkey(iotxaddress.IsTestnet, iotxaddress.ChainID, selfPubKey)
	require.NoError(t, err)
	vote, err := action.NewVote(2, address.RawAddress, address.RawAddress, uint64(100000), big.NewInt(10))
	require.NoError(t, err)
	var prevHash hash.Hash32B
	lastBlk := blockchain.NewBlock(
		config.Default.Chain.ID,
		1,
		prevHash,
		clock,
		make([]*action.Transfer, 0),
		make([]*action.Vote, 0),
		make([]*action.Execution, 0),
	)
	blkToMint := blockchain.NewBlock(
		config.Default.Chain.ID,
		2,
		lastBlk.HashBlock(),
		clock,
		[]*action.Transfer{transfer},
		[]*action.Vote{vote},
		nil,
	)
	blkToMint.SignBlock(proposer)
	ctx := makeTestRollDPoSCtx(
		addr,
		ctrl,
		config.RollDPoS{
			EventChanSize:    2,
			NumDelegates:     uint(len(delegates)),
			EnableDummyBlock: true,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			blockchain.EXPECT().GetBlockByHeight(uint64(1)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().
				MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(blkToMint, nil).
				AnyTimes()
			if mockChain == nil {
				blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
					{Address: delegates[0]},
					{Address: delegates[1]},
					{Address: delegates[2]},
					{Address: delegates[3]},
				}, nil).AnyTimes()
				blockchain.EXPECT().TipHeight().Return(uint64(1)).AnyTimes()
				blockchain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			} else {
				mockChain(blockchain)
			}
		},
		func(actPool *mock_actpool.MockActPool) {
			actPool.EXPECT().
				PickActs().
				Return([]*action.Transfer{transfer}, []*action.Vote{vote}, []*action.Execution{}).
				AnyTimes()
			actPool.EXPECT().Reset().AnyTimes()
		},
		func(p2p *mock_network.MockOverlay) {
			if mockP2P == nil {
				p2p.EXPECT().Broadcast(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			} else {
				mockP2P(p2p)
			}
		},
		clock,
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	return cfsm
}

func newTestAddr() *iotxaddress.Address {
	addr, err := iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		logger.Panic().Err(err).Msg("error when creating test IoTeX address")
	}
	return addr
}

func TestUpdateSeed(t *testing.T) {
	require := require.New(t)
	lastSeed, _ := hex.DecodeString("9de6306b08158c423330f7a27243a1a5cbe39bfd764f07818437882d21241567")
	chain := blockchain.NewBlockchain(&config.Default, blockchain.InMemStateFactoryOption(), blockchain.InMemDaoOption())
	require.NoError(chain.Start(context.Background()))
	ctx := rollDPoSCtx{cfg: config.Default.Consensus.RollDPoS, chain: chain, epoch: epochCtx{seed: lastSeed}}
	fsm := cFSM{ctx: &ctx}

	var err error
	const numNodes = 21
	addresses := make([]*iotxaddress.Address, numNodes)
	skList := make([][]uint32, numNodes)
	idList := make([][]uint8, numNodes)
	coeffsList := make([][][]uint32, numNodes)
	sharesList := make([][][]uint32, numNodes)
	shares := make([][]uint32, numNodes)
	witnessesList := make([][][]byte, numNodes)
	sharestatusmatrix := make([][numNodes]bool, numNodes)
	qsList := make([][]byte, numNodes)
	pkList := make([][]byte, numNodes)
	askList := make([][]uint32, numNodes)

	// Generate 21 identifiers for the delegates
	for i := 0; i < numNodes; i++ {
		addresses[i], _ = iotxaddress.NewAddress(iotxaddress.IsTestnet, iotxaddress.ChainID)
		idList[i] = iotxaddress.CreateID(addresses[i].RawAddress)
		skList[i] = crypto.DKG.SkGeneration()
	}

	// Initialize DKG and generate secret shares
	for i := 0; i < numNodes; i++ {
		coeffsList[i], sharesList[i], witnessesList[i], err = crypto.DKG.Init(skList[i], idList)
		require.NoError(err)
	}

	// Verify all the received secret shares
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			result, err := crypto.DKG.ShareVerify(idList[i], sharesList[j][i], witnessesList[j])
			require.NoError(err)
			require.True(result)
			shares[j] = sharesList[j][i]
		}
		sharestatusmatrix[i], err = crypto.DKG.SharesCollect(idList[i], shares, witnessesList)
		require.NoError(err)
		for _, b := range sharestatusmatrix[i] {
			require.True(b)
		}
	}

	// Generate private and public key shares of a group key
	for i := 0; i < numNodes; i++ {
		for j := 0; j < numNodes; j++ {
			shares[j] = sharesList[j][i]
		}
		qsList[i], pkList[i], askList[i], err = crypto.DKG.KeyPairGeneration(shares, sharestatusmatrix)
		require.NoError(err)
	}

	// Generate dkg signature for each block
	require.NoError(err)
	dummy := chain.MintNewDummyBlock()
	err = chain.CommitBlock(dummy)
	require.NoError(err)
	for i := 1; i < numNodes; i++ {
		blk, err := chain.MintNewDKGBlock(nil, nil, nil, addresses[i],
			&iotxaddress.DKGAddress{PrivateKey: askList[i], PublicKey: pkList[i], ID: idList[i]},
			lastSeed, "")
		require.NoError(err)
		err = verifyDKGSignature(blk, lastSeed)
		require.NoError(err)
		err = chain.CommitBlock(blk)
		require.NoError(err)
		require.Equal(pkList[i], blk.Header.DKGPubkey)
		require.Equal(idList[i], blk.Header.DKGID)
		require.True(len(blk.Header.DKGBlockSig) > 0)
	}
	height := chain.TipHeight()
	require.Equal(int(height), 21)

	newSeed, err := fsm.updateSeed()
	require.NoError(err)
	require.True(len(newSeed) > 0)
	require.NotEqual(fsm.ctx.epoch.seed, newSeed)
	fmt.Println(fsm.ctx.epoch.seed)
	fmt.Println(newSeed)
}
