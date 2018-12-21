// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := makeTestRollDPoSCtx(
		testAddrs[0],
		ctrl,
		config.RollDPoS{
			EventChanSize: 1,
			EnableDKG:     true,
		},
		func(mockBlockchain *mock_blockchain.MockBlockchain) {
			mockBlockchain.EXPECT().TipHeight().Return(uint64(0)).Times(8)
		},
		func(_ *mock_actpool.MockActPool) {},
		nil,
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
		assert.Equal(t, uint(2), cfsm.ctx.epoch.numSubEpochs)
		crypto.SortCandidates(delegates, cfsm.ctx.epoch.num, crypto.CryptoSeed)
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
				mockBlockchain.EXPECT().TipHeight().Return(uint64(0)).Times(2)
				mockBlockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(nil, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sEpochStart, s)
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
			delegates,
			func(mockBlockchain *mock_blockchain.MockBlockchain) {
				mockBlockchain.EXPECT().TipHeight().Return(uint64(1)).Times(2)
				mockBlockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(nil, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		s, err := cfsm.handleRollDelegatesEvt(cfsm.newCEvt(eRollDelegates))
		assert.Equal(t, sEpochStart, s)
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

	delegates := make([]string, 21)
	test21Addrs := test21Addrs()
	for i, addr := range test21Addrs {
		delegates[i] = addr.RawAddress
	}
	t.Run("no-delay", func(t *testing.T) {
		cfsm := newTestCFSM(t, test21Addrs[2], test21Addrs[2], ctrl, delegates, nil, nil, clock.New())
		s, err := cfsm.handleGenerateDKGEvt(cfsm.newCEvt(eGenerateDKG))
		assert.Equal(t, sRoundStart, s)
		assert.NoError(t, err)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
	})
	t.Run("delay", func(t *testing.T) {
		cfsm := newTestCFSM(t, test21Addrs[2], test21Addrs[2], ctrl, delegates, nil, nil, clock.New())
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
		require := require.New(t)
		s, err := cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		require.NoError(err)
		require.Equal(sBlockPropose, s)
		require.Equal(uint64(0), cfsm.ctx.epoch.subEpochNum)
		require.NotNil(cfsm.ctx.round.proposer, delegates[2])
		require.NotNil(cfsm.ctx.round.endorsementSets, s)
		e := <-cfsm.evtq
		require.Equal(eInitBlockPropose, e.Type())
		e = <-cfsm.evtq
		require.Equal(eProposeBlockTimeout, e.Type())
		e = <-cfsm.evtq
		require.Equal(eEndorseProposalTimeout, e.Type())
		e = <-cfsm.evtq
		require.Equal(eEndorseLockTimeout, e.Type())
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
		require := require.New(t)
		s, err := cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		require.NoError(err)
		require.Equal(sBlockPropose, s)
		require.Equal(uint64(0), cfsm.ctx.epoch.subEpochNum)
		require.NotNil(cfsm.ctx.round.proposer, delegates[2])
		require.NotNil(cfsm.ctx.round.endorsementSets, s)
		evt := <-cfsm.evtq
		require.Equal(eInitBlockPropose, evt.Type())
		s, err = cfsm.handleInitBlockProposeEvt(evt)
		require.Equal(sAcceptPropose, s)
		require.NoError(err)
		evt = <-cfsm.evtq
		require.Equal(eProposeBlockTimeout, evt.Type())
		s, err = cfsm.handleProposeBlockTimeout(evt)
		require.Equal(sAcceptProposalEndorse, s)
		require.NoError(err)
		evt = <-cfsm.evtq
		require.Equal(eEndorseProposalTimeout, evt.Type())
		s, err = cfsm.handleEndorseProposalTimeout(evt)
		require.NoError(err)
		require.Equal(sAcceptLockEndorse, s)
		evt = <-cfsm.evtq
		require.Equal(eEndorseLockTimeout, evt.Type())
		s, err = cfsm.handleEndorseLockTimeout(evt)
		require.NoError(err)
		require.Equal(sRoundStart, s)
	})
}

func TestHandleInitBlockEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 21)
	test21Addrs := test21Addrs()
	for i, addr := range test21Addrs {
		delegates[i] = addr.RawAddress
	}

	t.Run("secret-block", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			test21Addrs[2],
			test21Addrs[2],
			ctrl,
			delegates,
			nil,
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch.numSubEpochs = uint(2)
		cfsm.ctx.round = roundCtx{
			endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
			proposer:        delegates[2],
		}
		cfsm.ctx.epoch.subEpochNum = uint64(0)
		s, err := cfsm.handleInitBlockProposeEvt(cfsm.newCEvt(eInitBlockPropose))
		require.NoError(t, err)
		require.Equal(t, sAcceptPropose, s)
		e := <-cfsm.evtq
		require.Equal(t, eProposeBlock, e.Type())
		pbe, ok := e.(*proposeBlkEvt)
		require.True(t, ok)
		require.NotNil(t, pbe.block)
		require.Equal(t, len(delegates), len(pbe.block.SecretProposals))
		require.NotNil(t, pbe.block.SecretWitness)
		assert.Equal(t, 1, broadcastCount)
	})
	t.Run("normal-block", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			test21Addrs[2],
			test21Addrs[2],
			ctrl,
			delegates,
			nil,
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch.numSubEpochs = uint(2)
		cfsm.ctx.round = roundCtx{
			endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
			proposer:        delegates[2],
		}
		cfsm.ctx.epoch.subEpochNum = uint64(1)
		s, err := cfsm.handleInitBlockProposeEvt(cfsm.newCEvt(eInitBlockPropose))
		require.NoError(t, err)
		require.Equal(t, sAcceptPropose, s)
		e := <-cfsm.evtq
		require.Equal(t, eProposeBlock, e.Type())
		pbe, ok := e.(*proposeBlkEvt)
		require.True(t, ok)
		require.NotNil(t, pbe.block)
		transfers, votes, _ := action.ClassifyActions(pbe.block.Actions)
		require.Equal(t, 1, len(transfers))
		require.Equal(t, 1, len(votes))
		assert.Equal(t, 1, broadcastCount)
	})
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
		height:          2,
		number:          0,
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[2],
	}

	t.Run("pass-validation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			nil,
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintCommonBlock()

		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))

		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.Equal(t, 1, broadcastCount)
	})

	t.Run("pass-validation-time-rotation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		clock := clock.NewMock()
		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			nil,
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock,
		)
		cfsm.ctx.cfg.TimeBasedRotation = true
		cfsm.ctx.cfg.ProposerInterval = 10 * time.Second
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		clock.Add(11 * time.Second)
		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())

		clock.Add(10 * time.Second)
		state, err = cfsm.handleStartRoundEvt(cfsm.newCEvt(eStartRound))
		assert.Equal(t, sBlockPropose, state)
		assert.NoError(t, err)
		e = <-cfsm.evtq
		cevt, ok := e.(*consensusEvt)
		require.True(t, ok)
		assert.Equal(t, eInitBlockPropose, cevt.Type())
		blk.Header.Pubkey = testAddrs[3].PublicKey
		err = blk.SignBlock(testAddrs[3])
		assert.NoError(t, err)
		state, err = cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		<-cfsm.evtq
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.Equal(t, 2, broadcastCount)
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

		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
	})

	t.Run("skip-validation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Times(0)
			},
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseProposal, evt.Type())
		assert.Equal(t, 1, broadcastCount)
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

		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
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
		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
		state, err = cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eProposeBlockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
	})

	t.Run("timeout", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			testAddrs[2],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Times(0)
			},
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eProposeBlockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		assert.Equal(t, 0, broadcastCount)
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
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[2],
	}

	t.Run("gather-endorses", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			testAddrs[0],
			testAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			},
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		// First endorse prepare
		eEvt := newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[0], cfsm.ctx.clock)
		state, err := cfsm.handleEndorseProposalEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)

		// Second endorse prepare
		eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[1], cfsm.ctx.clock)
		state, err = cfsm.handleEndorseProposalEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)

		// Third endorse prepare, could move on
		eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[2], cfsm.ctx.clock)
		state, err = cfsm.handleEndorseProposalEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptLockEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eEndorseLock, evt.Type())
		assert.Equal(t, 1, broadcastCount)
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
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			},
			nil,
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseProposalTimeout(cfsm.newCEvt(eEndorseProposalTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptLockEndorse, state)
	})
}

func TestHandleCommitEndorseEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 21)
	test21Addrs := test21Addrs()
	for i, addr := range test21Addrs {
		delegates[i] = addr.RawAddress
	}

	round := roundCtx{
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[2],
	}

	t.Run("gather-commits-secret-block", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			},
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch.numSubEpochs = uint(2)
		cfsm.ctx.epoch.subEpochNum = uint64(0)
		cfsm.ctx.epoch.delegates = delegates
		cfsm.ctx.epoch.committedSecrets = make(map[string][]uint32)
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		for i := 0; i < 14; i++ {
			eEvt := newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, test21Addrs[i], cfsm.ctx.clock)
			state, err := cfsm.handleEndorseLockEvt(eEvt)
			assert.NoError(t, err)
			assert.Equal(t, sAcceptLockEndorse, state)
			assert.Equal(t, 0, len(cfsm.ctx.epoch.committedSecrets))
		}

		// 15th endorse prepare, could move on
		eEvt := newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, test21Addrs[14], cfsm.ctx.clock)
		state, err := cfsm.handleEndorseLockEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)
		evt := <-cfsm.evtq
		assert.Equal(t, eEndorseCommit, evt.Type())
		state, err = cfsm.handleEndorseCommitEvt(evt)
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
		assert.Equal(t, 1, len(cfsm.ctx.epoch.committedSecrets))
		assert.Equal(t, 2, broadcastCount)
	})
	t.Run("gather-commits-common-block", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			},
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)
		cfsm.ctx.epoch.numSubEpochs = uint(2)
		cfsm.ctx.epoch.subEpochNum = uint64(1)
		cfsm.ctx.epoch.delegates = delegates
		cfsm.ctx.round = round

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		for i := 0; i < 14; i++ {
			eEvt := newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, test21Addrs[i], cfsm.ctx.clock)
			state, err := cfsm.handleEndorseLockEvt(eEvt)
			assert.NoError(t, err)
			assert.Equal(t, sAcceptLockEndorse, state)
		}

		// 15th endorse prepare, could move on
		eEvt := newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, test21Addrs[14], cfsm.ctx.clock)
		state, err := cfsm.handleEndorseLockEvt(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)
		evt := <-cfsm.evtq
		assert.Equal(t, eEndorseCommit, evt.Type())
		state, err = cfsm.handleEndorseCommitEvt(evt)
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
		assert.Equal(t, 2, broadcastCount)
	})
	t.Run("timeout-blocking", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(0)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			},
			func(_ proto.Message) error {
				broadcastMutex.Lock()
				defer broadcastMutex.Unlock()
				broadcastCount++
				return nil
			},
			clock.New(),
		)

		blk, err := cfsm.ctx.mintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseLockTimeout(cfsm.newCEvt(eEndorseLockTimeout))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eFinishEpoch, (<-cfsm.evtq).Type())
		assert.Equal(t, 0, broadcastCount)
	})
}

