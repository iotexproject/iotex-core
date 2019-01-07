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
	"github.com/iotexproject/iotex-core/blockchain/block"
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
	"github.com/iotexproject/iotex-core/test/mock/mock_factory"
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
		func(mockBlockchain *mock_blockchain.MockBlockchain) {},
		func(_ *mock_actpool.MockActPool) {},
		nil,
		clock.New(),
	)
	cfsm, err := newConsensusFSM(ctx)
	require.Nil(t, err)
	require.NotNil(t, cfsm)
	require.Equal(t, sPrepare, cfsm.currentState())

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
		epochNum := uint64(1)
		crypto.SortCandidates(delegates, epochNum, crypto.CryptoSeed)
		delegateAddrs := make([]*iotxaddress.Address, 4)
		for i := 0; i < 4; i++ {
			for j := 0; j < 4; j++ {
				if delegates[i] == testAddrs[j].RawAddress {
					delegateAddrs[i] = testAddrs[j]
					break
				}
			}
		}
		cfsm := newTestCFSM(t, delegateAddrs[0], delegateAddrs[2], ctrl, delegates, nil, nil, clock.New())
		cfsm.ctx.cfg.EnableDKG = false
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.Equal(t, sAcceptPropose, s)
		assert.NoError(t, err)
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.height)
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.num)
		assert.Equal(t, uint(1), cfsm.ctx.epoch.numSubEpochs)
		crypto.SortCandidates(delegates, cfsm.ctx.epoch.num, crypto.CryptoSeed)
		assert.Equal(t, delegates, cfsm.ctx.epoch.delegates)
	})
	t.Run("is-not-delegate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i+1].RawAddress
		}
		cfsm := newTestCFSM(t, testAddrs[0], testAddrs[2], ctrl, delegates, nil, nil, clock.New())
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.Equal(t, sPrepare, s)
		assert.NoError(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(1), cfsm.ctx.epoch.height)
		assert.Equal(t, ePrepare, (<-cfsm.evtq).Type())
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
				mockBlockchain.EXPECT().TipHeight().Return(uint64(0)).Times(3)
				mockBlockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(nil, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.Equal(t, sPrepare, s)
		assert.Error(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(0), cfsm.ctx.epoch.height)
		assert.Equal(t, ePrepare, (<-cfsm.evtq).Type())
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
				mockBlockchain.EXPECT().TipHeight().Return(uint64(1)).Times(3)
				mockBlockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return(nil, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.Equal(t, sPrepare, s)
		assert.Error(t, err)
		// epoch ctx not set
		assert.Equal(t, uint64(0), cfsm.ctx.epoch.height)
		assert.Equal(t, ePrepare, (<-cfsm.evtq).Type())
	})
}

/*
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
*/

