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
		mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
	}
	cs := createTestRollDPoS(ctrl, delegates[0], delegates, m, false, FixedProposer, nil)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock(Gen)
	blkHash := genesis.HashBlock()

	// Accept ROUND_START and then propose
	event := &fsm.Event{
		State: stateRoundStart,
	}
	err := cs.fsm.HandleTransition(event)
	assert.Error(t, err, "accept %s error", stateRoundStart)
	assert.Equal(t, stateRoundStart, cs.fsm.CurrentState(), "current state %s", stateRoundStart)

	// Accept PROPOSE and then prevote
	event = &fsm.Event{
		State:      stateAcceptPropose,
		SenderAddr: delegates[1],
		Block:      genesis,
	}
	err = cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept %s no error", stateAcceptPropose)
	assert.Equal(t, stateAcceptPrevote, cs.fsm.CurrentState(), "current state %s", stateAcceptPrevote)
	assert.Equal(t, genesis, cs.roundCtx.block, "roundCtx.block set")
	assert.Equal(t, &blkHash, cs.roundCtx.blockHash, "roundCtx.blockHash set")

	// Accept PREVOTE and then vote
	event = &fsm.Event{
		State:      stateAcceptPrevote,
		SenderAddr: delegates[1],
		BlockHash:  &blkHash,
	}
	err = cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept %s no error", stateAcceptPrevote)
	assert.Equal(t, stateAcceptVote, cs.fsm.CurrentState(), "current state %s", stateAcceptVote)
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
	assert.NoError(t, err, "accept %s no error", stateAcceptVote)
	assert.Equal(t, stateRoundStart, cs.fsm.CurrentState(), "current state %s", stateRoundStart)
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
		mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).AnyTimes()
	}
	cs := createTestRollDPoS(ctrl, delegates[0], delegates, m, false, FixedProposer, nil)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock(Gen)

	// Accept ROUND_START and then propose
	event := &fsm.Event{
		State: stateRoundStart,
	}
	err := cs.fsm.HandleTransition(event)
	assert.Error(t, err, "accept %s error", stateRoundStart)
	assert.Equal(t, stateRoundStart, cs.fsm.CurrentState(), "current state %s", stateRoundStart)

	// Accept PROPOSE and then prevote
	event = &fsm.Event{
		State:      stateAcceptPropose,
		SenderAddr: delegates[1],
		Block:      genesis,
	}
	err = cs.fsm.HandleTransition(event)
	assert.NoError(t, err, "accept %s no error", stateAcceptPropose)
	assert.Equal(t, stateAcceptPrevote, cs.fsm.CurrentState(), "current state %s", stateAcceptPrevote)
	assert.Nil(t, cs.roundCtx.block, "roundCtx.block nil")
	assert.Nil(t, cs.roundCtx.blockHash, "roundCtx.blockHash nil")

	time.Sleep(2 * time.Second)
	assert.Equal(
		t,
		fsm.State(stateRoundStart),
		cs.fsm.CurrentState(),
		"current state timeout back to %s",
		stateRoundStart)
}
