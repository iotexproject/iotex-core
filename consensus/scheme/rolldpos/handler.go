// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"net"
	"time"

	"github.com/iotexproject/iotex-core/consensus/fsm"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// epochStart is the initial and idle state of a round of epochStart. It initiates the epochStart context.
type epochStart struct {
	fsm.NilTimeout
	*RollDPoS
}

func (h *epochStart) Handle(event *fsm.Event) {

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
	h.enqueueEvent(&fsm.Event{
		State: stateRoundStart,
	})
}

// roundStart is the initial and idle state of a round of consensus. It initiates the round context.
type roundStart struct {
	*RollDPoS
}

// TimeoutDuration returns the duration for timeout
func (h *roundStart) TimeoutDuration() *time.Duration {
	return &h.cfg.RoundStartTTL
}

func (h *roundStart) Handle(_ *fsm.Event) {
	h.roundCtx = &roundCtx{
		prevotes: make(map[net.Addr]*hash.Hash32B),
		votes:    make(map[net.Addr]*hash.Hash32B),
	}
}

// initPropose proposes a new block and send it out.
type initPropose struct {
	*RollDPoS
}

// TimeoutDuration returns the duration for timeout
func (h *initPropose) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptProposeTTL
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
	return &h.cfg.AcceptPrevoteTTL
}

func (h *acceptPrevote) Handle(event *fsm.Event) {
	h.roundCtx.prevotes[event.SenderAddr] = event.BlockHash
}

// acceptPropose waits for the proposed block and validate or timeout.
type acceptPropose struct {
	*RollDPoS
}

func (h *acceptPropose) TimeoutDuration() *time.Duration {
	return &h.cfg.AcceptProposeTTL
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
	return &h.cfg.AcceptVoteTTL
}

func (h *acceptVote) Handle(event *fsm.Event) {
	h.roundCtx.votes[event.SenderAddr] = event.BlockHash
}
