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

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	. "github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/consensus/fsm"
)

func TestAcceptPrevoteAndProceedToEnd(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// arrange 2 consensus nodes
	delegates := []net.Addr{common.NewTCPNode("192.168.0.1:10001"), common.NewTCPNode("192.168.0.2:10002")}
	m := func(mcks mocks) {
		mcks.dp.EXPECT().AllDelegates().Return(delegates, nil).AnyTimes()
		mcks.dNet.EXPECT().Broadcast(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().ValidateBlock(gomock.Any()).AnyTimes()
		mcks.bc.EXPECT().AddBlockCommit(gomock.Any()).Times(1)
	}
	cs := createTestRDPoS(ctrl, delegates[0], delegates, m, false)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock(Gen)
	blkHash := genesis.HashBlock()

	// Accept PROPOSE and then prevote
	event := &fsm.Event{
		State:      stateAcceptPropose,
		SenderAddr: delegates[1],
		Block:      genesis,
	}
	err := cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept PROPOSE no error")
	assert.Equal(t, fsm.State("PREVOTE"), cs.fsm.CurrentState(), "current state PREVOTE")
	assert.Equal(t, genesis, cs.roundCtx.block, "roundCtx.block set")
	assert.Equal(t, &blkHash, cs.roundCtx.blockHash, "roundCtx.blockHash set")

	// Accept PREVOTE and then vote
	event = &fsm.Event{
		State:      stateAcceptPrevote,
		SenderAddr: delegates[1],
		BlockHash:  &blkHash,
	}
	err = cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept PREVOTE no error")
	assert.Equal(t, fsm.State("VOTE"), cs.fsm.CurrentState(), "current state VOTE")
	assert.Equal(
		t,
		map[net.Addr]*common.Hash32B{
			delegates[0]: &blkHash,
			delegates[1]: &blkHash,
		},
		cs.roundCtx.prevotes,
		"roundCtx.prevote set",
	)

	// Accept VOTE and then commit
	event = &fsm.Event{
		State:      stateAcceptVote,
		SenderAddr: delegates[1],
		BlockHash:  &blkHash,
	}
	err = cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept VOTE no error")
	assert.Equal(t, fsm.State("START"), cs.fsm.CurrentState(), "current state START")
	assert.Equal(
		t,
		map[net.Addr]*common.Hash32B{
			delegates[0]: &blkHash,
			delegates[1]: &blkHash,
		},
		cs.roundCtx.votes,
		"roundCtx.votes set",
	)
}

func TestAcceptPrevoteAndTimeoutToEnd(t *testing.T) {
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
		mcks.bc.EXPECT().ValidateBlock(gomock.Any()).Return(errors.New("error"))
		mcks.bc.EXPECT().AddBlockCommit(gomock.Any()).Times(0)
	}
	cs := createTestRDPoS(ctrl, delegates[0], delegates, m, false)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock(Gen)

	// Accept PROPOSE and then prevote
	event := &fsm.Event{
		State:      stateAcceptPropose,
		SenderAddr: delegates[1],
		Block:      genesis,
	}
	err := cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept PROPOSE no error")
	assert.Equal(t, fsm.State("PREVOTE"), cs.fsm.CurrentState(), "current state PREVOTE")
	assert.Nil(t, cs.roundCtx.block, "roundCtx.block nil")
	assert.Nil(t, cs.roundCtx.blockHash, "roundCtx.blockHash nil")

	time.Sleep(2 * time.Second)
	assert.Equal(t, fsm.State("START"), cs.fsm.CurrentState(), "current state timeout back to START")
}
