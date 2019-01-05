// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"bytes"
	"context"
	"encoding/hex"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	fsm "github.com/zjshen14/go-fsm"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
)

/**
 * TODO: For the nodes received correct proposal, add proposer's proposal endorse without signature, which could be replaced with real signature
 */
var (
	consensusMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_consensus",
			Help: "Consensus result",
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
	sBlockPropose          fsm.State = "S_BLOCK_PROPOSE"
	sAcceptPropose         fsm.State = "S_ACCEPT_PROPOSE"
	sAcceptProposalEndorse fsm.State = "S_ACCEPT_PROPOSAL_ENDORSE"
	sAcceptLockEndorse     fsm.State = "S_ACCEPT_LOCK_ENDORSE"
	sAcceptCommitEndorse   fsm.State = "S_ACCEPT_COMMIT_ENDORSE"

	// consensusEvt event types
	ePrepare                      fsm.EventType = "E_ROLL_DELEGATES"
	eInitBlockPropose             fsm.EventType = "E_INIT_BLOCK_PROPOSE"
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
		sBlockPropose,
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
			sBlockPropose,
			sAcceptPropose,
			sAcceptProposalEndorse,
			sAcceptLockEndorse,
			sAcceptCommitEndorse,
		).
		AddTransition(sPrepare, ePrepare, cm.prepare, []fsm.State{sPrepare, sBlockPropose}).
		AddTransition(sBlockPropose, eInitBlockPropose, cm.handleInitBlockProposeEvt, []fsm.State{sAcceptPropose}).
		AddTransition(
			sBlockPropose,
			eFailedToReceiveBlock,
			cm.handleProposeBlockTimeout,
			[]fsm.State{sAcceptProposalEndorse},
		).
		AddTransition(
			sAcceptPropose,
			eReceiveBlock,
			cm.handleProposeBlockEvt,
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
			cm.handleEndorseProposalEvt,
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
			cm.handleEndorseLockEvt,
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
				timeoutEvt, ok := evt.(*timeoutEvt)
				if ok && timeoutEvt.timestamp().Before(m.ctx.round.timestamp) {
					log.L().Debug("timeoutEvt is stale")
					continue
				}
				src := m.fsm.CurrentState()
				if err := m.fsm.Handle(evt); err != nil {
					if errors.Cause(err) == fsm.ErrTransitionNotFound {
						if m.ctx.clock.Now().Sub(evt.timestamp()) <= m.ctx.cfg.UnmatchedEventTTL {
							m.produce(evt, m.ctx.cfg.UnmatchedEventInterval)
							log.L().Debug("consensusEvt state transition could find the match",
								zap.String("src", string(src)),
								zap.String("evt", string(evt.Type())),
								zap.Error(err))
						}
					} else {
						log.L().Error("consensusEvt state transition fails",
							zap.String("src", string(src)),
							zap.String("evt", string(evt.Type())),
							zap.Error(err))
					}
				} else {
					dst := m.fsm.CurrentState()
					log.L().Debug("consensusEvt state transition happens",
						zap.String("src", string(src)),
						zap.String("dst", string(dst)),
						zap.String("evt", string(evt.Type())))
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
	delay, isDelegate, err := m.ctx.PrepareNextRound()
	if !isDelegate {
		m.produce(m.newCEvt(ePrepare), delay)
		return sPrepare, nil
	}
	if err != nil {
		m.produce(m.newCEvt(ePrepare), delay)
		return sPrepare, errors.Wrap(err, "error when prepare next round")
	}
	m.produce(m.newCEvt(eInitBlockPropose), delay)
	// Setup timeout for waiting for proposed block
	ttl := m.ctx.cfg.AcceptProposeTTL + delay
	m.produce(m.newTimeoutEvt(eFailedToReceiveBlock), ttl)
	ttl += m.ctx.cfg.AcceptProposalEndorseTTL
	m.produce(m.newTimeoutEvt(eNotEnoughProposalEndorsement), ttl)
	ttl += m.ctx.cfg.AcceptCommitEndorseTTL
	m.produce(m.newTimeoutEvt(eNotEnoughLockEndorsement), ttl)
	// TODO add timeout for commit collection

	return sBlockPropose, nil
}

func (m *cFSM) handleInitBlockProposeEvt(evt fsm.Event) (fsm.State, error) {
	if !m.ctx.IsProposer() {
		return sAcceptPropose, nil
	}
	blk, err := m.ctx.MintBlock()
	if err != nil {
		return sPrepare, errors.Wrap(err, "error when minting a block")
	}
	proposeBlkEvt := m.newProposeBlkEvt(blk)
	proposeBlkEvtProto := proposeBlkEvt.toProtoMsg()
	// Notify itself
	h := blk.Hash()
	log.L().Info("Broadcast init proposal.", zap.String("blockHash", hex.EncodeToString(h)))
	m.produce(proposeBlkEvt, 0)
	// Notify other delegates
	if err := m.ctx.Broadcast(proposeBlkEvtProto); err != nil {
		log.L().Error("Error when broadcasting proposeBlkEvt.", zap.Error(err))
	}
	return sAcceptPropose, nil
}

func (m *cFSM) handleProposeBlockEvt(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eReceiveBlock {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	proposeBlkEvt, ok := evt.(*proposeBlkEvt)
	if !ok {
		return sPrepare, errors.Wrap(ErrEvtCast, "the event is not a proposeBlkEvt")
	}
	if err := m.ctx.ProcessProposeBlock(proposeBlkEvt.block); err != nil {
		return sAcceptPropose, err
	}
	m.broadcastConsensusVote(proposeBlkEvt.block.Hash(), endorsement.PROPOSAL)

	return sAcceptProposalEndorse, nil
}

func (m *cFSM) handleProposeBlockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eFailedToReceiveBlock {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	log.L().Warn("Didn't receive the proposed block before timeout.",
		zap.String("proposer", m.ctx.round.proposer),
		zap.Uint64("height", m.ctx.round.height),
		zap.Uint32("round", m.ctx.round.number))

	return sAcceptProposalEndorse, nil
}

func (m *cFSM) isDelegateEndorsement(endorser string) bool {
	for _, delegate := range m.ctx.epoch.delegates {
		if delegate == endorser {
			return true
		}
	}
	return false
}

func (m *cFSM) validateEndorse(
	en *endorsement.Endorsement,
	expectedConsensusTopics map[endorsement.ConsensusVoteTopic]bool,
) bool {
	if !m.isDelegateEndorsement(en.Endorser()) {
		log.L().Error("Error when validating the endorser's delegation.",
			zap.String("endorser", en.Endorser()))
		return false
	}
	vote := en.ConsensusVote()
	if _, ok := expectedConsensusTopics[vote.Topic]; !ok {
		log.L().Error("Error when validating the endorse topic.",
			zap.Any("expectedConsensusTopics", expectedConsensusTopics),
			zap.Uint8("consensusTopic", uint8(vote.Topic)))
		return false
	}
	if vote.Height != m.ctx.round.height {
		log.L().Error("Error when validating the endorse height.",
			zap.Uint64("height", vote.Height),
			zap.Uint64("expectedHeight", m.ctx.round.height))
		return false
	}

	return true
}

func (m *cFSM) isProposedBlock(hash []byte) bool {
	if m.ctx.round.block == nil {
		return false
	}
	blkHash := m.ctx.round.block.Hash()

	return bytes.Equal(hash[:], blkHash[:])
}

func (m *cFSM) processEndorseEvent(
	evt fsm.Event,
	expectedEventType fsm.EventType,
	expectedConsensusTopics map[endorsement.ConsensusVoteTopic]bool,
) (*endorsement.Set, error) {
	if evt.Type() != expectedEventType {
		return nil, errors.Wrapf(ErrEvtType, "invalid endorsement event type %s", evt.Type())
	}
	endorseEvt, ok := evt.(*endorseEvt)
	if !ok {
		return nil, errors.Wrap(ErrEvtCast, "the event is not an endorseEvt")
	}
	endorse := endorseEvt.endorse
	vote := endorse.ConsensusVote()
	if !m.isProposedBlock(vote.BlkHash[:]) {
		return nil, errors.New("the endorsed block was not the proposed block")
	}
	if !m.validateEndorse(endorse, expectedConsensusTopics) {
		return nil, errors.New("invalid endorsement")
	}

	return m.addEndorsement(endorse, expectedConsensusTopics)
}

func (m *cFSM) addEndorsement(
	en *endorsement.Endorsement,
	expectedConsensusTopics map[endorsement.ConsensusVoteTopic]bool,
) (*endorsement.Set, error) {
	blkHash := en.ConsensusVote().BlkHash
	blkHashHex := hex.EncodeToString(blkHash)
	endorsementSet, ok := m.ctx.round.endorsementSets[blkHashHex]
	if !ok {
		endorsementSet = endorsement.NewSet(blkHash)
		m.ctx.round.endorsementSets[blkHashHex] = endorsementSet
	}
	if err := endorsementSet.AddEndorsement(en); err != nil {
		return nil, err
	}
	validNum := endorsementSet.NumOfValidEndorsements(
		expectedConsensusTopics,
		m.ctx.epoch.delegates,
	)
	numDelegates := len(m.ctx.epoch.delegates)
	if numDelegates >= 4 && validNum > numDelegates*2/3 || numDelegates < 4 && validNum >= numDelegates {
		return endorsementSet, nil
	}

	return nil, nil
}

func (m *cFSM) broadcastConsensusVote(
	blkHash []byte,
	topic endorsement.ConsensusVoteTopic,
) {
	cEvt := m.ctx.newEndorseEvt(blkHash, topic)
	cEvtProto := cEvt.toProtoMsg()
	// Notify itself
	m.produce(cEvt, 0)
	// Notify other delegates
	if err := m.ctx.Broadcast(cEvtProto); err != nil {
		log.L().Error("Error when broadcasting commitEvtProto.", zap.Error(err))
	}
}

func (m *cFSM) handleEndorseProposalEvt(evt fsm.Event) (fsm.State, error) {
	endorsementSet, err := m.processEndorseEvent(
		evt,
		eReceiveProposalEndorsement,
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.PROPOSAL: true,
			endorsement.COMMIT:   true, // commit endorse is counted as one proposal endorse
		},
	)
	if errors.Cause(err) == ErrEvtCast || errors.Cause(err) == ErrEvtType {
		return sPrepare, err
	}
	if err != nil || endorsementSet == nil {
		return sAcceptProposalEndorse, err
	}
	// Gather enough proposal endorsements
	m.ctx.round.proofOfLock = endorsementSet
	m.broadcastConsensusVote(endorsementSet.BlockHash(), endorsement.LOCK)

	return sAcceptLockEndorse, nil
}

func (m *cFSM) handleEndorseProposalTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eNotEnoughProposalEndorsement {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	numProposalEndorsements := 0
	if m.ctx.round.block != nil {
		blkHashHex := hex.EncodeToString(m.ctx.round.block.Hash())
		endorsementSet := m.ctx.round.endorsementSets[blkHashHex]
		if endorsementSet != nil {
			numProposalEndorsements = endorsementSet.NumOfValidEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.PROPOSAL: true,
					endorsement.COMMIT:   false,
				},
				m.ctx.epoch.delegates,
			)
		}
	}

	log.L().Warn("Didn't collect enough proposal endorses before timeout.",
		zap.Uint64("height", m.ctx.round.height),
		zap.Int("numProposalEndorsements", numProposalEndorsements))

	return sAcceptLockEndorse, nil
}

