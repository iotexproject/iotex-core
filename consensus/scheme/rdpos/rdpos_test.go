// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rdpos

import (
	"net"
	"testing"
	"time"

	"github.com/golang/glog"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/blockchain"
	. "github.com/iotexproject/iotex-core/blockchain"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/test/mock/mock_delegate"
	. "github.com/iotexproject/iotex-core/test/mock/mock_rdpos"
	"github.com/iotexproject/iotex-core/txpool"
)

type mocks struct {
	dNet *MockDNet
	bc   *mock_blockchain.MockBlockchain
	dp   *mock_delegate.MockPool
}

type mockFn func(mcks mocks)

func createTestRDPoS(ctrl *gomock.Controller, self net.Addr, delegates []net.Addr, mockFn mockFn, enableProposerRotation bool) *RDPoS {
	bc := mock_blockchain.NewMockBlockchain(ctrl)

	tp := txpool.New(bc)
	createblockCB := func() (*blockchain.Block, error) {
		blk, err := bc.MintNewBlock(tp.PickTxs(), iotxaddress.Address{}, "")
		if err != nil {
			glog.Errorf("Failed to mint a new block")
			return nil, err
		}
		glog.Infof("created a new block at height %v with %v txs", blk.Height(), len(blk.Tranxs))
		return blk, nil
	}
	commitBlockCB := func(blk *blockchain.Block) error {
		return bc.AddBlockCommit(blk)
	}
	broadcastBlockCB := func(blk *blockchain.Block) error {
		return nil
	}
	dp := mock_delegate.NewMockPool(ctrl)
	dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
	dNet := NewMockDNet(ctrl)
	dNet.EXPECT().Self().Return(self)
	tellblockCB := func(msg proto.Message) error {
		return dNet.Broadcast(msg)
	}
	csCfg := config.RDPoS{
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
	}
	if enableProposerRotation {
		csCfg.ProposerRotation.Enabled = enableProposerRotation
		csCfg.ProposerRotation.Interval = time.Second
	}
	mockFn(mocks{
		dNet: dNet,
		bc:   bc,
		dp:   dp,
	})
	return NewRDPoS(csCfg, createblockCB, tellblockCB, commitBlockCB, broadcastBlockCB, bc, dNet.Self(), dp)
}

type testCs struct {
	cs    *RDPoS
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
	genesis := NewGenesisBlock(Gen)
	blkHash := genesis.HashBlock()

	// arrange 4 consensus nodes
	tcss := make(map[net.Addr]testCs)
	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.0:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
		cm.NewTCPNode("192.168.0.2:10002"),
		cm.NewTCPNode("192.168.0.3:10003"),
	}
	for _, d := range delegates {
		cur := d // watch out for the callback

		tcs := testCs{}
		m := func(mcks mocks) {
			mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
			mcks.dNet.EXPECT().Self().Return(cur).AnyTimes()
			mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis, nil).AnyTimes()
			mcks.bc.EXPECT().ValidateBlock(gomock.Any()).Do(func(blk *Block) error {
				if blk == nil {
					return errors.New("invalid block")
				}
				return nil
			}).AnyTimes()

			// =====================
			// expect AddBlockCommit
			// =====================
			mcks.bc.EXPECT().AddBlockCommit(gomock.Any()).AnyTimes().Do(func(blk *Block) error {
				if proposerNode == 0 {
					// commit nil when proposer is faulty
					assert.Nil(t, blk, "final block committed")
				} else {
					// commit block when proposer is trusty
					assert.Equal(t, *genesis, *blk, "final block committed")
				}
				t.Log(cur, "AddBlockCommit: ", blk)
				return nil
			})
			tcs.mocks = mcks
		}
		tcs.cs = createTestRDPoS(ctrl, cur, delegates, m, false)
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

	// act
	// trigger proposerNode as proposer
	tcss[delegates[proposerNode]].cs.fsm.HandleTransition(&fsm.Event{
		State: stateInitPropose,
	})

	// assert
	time.Sleep(time.Second)
	for _, tcs := range tcss {
		assert.Equal(t, fsm.State("START"), tcs.cs.fsm.CurrentState(), "back to START in the end")
	}
	voteStats(t, tcss)
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

func TestRDPoSFourTrustyNodes(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange proposal request
	genesis := NewGenesisBlock(Gen)

	// arrange 4 consensus nodes
	tcss := make(map[net.Addr]testCs)
	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.0:10000"),
		cm.NewTCPNode("192.168.0.1:10001"),
		cm.NewTCPNode("192.168.0.2:10002"),
		cm.NewTCPNode("192.168.0.3:10003"),
	}
	for _, d := range delegates {
		cur := d // watch out for the callback

		tcs := testCs{}
		m := func(mcks mocks) {
			mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
			mcks.dNet.EXPECT().Self().Return(cur).AnyTimes()
			mcks.bc.EXPECT().MintNewBlock(gomock.Any(), gomock.Any(), gomock.Any()).Return(genesis, nil).AnyTimes()
			mcks.bc.EXPECT().ValidateBlock(gomock.Any()).AnyTimes()

			// =====================
			// expect AddBlockCommit
			// =====================
			mcks.bc.EXPECT().AddBlockCommit(gomock.Any()).AnyTimes().Do(func(blk *Block) error {
				t.Log(cur, "AddBlockCommit: ", blk)
				assert.Equal(t, *genesis, *blk, "final block committed")
				return nil
			})
			tcs.mocks = mcks
		}
		tcs.cs = createTestRDPoS(ctrl, cur, delegates, m, false)
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

	// act
	// trigger 2 as proposer
	tcss[delegates[2]].cs.fsm.HandleTransition(&fsm.Event{
		State: stateInitPropose,
	})

	// assert
	time.Sleep(time.Second)
	for _, tcs := range tcss {
		assert.Equal(t, fsm.State("START"), tcs.cs.fsm.CurrentState(), "back to START in the end")
	}
}

// Delegate0 receives PROPOSE from Delegate1 and hence move to PREVOTE state and timeout to other states and finally to start
func TestRDPoSConsumePROPOSE(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10001"),
		cm.NewTCPNode("192.168.0.2:10002"),
	}
	m := func(mcks mocks) {
		mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
		mcks.dNet.EXPECT().Broadcast(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().ValidateBlock(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().AddBlockCommit(gomock.Any()).Times(0)
	}
	cs := createTestRDPoS(ctrl, delegates[0], delegates, m, false)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock(Gen)
	proposal := &iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_PROPOSE,
		Block:      genesis.ConvertToBlockPb(),
		SenderAddr: delegates[1].String(),
	}

	// act
	cs.Handle(proposal)

	// assert
	time.Sleep(time.Second)
	assert.Equal(t, fsm.State("START"), cs.fsm.CurrentState(), "Back to start because of timeout")
	assert.Equal(t, genesis, cs.roundCtx.block)
}

// Delegate0 receives unmatched VOTE from Delegate1 and stays in START
func TestRDPoSConsumeErrorStateHandlerNotMatched(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{
		cm.NewTCPNode("192.168.0.1:10001"),
		cm.NewTCPNode("192.168.0.2:10002"),
	}
	m := func(mcks mocks) {}
	cs := createTestRDPoS(ctrl, delegates[0], delegates, m, false)

	cs.Start()
	defer cs.Stop()

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
	assert.Equal(t, fsm.State("START"), cs.fsm.CurrentState())
}
