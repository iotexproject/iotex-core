// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zjshen14/go-fsm"
)

/**
 * TODO: For the nodes received correct proposal, add proposer's proposal endorse without signature, which could be replaced with real signature
 */
var (
	consensusMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_consensus",
			Help: "Consensus stats",
		},
		[]string{"result"},
	)
)

func init() {
	prometheus.MustRegister(consensusMtc)
}

const (
	// consensusEvt states
	sPrepare               fsm.State = "S_PREPARE"
	sAcceptPropose         fsm.State = "S_ACCEPT_PROPOSE"
	sAcceptProposalEndorse fsm.State = "S_ACCEPT_PROPOSAL_ENDORSE"
	sAcceptLockEndorse     fsm.State = "S_ACCEPT_LOCK_ENDORSE"
	sAcceptCommitEndorse   fsm.State = "S_ACCEPT_COMMIT_ENDORSE"

	// consensusEvt event types
	ePrepare                      fsm.EventType = "E_PREPARE"
	eReceiveBlock                 fsm.EventType = "E_RECEIVE_BLOCK"
	eFailedToReceiveBlock         fsm.EventType = "E_FAILED_TO_RECEIVE_BLOCK"
	eReceiveProposalEndorsement   fsm.EventType = "E_RECEIVE_PROPOSAL_ENDORSEMENT"
	eNotEnoughProposalEndorsement fsm.EventType = "E_NOT_ENOUGH_PROPOSAL_ENDORSEMENT"
	eReceiveLockEndorsement       fsm.EventType = "E_RECEIVE_LOCK_ENDORSEMENT"
	eNotEnoughLockEndorsement     fsm.EventType = "E_NOT_ENOUGH_LOCK_ENDORSEMENT"
	eReceiveCommitEndorsement     fsm.EventType = "E_RECEIVE_COMMIT_ENDORSEMENT"
	// eStopReceivingCommitEndorsement fsm.EventType = "E_STOP_RECEIVING_COMMIT_ENDORSEMENT"

	// eBackdoor indicates an backdoor event type
	eBackdoor fsm.EventType = "E_BACKDOOR"
)