func TestOneDelegate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []string{testAddrs[0].RawAddress}
	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		height:          2,
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[0],
	}
	broadcastCount := 0
	var broadcastMutex sync.Mutex
	cfsm := newTestCFSM(
		t,
		testAddrs[0],
		testAddrs[0],
		ctrl,
		delegates,
		func(chain *mock_blockchain.MockBlockchain) {
			chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
			chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			chain.EXPECT().TipHeight().Return(uint64(2)).Times(1)
		},
		func(_ proto.Message) error {
			broadcastMutex.Lock()
			defer broadcastMutex.Unlock()
			broadcastCount++
			return nil
		},
		clock.New(),
	)
	cfsm.ctx.epoch = epoch
	cfsm.ctx.round = round
	cfsm.ctx.cfg.EnableDKG = false

	// propose block
	blk, err := cfsm.ctx.mintCommonBlock()
	require.NoError(err)
	state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	evt := <-cfsm.evtq
	eEvt, ok := evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseProposal, eEvt.Type())
	// endorse proposal
	state, err = cfsm.handleEndorseProposalEvt(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseLock, eEvt.Type())
	// endorse lock
	state, err = cfsm.handleEndorseLockEvt(eEvt)
	require.NoError(err)
	require.Equal(sAcceptCommitEndorse, state)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseCommit, eEvt.Type())
	// endorse commit
	state, err = cfsm.handleEndorseCommitEvt(eEvt)
	require.NoError(err)
	require.Equal(sRoundStart, state)
	evt = <-cfsm.evtq
	cEvt, ok := evt.(*consensusEvt)
	require.True(ok)
	require.Equal(eFinishEpoch, cEvt.Type())
	// new round
	state, err = cfsm.handleFinishEpochEvt(cEvt)
	require.Equal(sEpochStart, state)
	require.NoError(err)
	assert.Equal(t, 4, broadcastCount)
}

