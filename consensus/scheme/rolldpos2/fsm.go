// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"context"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

const (
	// consensusEvt states
	sEpochStart    fsm.State = "S_EPOCH_START"
	sDKGGeneration fsm.State = "S_DKG_GENERATION"
	sRoundStart    fsm.State = "S_ROUND_START"
	sInitPropose   fsm.State = "S_INIT_PROPOSE"
	sAcceptPropose fsm.State = "S_PROPOSE"
	sAcceptPrevote fsm.State = "S_PREVOTE"
	sAcceptVote    fsm.State = "S_VOTE"

	// sInvalid indicates an invalid state. It doesn't matter what dst state to return when there's an error. Transition
	// to dst state will not happen. However, we should always return to this state to be consistent.
	sInvalid fsm.State = "S_INVALID"

	// consensusEvt event types
	eRollDelegates       fsm.EventType = "E_ROLL_DELEGATES"
	eGenerateDKG         fsm.EventType = "E_GENERATE_DKG"
	eStartRound          fsm.EventType = "E_START_ROUND"
	eInitBlock           fsm.EventType = "E_INIT_BLOCK"
	eProposeBlock        fsm.EventType = "E_PROPOSE_BLOCK"
	eProposeBlockTimeout fsm.EventType = "E_PROPOSE_BLOCK_TIMEOUT"
	ePrevote             fsm.EventType = "E_PREVOTE"
	ePrevoteTimeout      fsm.EventType = "E_PREVOTE_TIMEOUT"
	eVote                fsm.EventType = "E_VOTE"
	eVoteTimeout         fsm.EventType = "E_VOTE_TIMEOUT"
	eFinishEpoch         fsm.EventType = "E_FINISH_EPOCH"

	// eBackdoor indicates an backdoor event type
	eBackdoor fsm.EventType = "E_BACKDOOR"
)

var (
	// errEvtCast indicates the error of casting the event
	errEvtCast = errors.New("error when casting the event")
	// errEvtConvert indicates the error of converting the event from/to the proto message
	errEvtConvert = errors.New("error when converting the event from/to the proto message")

	// consensusStates is a slice consisting of all consensusEvt states
	consensusStates = []fsm.State{
		sEpochStart,
		sDKGGeneration,
		sRoundStart,
		sInitPropose,
		sAcceptPropose,
		sAcceptPrevote,
		sAcceptVote,
	}
)

// iConsensusEvt is the interface of all events for the consensusEvt FSM
type iConsensusEvt interface {
	fsm.Event
	timestamp() time.Time
	toProtoMsg() (*iproto.ViewChangeMsg, error)
	fromProtoMsg(msg *iproto.ViewChangeMsg) error
	// TODO: we need to add height or some other ctx to identify which consensus round the event is associated to
}

type consensusEvt struct {
	t  fsm.EventType
	ts time.Time
}

func newCEvt(t fsm.EventType, c clock.Clock) *consensusEvt {
	return &consensusEvt{
		t:  t,
		ts: c.Now(),
	}
}

func (e *consensusEvt) Type() fsm.EventType { return e.t }

func (e *consensusEvt) timestamp() time.Time { return e.ts }

func (e *consensusEvt) toProtoMsg() (*iproto.ViewChangeMsg, error) {
	return nil, errors.Wrap(errEvtConvert, "converting to the proto message is not implemented")
}

func (e *consensusEvt) fromProtoMsg(pMsg *iproto.ViewChangeMsg) error {
	return errors.Wrap(errEvtConvert, "converting from the proto message is not implemented")
}

type proposeBlkEvt struct {
	consensusEvt

	blk *blockchain.Block
}

func newProposeBlkEvt(blk *blockchain.Block, c clock.Clock) *proposeBlkEvt {
	return &proposeBlkEvt{
		consensusEvt: *newCEvt(eProposeBlock, c),
		blk:          blk,
	}
}

func (e *proposeBlkEvt) toProtoMsg() (*iproto.ViewChangeMsg, error) {
	return &iproto.ViewChangeMsg{
		Vctype: iproto.ViewChangeMsg_PROPOSE,
		Block:  e.blk.ConvertToBlockPb(),
	}, nil
}

