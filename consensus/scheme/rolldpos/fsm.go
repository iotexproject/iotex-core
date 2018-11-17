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
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
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
	sEpochStart            fsm.State = "S_EPOCH_START"
	sDKGGeneration         fsm.State = "S_DKG_GENERATION"
	sRoundStart            fsm.State = "S_ROUND_START"
	sBlockPropose          fsm.State = "S_BLOCK_PROPOSE"
	sAcceptPropose         fsm.State = "S_ACCEPT_PROPOSE"
	sAcceptProposalEndorse fsm.State = "S_ACCEPT_PROPOSAL_ENDORSE"
	sAcceptLockEndorse     fsm.State = "S_ACCEPT_LOCK_ENDORSE"
	sAcceptCommitEndorse   fsm.State = "S_ACCEPT_COMMIT_ENDORSE"

	// consensusEvt event types
	eRollDelegates          fsm.EventType = "E_ROLL_DELEGATES"
	eGenerateDKG            fsm.EventType = "E_GENERATE_DKG"
	eStartRound             fsm.EventType = "E_START_ROUND"
	eInitBlockPropose       fsm.EventType = "E_INIT_BLOCK_PROPOSE"
	eProposeBlock           fsm.EventType = "E_PROPOSE_BLOCK"
	eProposeBlockTimeout    fsm.EventType = "E_PROPOSE_BLOCK_TIMEOUT"
	eEndorseProposal        fsm.EventType = "E_ENDORSE_PROPOSAL"
	eEndorseProposalTimeout fsm.EventType = "E_ENDORSE_PROPOSAL_TIMEOUT"
	eEndorseLock            fsm.EventType = "E_ENDORSE_LOCK"
	eEndorseLockTimeout     fsm.EventType = "E_ENDORSE_LOCK_TIMEOUT"
	eEndorseCommit          fsm.EventType = "E_ENDORSE_COMMIT"
	eFinishEpoch            fsm.EventType = "E_FINISH_EPOCH"

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
		sEpochStart,
		sDKGGeneration,
		sRoundStart,
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
		AddInitialState(sEpochStart).
		AddStates(
			sDKGGeneration,
			sRoundStart,
			sBlockPropose,
			sAcceptPropose,
			sAcceptProposalEndorse,
			sAcceptLockEndorse,
			sAcceptCommitEndorse,
		).
		AddTransition(sEpochStart, eRollDelegates, cm.handleRollDelegatesEvt, []fsm.State{sEpochStart, sDKGGeneration}).
		AddTransition(sDKGGeneration, eGenerateDKG, cm.handleGenerateDKGEvt, []fsm.State{sRoundStart}).
		AddTransition(sRoundStart, eStartRound, cm.handleStartRoundEvt, []fsm.State{sBlockPropose}).
		AddTransition(sRoundStart, eFinishEpoch, cm.handleFinishEpochEvt, []fsm.State{sEpochStart, sRoundStart}).
		AddTransition(sBlockPropose, eInitBlockPropose, cm.handleInitBlockProposeEvt, []fsm.State{sAcceptPropose}).
		AddTransition(sBlockPropose, eProposeBlockTimeout, cm.handleProposeBlockTimeout, []fsm.State{sAcceptProposalEndorse}).
		AddTransition(
			sAcceptPropose,
			eProposeBlock,
			cm.handleProposeBlockEvt,
			[]fsm.State{
				sAcceptPropose,         // proposed block invalid
				sAcceptProposalEndorse, // receive valid block, jump to next step
			}).
		AddTransition(
			sAcceptPropose,
			eProposeBlockTimeout,
			cm.handleProposeBlockTimeout,
			[]fsm.State{
				sAcceptProposalEndorse, // no valid block, jump to next step
			}).
		AddTransition(
			sAcceptProposalEndorse,
			eEndorseProposal,
			cm.handleEndorseProposalEvt,
			[]fsm.State{
				sAcceptProposalEndorse, // haven't reach agreement yet
				sAcceptLockEndorse,     // reach agreement
			}).
		AddTransition(
			sAcceptProposalEndorse,
			eEndorseProposalTimeout,
			cm.handleEndorseProposalTimeout,
			[]fsm.State{
				sAcceptLockEndorse, // timeout, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorse,
			eEndorseLock,
			cm.handleEndorseLockEvt,
			[]fsm.State{
				sAcceptLockEndorse,   // haven't reach agreement yet
				sAcceptCommitEndorse, // reach commit agreement, jump to next step
			}).
		AddTransition(
			sAcceptLockEndorse,
			eEndorseLockTimeout,
			cm.handleEndorseLockTimeout,
			[]fsm.State{
				sAcceptCommitEndorse, // reach commit agreement, jump to next step
				sRoundStart,          // timeout, jump to next round
			}).
		AddTransition(
			sAcceptCommitEndorse,
			eEndorseCommit,
			cm.handleEndorseCommitEvt,
			[]fsm.State{
				sRoundStart,
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
					logger.Debug().Msg("timeoutEvt is stale")
					continue
				}
				chainHeight := m.ctx.chain.TipHeight()
				eventHeight := evt.height()
				if _, ok := evt.(*proposeBlkEvt); ok && eventHeight <= chainHeight {
					logger.Debug().Uint64("event height", eventHeight).Uint64("chain height", chainHeight).Msg("skip old proposal")
					continue
				}
				if eEvt, ok := evt.(*endorseEvt); ok && eventHeight <= chainHeight && eEvt.endorse.Endorser() != m.ctx.addr.RawAddress {
					logger.Debug().Uint64("event height", eventHeight).Uint64("chain height", chainHeight).Msg("skip old endorsement")
					continue
				}
				src := m.fsm.CurrentState()
				if err := m.fsm.Handle(evt); err != nil {
					if errors.Cause(err) == fsm.ErrTransitionNotFound {
						if m.ctx.clock.Now().Sub(evt.timestamp()) <= m.ctx.cfg.UnmatchedEventTTL {
							m.produce(evt, m.ctx.cfg.UnmatchedEventInterval)
							logger.Debug().
								Str("src", string(src)).
								Str("evt", string(evt.Type())).
								Err(err).
								Msg("consensusEvt state transition could find the match")
						}
					} else {
						logger.Error().
							Str("src", string(src)).
							Str("evt", string(evt.Type())).
							Err(err).
							Msg("consensusEvt state transition fails")
					}
				} else {
					dst := m.fsm.CurrentState()
					logger.Debug().
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

func (m *cFSM) handleRollDelegatesEvt(_ fsm.Event) (fsm.State, error) {
	epochNum, epochHeight, err := m.ctx.calcEpochNumAndHeight()
	if err != nil {
		// Even if error happens, we still need to schedule next check of delegate to tolerate transit error
		m.produce(m.newCEvt(eRollDelegates), m.ctx.cfg.DelegateInterval)
		return sEpochStart, errors.Wrap(
			err,
			"error when determining the epoch ordinal number and start height offset",
		)
	}
	// Update CryptoSort seed
	// TODO: Consider persist the most recent seed
	if !m.ctx.cfg.EnableDKG {
		m.ctx.epoch.seed = crypto.CryptoSeed
	} else if m.ctx.epoch.seed, err = m.ctx.updateSeed(); err != nil {
		logger.Error().Err(err).Msg("Failed to generate new seed from last epoch")
	}
	delegates, err := m.ctx.rollingDelegates(epochNum)
	if err != nil {
		// Even if error happens, we still need to schedule next check of delegate to tolerate transit error
		m.produce(m.newCEvt(eRollDelegates), m.ctx.cfg.DelegateInterval)
		return sEpochStart, errors.Wrap(
			err,
			"error when determining if the node will participate into next epoch",
		)
	}
	// If the current node is the delegate, move to the next state
	if m.isDelegate(delegates) {
		// The epochStart start height is going to be the next block to generate
		m.ctx.epoch.num = epochNum
		m.ctx.epoch.height = epochHeight
		m.ctx.epoch.delegates = delegates
		m.ctx.epoch.numSubEpochs = m.ctx.getNumSubEpochs()
		m.ctx.epoch.subEpochNum = uint64(0)
		m.ctx.epoch.committedSecrets = make(map[string][]uint32)

		// Trigger the event to generate DKG
		m.produce(m.newCEvt(eGenerateDKG), 0)

		logger.Info().
			Uint64("epoch", epochNum).
			Msg("current node is the delegate")
		return sDKGGeneration, nil
	}
	// Else, stay at the current state and check again later
	m.produce(m.newCEvt(eRollDelegates), m.ctx.cfg.DelegateInterval)
	logger.Info().
		Uint64("epoch", epochNum).
		Msg("current node is not the delegate")
	return sEpochStart, nil
}

func (m *cFSM) handleGenerateDKGEvt(_ fsm.Event) (fsm.State, error) {
	if m.ctx.shouldHandleDKG() {
		// TODO: numDelegates will be configurable later on
		if len(m.ctx.epoch.delegates) != 21 {
			logger.Panic().Msg("Number of delegates is incorrect for DKG generation")
		}
		secrets, witness, err := m.ctx.generateDKGSecrets()
		if err != nil {
			return sEpochStart, err
		}
		m.ctx.epoch.secrets = secrets
		m.ctx.epoch.witness = witness
	}
	if err := m.produceStartRoundEvt(); err != nil {
		return sEpochStart, errors.Wrapf(err, "error when producing %s", eStartRound)
	}
	return sRoundStart, nil
}

func (m *cFSM) handleStartRoundEvt(_ fsm.Event) (fsm.State, error) {
	subEpochNum, err := m.ctx.calcSubEpochNum()
	if err != nil {
		return sEpochStart, errors.Wrap(
			err,
			"error when determining the sub-epoch ordinal number",
		)
	}
	m.ctx.epoch.subEpochNum = subEpochNum

	proposer, height, round, err := m.ctx.rotatedProposer()
	if err != nil {
		logger.Error().
			Err(err).
			Msg("error when getting the proposer")
		return sEpochStart, err
	}
	if m.ctx.round.height != height {
		m.ctx.round = roundCtx{
			height:          height,
			endorsementSets: make(map[hash.Hash32B]*endorsement.Set),
		}
	}
	m.ctx.round.number = round
	m.ctx.round.proposer = proposer
	m.ctx.round.timestamp = m.ctx.clock.Now()

	m.produce(m.newCEvt(eInitBlockPropose), 0)
	// Setup timeout for waiting for proposed block
	ttl := m.ctx.cfg.AcceptProposeTTL
	m.produce(m.newTimeoutEvt(eProposeBlockTimeout), ttl)
	ttl += m.ctx.cfg.AcceptProposalEndorseTTL
	m.produce(m.newTimeoutEvt(eEndorseProposalTimeout), ttl)
	ttl += m.ctx.cfg.AcceptCommitEndorseTTL
	m.produce(m.newTimeoutEvt(eEndorseLockTimeout), ttl)

	return sBlockPropose, nil
}

func (m *cFSM) handleInitBlockProposeEvt(evt fsm.Event) (fsm.State, error) {
	log := logger.Info().
		Str("proposer", m.ctx.round.proposer).
		Uint64("height", m.ctx.round.height).
		Uint32("round", m.ctx.round.number)
	if m.ctx.round.proposer != m.ctx.addr.RawAddress {
		log.Msg("current node is not the proposer")
		return sAcceptPropose, nil
	}
	log.Msg("current node is the proposer")
	blk := m.ctx.round.block
	if blk == nil {
		var err error
		blk, err = m.ctx.mintBlock()
		if err != nil {
			return sEpochStart, errors.Wrap(err, "error when minting a block")
		}
	}
	proposeBlkEvt := m.newProposeBlkEvt(blk)
	proposeBlkEvtProto := proposeBlkEvt.toProtoMsg()
	// Notify itself
	h := blk.HashBlock()
	logger.Info().Str("block", hex.EncodeToString(h[:])).Msgf("Broadcast init proposal %+v", blk)
	m.produce(proposeBlkEvt, 0)
	// Notify other delegates
	if err := m.ctx.p2p.Broadcast(m.ctx.chain.ChainID(), proposeBlkEvtProto); err != nil {
		logger.Error().
			Err(err).
			Msg("error when broadcasting proposeBlkEvt")
	}
	return sAcceptPropose, nil
}

func (m *cFSM) validateProposeBlock(blk *blockchain.Block, expectedProposer string) bool {
	blkHash := blk.HashBlock()
	errorLog := logger.Error().
		Uint64("expectedHeight", m.ctx.round.height).
		Str("expectedProposer", expectedProposer).
		Str("hash", hex.EncodeToString(blkHash[:]))
	if blk.Height() != m.ctx.round.height {
		errorLog.Uint64("blockHeight", blk.Height()).
			Msg("error when validating the block height")
		return false
	}
	producer := blk.ProducerAddress()

	if producer == "" || producer != expectedProposer {
		errorLog.Str("proposer", producer).
			Msg("error when validating the block proposer")
		return false
	}
	if !blk.VerifySignature() {
		errorLog.Msg("error when validating the block signature")
		return false
	}
	if producer == m.ctx.addr.RawAddress {
		// If the block is self proposed, skip validation
		return true
	}
	containCoinbase := true
	if m.ctx.cfg.EnableDKG {
		if m.ctx.shouldHandleDKG() {
			containCoinbase = false
		} else if err := verifyDKGSignature(blk, m.ctx.epoch.seed); err != nil {
			// Verify dkg signature failed
			errorLog.Err(err).Msg("Failed to verify the DKG signature")
			return false
		}

	}
	if err := m.ctx.chain.ValidateBlock(blk, containCoinbase); err != nil {
		errorLog.Err(err).Msg("error when validating the proposed block")
		return false
	}

	return true
}

func (m *cFSM) handleProposeBlockEvt(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eProposeBlock {
		return sEpochStart, errors.Errorf("invalid event type %s", evt.Type())
	}
	proposeBlkEvt, ok := evt.(*proposeBlkEvt)
	if !ok {
		return sEpochStart, errors.Wrap(ErrEvtCast, "the event is not a proposeBlkEvt")
	}
	if !m.validateProposeBlock(proposeBlkEvt.block, m.ctx.round.proposer) {
		return sAcceptPropose, nil
	}
	m.ctx.round.block = proposeBlkEvt.block
	m.broadcastConsensusVote(m.ctx.round.block.HashBlock(), endorsement.PROPOSAL)

	return sAcceptProposalEndorse, nil
}

func (m *cFSM) handleProposeBlockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eProposeBlockTimeout {
		return sEpochStart, errors.Errorf("invalid event type %s", evt.Type())
	}
	logger.Warn().
		Str("proposer", m.ctx.round.proposer).
		Uint64("height", m.ctx.round.height).
		Uint32("round", m.ctx.round.number).
		Msg("didn't receive the proposed block before timeout")

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
		logger.Error().
			Str("endorser", en.Endorser()).
			Msg("error when validating the endorser's delegation")
		return false
	}
	vote := en.ConsensusVote()
	if _, ok := expectedConsensusTopics[vote.Topic]; !ok {
		logger.Error().
			Interface("expectedConsensusTopics", expectedConsensusTopics).
			Uint8("consensusTopic", uint8(vote.Topic)).
			Msg("error when validating the endorse topic")
		return false
	}
	if vote.Height != m.ctx.round.height {
		logger.Error().
			Uint64("height", vote.Height).
			Uint64("expectedHeight", m.ctx.round.height).
			Msg("error when validating the endorse height")
		return false
	}

	return true
}

func (m *cFSM) isProposedBlock(hash []byte) bool {
	if m.ctx.round.block == nil {
		return false
	}
	blkHash := m.ctx.round.block.HashBlock()

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
	endorsementSet, ok := m.ctx.round.endorsementSets[blkHash]
	if !ok {
		endorsementSet = endorsement.NewSet(blkHash)
		m.ctx.round.endorsementSets[blkHash] = endorsementSet
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
	blkHash hash.Hash32B,
	topic endorsement.ConsensusVoteTopic,
) {
	cEvt := m.newEndorseEvt(blkHash, topic)
	cEvtProto := cEvt.toProtoMsg()
	// Notify itself
	m.produce(cEvt, 0)
	// Notify other delegates
	if err := m.ctx.p2p.Broadcast(m.ctx.chain.ChainID(), cEvtProto); err != nil {
		logger.Error().
			Err(err).
			Msg("error when broadcasting commitEvtProto")
	}
}

func (m *cFSM) handleEndorseProposalEvt(evt fsm.Event) (fsm.State, error) {
	endorsementSet, err := m.processEndorseEvent(
		evt,
		eEndorseProposal,
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.PROPOSAL: true,
			endorsement.COMMIT:   true, // commit endorse is counted as one proposal endorse
		},
	)
	if errors.Cause(err) == ErrEvtCast || errors.Cause(err) == ErrEvtType {
		return sEpochStart, err
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
	if evt.Type() != eEndorseProposalTimeout {
		return sEpochStart, errors.Errorf("invalid event type %s", evt.Type())
	}
	logger.Warn().
		Uint64("height", m.ctx.round.height).
		Msg("didn't collect enough proposal endorses before timeout")

	return sAcceptLockEndorse, nil
}

func (m *cFSM) handleEndorseLockEvt(evt fsm.Event) (fsm.State, error) {
	endorsementSet, err := m.processEndorseEvent(
		evt,
		eEndorseLock,
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.LOCK:   true,
			endorsement.COMMIT: true, // commit endorse is counted as one lock endorse
		},
	)
	if errors.Cause(err) == ErrEvtCast || errors.Cause(err) == ErrEvtType {
		return sEpochStart, err
	}
	if err != nil || endorsementSet == nil {
		// Wait for more lock votes to come
		return sAcceptLockEndorse, err
	}
	m.broadcastConsensusVote(endorsementSet.BlockHash(), endorsement.COMMIT)

	return sAcceptCommitEndorse, nil
}

func (m *cFSM) handleEndorseLockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eEndorseLockTimeout {
		return sEpochStart, errors.Errorf("invalid event type %s", evt.Type())
	}
	logger.Warn().
		Uint64("height", m.ctx.round.height).
		Msg("didn't collect enough commit endorse before timeout")

	m.produce(m.newCEvt(eFinishEpoch), 0)
	return sRoundStart, nil
}
func (m *cFSM) handleEndorseCommitEvt(evt fsm.Event) (fsm.State, error) {
	pendingBlock := m.ctx.round.block
	logger.Info().
		Uint64("block", m.ctx.round.height).
		Msg("consensus reached")
	consensusMtc.WithLabelValues("true").Inc()
	// If the pending block is a secret block, record the secret share generated by producer
	if m.ctx.shouldHandleDKG() {
		for _, secretProposal := range pendingBlock.SecretProposals {
			if secretProposal.DstAddr() == m.ctx.addr.RawAddress {
				m.ctx.epoch.committedSecrets[secretProposal.SrcAddr()] = secretProposal.Secret()
				break
			}
		}
	}
	// Commit and broadcast the pending block
	if err := m.ctx.chain.CommitBlock(pendingBlock); err != nil {
		logger.Error().
			Err(err).
			Uint64("block", pendingBlock.Height()).
			Msg("error when committing a block")
	}
	// Remove transfers in this block from ActPool and reset ActPool state
	m.ctx.actPool.Reset()
	// Broadcast the committed block to the network
	if blkProto := pendingBlock.ConvertToBlockPb(); blkProto != nil {
		if err := m.ctx.p2p.Broadcast(m.ctx.chain.ChainID(), blkProto); err != nil {
			logger.Error().
				Err(err).
				Uint64("block", pendingBlock.Height()).
				Msg("error when broadcasting blkProto")
		}

		// putblock to parent chain if the current node is proposer and current chain is a sub chain
		if m.ctx.round.proposer == m.ctx.addr.RawAddress && m.ctx.chain.ChainAddress() != "" {
			putBlockToParentChain(m.ctx.rootChainAPI, m.ctx.chain.ChainAddress(), m.ctx.addr, pendingBlock)
		}
	} else {
		logger.Error().
			Uint64("block", pendingBlock.Height()).
			Msg("error when converting a block into a proto msg")
	}
	m.produce(m.newCEvt(eFinishEpoch), 0)
	return sRoundStart, nil
}

