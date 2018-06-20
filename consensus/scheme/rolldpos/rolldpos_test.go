// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	"github.com/iotexproject/iotex-core/test/mock/mock_rolldpos"
	"github.com/iotexproject/iotex-core/test/mock/mock_state"
)

type mocks struct {
	dNet *mock_rolldpos.MockDNet
	bc   *mock_blockchain.MockBlockchain
	dp   *mock_delegate.MockPool
	sf   *mock_state.MockFactory
}

type mockFn func(mcks mocks)

var testDKG = common.DKGHash{4, 6, 8, 9, 4, 6, 8, 9, 4, 6, 8, 9, 4, 6, 8, 9, 4, 6, 8, 9}

func createTestRollDPoS(
	ctrl *gomock.Controller,
	self net.Addr,
	delegates []net.Addr,
	mockFn mockFn,
	prCb scheme.GetProposerCB,
	prDelay time.Duration,
	epochCb scheme.StartNextEpochCB,
	bcCnt *int) *RollDPoS {
	bc := mock_blockchain.NewMockBlockchain(ctrl)

	createblockCB := func() (*blockchain.Block, error) {
		blk, err := bc.MintNewBlock(nil, nil, &iotxaddress.Address{}, "")
		if err != nil {
			logger.Error().Msg("Failed to mint a new block")
			return nil, err
		}
		logger.Info().
			Uint64("height", blk.Height()).
			Int("transfers", len(blk.Transfers)).
			Msg("Created a new block")
		return blk, nil
	}
	commitBlockCB := func(blk *blockchain.Block) error {
		return bc.CommitBlock(blk)
	}
	broadcastBlockCB := func(blk *blockchain.Block) error {
		if bcCnt != nil {
			*bcCnt++
		}
		return nil
	}
	generateDKGCB := func() (common.DKGHash, error) {
		return testDKG, nil
	}
	dp := mock_delegate.NewMockPool(ctrl)
	dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
	dp.EXPECT().RollDelegates(gomock.Any()).Return(delegates, nil).AnyTimes()
	dp.EXPECT().NumDelegatesPerEpoch().Return(uint(len(delegates)), nil).AnyTimes()
	dNet := mock_rolldpos.NewMockDNet(ctrl)
	dNet.EXPECT().Self().Return(self)
	tellblockCB := func(msg proto.Message) error {
		return dNet.Broadcast(msg)
	}
	csCfg := config.RollDPoS{
		UnmatchedEventTTL: 300 * time.Millisecond,
		AcceptPropose: config.AcceptPropose{
			TTL: 300 * time.Millisecond,
		},
		AcceptPrevote: config.AcceptPrevote{
			TTL: 300 * time.Millisecond,
		},
		AcceptVote: config.AcceptVote{
			TTL: 300 * time.Millisecond,
		},
		Delay:            10 * time.Second,
		EventChanSize:    100,
		DelegateInterval: time.Hour,
		NumSubEpochs:     1,
	}
	csCfg.ProposerInterval = prDelay
	sf := mock_state.NewMockFactory(ctrl)
	mockFn(mocks{
		dNet: dNet,
		bc:   bc,
		dp:   dp,
		sf:   sf,
	})
	return NewRollDPoS(
		csCfg,
		createblockCB,
		tellblockCB,
		commitBlockCB,
		broadcastBlockCB,
		prCb,
		epochCb,
		generateDKGCB,
		bc,
		dNet.Self(),
		dp,
		sf,
	)
}

type testCs struct {
	cs    *RollDPoS
	mocks mocks
}

// 1 faulty node and 3 trusty nodes
func TestByzantineFault(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc         string
		proposerNode int
	}{
		{
			desc:         "faulty node(0) is a proposer",
			proposerNode: 0,
		},
		{
			desc:         "faulty node(0) is a validator",
			proposerNode: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			testByzantineFault(t, tt.proposerNode)
		})
	}
}