func (e *proposeBlkEvt) fromProtoMsg(pMsg *iproto.ViewChangeMsg) error {
	if pMsg.Vctype != iproto.ViewChangeMsg_PROPOSE {
		return errors.Wrapf(errEvtConvert, "pMsg Vctype is %d", pMsg.Vctype)
	}
	if pMsg.Block != nil {
		e.blk.ConvertFromBlockPb(pMsg.Block)
	}
	return nil
}

// backdoorEvt is used for testing purpose to set the consensusEvt FSM to any particular state
type backdoorEvt struct {
	consensusEvt
	dst fsm.State
}

func newBackdoorEvt(dst fsm.State, c clock.Clock) *backdoorEvt {
	return &backdoorEvt{
		consensusEvt: *newCEvt(eBackdoor, c),
		dst:          dst,
	}
}

// cFSM wraps over the general purpose FSM and implements the consensusEvt logic
type cFSM struct {
	fsm   fsm.FSM
	evtq  chan iConsensusEvt
	close chan interface{}
	ctx   *rollDPoSCtx
	wg    sync.WaitGroup
}

func newConsensusFSM(ctx *rollDPoSCtx) (*cFSM, error) {
	cm := &cFSM{
		evtq:  make(chan iConsensusEvt, ctx.cfg.EventChanSize),
		close: make(chan interface{}),
		ctx:   ctx,
	}
	b := fsm.NewBuilder().
		AddInitialState(sEpochStart).
		AddStates(sDKGGeneration, sRoundStart, sInitPropose, sAcceptPropose, sAcceptPrevote, sAcceptVote).
		AddTransition(sEpochStart, eRollDelegates, cm.handleRollDelegatesEvt, []fsm.State{sEpochStart, sDKGGeneration}).
		AddTransition(sDKGGeneration, eGenerateDKG, cm.handleGenerateDKGEvt, []fsm.State{sRoundStart}).
		AddTransition(sRoundStart, eStartRound, cm.handleStartRoundEvt, []fsm.State{sInitPropose, sAcceptPropose}).
		AddTransition(sInitPropose, eInitBlock, cm.handleInitBlockEvt, []fsm.State{sAcceptPrevote}).
		AddTransition(sAcceptPropose, eInitBlock, cm.handleProposeBlockEvt, []fsm.State{sAcceptPrevote}).
		AddTransition(sAcceptPropose, eProposeBlockTimeout, cm.handleProposeBlockEvt, []fsm.State{sAcceptPrevote}).
		AddTransition(sAcceptPrevote, ePrevote, cm.handlePrevoteEvt, []fsm.State{sAcceptPrevote, sAcceptVote}).
		AddTransition(sAcceptPrevote, ePrevoteTimeout, cm.handlePrevoteEvt, []fsm.State{sAcceptVote}).
		AddTransition(sAcceptVote, eVote, cm.handleVoteEvt, []fsm.State{sAcceptVote, sRoundStart}).
		AddTransition(sAcceptVote, eVoteTimeout, cm.handleVoteEvt, []fsm.State{sRoundStart}).
		AddTransition(sRoundStart, eFinishEpoch, cm.handleFinishEpochEvt, []fsm.State{sEpochStart, sRoundStart})
	// Add the backdoor transition so that we could unit test the transition from any given state
	for _, state := range consensusStates {
		b = b.AddTransition(state, eBackdoor, cm.handleBackdoorEvt, consensusStates)
	}
	m, err := b.Build()
	if err != nil {
		return nil, err
	}
	cm.fsm = m
	return cm, nil
}

func (m *cFSM) Start(c context.Context) error {
	m.wg.Add(1)
	go func() {
		running := true
		for running {
			select {
			case <-m.close:
				running = false
			case evt := <-m.evtq:
				src := m.fsm.CurrentState()
				if err := m.fsm.Handle(evt); err != nil {
					if errors.Cause(err) == fsm.ErrTransitionNotFound {
						if time.Since(evt.timestamp()) <= m.ctx.cfg.UnmatchedEventTTL {
							// TODO: avoid putting the unmatched event into the event queue immediately
							m.produce(evt, 0)
							logger.Debug().
								Str("id", m.ctx.addr.RawAddress).
								Str("src", string(src)).
								Err(err).
								Msg("consensusEvt state transition could find the match")
						}
					} else {
						logger.Error().
							Str("id", m.ctx.addr.RawAddress).
							Str("src", string(src)).
							Err(err).
							Msg("consensusEvt state transition fails")
					}
				} else {
					dst := m.fsm.CurrentState()
					logger.Info().
						Str("id", m.ctx.addr.RawAddress).
						Str("src", string(src)).
						Str("dst", string(dst)).
						Msg("consensusEvt state transition happens")
				}
			}
		}
		m.wg.Done()
	}()
	return nil
}

