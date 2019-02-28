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
	"github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/pkg/log"
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

	// BackdoorEvent indicates a backdoor event type
	BackdoorEvent fsm.EventType = "E_BACKDOOR"
)

var (
	// ErrEvtCast indicates the error of casting the event
	ErrEvtCast = errors.New("error when casting the event")
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

// ProducePrepareEvent produces an ePrepare event after delay
func (m *ConsensusFSM) ProducePrepareEvent(delay time.Duration) {
	m.produceConsensusEvent(ePrepare, delay)
}

// ProduceReceiveBlockEvent produces an eReceiveBlock event after delay
func (m *ConsensusFSM) ProduceReceiveBlockEvent(block Endorsement) {
	m.produce(m.ctx.NewConsensusEvent(eReceiveBlock, block), 0)
}

// ProduceReceiveProposalEndorsementEvent produces an eReceiveProposalEndorsement event right away
func (m *ConsensusFSM) ProduceReceiveProposalEndorsementEvent(endorsement Endorsement) {
	m.produce(m.ctx.NewConsensusEvent(eReceiveProposalEndorsement, endorsement), 0)
}

// ProduceReceiveLockEndorsementEvent produces an eReceiveLockEndorsement event right away
func (m *ConsensusFSM) ProduceReceiveLockEndorsementEvent(endorsement Endorsement) {
	m.produce(m.ctx.NewConsensusEvent(eReceiveLockEndorsement, endorsement), 0)
}

// ProduceReceivePreCommitEndorsementEvent produces an eReceivePreCommitEndorsement event right away
func (m *ConsensusFSM) ProduceReceivePreCommitEndorsementEvent(endorsement Endorsement) {
	m.produce(m.ctx.NewConsensusEvent(eReceivePreCommitEndorsement, endorsement), 0)
}

func (m *ConsensusFSM) produceConsensusEvent(et fsm.EventType, delay time.Duration) {
	m.produce(m.ctx.NewConsensusEvent(et, nil), delay)
}

// produce adds an event into the queue for the consensus FSM to process
func (m *ConsensusFSM) produce(evt *ConsensusEvent, delay time.Duration) {
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
		m.ctx.Logger().Debug("stale event", zap.Any("event", evt))
		return nil
	}
	if m.ctx.IsFutureEvent(evt) {
		m.ctx.Logger().Debug("future event", zap.Any("event", evt))
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
	m.ProducePrepareEvent(0)
	return sPrepare, nil
}

func (m *ConsensusFSM) prepare(_ fsm.Event) (fsm.State, error) {
	delay, err := m.ctx.Prepare()
	switch {
	case err != nil:
		m.ctx.Logger().Error("Error during prepare", zap.Error(err))
		fallthrough
	case !m.ctx.IsDelegate():
		m.ProducePrepareEvent(delay)
		return sPrepare, nil
	}
	m.ctx.Logger().Info("Start a new round", zap.Duration("delay", delay))
	isProposer := m.ctx.IsProposer()
	var blk Endorsement
	if isProposer {
		m.ctx.Logger().Info("current node is the proposer")
		if blk, err = m.ctx.MintBlock(); err != nil || blk == nil {
			// TODO: review the return state
			m.ctx.Logger().Error("Error when minting a block", zap.Error(err))
			m.ProducePrepareEvent(0)
			return sPrepare, nil
		}
	}
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
	// TODO add timeout for commit collection
	if isProposer {
		m.ctx.Logger().Info("Broadcast init proposal.", log.Hex("blockHash", blk.Hash()))
		m.ProduceReceiveBlockEvent(blk)
		m.ctx.BroadcastBlockProposal(blk)
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
	block, ok := cEvt.Data().(Endorsement)
	if !ok {
		m.ctx.Logger().Error("invalid data type", zap.Any("data", cEvt.Data()))
		return sAcceptBlockProposal, nil
	}
	en, err := m.ctx.NewProposalEndorsement(block)
	if err != nil {
		m.ctx.Logger().Debug("Failed to generate proposal endorsement", zap.Error(err))
		return sAcceptBlockProposal, nil
	}
	m.ProduceReceiveProposalEndorsementEvent(en)
	m.ctx.BroadcastEndorsement(en)

	return sAcceptProposalEndorsement, nil
}

func (m *ConsensusFSM) onFailedToReceiveBlock(evt fsm.Event) (fsm.State, error) {
	m.ctx.Logger().Warn("didn't receive the proposed block before timeout")
	/*
		TODO: Produce an endorsement of nil
		en, err := m.ctx.NewProposalEndorsement(nil)
		if err == nil {
			m.ProduceReceiveProposalEndorsementEvent(en)
			m.ctx.BroadcastEndorsement(en)
		} else {
			m.ctx.Logger().Debug("Failed to generate proposal endorsement", zap.Error(err))
		}
		return sAcceptProposalEndorsement, err
	*/
	return sAcceptProposalEndorsement, nil
}

func (m *ConsensusFSM) onReceiveProposalEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		m.ctx.Logger().Error("failed to cast to consensus event", zap.Any("event", evt))
		return sAcceptProposalEndorsement, nil
	}
	en, ok := cEvt.Data().(Endorsement)
	if !ok {
		m.ctx.Logger().Error("invalid data type", zap.Any("data", cEvt.Data()))
		return sAcceptProposalEndorsement, nil
	}
	err := m.ctx.AddProposalEndorsement(en)
	if err != nil || !m.ctx.IsLocked() {
		m.ctx.Logger().Debug("Failed to add proposal endorsement", zap.Error(err))
		return sAcceptProposalEndorsement, nil
	}
	m.ctx.LoggerWithStats().Debug("Locked")
	lockEndorsement, err := m.ctx.NewLockEndorsement()
	if err != nil {
		// TODO: review return state
		m.ctx.Logger().Error("error when producing lock endorsement", zap.Error(err))
		m.ProducePrepareEvent(0)
		return sPrepare, nil
	}
	m.ProduceReceiveLockEndorsementEvent(lockEndorsement)
	m.ctx.BroadcastEndorsement(lockEndorsement)

	return sAcceptLockEndorsement, nil
}