func TestTwoDelegates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []string{testAddrs[0].RawAddress, testAddrs[1].RawAddress}
	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(2),
	}
	round := roundCtx{
		height:          2,
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[0],
	}
	broadcastCount := 0
	var broadcastMutex sync.Mutex
	cfsm := newTestCFSM(
		t,
		testAddrs[0],
		testAddrs[0],
		ctrl,
		delegates,
		func(chain *mock_blockchain.MockBlockchain) {
			chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
			chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			chain.EXPECT().TipHeight().Return(uint64(2)).Times(2)
		},
		func(_ proto.Message) error {
			broadcastMutex.Lock()
			defer broadcastMutex.Unlock()
			broadcastCount++
			return nil
		},
		clock.New(),
	)
	cfsm.ctx.epoch = epoch
	cfsm.ctx.round = round
	cfsm.ctx.cfg.EnableDKG = false

	// propose block
	blk, err := cfsm.ctx.mintCommonBlock()
	require.NoError(err)
	state, err := cfsm.handleProposeBlockEvt(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	evt := <-cfsm.evtq
	eEvt, ok := evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseProposal, eEvt.Type())
	// endorse proposal
	state, err = cfsm.handleEndorseProposalEvt(eEvt)
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.handleEndorseProposalEvt(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseLock, eEvt.Type())
	// endorse lock
	state, err = cfsm.handleEndorseLockEvt(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	eEvt = newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, testAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.handleEndorseLockEvt(eEvt)
	require.Equal(sAcceptCommitEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseCommit, eEvt.Type())
	// endorse lock
	state, err = cfsm.handleEndorseCommitEvt(eEvt)
	require.NoError(err)
	require.Equal(sRoundStart, state)
	evt = <-cfsm.evtq
	cEvt, ok := evt.(*consensusEvt)
	require.True(ok)
	require.Equal(eFinishEpoch, cEvt.Type())
	// new round
	state, err = cfsm.handleFinishEpochEvt(cEvt)
	require.Equal(sRoundStart, state)
	require.NoError(err)
	assert.Equal(t, 4, broadcastCount)
}

func TestThreeDelegates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []string{testAddrs[0].RawAddress, testAddrs[1].RawAddress, testAddrs[2].RawAddress}
	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(3),
	}
	round := roundCtx{
		height:          2,
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[2],
	}
	broadcastCount := 0
	var broadcastMutex sync.Mutex
	cfsm := newTestCFSM(
		t,
		testAddrs[0],
		testAddrs[2],
		ctrl,
		delegates,
		func(chain *mock_blockchain.MockBlockchain) {
			chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
			chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			chain.EXPECT().TipHeight().Return(uint64(2)).Times(2)
		},
		func(_ proto.Message) error {
			broadcastMutex.Lock()
			defer broadcastMutex.Unlock()
			broadcastCount++
			return nil
		},
		clock.New(),
	)
	cfsm.ctx.epoch = epoch
	cfsm.ctx.round = round
	cfsm.ctx.cfg.EnableDKG = false

	blk, err := cfsm.ctx.mintCommonBlock()
	require.NoError(err)
	cfsm.ctx.round.block = blk
	// endorse proposal
	// handle self endorsement
	eEvt := newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[0], cfsm.ctx.clock)
	state, err := cfsm.handleEndorseProposalEvt(eEvt)
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	// handle delegate 1's endorsement
	eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.handleEndorseProposalEvt(eEvt)
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	// handle delegate 2's endorsement
	eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.HashBlock(), round.height, round.number, testAddrs[2], cfsm.ctx.clock)
	state, err = cfsm.handleEndorseProposalEvt(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	evt := <-cfsm.evtq
	eEvt, ok := evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseLock, eEvt.Type())
	// endorse lock
	// handle self endorsement
	state, err = cfsm.handleEndorseLockEvt(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	// handle delegate 1's endorsement
	eEvt = newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, testAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.handleEndorseLockEvt(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	// handle delegate 2's endorsement
	eEvt = newEndorseEvt(endorsement.LOCK, blk.HashBlock(), round.height, round.number, testAddrs[2], cfsm.ctx.clock)
	state, err = cfsm.handleEndorseLockEvt(eEvt)
	require.NoError(err)
	require.Equal(sAcceptCommitEndorse, state)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eEndorseCommit, eEvt.Type())
	state, err = cfsm.handleEndorseCommitEvt(eEvt)
	require.NoError(err)
	require.Equal(sRoundStart, state)
	evt = <-cfsm.evtq
	cEvt, ok := evt.(*consensusEvt)
	require.True(ok)
	require.Equal(eFinishEpoch, cEvt.Type())
	// new round
	state, err = cfsm.handleFinishEpochEvt(cEvt)
	require.Equal(sRoundStart, state)
	require.NoError(err)
	assert.Equal(t, 3, broadcastCount)
}