func TestStartRoundEvt(t *testing.T) {
	t.Parallel()

	t.Run("is-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		epochNum := uint64(0)
		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		crypto.SortCandidates(delegates, epochNum+1, crypto.CryptoSeed)
		proposerIdx := 0
		for i := 0; i < 4; i++ {
			if delegates[2] == testAddrs[i].RawAddress {
				proposerIdx = i
				break
			}
		}
		cfsm := newTestCFSM(t, testAddrs[proposerIdx], testAddrs[proposerIdx], ctrl, delegates, nil, nil, clock.New())
		cfsm.ctx.epoch = epochCtx{
			delegates:    delegates,
			num:          epochNum,
			height:       uint64(1),
			numSubEpochs: uint(1),
		}
		cfsm.ctx.cfg.EnableDKG = false
		require := require.New(t)
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		require.NoError(err)
		require.Equal(sAcceptPropose, s)
		require.Equal(uint64(0), cfsm.ctx.epoch.subEpochNum)
		require.Equal(cfsm.ctx.round.proposer, testAddrs[proposerIdx].RawAddress)
		require.NotNil(cfsm.ctx.round.endorsementSets)
		e := <-cfsm.evtq
		require.Equal(eReceiveBlock, e.Type())
	})
	t.Run("is-not-proposer", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		epochNum := uint64(0)
		delegates := make([]string, 4)
		for i := 0; i < 4; i++ {
			delegates[i] = testAddrs[i].RawAddress
		}
		crypto.SortCandidates(delegates, epochNum+1, crypto.CryptoSeed)
		proposerIdx := 0
		for i := 0; i < 4; i++ {
			if delegates[2] == testAddrs[i].RawAddress {
				proposerIdx = i
				break
			}
		}
		nonproposerIdx := 3
		if proposerIdx > 0 {
			nonproposerIdx = proposerIdx - 1
		}
		cfsm := newTestCFSM(t, testAddrs[nonproposerIdx], testAddrs[proposerIdx], ctrl, delegates, nil, nil, clock.New())
		cfsm.ctx.epoch = epochCtx{
			delegates:    delegates,
			num:          epochNum,
			height:       uint64(1),
			numSubEpochs: uint(1),
		}
		cfsm.ctx.cfg.EnableDKG = false
		require := require.New(t)
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		require.NoError(err)
		require.Equal(sAcceptPropose, s)
		require.Equal(uint64(0), cfsm.ctx.epoch.subEpochNum)
		require.Equal(cfsm.ctx.round.proposer, testAddrs[proposerIdx].RawAddress)
		require.NotNil(cfsm.ctx.round.endorsementSets)
		evt := <-cfsm.evtq
		require.Equal(eFailedToReceiveBlock, evt.Type())
		s, err = cfsm.handleProposeBlockTimeout(evt)
		require.Equal(sAcceptProposalEndorse, s)
		require.NoError(err)
		evt = <-cfsm.evtq
		require.Equal(eNotEnoughProposalEndorsement, evt.Type())
		s, err = cfsm.handleEndorseProposalTimeout(evt)
		require.NoError(err)
		require.Equal(sAcceptLockEndorse, s)
		evt = <-cfsm.evtq
		require.Equal(eNotEnoughLockEndorsement, evt.Type())
		s, err = cfsm.handleEndorseLockTimeout(evt)
		require.NoError(err)
		require.Equal(sPrepare, s)
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
	epochNum := uint64(1)
	crypto.SortCandidates(delegates, epochNum, crypto.CryptoSeed)
	delegateAddrs := make([]*iotxaddress.Address, 21)
	for i := 0; i < 21; i++ {
		for j := 0; j < 21; j++ {
			if delegates[i] == test21Addrs[j].RawAddress {
				delegateAddrs[i] = test21Addrs[j]
				break
			}
		}
	}
	/*
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
				endorsementSets: make(map[string]*endorsement.Set),
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
			blk := &blockchain.Block{}
			require.NoError(t, blk.Deserialize(pbe.block.Data()))
			require.Equal(t, len(delegates), len(blk.SecretProposals))
			require.NotNil(t, blk.SecretWitness)
		})
	*/
	t.Run("normal-block", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			delegateAddrs[2],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false
		cfsm.ctx.epoch.numSubEpochs = uint(2)
		cfsm.ctx.round = roundCtx{
			endorsementSets: make(map[string]*endorsement.Set),
			proposer:        delegates[2],
		}
		cfsm.ctx.epoch.subEpochNum = uint64(1)
		s, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		require.NoError(t, err)
		require.Equal(t, sAcceptPropose, s)
		e := <-cfsm.evtq
		pEvt, ok := e.(*proposeBlkEvt)
		require.True(t, ok)
		blk := pEvt.block.(*blockWrapper)
		transfers, votes, _ := action.ClassifyActions(blk.Actions)
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
	epochNum := uint64(1)
	crypto.SortCandidates(delegates, epochNum, crypto.CryptoSeed)
	delegateAddrs := make([]*iotxaddress.Address, 4)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if delegates[i] == testAddrs[j].RawAddress {
				delegateAddrs[i] = testAddrs[j]
				break
			}
		}
	}
	epoch := epochCtx{
		delegates:    delegates,
		num:          epochNum,
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		height:          2,
		number:          0,
		endorsementSets: make(map[string]*endorsement.Set),
		proposer:        delegates[2],
	}

	t.Run("pass-validation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		assert.NotNil(t, blk)
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))

		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eReceiveProposalEndorsement, evt.Type())
		assert.Equal(t, 1, broadcastCount)
	})

	t.Run("pass-validation-time-rotation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		clock := clock.NewMock()
		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false
		cfsm.ctx.cfg.ProposerInterval = 10 * time.Second
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round

		clock.Add(11 * time.Second)
		blk, err := cfsm.ctx.mintCommonBlock()
		assert.NoError(t, err)
		block := &blockWrapper{
			blk,
			cfsm.ctx.round.number,
		}
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(block, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eReceiveProposalEndorsement, evt.Type())

		clock.Add(10 * time.Second)
		state, err = cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
		time.Sleep(1 * time.Second)
		clock.Add(10 * time.Second)
		assert.Equal(t, delegates[0], cfsm.ctx.round.proposer)
	})

	t.Run("fail-validation", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.Error(t, err)
		assert.Equal(t, sAcceptPropose, state)
	})

	t.Run("skip-validation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			delegateAddrs[2],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		blk.(*blockWrapper).WorkingSet = mock_factory.NewMockWorkingSet(ctrl)
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eReceiveProposalEndorsement, evt.Type())
		assert.Equal(t, 1, broadcastCount)
	})

	t.Run("cannot-skip-validation", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			delegateAddrs[2],
			delegateAddrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().ValidateBlock(gomock.Any(), gomock.Any()).Times(1)
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

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		// The block's working set is nil though the blocker's producer is the current node
		blk.(*blockWrapper).WorkingSet = nil
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eReceiveProposalEndorsement, evt.Type())
		assert.Equal(t, 1, broadcastCount)
	})

	t.Run("invalid-proposer", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			delegateAddrs[2],
			delegateAddrs[3],
			ctrl,
			delegates,
			nil,
			nil,
			clock.New(),
		)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round
		cfsm.ctx.cfg.EnableDKG = false

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.Error(t, err)
		assert.Equal(t, sAcceptPropose, state)
		state, err = cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eFailedToReceiveBlock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
	})

	t.Run("invalid-proposer-time-rotation", func(t *testing.T) {
		clock := clock.NewMock()
		cfsm := newTestCFSM(
			t,
			delegateAddrs[2],
			delegateAddrs[3],
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
		cfsm.ctx.cfg.EnableDKG = false

		clock.Add(11 * time.Second)
		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
		assert.Error(t, err)
		assert.Equal(t, sAcceptPropose, state)
		state, err = cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eFailedToReceiveBlock))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)
	})

	t.Run("timeout", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			delegateAddrs[2],
			delegateAddrs[2],
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

		state, err := cfsm.handleProposeBlockTimeout(cfsm.newCEvt(eFailedToReceiveBlock))
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
	epochNum := uint64(1)
	crypto.SortCandidates(delegates, epochNum, crypto.CryptoSeed)
	delegateAddrs := make([]*iotxaddress.Address, 4)
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if delegates[i] == testAddrs[j].RawAddress {
				delegateAddrs[i] = testAddrs[j]
				break
			}
		}
	}
	epoch := epochCtx{
		delegates:    delegates,
		num:          epochNum,
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	round := roundCtx{
		endorsementSets: make(map[string]*endorsement.Set),
		proposer:        delegates[2],
	}

	t.Run("gather-endorses", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex

		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		assert.NotNil(t, blk)
		cfsm.ctx.round.block = blk
		blkHash := blk.Hash()

		// First endorse prepare
		eEvt := newEndorseEvt(endorsement.PROPOSAL, blkHash, round.height, round.number, delegateAddrs[0], cfsm.ctx.clock)
		state, err := cfsm.onReceiveProposalEndorsement(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)

		// Second endorse prepare
		eEvt = newEndorseEvt(endorsement.PROPOSAL, blkHash, round.height, round.number, delegateAddrs[1], cfsm.ctx.clock)
		state, err = cfsm.onReceiveProposalEndorsement(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptProposalEndorse, state)

		// Third endorse prepare, could move on
		eEvt = newEndorseEvt(endorsement.PROPOSAL, blkHash, round.height, round.number, delegateAddrs[2], cfsm.ctx.clock)
		state, err = cfsm.onReceiveProposalEndorsement(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptLockEndorse, state)
		e := <-cfsm.evtq
		evt, ok := e.(*endorseEvt)
		require.True(t, ok)
		assert.Equal(t, eReceiveLockEndorsement, evt.Type())
		assert.Equal(t, 1, broadcastCount)
	})

	t.Run("timeout", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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
		cfsm.ctx.cfg.EnableDKG = false

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseProposalTimeout(cfsm.newCEvt(eNotEnoughProposalEndorsement))
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
	epochNum := uint64(1)
	crypto.SortCandidates(delegates, epochNum, crypto.CryptoSeed)
	delegateAddrs := make([]*iotxaddress.Address, 21)
	for i := 0; i < 21; i++ {
		for j := 0; j < 21; j++ {
			if delegates[i] == test21Addrs[j].RawAddress {
				delegateAddrs[i] = test21Addrs[j]
				break
			}
		}
	}
	round := roundCtx{
		endorsementSets: make(map[string]*endorsement.Set),
		proposer:        delegates[2],
	}
	/*
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

			blk, err := cfsm.ctx.MintBlock()
			assert.NoError(t, err)
			cfsm.ctx.round.block = blk
			blkHash := blk.Hash()

			for i := 0; i < 14; i++ {
				eEvt := newEndorseEvt(endorsement.LOCK, blkHash, round.height, round.number, test21Addrs[i], cfsm.ctx.clock)
				state, err := cfsm.handleEndorseLockEvt(eEvt)
				assert.NoError(t, err)
				assert.Equal(t, sAcceptLockEndorse, state)
				assert.Equal(t, 0, len(cfsm.ctx.epoch.committedSecrets))
			}

			// 15th endorse prepare, could move on
			eEvt := newEndorseEvt(endorsement.LOCK, blkHash, round.height, round.number, test21Addrs[14], cfsm.ctx.clock)
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
	*/
	t.Run("gather-commits-common-block", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk
		blkHash := blk.Hash()

		for i := 0; i < 14; i++ {
			eEvt := newEndorseEvt(endorsement.LOCK, blkHash, round.height, round.number, delegateAddrs[i], cfsm.ctx.clock)
			state, err := cfsm.onReceiveLockEndorsement(eEvt)
			assert.NoError(t, err)
			assert.Equal(t, sAcceptLockEndorse, state)
		}

		// 15th endorse prepare, could move on
		eEvt := newEndorseEvt(endorsement.LOCK, blkHash, round.height, round.number, delegateAddrs[14], cfsm.ctx.clock)
		state, err := cfsm.onReceiveLockEndorsement(eEvt)
		assert.NoError(t, err)
		assert.Equal(t, sAcceptCommitEndorse, state)
		evt := <-cfsm.evtq
		assert.Equal(t, eReceiveCommitEndorsement, evt.Type())
		state, err = cfsm.handleEndorseCommitEvt(evt)
		assert.NoError(t, err)
		assert.Equal(t, sPrepare, state)
		assert.Equal(t, ePrepare, (<-cfsm.evtq).Type())
		assert.Equal(t, 2, broadcastCount)
	})
	t.Run("timeout-blocking", func(t *testing.T) {
		broadcastCount := 0
		var broadcastMutex sync.Mutex
		cfsm := newTestCFSM(
			t,
			delegateAddrs[0],
			delegateAddrs[2],
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

		blk, err := cfsm.ctx.MintBlock()
		assert.NoError(t, err)
		cfsm.ctx.round.block = blk

		state, err := cfsm.handleEndorseLockTimeout(cfsm.newCEvt(eNotEnoughLockEndorsement))
		assert.NoError(t, err)
		assert.Equal(t, sPrepare, state)
		assert.Equal(t, ePrepare, (<-cfsm.evtq).Type())
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
		endorsementSets: make(map[string]*endorsement.Set),
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
			chain.EXPECT().TipHeight().Return(uint64(2)).Times(4)
			candidates := make([]*state.Candidate, 0)
			for _, delegate := range delegates {
				candidates = append(candidates, &state.Candidate{Address: delegate})
			}
			chain.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(1)
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
	blk, err := cfsm.ctx.MintBlock()
	require.NoError(err)
	blk.(*blockWrapper).WorkingSet = mock_factory.NewMockWorkingSet(ctrl)
	state, err := cfsm.onReceiveBlock(newProposeBlkEvt(blk, nil, cfsm.ctx.round.number, cfsm.ctx.clock))
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	evt := <-cfsm.evtq
	eEvt, ok := evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveProposalEndorsement, eEvt.Type())
	// endorse proposal
	state, err = cfsm.onReceiveProposalEndorsement(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveLockEndorsement, eEvt.Type())
	// endorse lock
	state, err = cfsm.onReceiveLockEndorsement(eEvt)
	require.NoError(err)
	require.Equal(sAcceptCommitEndorse, state)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveCommitEndorsement, eEvt.Type())
	// endorse commit
	state, err = cfsm.handleEndorseCommitEvt(eEvt)
	require.NoError(err)
	require.Equal(sPrepare, state)
	evt = <-cfsm.evtq
	cEvt, ok := evt.(*consensusEvt)
	require.True(ok)
	require.Equal(ePrepare, cEvt.Type())
	// new round
	state, err = cfsm.prepare(cEvt)
	require.Equal(sAcceptPropose, state)
	require.NoError(err)
	assert.Equal(t, 5, broadcastCount)
}

