// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"fmt"
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
		first := mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).Times(1)
		mcks.bc.EXPECT().TipHeight().Return(uint64(2), nil).After(first).AnyTimes()
	}
	cs := createTestRollDPoS(
		ctrl, delegates[0], delegates, m, FixedProposer, time.Hour, NeverStartNewEpoch, nil)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock()
	blkHash := genesis.HashBlock()

	// Accept GENERATE_DKG and it will transit to dkg generate and then automatically to round start
	event := &fsm.Event{
		State: stateDKGGenerate,
	}
	err := cs.fsm.HandleTransition(event)
	assert.Error(t, err, "accept %s error", stateRoundStart)
	waitFor(
		t,
		func() bool { return cs.fsm.CurrentState() == stateRoundStart },
		100*time.Millisecond,
		fmt.Sprintf("expected state %s", string(stateRoundStart)))

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

	// Accept VOTE and then commit and then transit to epochStart start
	event = &fsm.Event{
		State:      stateAcceptVote,
		SenderAddr: delegates[1],
		BlockHash:  &blkHash,
	}
	err = cs.fsm.HandleTransition(event)
	assert.Nil(err)

	waitFor(
		t,
		func() bool { return cs.fsm.CurrentState() == stateEpochStart },
		time.Second,
		fmt.Sprintf("expected state %s", string(stateEpochStart)))
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
		first := mcks.bc.EXPECT().TipHeight().Return(uint64(0), nil).Times(1)
		mcks.bc.EXPECT().TipHeight().Return(uint64(2), nil).After(first).AnyTimes()
	}
	cs := createTestRollDPoS(
		ctrl, delegates[0], delegates, m, FixedProposer, time.Hour, NeverStartNewEpoch, nil)
	cs.Start()
	defer cs.Stop()

	// arrange proposal request
	genesis := NewGenesisBlock()

	// Accept GENERATE_DKG and it will transit to dkg generate and then automatically to round start
	event := &fsm.Event{
		State: stateDKGGenerate,
	}
	err := cs.fsm.HandleTransition(event)
	assert.Error(t, err, "accept %s error", stateRoundStart)
	waitFor(
		t,
		func() bool { return cs.fsm.CurrentState() == stateRoundStart },
		100*time.Millisecond,
		fmt.Sprintf("expected state %s", string(stateRoundStart)))

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

	waitFor(
		t,
		func() bool { return cs.fsm.CurrentState() == stateEpochStart },
		time.Second,
		fmt.Sprintf("expected state %s", string(stateEpochStart)))
}

func waitFor(t *testing.T, satisfy func() bool, timeout time.Duration, msg string) {
	ready := make(chan bool)
	go func() {
		for !satisfy() {
			time.Sleep(10 * time.Millisecond)
		}
		ready <- true
	}()
	select {
	case <-ready:
	case <-time.After(timeout):
		assert.Fail(t, msg)
	}
}