func (m *cFSM) handleFinishEpochEvt(evt fsm.Event) (fsm.State, error) {
	if m.ctx.shouldHandleDKG() && m.ctx.isDKGFinished() {
		dkgPubKey, dkgPriKey, err := m.ctx.generateDKGKeyPair()
		if err != nil {
			return sEpochStart, errors.Wrap(err, "error when generating DKG key pair")
		}
		m.ctx.epoch.dkgAddress.PublicKey = dkgPubKey
		m.ctx.epoch.dkgAddress.PrivateKey = dkgPriKey
	}

	epochFinished, err := m.ctx.isEpochFinished()
	if err != nil {
		return sEpochStart, errors.Wrap(err, "error when checking if the epoch is finished")
	}
	if epochFinished {
		m.produce(m.newCEvt(eRollDelegates), 0)
		return sEpochStart, nil
	}
	if err := m.produceStartRoundEvt(); err != nil {
		return sEpochStart, errors.Wrapf(err, "error when producing %s", eStartRound)
	}
	return sRoundStart, nil
}

func (m *cFSM) isDelegate(delegates []string) bool {
	for _, d := range delegates {
		if m.ctx.addr.RawAddress == d {
			return true
		}
	}
	return false
}

func (m *cFSM) produceStartRoundEvt() error {
	var (
		duration time.Duration
		err      error
	)
	// If we have the cached last block, we get the timestamp from it
	if duration, err = m.ctx.calcDurationSinceLastBlock(); err != nil {
		// Otherwise, we read it from blockchain
		return errors.Wrap(err, "error when computing the duration since last block time")

	}
	// If the proposal interval is not set (not zero), the next round will only be started after the configured duration
	// after last block's creation time, so that we could keep the constant
	waitDuration := time.Duration(0)
	if m.ctx.cfg.ProposerInterval > 0 {
		waitDuration = (m.ctx.cfg.ProposerInterval - (duration % m.ctx.cfg.ProposerInterval)) % m.ctx.cfg.ProposerInterval
	}
	m.produce(m.newCEvt(eStartRound), waitDuration)

	return nil
}