func TestTwoDelegates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []string{testAddrs[0].RawAddress, testAddrs[1].RawAddress}
	epochNum := uint64(0)
	crypto.SortCandidates(delegates, epochNum+1, crypto.CryptoSeed)
	delegateAddrs := make([]*iotxaddress.Address, 2)
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			if delegates[i] == testAddrs[j].RawAddress {
				delegateAddrs[i] = testAddrs[j]
				break
			}
		}
	}
	epoch := epochCtx{
		delegates:    delegates,
		num:          epochNum,
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	broadcastCount := 0
	var broadcastMutex sync.Mutex
	cfsm := newTestCFSM(
		t,
		delegateAddrs[0],
		delegateAddrs[0],
		ctrl,
		delegates,
		func(chain *mock_blockchain.MockBlockchain) {
			chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
			chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			chain.EXPECT().TipHeight().Return(uint64(1)).Times(8)
			candidates := make([]*state.Candidate, 0)
			for _, delegate := range delegates {
				candidates = append(candidates, &state.Candidate{Address: delegate})
			}
			chain.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(1)
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
	//cfsm.ctx.round = round
	cfsm.ctx.cfg.EnableDKG = false

	state, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
	require.NoError(err)
	require.Equal(sAcceptPropose, state)
	evt := <-cfsm.evtq
	pEvt, ok := evt.(*proposeBlkEvt)
	require.True(ok)
	blk := pEvt.block
	blk.(*blockWrapper).WorkingSet = mock_factory.NewMockWorkingSet(ctrl)
	state, err = cfsm.onReceiveBlock(pEvt)
	require.NoError(err)
	require.Equal(sAcceptProposalEndorse, state)
	evt = <-cfsm.evtq
	eEvt, ok := evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveProposalEndorsement, eEvt.Type())
	// endorse proposal
	state, err = cfsm.onReceiveProposalEndorsement(eEvt)
	require.NoError(err)
	require.Equal(sAcceptProposalEndorse, state)
	eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.Hash(), cfsm.ctx.round.height, cfsm.ctx.round.number, delegateAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.onReceiveProposalEndorsement(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveLockEndorsement, eEvt.Type())
	// endorse lock
	state, err = cfsm.onReceiveLockEndorsement(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	eEvt = newEndorseEvt(endorsement.LOCK, blk.Hash(), cfsm.ctx.round.height, cfsm.ctx.round.number, delegateAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.onReceiveLockEndorsement(eEvt)
	require.Equal(sAcceptCommitEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveCommitEndorsement, eEvt.Type())
	// endorse lock
	state, err = cfsm.handleEndorseCommitEvt(eEvt)
	require.NoError(err)
	require.Equal(sPrepare, state)
	evt = <-cfsm.evtq
	cEvt, ok := evt.(*consensusEvt)
	require.True(ok)
	require.Equal(ePrepare, cEvt.Type())
	// new round
	state, err = cfsm.prepare(cEvt)
	require.NoError(err)
	require.Equal(sAcceptPropose, state)
	assert.Equal(t, 6, broadcastCount)
}

func TestThreeDelegates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	delegates := []string{testAddrs[0].RawAddress, testAddrs[1].RawAddress, testAddrs[2].RawAddress}
	epochNum := uint64(0)
	crypto.SortCandidates(delegates, epochNum+1, crypto.CryptoSeed)
	delegateAddrs := make([]*iotxaddress.Address, 3)
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if delegates[i] == testAddrs[j].RawAddress {
				delegateAddrs[i] = testAddrs[j]
				break
			}
		}
	}
	epoch := epochCtx{
		delegates:    delegates,
		num:          epochNum,
		height:       uint64(1),
		numSubEpochs: uint(1),
	}
	broadcastCount := 0
	var broadcastMutex sync.Mutex
	cfsm := newTestCFSM(
		t,
		delegateAddrs[2],
		delegateAddrs[2],
		ctrl,
		delegates,
		func(chain *mock_blockchain.MockBlockchain) {
			chain.EXPECT().CommitBlock(gomock.Any()).Return(nil).Times(1)
			chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
			chain.EXPECT().TipHeight().Return(uint64(1)).Times(8)
			candidates := make([]*state.Candidate, 0)
			for _, delegate := range delegates {
				candidates = append(candidates, &state.Candidate{Address: delegate})
			}
			chain.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(1)
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
	cfsm.ctx.cfg.EnableDKG = false

	state, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
	require.NoError(err)
	require.Equal(sAcceptPropose, state)
	evt := <-cfsm.evtq
	pEvt, ok := evt.(*proposeBlkEvt)
	require.True(ok)
	blk := pEvt.block
	blk.(*blockWrapper).WorkingSet = mock_factory.NewMockWorkingSet(ctrl)
	state, err = cfsm.onReceiveBlock(pEvt)
	require.NoError(err)
	require.Equal(sAcceptProposalEndorse, state)
	evt = <-cfsm.evtq
	eEvt, ok := evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveProposalEndorsement, eEvt.Type())
	// endorse proposal
	// handle self endorsement
	state, err = cfsm.onReceiveProposalEndorsement(eEvt)
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	// handle delegate 1's endorsement
	eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.Hash(), cfsm.ctx.round.height, cfsm.ctx.round.number, delegateAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.onReceiveProposalEndorsement(eEvt)
	require.Equal(sAcceptProposalEndorse, state)
	require.NoError(err)
	// handle delegate 2's endorsement
	eEvt = newEndorseEvt(endorsement.PROPOSAL, blk.Hash(), cfsm.ctx.round.height, cfsm.ctx.round.number, delegateAddrs[0], cfsm.ctx.clock)
	state, err = cfsm.onReceiveProposalEndorsement(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveLockEndorsement, eEvt.Type())
	// endorse lock
	// handle self endorsement
	state, err = cfsm.onReceiveLockEndorsement(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	// handle delegate 1's endorsement
	eEvt = newEndorseEvt(endorsement.LOCK, blk.Hash(), cfsm.ctx.round.height, cfsm.ctx.round.number, delegateAddrs[1], cfsm.ctx.clock)
	state, err = cfsm.onReceiveLockEndorsement(eEvt)
	require.Equal(sAcceptLockEndorse, state)
	require.NoError(err)
	// handle delegate 2's endorsement
	eEvt = newEndorseEvt(endorsement.LOCK, blk.Hash(), cfsm.ctx.round.height, cfsm.ctx.round.number, delegateAddrs[0], cfsm.ctx.clock)
	state, err = cfsm.onReceiveLockEndorsement(eEvt)
	require.NoError(err)
	require.Equal(sAcceptCommitEndorse, state)
	evt = <-cfsm.evtq
	eEvt, ok = evt.(*endorseEvt)
	require.True(ok)
	require.Equal(eReceiveCommitEndorsement, eEvt.Type())
	state, err = cfsm.handleEndorseCommitEvt(eEvt)
	require.NoError(err)
	require.Equal(sPrepare, state)
	evt = <-cfsm.evtq
	cEvt, ok := evt.(*consensusEvt)
	require.True(ok)
	require.Equal(ePrepare, cEvt.Type())
	// new round
	state, err = cfsm.prepare(cEvt)
	require.Equal(sAcceptPropose, state)
	require.NoError(err)
	assert.Equal(t, 6, broadcastCount)
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
		endorsementSets: make(map[string]*endorsement.Set),
		proposer:        delegates[2],
	}
	/*
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
	*/
	t.Run("epoch-not-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(22)).Times(4)
				candidates := make([]*state.Candidate, 0)
				for _, delegate := range delegates {
					candidates = append(candidates, &state.Candidate{Address: delegate})
				}
				chain.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).AnyTimes()
			},
			nil,
			clock.New(),
		)
		epoch.subEpochNum = uint64(1)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round
		cfsm.ctx.cfg.EnableDKG = false

		state, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
		assert.Equal(t, eFailedToReceiveBlock, (<-cfsm.evtq).Type())
	})
	t.Run("epoch-finished", func(t *testing.T) {
		cfsm := newTestCFSM(
			t,
			test21Addrs[0],
			test21Addrs[2],
			ctrl,
			delegates,
			func(chain *mock_blockchain.MockBlockchain) {
				chain.EXPECT().TipHeight().Return(uint64(42)).Times(4)
				chain.EXPECT().ChainID().AnyTimes().Return(config.Default.Chain.ID)
				chain.EXPECT().ChainAddress().AnyTimes().Return(config.Default.Chain.Address)
				candidates := make([]*state.Candidate, 0)
				for _, delegate := range delegates {
					candidates = append(candidates, &state.Candidate{Address: delegate})
				}
				chain.EXPECT().CandidatesByHeight(gomock.Any()).Return(candidates, nil).Times(1)
			},
			nil,
			clock.New(),
		)
		epoch.subEpochNum = uint64(1)
		cfsm.ctx.epoch = epoch
		cfsm.ctx.round = round
		cfsm.ctx.cfg.EnableDKG = false

		state, err := cfsm.prepare(cfsm.newCEvt(ePrepare))
		assert.NoError(t, err)
		assert.Equal(t, sAcceptPropose, state)
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
	a := testaddress.IotxAddrinfo["alfa"]
	b := testaddress.IotxAddrinfo["bravo"]
	transfer, err := testutil.SignedTransfer(a, b, 1, big.NewInt(100), []byte{}, 100000, big.NewInt(10))
	require.NoError(t, err)
	vote, err := testutil.SignedVote(a, a, 2, 100000, big.NewInt(10))
	require.NoError(t, err)
	var prevHash hash.Hash32B
	lastBlk := block.NewBlockDeprecated(
		config.Default.Chain.ID,
		1,
		prevHash,
		testutil.TimestampNowFromClock(clock),
		proposer.PublicKey,
		make([]action.SealedEnvelope, 0),
	)
	blkToMint, err := block.NewTestingBuilder().
		SetChainID(config.Default.Chain.ID).
		SetHeight(2).
		SetPrevBlockHash(lastBlk.HashBlock()).
		SetTimeStamp(testutil.TimestampNowFromClock(clock)).
		AddActions(transfer, vote).
		SignAndBuild(proposer)
	require.NoError(t, err)

	var secretBlkToMint block.Block
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

		secretBlkToMint, err = block.NewTestingBuilder().
			SetChainID(config.Default.Chain.ID).
			SetHeight(2).
			SetPrevBlockHash(lastBlk.HashBlock()).
			SetTimeStamp(testutil.TimestampNowFromClock(clock)).
			SetSecretProposals(secretProposals).
			SetSecretWitness(secretWitness).
			SignAndBuild(proposer)
		require.NoError(t, err)
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
			blockchain.EXPECT().GetBlockByHeight(uint64(42)).Return(lastBlk, nil).AnyTimes()
			blockchain.EXPECT().
				MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&blkToMint, nil).AnyTimes()
			blockchain.EXPECT().
				MintNewSecretBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(&secretBlkToMint, nil).AnyTimes()
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