func (m *cFSM) Stop(_ context.Context) error {
	close(m.close)
	m.wg.Wait()
	return nil
}

func (m *cFSM) currentState() fsm.State {
	return m.fsm.CurrentState()
}

// produce adds an event into the queue for the consensus FSM to process
func (m *cFSM) produce(evt iConsensusEvt, delay time.Duration) {
	if delay > 0 {
		m.wg.Add(1)
		go func() {
			select {
			case <-m.close:
			case <-time.After(delay):
				m.evtq <- evt
			}
			m.wg.Done()
		}()
	} else {
		m.evtq <- evt
	}
}

func (m *cFSM) handleRollDelegatesEvt(_ fsm.Event) (fsm.State, error) {
	epochNum, epochHeight, err := m.ctx.calcEpochNumAndHeight()
	if err != nil {
		return sInvalid, errors.Wrap(
			err,
			"error when determining the epoch ordinal number and start height offset",
		)
	}
	delegates, err := m.ctx.rollingDelegates(epochNum)
	if err != nil {
		return sInvalid, errors.Wrap(
			err,
			"error when determining if the node will participate into next epoch",
		)
	}
	// If the current node is the delegate, move to the next state
	if m.isDelegate(delegates) {
		// Get the sub-epoch number
		numSubEpochs := uint(1)
		if m.ctx.cfg.NumSubEpochs > 0 {
			numSubEpochs = m.ctx.cfg.NumSubEpochs
		}

		// The epochStart start height is going to be the next block to generate
		m.ctx.epoch = epochCtx{
			num:          epochNum,
			height:       epochHeight,
			delegates:    delegates,
			numSubEpochs: numSubEpochs,
		}

		// Trigger the event to generate DKG
		m.produce(m.newCEvt(eGenerateDKG), 0)

		logger.Info().
			Str("id", m.ctx.addr.RawAddress).
			Uint64("epoch", epochNum).
			Uint64("height", epochHeight).
			Msg("current node is the delegate")
		return sDKGGeneration, nil
	}
	// Else, stay at the current state and check again later
	m.produce(m.newCEvt(eRollDelegates), m.ctx.cfg.DelegateInterval)
	logger.Info().
		Str("id", m.ctx.addr.RawAddress).
		Uint64("epoch", epochNum).
		Uint64("height", epochHeight).
		Msg("current node is not the delegate")
	return sEpochStart, nil
}

func (m *cFSM) handleGenerateDKGEvt(_ fsm.Event) (fsm.State, error) {
	dkg, err := m.ctx.generateDKG()
	if err != nil {
		return sInvalid, err
	}
	m.ctx.epoch.dkg = dkg

	duration, err := m.ctx.calcDurationSinceLastBlock()
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when computing the duration since last block time")
	}
	// If the proposal interval is not set (not zero), the next round will only be started after the configured duration
	// after last block's creation time, so that we could keep the constant
	if duration >= m.ctx.cfg.ProposerInterval {
		m.produce(m.newCEvt(eStartRound), 0)
	} else {
		m.produce(m.newCEvt(eStartRound), m.ctx.cfg.ProposerInterval-duration)
	}
	return sRoundStart, nil
}