// 1 faulty node and 3 trusty nodes
func testByzantineFault(t *testing.T, proposerNode int) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange proposal request
	genesis := blockchain.NewGenesisBlock()
	blkHash := genesis.HashBlock()

	t.Log(genesis)

	// arrange 4 consensus nodes
	tcss := make(map[net.Addr]testCs)
	delegates := []net.Addr{
		common.NewTCPNode("192.168.0.0:10000"),
		common.NewTCPNode("192.168.0.1:10001"),
		common.NewTCPNode("192.168.0.2:10002"),
		common.NewTCPNode("192.168.0.3:10003"),
	}

	bcCnt := 0
	for _, d := range delegates {
		cur := d // watch out for the callback

		tcs := testCs{}
		m := func(mcks mocks) {
			mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
			mcks.dNet.EXPECT().Self().Return(cur).AnyTimes()
			mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis, nil).AnyTimes()
			mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
			mcks.bc.EXPECT().ValidateBlock(gomock.Any()).Do(func(blk *blockchain.Block) error {
				if blk == nil {
					return errors.New("invalid block")
				}
				return nil
			}).AnyTimes()

			// =====================
			// expect CommitBlock
			// =====================
			mcks.bc.EXPECT().CommitBlock(gomock.Any()).AnyTimes().Do(func(blk *blockchain.Block) error {
				if proposerNode == 0 {
					// commit nil when proposer is faulty
					assert.Nil(t, blk, "final block committed")
				} else {
					// commit block when proposer is trusty
					assert.Equal(t, *genesis, *blk, "final block committed")
				}
				t.Log(cur, "CommitBlock: ", blk)
				return nil
			})
			tcs.mocks = mcks
		}
		tcs.cs = createTestRollDPoS(
			ctrl, cur, delegates, m, FixedProposer, time.Hour, NeverStartNewEpoch, &bcCnt)
		tcs.cs.Start()
		defer tcs.cs.Stop()
		tcss[cur] = tcs
	}

	// arrange network call
	for i, d := range delegates {
		sender := d // watch out for the callback
		if i == 0 {
			// faulty node
			tcss[sender].mocks.dNet.EXPECT().Broadcast(gomock.Any()).Do(func(msg proto.Message) error {
				for ti, target := range delegates {
					event, _ := eventFromProto(msg)
					// I (node 0) send authentic msg to 1 but send reversed msg to 2 and 3
					if ti == 2 || ti == 3 {
						if event.Block != nil {
							event.Block = nil
						} else {
							event.Block = genesis
						}
						if event.BlockHash != nil {
							event.BlockHash = nil
						} else {
							event.BlockHash = &blkHash
						}
					}
					faultyMsg := protoFromEvent(event)
					tcss[target].cs.Handle(faultyMsg)
				}
				return nil
			}).AnyTimes()
		} else {
			// trusty nodes
			tcss[sender].mocks.dNet.EXPECT().Broadcast(gomock.Any()).Do(func(msg proto.Message) error {
				for _, target := range delegates {
					tcss[target].cs.Handle(msg)
				}
				return nil
			}).AnyTimes()
		}
	}

	for _, d := range delegates {
		tcss[d].cs.fsm.HandleTransition(&fsm.Event{
			State: stateDKGGenerate,
		})
	}
	time.Sleep(time.Second)

	// act
	// trigger proposerNode as proposer
	tcss[delegates[proposerNode]].cs.fsm.HandleTransition(&fsm.Event{
		State: stateInitPropose,
	})

	// assert
	time.Sleep(time.Second)
	for _, tcs := range tcss {
		assert.Equal(
			t,
			stateRoundStart,
			tcs.cs.fsm.CurrentState(),
			"back to %s in the end",
			stateRoundStart)
		assert.Equal(t, testDKG, tcs.cs.epochCtx.dkg)
	}
	voteStats(t, tcss)
	if proposerNode == 0 {
		assert.Equal(t, 0, bcCnt)
	} else {
		assert.Equal(t, 4, bcCnt)
	}
}

func voteStats(t *testing.T, tcss map[net.Addr]testCs) {
	// prevote
	for _, tcs := range tcss {
		for i, v := range tcs.cs.roundCtx.prevotes {
			t.Log(tcs.cs.self, "collect prevotes [", i, "]", v != nil)
		}
	}

	// votes
	for _, tcs := range tcss {
		for i, v := range tcs.cs.roundCtx.votes {
			t.Log(tcs.cs.self, "collect votes [", i, "]", v != nil)
		}
	}
}

