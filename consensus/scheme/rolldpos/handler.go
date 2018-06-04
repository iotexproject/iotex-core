// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"time"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/consensus/fsm"
)

// epochStart is the initial and idle state of a round of epoch. It initiates the epoch context.
type epochStart struct {
	fsm.NilTimeout
	*RollDPoS
}

func (h *epochStart) Handle(event *fsm.Event) {
	// Assume the view of the delegates are fixed during a consensus round
	delegates, err := h.pool.AllDelegates()
	if err != nil {
		event.Err = err
		return
	}
	height, err := h.bc.TipHeight()
	if err != nil {
		event.Err = err
		return
	}
	h.epochCtx = &epochCtx{height: height, delegates: delegates}
}

// dkgGenerate is the state of generating DKG.
type dkgGenerate struct {
	fsm.NilTimeout
	*RollDPoS
}

func (h *dkgGenerate) Handle(event *fsm.Event) {
	dkg, err := h.dkgCb()
	if err != nil {
		event.Err = err
		return
	}
	h.epochCtx.dkg = dkg
	h.handleEvent(&fsm.Event{
		State: stateRoundStart,
	})
}

// roundStart is the initial and idle state of a round of consensus. It initiates the round context.
type roundStart struct {
	fsm.NilTimeout
	*RollDPoS
}

func (h *roundStart) Handle(_ *fsm.Event) {
	h.roundCtx = &roundCtx{
		prevotes: make(map[net.Addr]*common.Hash32B),
		votes:    make(map[net.Addr]*common.Hash32B),
	}
}

// initPropose proposes a new block and send it out.
type initPropose struct {
	fsm.NilTimeout
	*RollDPoS
}

// TimeoutDuration returns the duration for timeout
func (h *initPropose) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptPropose.TTL
}

func (h *initPropose) Handle(event *fsm.Event) {
	h.roundCtx.isPr = true
	blk, err := h.propCb()
	if err != nil {
		event.Err = err
		return
	}

	h.roundCtx.block = blk
}

// acceptPrevote waits for 2k prevote messages for the same block from others or timeout.
type acceptPrevote struct {
	*RollDPoS
}

func (h *acceptPrevote) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptPrevote.TTL
}

func (h *acceptPrevote) Handle(event *fsm.Event) {
	h.roundCtx.prevotes[event.SenderAddr] = event.BlockHash
}

// acceptPropose waits for the proposed block and validate or timeout.
type acceptPropose struct {
	*RollDPoS
}

func (h *acceptPropose) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptPropose.TTL
}

func (h *acceptPropose) Handle(event *fsm.Event) {
	h.roundCtx.prevotes[event.SenderAddr] = event.BlockHash
	event.Err = h.bc.ValidateBlock(event.Block)
}

// acceptVote waits for 2k vote messages from others or timeout.
type acceptVote struct {
	*RollDPoS
}

func (h *acceptVote) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptVote.TTL
}

func (h *acceptVote) Handle(event *fsm.Event) {
	h.roundCtx.votes[event.SenderAddr] = event.BlockHash
}
