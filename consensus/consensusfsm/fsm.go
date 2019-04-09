// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensusfsm

import (
	"context"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	fsm "github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

/**
 * TODO: For the nodes received correct proposal, add proposer's proposal endorse
 * without signature, which could be replaced with real signature
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
	// consensus states
	sPrepare                    fsm.State = "S_PREPARE"
	sAcceptBlockProposal        fsm.State = "S_ACCEPT_BLOCK_PROPOSAL"
	sAcceptProposalEndorsement  fsm.State = "S_ACCEPT_PROPOSAL_ENDORSEMENT"
	sAcceptLockEndorsement      fsm.State = "S_ACCEPT_LOCK_ENDORSEMENT"
	sAcceptPreCommitEndorsement fsm.State = "S_ACCEPT_PRECOMMIT_ENDORSEMENT"

	// consensus event types
	eCalibrate                        fsm.EventType = "E_CALIBRATE"
	ePrepare                          fsm.EventType = "E_PREPARE"
	eReceiveBlock                     fsm.EventType = "E_RECEIVE_BLOCK"
	eFailedToReceiveBlock             fsm.EventType = "E_FAILED_TO_RECEIVE_BLOCK"
	eReceiveProposalEndorsement       fsm.EventType = "E_RECEIVE_PROPOSAL_ENDORSEMENT"
	eStopReceivingProposalEndorsement fsm.EventType = "E_STOP_RECEIVING_PROPOSAL_ENDORSEMENT"
	eReceiveLockEndorsement           fsm.EventType = "E_RECEIVE_LOCK_ENDORSEMENT"
	eStopReceivingLockEndorsement     fsm.EventType = "E_STOP_RECEIVING_LOCK_ENDORSEMENT"
	eReceivePreCommitEndorsement      fsm.EventType = "E_RECEIVE_PRECOMMIT_ENDORSEMENT"
	eBroadcastPreCommitEndorsement    fsm.EventType = "E_BROADCAST_PRECOMMIT_ENDORSEMENT"

	// BackdoorEvent indicates a backdoor event type
	BackdoorEvent fsm.EventType = "E_BACKDOOR"

	// InitState refers the initial state of the consensus fsm
	InitState = sPrepare
)

var (
	// ErrEvtCast indicates the error of casting the event
	ErrEvtCast = errors.New("error when casting the event")
	// ErrMsgCast indicates the error of casting to endorsed message
	ErrMsgCast = errors.New("error when casting to endorsed message")
	// ErrEvtConvert indicates the error of converting the event from/to the proto message
	ErrEvtConvert = errors.New("error when converting the event from/to the proto message")
	// ErrEvtType represents an unexpected event type error
	ErrEvtType = errors.New("error when check the event type")

	// consensusStates is a slice consisting of all consensus states
	consensusStates = []fsm.State{
		sPrepare,
		sAcceptBlockProposal,
		sAcceptProposalEndorsement,
		sAcceptLockEndorsement,
		sAcceptPreCommitEndorsement,
	}
)

// Config defines a set of time durations used in fsm and event queue size
type Config struct {
	EventChanSize                uint          `yaml:"eventChanSize"`
	UnmatchedEventTTL            time.Duration `yaml:"unmatchedEventTTL"`
	UnmatchedEventInterval       time.Duration `yaml:"unmatchedEventInterval"`
	AcceptBlockTTL               time.Duration `yaml:"acceptBlockTTL"`
	AcceptProposalEndorsementTTL time.Duration `yaml:"acceptProposalEndorsementTTL"`
	AcceptLockEndorsementTTL     time.Duration `yaml:"acceptLockEndorsementTTL"`
	CommitTTL                    time.Duration `yaml:"commitTTL"`
}

// ConsensusFSM wraps over the general purpose FSM and implements the consensus logic
type ConsensusFSM struct {
	fsm   fsm.FSM
	evtq  chan *ConsensusEvent
	close chan interface{}
	clock clock.Clock
	cfg   Config
	ctx   Context
	wg    sync.WaitGroup
}

// NewConsensusFSM returns a new fsm
func NewConsensusFSM(cfg Config, ctx Context, clock clock.Clock) (*ConsensusFSM, error) {
	cm := &ConsensusFSM{
		evtq:  make(chan *ConsensusEvent, cfg.EventChanSize),
		close: make(chan interface{}),
		cfg:   cfg,
		ctx:   ctx,
		clock: clock,
	}
	b := fsm.NewBuilder().
		AddInitialState(sPrepare).
		AddStates(
			sAcceptBlockProposal,
			sAcceptProposalEndorsement,
			sAcceptLockEndorsement,
			sAcceptPreCommitEndorsement,
		).
		AddTransition(sPrepare, ePrepare, cm.prepare, []fsm.State{
			sPrepare,
			sAcceptBlockProposal,
		}).
		AddTransition(
			sAcceptBlockProposal,
			eReceiveBlock,
			cm.onReceiveBlock,
			[]fsm.State{
				sAcceptBlockProposal,       // proposed block invalid
				sAcceptProposalEndorsement, // receive valid block, jump to next step
			}).
		AddTransition(
			sAcceptBlockProposal,
			eFailedToReceiveBlock,
			cm.onFailedToReceiveBlock,
			[]fsm.State{
				sAcceptProposalEndorsement, // no valid block, jump to next step
			}).
		AddTransition(
			sAcceptProposalEndorsement,
			eReceiveProposalEndorsement,
			cm.onReceiveProposalEndorsement,
			[]fsm.State{
				sAcceptProposalEndorsement, // not enough endorsements
				sAcceptLockEndorsement,     // enough endorsements
			}).
		AddTransition(
			sAcceptProposalEndorsement,
			eReceivePreCommitEndorsement,
			cm.onReceiveProposalEndorsement,
			[]fsm.State{
				sAcceptProposalEndorsement, // not enough endorsements
				sAcceptLockEndorsement,     // enough endorsements
			}).
		AddTransition(
			sAcceptProposalEndorsement,
			eStopReceivingProposalEndorsement,
			cm.onStopReceivingProposalEndorsement,
			[]fsm.State{
				sAcceptLockEndorsement, // timeout, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorsement,
			eReceiveLockEndorsement,
			cm.onReceiveLockEndorsement,
			[]fsm.State{
				sAcceptLockEndorsement,      // not enough endorsements
				sAcceptPreCommitEndorsement, // reach commit agreement, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorsement,
			eReceivePreCommitEndorsement,
			cm.onReceiveLockEndorsement,
			[]fsm.State{
				sAcceptLockEndorsement,      // not enough endorsements
				sAcceptPreCommitEndorsement, // reach commit agreement, jump to next step
			}).
		AddTransition(
			sAcceptPreCommitEndorsement,
			eBroadcastPreCommitEndorsement,
			cm.onBroadcastPreCommitEndorsement,
			[]fsm.State{
				sAcceptPreCommitEndorsement,
			}).
		AddTransition(
			sAcceptLockEndorsement,
			eStopReceivingLockEndorsement,
			cm.onStopReceivingLockEndorsement,
			[]fsm.State{
				sPrepare, // timeout, jump to next round
			}).
		AddTransition(
			sAcceptPreCommitEndorsement,
			eReceivePreCommitEndorsement,
			cm.onReceivePreCommitEndorsement,
			[]fsm.State{
				sAcceptPreCommitEndorsement,
				sPrepare, // reach consensus, start next epoch
			})
	// Add the backdoor transition so that we could unit test the transition from any given state
	for _, state := range consensusStates {
		b = b.AddTransition(state, BackdoorEvent, cm.handleBackdoorEvt, consensusStates)
		if state != sPrepare {
			b = b.AddTransition(state, eCalibrate, cm.calibrate, []fsm.State{sPrepare, state})
		}
	}
	m, err := b.Build()
	if err != nil {
		return nil, errors.Wrap(err, "error when building the FSM")
	}
	cm.fsm = m
	return cm, nil
}

// Start starts the fsm and get in initial state
func (m *ConsensusFSM) Start(c context.Context) error {
	m.wg.Add(1)
	go func() {
		running := true
		for running {
			select {
			case <-m.close:
				running = false
			case evt := <-m.evtq:
				if err := m.handle(evt); err != nil {
					m.ctx.Logger().Error(
						"consensus state transition fails",
						zap.Error(err),
					)
				}
			}
		}
		m.wg.Done()
	}()
	return nil
}

// Stop stops the consensus fsm
func (m *ConsensusFSM) Stop(_ context.Context) error {
	close(m.close)
	m.wg.Wait()
	return nil
}

// CurrentState returns the current state
func (m *ConsensusFSM) CurrentState() fsm.State {
	return m.fsm.CurrentState()
}

// NumPendingEvents returns the number of pending events
func (m *ConsensusFSM) NumPendingEvents() int {
	return len(m.evtq)
}

// Calibrate calibrates the state if necessary
func (m *ConsensusFSM) Calibrate(height uint64) {
	m.produce(m.ctx.NewConsensusEvent(eCalibrate, height), 0)
}

// BackToPrepare produces an ePrepare event after delay
func (m *ConsensusFSM) BackToPrepare(delay time.Duration) (fsm.State, error) {
	m.produceConsensusEvent(ePrepare, delay)

	return sPrepare, nil
}

// ProduceReceiveBlockEvent produces an eReceiveBlock event after delay
func (m *ConsensusFSM) ProduceReceiveBlockEvent(block interface{}) {
	m.produce(m.ctx.NewConsensusEvent(eReceiveBlock, block), 0)
}

// ProduceReceiveProposalEndorsementEvent produces an eReceiveProposalEndorsement event right away
func (m *ConsensusFSM) ProduceReceiveProposalEndorsementEvent(vote interface{}) {
	m.produce(m.ctx.NewConsensusEvent(eReceiveProposalEndorsement, vote), 0)
}

// ProduceReceiveLockEndorsementEvent produces an eReceiveLockEndorsement event right away
func (m *ConsensusFSM) ProduceReceiveLockEndorsementEvent(vote interface{}) {
	m.produce(m.ctx.NewConsensusEvent(eReceiveLockEndorsement, vote), 0)
}

// ProduceReceivePreCommitEndorsementEvent produces an eReceivePreCommitEndorsement event right away
func (m *ConsensusFSM) ProduceReceivePreCommitEndorsementEvent(vote interface{}) {
	m.produce(m.ctx.NewConsensusEvent(eReceivePreCommitEndorsement, vote), 0)
}

func (m *ConsensusFSM) produceConsensusEvent(et fsm.EventType, delay time.Duration) {
	m.produce(m.ctx.NewConsensusEvent(et, nil), delay)
}

// produce adds an event into the queue for the consensus FSM to process
func (m *ConsensusFSM) produce(evt *ConsensusEvent, delay time.Duration) {
	if evt == nil {
		return
	}
	consensusMtc.WithLabelValues(string(evt.Type())).Inc()
	if delay > 0 {
		m.wg.Add(1)
		go func() {
			select {
			case <-m.close:
			case <-m.clock.After(delay):
				m.evtq <- evt
			}
			m.wg.Done()
		}()
	} else {
		m.evtq <- evt
	}
}

func (m *ConsensusFSM) handle(evt *ConsensusEvent) error {
	if m.ctx.IsStaleEvent(evt) {
		m.ctx.Logger().Debug("stale event", zap.Any("event", evt.Type()))
		return nil
	}
	if m.ctx.IsFutureEvent(evt) {
		m.ctx.Logger().Debug("future event", zap.Any("event", evt.Type()))
		// TODO: find a more appropriate delay
		m.produce(evt, m.cfg.UnmatchedEventInterval)
		return nil
	}
	src := m.fsm.CurrentState()
	err := m.fsm.Handle(evt)
	switch errors.Cause(err) {
	case nil:
		m.ctx.Logger().Debug(
			"consensus state transition happens",
			zap.String("src", string(src)),
			zap.String("dst", string(m.fsm.CurrentState())),
			zap.String("evt", string(evt.Type())),
		)
	case fsm.ErrTransitionNotFound:
		if m.ctx.IsStaleUnmatchedEvent(evt) {
			return nil
		}
		m.produce(evt, m.cfg.UnmatchedEventInterval)
		m.ctx.Logger().Debug(
			"consensus state transition could find the match",
			zap.String("src", string(src)),
			zap.String("evt", string(evt.Type())),
			zap.Error(err),
		)
	default:
		return errors.Wrapf(
			err,
			"failed to handle event %s with src %s",
			string(evt.Type()),
			string(src),
		)
	}
	return nil
}

func (m *ConsensusFSM) calibrate(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sPrepare, errors.New("invalid fsm event")
	}
	height, ok := cEvt.Data().(uint64)
	if !ok {
		return sPrepare, errors.New("invalid data type")
	}
	consensusHeight := m.ctx.Height()
	if consensusHeight > height {
		return sPrepare, errors.New("ignore old calibrate event")
	}
	m.ctx.Logger().Debug(
		"Calibrate consensus context",
		zap.Uint64("consensusHeight", consensusHeight),
		zap.Uint64("height", height),
	)
	return m.BackToPrepare(0)
}

func (m *ConsensusFSM) prepare(_ fsm.Event) (fsm.State, error) {
	active, isProposer, proposal, isDelegate, locked, delay, err := m.ctx.Prepare()
	switch {
	case err != nil:
		m.ctx.Logger().Error("Error during prepare", zap.Error(err))
		fallthrough
	case !active:
		fallthrough
	case !isDelegate:
		return m.BackToPrepare(delay)
	}
	m.ctx.Logger().Info("Start a new round", zap.Duration("delay", delay))
	if delay > 0 {
		time.Sleep(delay)
	}
	// Setup timeout for waiting for proposed block
	ttl := m.cfg.AcceptBlockTTL
	m.produceConsensusEvent(eFailedToReceiveBlock, ttl)
	ttl += m.cfg.AcceptProposalEndorsementTTL
	m.produceConsensusEvent(eStopReceivingProposalEndorsement, ttl)
	ttl += m.cfg.AcceptLockEndorsementTTL
	m.produceConsensusEvent(eStopReceivingLockEndorsement, ttl)
	switch {
	case isProposer:
		m.ctx.Broadcast(proposal)
		fallthrough
	case locked:
		m.ProduceReceiveBlockEvent(proposal)
	}

	return sAcceptBlockProposal, nil
}

func (m *ConsensusFSM) onReceiveBlock(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Debug("Receive block")
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		m.ctx.Logger().Error("invalid fsm event", zap.Any("event", evt))
		return sAcceptBlockProposal, nil
	}
	if err := m.processBlock(cEvt.Data()); err != nil {
		m.ctx.Logger().Debug("Failed to generate proposal endorsement", zap.Error(err))
		return sAcceptBlockProposal, nil
	}

	return sAcceptProposalEndorsement, nil
}

func (m *ConsensusFSM) processBlock(block interface{}) error {
	en, err := m.ctx.NewProposalEndorsement(block)
	if err != nil {
		return err
	}
	m.ProduceReceiveProposalEndorsementEvent(en)
	m.ctx.Broadcast(en)
	return nil
}

func (m *ConsensusFSM) onFailedToReceiveBlock(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Warn("didn't receive the proposed block before timeout")
	if err := m.processBlock(nil); err != nil {
		m.ctx.Logger().Debug("Failed to generate proposal endorsement", zap.Error(err))
	}

	return sAcceptProposalEndorsement, nil
}

func (m *ConsensusFSM) onReceiveProposalEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sAcceptProposalEndorsement, errors.Wrap(ErrEvtCast, "failed to cast to consensus event")
	}
	lockEndorsement, err := m.ctx.NewLockEndorsement(cEvt.Data())
	if err != nil {
		m.ctx.Logger().Debug("Failed to add proposal endorsement", zap.Error(err))
		return sAcceptProposalEndorsement, nil
	}
	if lockEndorsement == nil {
		return sAcceptProposalEndorsement, nil
	}
	m.ProduceReceiveLockEndorsementEvent(lockEndorsement)
	m.ctx.Broadcast(lockEndorsement)

	return sAcceptLockEndorsement, err
}

func (m *ConsensusFSM) onStopReceivingProposalEndorsement(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Warn("Not enough proposal endorsements")

	return sAcceptLockEndorsement, nil
}

func (m *ConsensusFSM) onReceiveLockEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sAcceptLockEndorsement, errors.Wrap(ErrEvtCast, "failed to cast to consensus event")
	}
	preCommitEndorsement, err := m.ctx.NewPreCommitEndorsement(cEvt.Data())
	if err != nil {
		return sAcceptLockEndorsement, err
	}
	if preCommitEndorsement == nil {
		return sAcceptLockEndorsement, nil
	}
	m.ProduceReceivePreCommitEndorsementEvent(preCommitEndorsement)
	m.produce(m.ctx.NewConsensusEvent(eBroadcastPreCommitEndorsement, preCommitEndorsement), 0)

	return sAcceptPreCommitEndorsement, nil
}

func (m *ConsensusFSM) onBroadcastPreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sAcceptPreCommitEndorsement, errors.Wrap(ErrEvtCast, "failed to cast to consensus event")
	}
	m.ctx.Logger().Debug("broadcast pre-commit endorsement")
	m.ctx.Broadcast(cEvt.Data())
	m.produce(cEvt, m.cfg.CommitTTL)

	return sAcceptPreCommitEndorsement, nil
}

func (m *ConsensusFSM) onStopReceivingLockEndorsement(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Warn("Not enough lock endorsements")

	return m.BackToPrepare(0)
}

func (m *ConsensusFSM) onReceivePreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sAcceptPreCommitEndorsement, errors.Wrap(ErrEvtCast, "failed to cast to consensus event")
	}
	committed, err := m.ctx.Commit(cEvt.Data())
	if err != nil || !committed {
		return sAcceptPreCommitEndorsement, err
	}
	consensusMtc.WithLabelValues("ReachConsenus").Inc()
	return m.BackToPrepare(0)
}

// handleBackdoorEvt takes the dst state from the event and move the FSM into it
func (m *ConsensusFSM) handleBackdoorEvt(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		return sPrepare, errors.Wrap(ErrEvtCast, "the event is not a backdoor event")
	}
	dst, ok := cEvt.Data().(fsm.State)
	if !ok {
		return sPrepare, errors.Wrap(ErrEvtCast, "the data is not a fsm.State")
	}

	return dst, nil
}