func (m *ConsensusFSM) onStopReceivingProposalEndorsement(evt fsm.Event) (fsm.State, error) {
	m.ctx.LoggerWithStats().Warn("Not enough proposal endorsements")

	return sAcceptLockEndorsement, nil
}

func (m *ConsensusFSM) onReceiveLockEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		m.ctx.Logger().Error("failed to cast to consensus event", zap.Any("event", evt))
		return sAcceptLockEndorsement, nil
	}
	en, ok := cEvt.Data().(Endorsement)
	if !ok {
		m.ctx.Logger().Error("invalid data type", zap.Any("data", cEvt.Data()))
		return sAcceptLockEndorsement, nil
	}
	err := m.ctx.AddLockEndorsement(en)
	switch {
	case err != nil:
		m.ctx.Logger().Error("failed to add lock endorsement", zap.Error(err))
		fallthrough
	case !m.ctx.ReadyToPreCommit():
		return sAcceptLockEndorsement, nil
	}
	m.ctx.LoggerWithStats().Debug("Ready to pre-commit")
	preCommitEndorsement, err := m.ctx.NewPreCommitEndorsement()
	if err != nil {
		// TODO: Review return state
		m.ctx.Logger().Error("error when producing pre-commit endorsement", zap.Error(err))
		m.ProducePrepareEvent(0)

		return sPrepare, nil
	}
	m.ProduceReceivePreCommitEndorsementEvent(preCommitEndorsement)
	m.ctx.BroadcastEndorsement(preCommitEndorsement)

	return sAcceptPreCommitEndorsement, nil
}

func (m *ConsensusFSM) onStopReceivingLockEndorsement(evt fsm.Event) (fsm.State, error) {
	m.ctx.LoggerWithStats().Warn("Not enough lock endorsements")

	m.ProducePrepareEvent(0)

	return sPrepare, nil
}

func (m *ConsensusFSM) onReceivePreCommitEndorsement(evt fsm.Event) (fsm.State, error) {
	cEvt, ok := evt.(*ConsensusEvent)
	if !ok {
		m.ctx.Logger().Error("failed to cast to consensus event", zap.Any("event", evt))
		return sAcceptPreCommitEndorsement, nil
	}
	en, ok := cEvt.Data().(Endorsement)
	if !ok {
		m.ctx.Logger().Error("invalid data type", zap.Any("data", cEvt.Data()))
		return sAcceptPreCommitEndorsement, nil
	}
	if err := m.ctx.AddPreCommitEndorsement(en); err != nil {
		m.ctx.Logger().Error("error when adding pre-commit endorsement", zap.Error(err))
		return sAcceptPreCommitEndorsement, nil
	}
	if !m.ctx.ReadyToCommit() {
		return sAcceptPreCommitEndorsement, nil
	}
	m.ctx.LoggerWithStats().Debug("Ready to commit")

	consensusMtc.WithLabelValues("ReachConsenus").Inc()
	m.ctx.OnConsensusReached()
	m.ProducePrepareEvent(0)

	return sPrepare, nil
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