func TestHandleFinishEpochEvt(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := make([]string, 21)
	test21Addrs := test21Addrs()
	for i, addr := range test21Addrs {
		delegates[i] = addr.RawAddress
	}

	epoch := epochCtx{
		delegates:    delegates,
		num:          uint64(1),
		height:       uint64(1),
		numSubEpochs: uint(2),
	}
	round := roundCtx{
		endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		proposer:        delegates[2],
	}
	t.Run("dkg-not-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(1)).Times(3)
			},
			nil,
			clock.New(),
		)
		epoch.subEpochNum = uint64(0)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleFinishEpochEvt(cfsm.newCEvt(eFinishEpoch))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
		assert.Nil(t, cfsm.ctx.epoch.dkgAddress.PublicKey)
		assert.Nil(t, cfsm.ctx.epoch.dkgAddress.PrivateKey)
	})
	t.Run("dkg-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(21)).Times(3)
			},
			nil,
			clock.New(),
		)
		epoch.subEpochNum = uint64(0)
		epoch.committedSecrets = make(map[string][]uint32)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		idList := make([][]uint8, 0)
		for _, addr := range delegates {
			dkgID := iotxaddress.CreateID(addr)
			idList = append(idList, dkgID)
		}
		for i, delegate := range delegates {
			_, secrets, _, err := crypto.DKG.Init(crypto.DKG.SkGeneration(), idList)
			assert.NoError(t, err)
			assert.NotNil(t, secrets)
			if i%2 != 0 {
				cfsm.ctx.epoch.committedSecrets[delegate] = secrets[0]
			}
		}

		state, err := cfsm.handleFinishEpochEvt(cfsm.newCEvt(eFinishEpoch))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
		assert.NotNil(t, cfsm.ctx.epoch.dkgAddress.PublicKey)
		assert.NotNil(t, cfsm.ctx.epoch.dkgAddress.PrivateKey)
	})
	t.Run("epoch-not-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(22)).Times(2)
			},
			nil,
			clock.New(),
		)
		epoch.subEpochNum = uint64(1)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		state, err := cfsm.handleFinishEpochEvt(cfsm.newCEvt(eFinishEpoch))
		assert.NoError(t, err)
		assert.Equal(t, sRoundStart, state)
		assert.Equal(t, eStartRound, (<-cfsm.evtq).Type())
	})
	t.Run("epoch-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(42)).Times(1)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			},
			nil,
			clock.New(),
		)
		epoch.subEpochNum = uint64(1)
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
	broadcastCB func(proto.Message) error,
	clock clock.Clock,
) *cFSM {
	a := testaddress.Addrinfo["alfa"]
	b := testaddress.Addrinfo["bravo"]
	transfer, err := testutil.SignedTransfer(a, b, 1, big.NewInt(100), []byte{}, 100000, big.NewInt(10))
	require.NoError(t, err)
	vote, err := testutil.SignedVote(a, a, 2, 100000, big.NewInt(10))
	require.NoError(t, err)
	var prevHash hash.Hash32B
	lastBlk := blockchain.NewBlock(
		config.Default.Chain.ID,
		1,
		prevHash,
		testutil.TimestampNowFromClock(clock),
		proposer.PublicKey,
		make([]action.SealedEnvelope, 0),
	)
	blkToMint := blockchain.NewBlock(
		config.Default.Chain.ID,
		2,
		lastBlk.HashBlock(),
		testutil.TimestampNowFromClock(clock),
		proposer.PublicKey,
		[]action.SealedEnvelope{transfer, vote},
	)
	blkToMint.SignBlock(proposer)

	var secretBlkToMint *blockchain.Block
	var proposerSecrets [][]uint32
	var proposerWitness [][]byte
	if len(delegates) == 21 {
		idList := make([][]uint8, 0)
		for _, addr := range delegates {
			dkgID := iotxaddress.CreateID(addr)
			idList = append(idList, dkgID)
		}
		_, secrets, witness, err := crypto.DKG.Init(crypto.DKG.SkGeneration(), idList)
		require.NoError(t, err)
		proposerSecrets = secrets
		proposerWitness = witness
		nonce := uint64(1)
		secretProposals := make([]*action.SecretProposal, 0)
		for i, delegate := range delegates {
			secretProposal, err := action.NewSecretProposal(nonce, proposer.RawAddress, delegate, secrets[i])
			require.NoError(t, err)
			secretProposals = append(secretProposals, secretProposal)
			nonce++
		}
		secretWitness, err := action.NewSecretWitness(nonce, proposer.RawAddress, witness)
		require.NoError(t, err)

		secretBlkToMint = blockchain.NewSecretBlock(
			config.Default.Chain.ID,
			2,
			lastBlk.HashBlock(),
			testutil.TimestampNowFromClock(clock),
			proposer.PublicKey,
			secretProposals,
			secretWitness,
		)
		secretBlkToMint.SignBlock(proposer)
	}

	ctx := makeTestRollDPoSCtx(
		addr,
		ctrl,
		config.RollDPoS{
			AcceptProposeTTL:         300 * time.Millisecond,
			AcceptProposalEndorseTTL: 300 * time.Millisecond,
			AcceptCommitEndorseTTL:   300 * time.Millisecond,
			EventChanSize:            2,
			NumDelegates:             uint(len(delegates)),
			EnableDKG:                true,
		},
		func(blockchain *mock_blockchain.MockBlockchain) {
			blockchain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			blockchain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
			blockchain.EXPECT().GetBlockByHeight(uint64(1)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().GetBlockByHeight(uint64(2)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().GetBlockByHeight(uint64(21)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().GetBlockByHeight(uint64(22)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().
				MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(blkToMint, nil).AnyTimes()
			blockchain.EXPECT().
				MintNewSecretBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(secretBlkToMint, nil).AnyTimes()
			blockchain.EXPECT().
				Nonce(gomock.Any()).Return(uint64(0), nil).AnyTimes()
			if mockChain == nil {
				candidates := make([]*state.Candidate, 0)
				for _, delegate := range delegates {
					candidates = append(candidates, &state.Candidate{Address: delegate})
				}
				blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).AnyTimes()
				blockchain.EXPECT().TipHeight().Return(uint64(1)).AnyTimes()
				blockchain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			} else {
				mockChain(blockchain)
			}
		},
		func(actPool *mock_actpool.MockActPool) {
			actPool.EXPECT().
				PickActs().
				Return([]action.SealedEnvelope{transfer, vote}).
				AnyTimes()
			actPool.EXPECT().Reset().AnyTimes()
		},
		broadcastCB,
		clock,
	)
	ctx.epoch.delegates = delegates
	ctx.epoch.secrets = proposerSecrets
	ctx.epoch.witness = proposerWitness
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	return cfsm
}

func newTestAddr() *iotxaddress.Address {
	pk, sk, err := crypto.EC283.NewKeyPair()
	if err != nil {
		logger.Panic().Err(err).Msg("error when creating test IoTeX address")
	}
	pkHash := keypair.HashPubKey(pk)
	addr := address.New(config.Default.Chain.ID, pkHash[:])
	iotxAddr := iotxaddress.Address{
		PublicKey:  pk,
		PrivateKey: sk,
		RawAddress: addr.IotxAddress(),
	}
	return &iotxAddr
}

func test21Addrs() []*iotxaddress.Address {
	addrs := make([]*iotxaddress.Address, 0)
	for i := 0; i < 21; i++ {
		addrs = append(addrs, newTestAddr())
	}
	return addrs
}