func TestRollDPoSFourTrustyNodes(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange proposal request
	genesis := blockchain.NewGenesisBlock()

	// arrange 4 consensus nodes
	tcss := make(map[net.Addr]testCs)
	delegates := []net.Addr{
		common.NewTCPNode("192.168.0.0:10000"),
		common.NewTCPNode("192.168.0.1:10001"),
		common.NewTCPNode("192.168.0.2:10002"),
		common.NewTCPNode("192.168.0.3:10003"),
	}

	bcCnt := 0
	for _, d := range delegates {
		cur := d // watch out for the callback

		tcs := testCs{}
		m := func(mcks mocks) {
			mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
			mcks.dNet.EXPECT().Self().Return(cur).AnyTimes()
			mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis, nil).AnyTimes()
			mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
			mcks.bc.EXPECT().ValidateBlock(gomock.Any()).AnyTimes()

			// =====================
			// expect CommitBlock
			// =====================
			mcks.bc.EXPECT().CommitBlock(gomock.Any()).AnyTimes().Do(func(blk *blockchain.Block) error {
				t.Log(cur, "CommitBlock: ", blk)
				assert.Equal(t, *genesis, *blk, "final block committed")
				return nil
			})
			tcs.mocks = mcks
		}
		tcs.cs = createTestRollDPoS(ctrl, cur, delegates, m, FixedProposer, time.Hour, NeverStartNewEpoch, &bcCnt)
		tcs.cs.Start()
		defer tcs.cs.Stop()
		tcss[cur] = tcs
	}

	// arrange network call
	for _, d := range delegates {
		sender := d // watch out for the callback
		// trusty nodes
		tcss[sender].mocks.dNet.EXPECT().Broadcast(gomock.Any()).Do(func(msg proto.Message) error {
			for _, target := range delegates {
				tcss[target].cs.Handle(msg)
			}
			return nil
		}).AnyTimes()
	}

	for _, d := range delegates {
		tcss[d].cs.fsm.HandleTransition(&fsm.Event{
			State: stateDKGGenerate,
		})
	}
	time.Sleep(time.Second)

	// act
	// trigger 2 as proposer
	tcss[delegates[2]].cs.fsm.HandleTransition(&fsm.Event{
		State: stateInitPropose,
	})

	// assert
	time.Sleep(time.Second)
	for _, tcs := range tcss {
		assert.Equal(
			t,
			stateRoundStart,
			tcs.cs.fsm.CurrentState(),
			"back to %s in the end", stateRoundStart)
		assert.Equal(t, testDKG, tcs.cs.epochCtx.dkg)

		metrics, err := tcs.cs.Metrics()
		require.Nil(t, err)
		require.NotNil(t, metrics)
		require.Equal(t, uint64(1), metrics.LatestEpoch)
		require.Equal(t, delegates, metrics.LatestDelegates)
		require.Equal(t, delegates[0], metrics.LatestBlockProducer)
	}
	assert.Equal(t, 4, bcCnt)
}

// Delegate0 receives PROPOSE from Delegate1 and hence move to PREVOTE state and timeout to other states and finally to roundStart
func TestRollDPoSConsumePROPOSE(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{
		common.NewTCPNode("192.168.0.1:10001"),
		common.NewTCPNode("192.168.0.2:10002"),
	}
	m := func(mcks mocks) {
		mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
		mcks.dNet.EXPECT().Broadcast(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().ValidateBlock(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().CommitBlock(gomock.Any()).Times(0)
		mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()

	}
	cs := createTestRollDPoS(
		ctrl, delegates[0], delegates, m, FixedProposer, time.Hour, NeverStartNewEpoch, nil)
	cs.Start()
	defer cs.Stop()
	cs.fsm.HandleTransition(&fsm.Event{
		State: stateDKGGenerate,
	})
	time.Sleep(time.Second)

	// arrange proposal request
	genesis := blockchain.NewGenesisBlock()
	proposal := &iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_PROPOSE,
		Block:      genesis.ConvertToBlockPb(),
		SenderAddr: delegates[1].String(),
	}

	// act
	cs.Handle(proposal)

	// assert
	time.Sleep(time.Second)
	assert.Equal(
		t,
		stateRoundStart,
		cs.fsm.CurrentState(),
		"Back to %s because of timeout",
		stateRoundStart)
	assert.Equal(t, genesis, cs.roundCtx.block)
}

// Delegate0 receives unmatched VOTE from Delegate1 and stays in START
func TestRollDPoSConsumeErrorStateHandlerNotMatched(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{
		common.NewTCPNode("192.168.0.1:10001"),
		common.NewTCPNode("192.168.0.2:10002"),
	}
	m := func(mcks mocks) {
		mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
	}
	cs := createTestRollDPoS(
		ctrl, delegates[0], delegates, m, FixedProposer, time.Hour, NeverStartNewEpoch, nil)

	cs.Start()
	defer cs.Stop()
	cs.fsm.HandleTransition(&fsm.Event{
		State: stateDKGGenerate,
	})
	time.Sleep(time.Second)

	// arrange unmatched VOTE proposal request
	proposal := &iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_VOTE,
		Block:      nil,
		SenderAddr: delegates[1].String(),
	}

	// act
	cs.Handle(proposal)

	// assert
	time.Sleep(time.Second)
	assert.Equal(t, stateRoundStart, cs.fsm.CurrentState())
}
