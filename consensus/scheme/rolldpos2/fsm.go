// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos2

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core-internal/logger"
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
	Time() time.Time
	// TODO: we need to add height or some other ctx to identify which consensus round the event is associated to
}

type consensusEvt struct {
	t    fsm.EventType
	time time.Time
}

func newCEvt(t fsm.EventType, c clock.Clock) *consensusEvt {
	return &consensusEvt{
		t:    t,
		time: c.Now(),
	}
}

func (e *consensusEvt) Type() fsm.EventType { return e.t }

func (e *consensusEvt) Time() time.Time { return e.time }

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
		AddTransition(sInitPropose, eProposeBlock, cm.handleProposeBlockEvt, []fsm.State{sAcceptPrevote}).
		AddTransition(sAcceptPropose, eProposeBlock, cm.handleProposeBlockEvt, []fsm.State{sAcceptPrevote}).
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
						if time.Since(evt.Time()) <= m.ctx.cfg.UnmatchedEventTTL {
							// TODO: avoid putting the unmatched event into the event queue immediately
							m.enqueue(evt, 0)
							logger.Debug().
								Str("id", m.ctx.id.String()).
								Str("src", string(src)).
								Err(err).
								Msg("consensusEvt state transition could find the match")
						}
					} else {
						logger.Error().
							Str("id", m.ctx.id.String()).
							Str("src", string(src)).
							Err(err).
							Msg("consensusEvt state transition fails")
					}
				} else {
					dst := m.fsm.CurrentState()
					logger.Info().
						Str("id", m.ctx.id.String()).
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

func (m *cFSM) enqueue(evt iConsensusEvt, delay time.Duration) {
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

func (m *cFSM) handleRollDelegatesEvt(evt fsm.Event) (fsm.State, error) {
	epochNum, epochHeight, err := m.ctx.calcEpochNumAndHeight()
	if err != nil {
		return sInvalid, errors.Wrap(
			err,
			"error when determining the epoch ordinal number and start height offset",
		)
	}
	delegates, err := m.ctx.getRollingDelegates(epochNum)
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
		m.enqueue(m.newCEvt(eGenerateDKG), 0)

		logger.Info().
			Str("id", m.ctx.id.String()).
			Uint64("epoch", epochNum).
			Uint64("height", epochHeight).
			Msg("current node is the delegate")
		return sDKGGeneration, nil
	}
	// Else, stay at the current state and check again later
	m.enqueue(m.newCEvt(eRollDelegates), m.ctx.cfg.DelegateInterval)
	logger.Info().
		Str("id", m.ctx.id.String()).
		Uint64("epoch", epochNum).
		Uint64("height", epochHeight).
		Msg("current node is not the delegate")
	return sEpochStart, nil
}

func (m *cFSM) handleGenerateDKGEvt(evt fsm.Event) (fsm.State, error) {
	// TODO: generate DKG

	m.enqueue(m.newCEvt(eStartRound), 0)
	return sRoundStart, nil
}

func (m *cFSM) handleStartRoundEvt(evt fsm.Event) (fsm.State, error) {
	// TODO: setup round context and check the if the current node is the block producer
	elected := true
	if elected {
		// TODO: propose a block
		m.enqueue(m.newCEvt(eProposeBlock), 0)
		// TODO: we may need timeout event for block producer too
		return sInitPropose, nil
	}
	m.enqueue(m.newCEvt(ePrevoteTimeout), m.ctx.cfg.AcceptProposeTTL)
	return sAcceptPropose, nil
}

func (m *cFSM) handleProposeBlockEvt(evt fsm.Event) (fsm.State, error) {
	switch evt.Type() {
	case eProposeBlock:
		// TODO: validate a block and prevote based on the result
	case eProposeBlockTimeout:
		// TODO: prevote failure
	}
	m.enqueue(m.newCEvt(ePrevote), 0)
	m.enqueue(m.newCEvt(ePrevoteTimeout), m.ctx.cfg.AcceptPrevoteTTL)
	return sAcceptPrevote, nil
}

func (m *cFSM) handlePrevoteEvt(evt fsm.Event) (fsm.State, error) {
	switch evt.Type() {
	case ePrevote:
		// TODO: collect 2/3 quorum and vote based on the result
	case ePrevoteTimeout:
		// TODO: vote failure
	}
	m.enqueue(m.newCEvt(eVote), 0)
	m.enqueue(m.newCEvt(eVoteTimeout), m.ctx.cfg.AcceptPrevoteTTL)
	return sAcceptVote, nil
}

func (m *cFSM) handleVoteEvt(evt fsm.Event) (fsm.State, error) {
	switch evt.Type() {
	case eVote:
		// TODO: commit and broadcast a block with consensusEvt
	case eVoteTimeout:
		// TODO: commit a dummy block
	}
	m.enqueue(m.newCEvt(eFinishEpoch), 0)
	return sAcceptVote, nil
}

func (m *cFSM) handleFinishEpochEvt(evt fsm.Event) (fsm.State, error) {
	// TODO: determine if the epoch has finished
	finished := true
	if finished {
		m.enqueue(m.newCEvt(eRollDelegates), 0)
		return sEpochStart, nil
	}
	m.enqueue(m.newCEvt(eStartRound), 0)
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

func (m *cFSM) isDelegate(delegates []net.Addr) bool {
	for _, d := range delegates {
		if m.ctx.id.String() == d.String() {
			return true
		}
	}
	return false
}

func (m *cFSM) newCEvt(t fsm.EventType) iConsensusEvt {
	return newCEvt(t, m.ctx.clock)
}

func (m *cFSM) newBackdoorEvt(dst fsm.State) iConsensusEvt {
	return newBackdoorEvt(dst, m.ctx.clock)
}