var (
	// ErrEvtCast indicates the error of casting the event
	ErrEvtCast = errors.New("error when casting the event")
	// ErrEvtConvert indicates the error of converting the event from/to the proto message
	ErrEvtConvert = errors.New("error when converting the event from/to the proto message")
	// ErrEvtType represents an unexpected event type error
	ErrEvtType = errors.New("error when check the event type")

	// consensusStates is a slice consisting of all consensusEvt states
	consensusStates = []fsm.State{
		sPrepare,
		sAcceptPropose,
		sAcceptProposalEndorse,
		sAcceptLockEndorse,
		sAcceptCommitEndorse,
	}
)

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
		AddInitialState(sPrepare).
		AddStates(
			sAcceptPropose,
			sAcceptProposalEndorse,
			sAcceptLockEndorse,
			sAcceptCommitEndorse,
		).
		AddTransition(sPrepare, ePrepare, cm.prepare, []fsm.State{
			sPrepare,
			sAcceptPropose,
		}).
		AddTransition(
			sAcceptPropose,
			eReceiveBlock,
			cm.onReceiveBlock,
			[]fsm.State{
				sAcceptPropose,         // proposed block invalid
				sAcceptProposalEndorse, // receive valid block, jump to next step
			}).
		AddTransition(
			sAcceptPropose,
			eFailedToReceiveBlock,
			cm.handleProposeBlockTimeout,
			[]fsm.State{
				sAcceptProposalEndorse, // no valid block, jump to next step
			}).
		AddTransition(
			sAcceptProposalEndorse,
			eReceiveProposalEndorsement,
			cm.onReceiveProposalEndorsement,
			[]fsm.State{
				sAcceptProposalEndorse, // haven't reach agreement yet
				sAcceptLockEndorse,     // reach agreement
			}).
		AddTransition(
			sAcceptProposalEndorse,
			eNotEnoughProposalEndorsement,
			cm.handleEndorseProposalTimeout,
			[]fsm.State{
				sAcceptLockEndorse, // timeout, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorse,
			eReceiveLockEndorsement,
			cm.onReceiveLockEndorsement,
			[]fsm.State{
				sAcceptLockEndorse,   // haven't reach agreement yet
				sAcceptCommitEndorse, // reach commit agreement, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorse,
			eNotEnoughLockEndorsement,
			cm.handleEndorseLockTimeout,
			[]fsm.State{
				sAcceptCommitEndorse, // reach commit agreement, jump to next step
				sPrepare,             // timeout, jump to next round
			}).
		AddTransition(
			sAcceptCommitEndorse,
			eReceiveCommitEndorsement,
			cm.handleEndorseCommitEvt,
			[]fsm.State{
				sPrepare, // reach consensus, start next epoch
			})
	// Add the backdoor transition so that we could unit test the transition from any given state
	for _, state := range consensusStates {
		b = b.AddTransition(state, eBackdoor, cm.handleBackdoorEvt, consensusStates)
	}
	m, err := b.Build()
	if err != nil {
		return nil, errors.Wrap(err, "error when building the FSM")
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
				if m.ctx.IsStaleEvent(evt) {
					m.ctx.Logger().Debug().Msg("stale event")
					continue
				}
				if m.ctx.IsFutureEvent(evt) {
					m.ctx.Logger().Debug().Msg("future event")
					// TODO: find a more appropriate delay
					m.produce(evt, m.ctx.cfg.UnmatchedEventInterval)
					continue
				}
				src := m.fsm.CurrentState()
				if err := m.fsm.Handle(evt); err != nil {
					if errors.Cause(err) == fsm.ErrTransitionNotFound {
						if !m.ctx.IsStaleUnmatchedEvent(evt) {
							m.produce(evt, m.ctx.cfg.UnmatchedEventInterval)
							m.ctx.Logger().Debug().
								Str("src", string(src)).
								Str("evt", string(evt.Type())).
								Err(err).
								Msg("consensusEvt state transition could find the match")
						}
					} else {
						m.ctx.Logger().Error().
							Str("src", string(src)).
							Str("evt", string(evt.Type())).
							Err(err).
							Msg("consensusEvt state transition fails")
					}
				} else {
					dst := m.fsm.CurrentState()
					m.ctx.Logger().Info().
						Str("src", string(src)).
						Str("dst", string(dst)).
						Str("evt", string(evt.Type())).
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
	consensusMtc.WithLabelValues(string(evt.Type())).Inc()
	if delay > 0 {
		m.wg.Add(1)
		go func() {
			select {
			case <-m.close:
			case <-m.ctx.clock.After(delay):
				m.evtq <- evt
			}
			m.wg.Done()
		}()
	} else {
		m.evtq <- evt
	}
}

func (m *cFSM) prepare(_ fsm.Event) (fsm.State, error) {
	delay, isDelegate, isProposer, err := m.ctx.PrepareNextRound()
	if err != nil {
		m.produce(m.newCEvt(ePrepare), delay)
		return sPrepare, errors.Wrap(err, "error when prepare next round")
	}
	if !isDelegate {
		m.produce(m.newCEvt(ePrepare), delay)
		return sPrepare, nil
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	// Setup timeout for waiting for proposed block
	ttl := m.ctx.cfg.AcceptProposeTTL
	m.produce(m.newTimeoutEvt(eFailedToReceiveBlock), ttl)
	ttl += m.ctx.cfg.AcceptProposalEndorseTTL
	m.produce(m.newTimeoutEvt(eNotEnoughProposalEndorsement), ttl)
	ttl += m.ctx.cfg.AcceptCommitEndorseTTL
	m.produce(m.newTimeoutEvt(eNotEnoughLockEndorsement), ttl)
	// TODO add timeout for commit collection
	if isProposer {
		blk, err := m.ctx.MintBlock()
		if err != nil {
			// TODO: review the return state
			return sPrepare, errors.Wrap(err, "error when minting a block")
		}
		proposeBlkEvt := m.newProposeBlkEvt(blk)
		m.ctx.Logger().Info().Hex("blockHash", blk.Hash()).Msg("Broadcast init proposal.")
		// Notify itself
		m.produce(proposeBlkEvt, 0)
		// Notify other delegates
		if err := m.ctx.Broadcast(proposeBlkEvt.toProtoMsg()); err != nil {
			m.ctx.Logger().Error().
				Err(err).
				Msg("error when broadcasting proposeBlkEvt")
		}
	}

	return sAcceptPropose, nil
}

func (m *cFSM) onReceiveBlock(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eReceiveBlock {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	proposeBlkEvt, ok := evt.(*proposeBlkEvt)
	if !ok {
		return sPrepare, errors.Wrap(ErrEvtCast, "the event is not a proposeBlkEvt")
	}
	if cEvt, err := m.ctx.ProcessProposeBlock(proposeBlkEvt.block); err == nil {
		m.produce(cEvt, 0)
	} else {
		return sAcceptPropose, err
	}

	return sAcceptProposalEndorse, nil
}

func (m *cFSM) handleProposeBlockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eFailedToReceiveBlock {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	m.ctx.Logger().Warn().
		Msg("didn't receive the proposed block before timeout")

	return sAcceptProposalEndorse, nil
}

func (m *cFSM) processEndorseEvent(
	evt fsm.Event,
	expectedEventType fsm.EventType,
) (Endorsement, error) {
	if evt.Type() != expectedEventType {
		return nil, errors.Wrapf(ErrEvtType, "invalid endorsement event type %s", evt.Type())
	}
	endorseEvt, ok := evt.(*endorseEvt)
	if !ok {
		return nil, errors.Wrap(ErrEvtCast, "the event is not an endorseEvt")
	}
	return endorseEvt.endorse, nil
}

func (m *cFSM) onReceiveProposalEndorsement(evt fsm.Event) (fsm.State, error) {
	en, err := m.processEndorseEvent(
		evt,
		eReceiveProposalEndorsement,
	)
	if errors.Cause(err) == ErrEvtCast || errors.Cause(err) == ErrEvtType {
		// TODO: review the return state
		return sPrepare, err
	}
	if err != nil {
		return sAcceptProposalEndorse, err
	}
	locked, cEvt, err := m.ctx.ProcessProposalEndorsement(en)
	if err != nil {
		return sAcceptProposalEndorse, errors.Wrap(
			err,
			"failed to process proposal endorsement",
		)
	}
	if !locked {
		return sAcceptProposalEndorse, nil
	}
	// Notify itself
	m.produce(cEvt, 0)

	return sAcceptLockEndorse, nil
}

func (m *cFSM) handleEndorseProposalTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eNotEnoughProposalEndorsement {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	m.ctx.LogProposalEndorsementStats()

	return sAcceptLockEndorse, nil
}

func (m *cFSM) onReceiveLockEndorsement(evt fsm.Event) (fsm.State, error) {
	en, err := m.processEndorseEvent(
		evt,
		eReceiveLockEndorsement,
	)
	if errors.Cause(err) == ErrEvtCast || errors.Cause(err) == ErrEvtType {
		return sPrepare, err
	}
	if err != nil {
		return sAcceptLockEndorse, err
	}
	enoughEndorsement, err := m.ctx.ProcessLockEndorsement(en)
	if err != nil {
		return sAcceptLockEndorse, errors.Wrap(
			err,
			"failed to process lock endorsement",
		)
	}
	if !enoughEndorsement {
		return sAcceptLockEndorse, nil
	}
	if cEvt, err := m.ctx.PreCommitBlock(); err == nil {
		m.produce(cEvt, 0)
	} else {
		m.ctx.Logger().Error().Err(err).Msg("error when precommitting a block")
	}

	return sAcceptCommitEndorse, nil
}

func (m *cFSM) handleEndorseLockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eNotEnoughLockEndorsement {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	m.ctx.LogLockEndorsementStats()
	m.produce(m.newCEvt(ePrepare), 0)

	return sPrepare, nil
}

func (m *cFSM) handleEndorseCommitEvt(evt fsm.Event) (fsm.State, error) {
	// Collect more commit endorsements
	consensusMtc.WithLabelValues("ReachConsenus").Inc()
	m.ctx.Logger().Info().
		Uint64("block", m.ctx.round.height).
		Msg("consensus reached")
	if err := m.ctx.OnConsensusReached(); err != nil {
		m.ctx.Logger().Error().Err(err).Msg("failed to commit block on consensus reached")
	}
	m.produce(m.newCEvt(ePrepare), 0)
	return sPrepare, nil
}

// handleBackdoorEvt takes the dst state from the event and move the FSM into it
func (m *cFSM) handleBackdoorEvt(evt fsm.Event) (fsm.State, error) {
	bEvt, ok := evt.(*backdoorEvt)
	if !ok {
		return sPrepare, errors.Wrap(ErrEvtCast, "the event is not a backdoorEvt")
	}
	return bEvt.dst, nil
}

func (m *cFSM) newCEvt(t fsm.EventType) *consensusEvt {
	return newCEvt(t, m.ctx.round.height, m.ctx.round.number, m.ctx.clock)
}

func (m *cFSM) newProposeBlkEvt(blk Block) *proposeBlkEvt {
	return newProposeBlkEvt(blk, m.ctx.round.proofOfLock, m.ctx.round.number, m.ctx.clock)
}

func (m *cFSM) newTimeoutEvt(t fsm.EventType) *timeoutEvt {
	return newTimeoutEvt(t, m.ctx.round.height, m.ctx.round.number, m.ctx.clock)
}

func (m *cFSM) newBackdoorEvt(dst fsm.State) *backdoorEvt {
	return newBackdoorEvt(dst, m.ctx.round.height, m.ctx.round.number, m.ctx.clock)
}