func (m *cFSM) handleStartRoundEvt(_ fsm.Event) (fsm.State, error) {
	proposer, height, err := m.ctx.rotatedProposer()
	if err != nil {
		logger.Error().
			Str("id", m.ctx.addr.RawAddress).
			Err(err).
			Msg("error when getting the proposer")
		return sInvalid, err
	}
	m.ctx.round = roundCtx{
		prevotes: make(map[string]*hash.Hash32B),
		votes:    make(map[string]*hash.Hash32B),
		proposer: proposer,
	}
	if proposer == m.ctx.addr.RawAddress {
		logger.Info().
			Str("proposer", proposer).
			Uint64("height", height).
			Msg("current node is the proposer")
		m.produce(m.newCEvt(eInitBlock), 0)
		// TODO: we may need timeout event for block producer too
		return sInitPropose, nil
	}
	logger.Info().
		Str("proposer", proposer).
		Uint64("height", height).
		Msg("current node is not the proposer")
	m.produce(m.newCEvt(eProposeBlockTimeout), m.ctx.cfg.AcceptProposeTTL)
	return sAcceptPropose, nil
}

func (m *cFSM) handleInitBlockEvt(evt fsm.Event) (fsm.State, error) {
	blk, err := m.ctx.mintBlock()
	if err != nil {
		return sInvalid, err
	}
	proposeBlkEvt := m.newProposeBlkEvt(blk)
	proposeBlkEvtProto, err := proposeBlkEvt.toProtoMsg()
	if err != nil {
		return sInvalid, err
	}
	// Notify itself
	m.produce(proposeBlkEvt, 0)
	// Notify other delegates
	m.ctx.p2p.Broadcast(proposeBlkEvtProto)
	return sAcceptPropose, nil
}

func (m *cFSM) handleProposeBlockEvt(evt fsm.Event) (fsm.State, error) {
	switch evt.Type() {
	case eProposeBlock:
		// TODO: validate a block and prevote based on the result
	case eProposeBlockTimeout:
		// TODO: prevote failure
	}
	m.produce(m.newCEvt(ePrevote), 0)
	m.produce(m.newCEvt(ePrevoteTimeout), m.ctx.cfg.AcceptPrevoteTTL)
	return sAcceptPrevote, nil
}

func (m *cFSM) handlePrevoteEvt(evt fsm.Event) (fsm.State, error) {
	switch evt.Type() {
	case ePrevote:
		// TODO: collect 2/3 quorum and vote based on the result
	case ePrevoteTimeout:
		// TODO: vote failure
	}
	m.produce(m.newCEvt(eVote), 0)
	m.produce(m.newCEvt(eVoteTimeout), m.ctx.cfg.AcceptPrevoteTTL)
	return sAcceptVote, nil
}

func (m *cFSM) handleVoteEvt(evt fsm.Event) (fsm.State, error) {
	switch evt.Type() {
	case eVote:
		// TODO: commit and broadcast a block with consensusEvt
	case eVoteTimeout:
		// TODO: commit a dummy block
	}
	m.produce(m.newCEvt(eFinishEpoch), 0)
	return sAcceptVote, nil
}

func (m *cFSM) handleFinishEpochEvt(evt fsm.Event) (fsm.State, error) {
	// TODO: determine if the epoch has finished
	finished := true
	if finished {
		m.produce(m.newCEvt(eRollDelegates), 0)
		return sEpochStart, nil
	}
	m.produce(m.newCEvt(eStartRound), 0)
	return sRoundStart, nil

}

// handleBackdoorEvt takes the dst state from the event and move the FSM into it
func (m *cFSM) handleBackdoorEvt(evt fsm.Event) (fsm.State, error) {
	bEvt, ok := evt.(*backdoorEvt)
	if !ok {
		return sInvalid, errors.Wrap(errEvtCast, "the event is not a backdoorEvt")
	}
	return bEvt.dst, nil
}

func (m *cFSM) isDelegate(delegates []string) bool {
	for _, d := range delegates {
		if m.ctx.addr.RawAddress == d {
			return true
		}
	}
	return false
}

func (m *cFSM) newCEvt(t fsm.EventType) *consensusEvt {
	return newCEvt(t, m.ctx.clock)
}

func (m *cFSM) newProposeBlkEvt(blk *blockchain.Block) *proposeBlkEvt {
	return newProposeBlkEvt(blk, m.ctx.clock)
}

func (m *cFSM) newBackdoorEvt(dst fsm.State) *backdoorEvt {
	return newBackdoorEvt(dst, m.ctx.clock)
}