func (m *cFSM) handleEndorseLockEvt(evt fsm.Event) (fsm.State, error) {
	endorsementSet, err := m.processEndorseEvent(
		evt,
		eReceiveLockEndorsement,
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.LOCK:   true,
			endorsement.COMMIT: true, // commit endorse is counted as one lock endorse
		},
	)
	if errors.Cause(err) == ErrEvtCast || errors.Cause(err) == ErrEvtType {
		return sPrepare, err
	}
	if err != nil || endorsementSet == nil {
		// Wait for more lock votes to come
		return sAcceptLockEndorse, err
	}
	m.broadcastConsensusVote(endorsementSet.BlockHash(), endorsement.COMMIT)

	return sAcceptCommitEndorse, nil
}

func (m *cFSM) handleEndorseLockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eNotEnoughLockEndorsement {
		return sPrepare, errors.Errorf("invalid event type %s", evt.Type())
	}
	consensusMtc.WithLabelValues("false").Inc()
	numCommitEndorsements := 0
	if m.ctx.round.block != nil {
		blkHashHex := hex.EncodeToString(m.ctx.round.block.Hash())
		endorsementSet := m.ctx.round.endorsementSets[blkHashHex]
		if endorsementSet != nil {
			numCommitEndorsements = endorsementSet.NumOfValidEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.LOCK:   true,
					endorsement.COMMIT: true,
				},
				m.ctx.epoch.delegates,
			)
		}
	}
	log.L().Warn("Consensus didn't reach: didn't collect enough commit endorse before timeout.",
		zap.Uint64("height", m.ctx.round.height),
		zap.Int("numCommitEndorsements", numCommitEndorsements))

	m.produce(m.newCEvt(ePrepare), 0)
	return sPrepare, nil
}

func (m *cFSM) handleEndorseCommitEvt(evt fsm.Event) (fsm.State, error) {
	consensusMtc.WithLabelValues("true").Inc()
	log.L().Info("Consensus reached.", zap.Uint64("block", m.ctx.round.height))
	if err := m.ctx.OnConsensusReached(); err != nil {
		log.L().Error("Failed to commit block on consensus reached.", zap.Error(err))
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