// handleBackdoorEvt takes the dst state from the event and move the FSM into it
func (m *cFSM) handleBackdoorEvt(evt fsm.Event) (fsm.State, error) {
	bEvt, ok := evt.(*backdoorEvt)
	if !ok {
		return sEpochStart, errors.Wrap(ErrEvtCast, "the event is not a backdoorEvt")
	}
	return bEvt.dst, nil
}

func (m *cFSM) newCEvt(t fsm.EventType) *consensusEvt {
	return newCEvt(t, m.ctx.round.height, m.ctx.round.number, m.ctx.clock)
}

func (m *cFSM) newProposeBlkEvt(blk *blockchain.Block) *proposeBlkEvt {
	return newProposeBlkEvt(blk, m.ctx.round.proofOfLock, m.ctx.round.number, m.ctx.clock)
}

func (m *cFSM) newProposeBlkEvtFromProposePb(pb *iproto.ProposePb) (*proposeBlkEvt, error) {
	evt := newProposeBlkEvtFromProtoMsg(pb, m.ctx.clock)
	if evt == nil {
		return nil, errors.New("error when casting a proto msg to proposeBlkEvt")
	}

	return evt, nil
}

func (m *cFSM) newEndorseEvtWithEndorsePb(ePb *iproto.EndorsePb) (*endorseEvt, error) {
	en, err := endorsement.FromProtoMsg(ePb)
	if err != nil {
		return nil, errors.Wrap(err, "error when casting a proto msg to endorse")
	}
	return newEndorseEvtWithEndorse(en, m.ctx.clock), nil
}

func (m *cFSM) newEndorseEvt(blkHash hash.Hash32B, topic endorsement.ConsensusVoteTopic) *endorseEvt {
	return newEndorseEvt(topic, blkHash, m.ctx.round.height, m.ctx.round.number, m.ctx.addr, m.ctx.clock)
}

func (m *cFSM) newTimeoutEvt(t fsm.EventType) *timeoutEvt {
	return newTimeoutEvt(t, m.ctx.round.height, m.ctx.round.number, m.ctx.clock)
}

func (m *cFSM) newBackdoorEvt(dst fsm.State) *backdoorEvt {
	return newBackdoorEvt(dst, m.ctx.round.height, m.ctx.round.number, m.ctx.clock)
}

func verifyDKGSignature(blk *blockchain.Block, seedByte []byte) error {
	return crypto.BLS.Verify(blk.Header.DKGPubkey, seedByte, blk.Header.DKGBlockSig)
}
