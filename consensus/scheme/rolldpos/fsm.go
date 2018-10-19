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

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
)

/**
 * TODO:
 *  1. Store endorse decisions of follow up status
 *  2. For the nodes received correct proposal, add proposer's proposal endorse without signature, which could be replaced with real signature
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
	sInitPropose           fsm.State = "S_INIT_PROPOSE"
	sAcceptPropose         fsm.State = "S_ACCEPT_PROPOSE"
	sAcceptProposalEndorse fsm.State = "S_ACCEPT_PROPOSAL_ENDROSE"
	sAcceptLockEndorse     fsm.State = "S_ACCEPT_LOCK_ENDORSE"

	// sInvalid indicates an invalid state. It doesn't matter what dst state to return when there's an error. Transition
	// to dst state will not happen. However, we should always return to this state to be consistent.
	sInvalid fsm.State = "S_INVALID"

	// consensusEvt event types
	eRollDelegates          fsm.EventType = "E_ROLL_DELEGATES"
	eGenerateDKG            fsm.EventType = "E_GENERATE_DKG"
	eStartRound             fsm.EventType = "E_START_ROUND"
	eInitBlock              fsm.EventType = "E_INIT_BLOCK"
	eProposeBlock           fsm.EventType = "E_PROPOSE_BLOCK"
	eProposeBlockTimeout    fsm.EventType = "E_PROPOSE_BLOCK_TIMEOUT"
	eEndorseProposal        fsm.EventType = "E_ENDORSE_PROPOSAL"
	eEndorseProposalTimeout fsm.EventType = "E_ENDORSE_PROPOSAL_TIMEOUT"
	eEndorseLock            fsm.EventType = "E_ENDORSE_LOCK"
	eEndorseLockTimeout     fsm.EventType = "E_ENDORSE_LOCK_TIMEOUT"
	eFinishEpoch            fsm.EventType = "E_FINISH_EPOCH"

	// eBackdoor indicates an backdoor event type
	eBackdoor fsm.EventType = "E_BACKDOOR"
)

var (
	// ErrEvtCast indicates the error of casting the event
	ErrEvtCast = errors.New("error when casting the event")
	// ErrEvtConvert indicates the error of converting the event from/to the proto message
	ErrEvtConvert = errors.New("error when converting the event from/to the proto message")

	// consensusStates is a slice consisting of all consensusEvt states
	consensusStates = []fsm.State{
		sEpochStart,
		sDKGGeneration,
		sRoundStart,
		sInitPropose,
		sAcceptPropose,
		sAcceptProposalEndorse,
		sAcceptLockEndorse,
	}
)

// iConsensusEvt is the interface of all events for the consensusEvt FSM
type iConsensusEvt interface {
	fsm.Event
	timestamp() time.Time
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

type proposeBlkEvt struct {
	consensusEvt
	block *blockchain.Block
}

func newProposeBlkEvt(block *blockchain.Block, c clock.Clock) *proposeBlkEvt {
	return &proposeBlkEvt{
		consensusEvt: *newCEvt(eProposeBlock, c),
		block:        block,
	}
}

func (e *proposeBlkEvt) toProtoMsg() *iproto.ProposePb {
	return &iproto.ProposePb{
		Block:    e.block.ConvertToBlockPb(),
		Proposer: e.block.ProducerAddress(),
	}
}

func (e *proposeBlkEvt) fromProtoMsg(pMsg *iproto.ProposePb) error {
	if pMsg.Block != nil {
		e.block = &blockchain.Block{}
		e.block.ConvertFromBlockPb(pMsg.Block)
	}
	return nil
}

type endorseEvt struct {
	consensusEvt
	endorse *endorsement.Endorsement
}

func newEndorseEvt(topic endorsement.ConsensusVoteTopic, blkHash hash.Hash32B, height uint64, endorser *iotxaddress.Address, c clock.Clock) (*endorseEvt, error) {
	endorse, err := endorsement.NewEndorsement(endorsement.NewConsensusVote(blkHash, height, topic), endorser)
	if err != nil {
		logger.Error().Err(err).Bytes("Block Hash", blkHash[:]).Str("endorser", endorser.RawAddress).Msg("failed to sign endorse for block")
		return nil, err
	}

	return newEndorseEvtWithEndorse(endorse, c), nil
}

func newEndorseEvtWithEndorse(endorse *endorsement.Endorsement, c clock.Clock) *endorseEvt {
	vote := endorse.ConsensusVote()
	var eventType fsm.EventType
	switch vote.Topic {
	case endorsement.PROPOSAL:
		eventType = eEndorseProposal
	case endorsement.LOCK:
		eventType = eEndorseLock
	}
	return &endorseEvt{
		consensusEvt: *newCEvt(eventType, c),
		endorse:      endorse,
	}
}

func (en *endorseEvt) toProtoMsg() *iproto.EndorsePb {
	return en.endorse.ToProtoMsg()
}

type timeoutEvt struct {
	consensusEvt
}

func newTimeoutEvt(t fsm.EventType, c clock.Clock) *timeoutEvt {
	return &timeoutEvt{
		consensusEvt: *newCEvt(t, c),
	}
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
		AddStates(sDKGGeneration, sRoundStart, sInitPropose, sAcceptPropose, sAcceptProposalEndorse, sAcceptLockEndorse).
		AddTransition(sEpochStart, eRollDelegates, cm.handleRollDelegatesEvt, []fsm.State{sEpochStart, sDKGGeneration}).
		AddTransition(sDKGGeneration, eGenerateDKG, cm.handleGenerateDKGEvt, []fsm.State{sRoundStart}).
		AddTransition(sRoundStart, eStartRound, cm.handleStartRoundEvt, []fsm.State{sInitPropose, sAcceptPropose}).
		AddTransition(sRoundStart, eFinishEpoch, cm.handleFinishEpochEvt, []fsm.State{sEpochStart, sRoundStart}).
		AddTransition(sInitPropose, eInitBlock, cm.handleInitBlockEvt, []fsm.State{sAcceptPropose}).
		AddTransition(
			sAcceptPropose,
			eProposeBlock,
			cm.handleProposeBlockEvt,
			[]fsm.State{
				sAcceptPropose,         // proposed block invalid
				sAcceptProposalEndorse, // proposed block valid
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
				sAcceptLockEndorse, // haven't reach agreement yet
				sRoundStart,        // reach commit agreement, jump to next round
			}).
		AddTransition(
			sAcceptLockEndorse,
			eEndorseLockTimeout,
			cm.handleEndorseLockTimeout,
			[]fsm.State{
				sRoundStart, // timeout, jump to next round
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
		return sInvalid, errors.Wrap(
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
		return sInvalid, errors.Wrap(
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
			return sInvalid, err
		}
		m.ctx.epoch.secrets = secrets
		m.ctx.epoch.witness = witness
	}
	if err := m.produceStartRoundEvt(); err != nil {
		return sInvalid, errors.Wrapf(err, "error when producing %s", eStartRound)
	}
	return sRoundStart, nil
}

func (m *cFSM) handleStartRoundEvt(_ fsm.Event) (fsm.State, error) {
	subEpochNum, err := m.ctx.calcSubEpochNum()
	if err != nil {
		return sInvalid, errors.Wrap(
			err,
			"error when determining the sub-epoch ordinal number",
		)
	}
	m.ctx.epoch.subEpochNum = subEpochNum

	proposer, height, err := m.ctx.rotatedProposer()
	if err != nil {
		logger.Error().
			Err(err).
			Msg("error when getting the proposer")
		return sInvalid, err
	}
	m.ctx.round = roundCtx{
		height:           height,
		timestamp:        m.ctx.clock.Now(),
		proposalEndorses: make(map[hash.Hash32B]map[string]bool),
		lockEndorses:     make(map[hash.Hash32B]map[string]bool),
		proposer:         proposer,
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
	// Setup timeout for waiting for proposed block
	m.produce(m.newTimeoutEvt(eProposeBlockTimeout, m.ctx.round.height), m.ctx.cfg.AcceptProposeTTL)
	return sAcceptPropose, nil
}

func (m *cFSM) handleInitBlockEvt(evt fsm.Event) (fsm.State, error) {
	blk, err := m.ctx.mintBlock()
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when minting a block")
	}
	proposeBlkEvt := m.newProposeBlkEvt(blk)
	proposeBlkEvtProto := proposeBlkEvt.toProtoMsg()
	// Notify itself
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

func (m *cFSM) moveToAcceptProposalEndorse() (fsm.State, error) {
	// Setup timeout for waiting for endorse
	m.produce(m.newTimeoutEvt(eEndorseProposalTimeout, m.ctx.round.height), m.ctx.cfg.AcceptProposalEndorseTTL)
	return sAcceptProposalEndorse, nil
}

func (m *cFSM) handleProposeBlockEvt(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eProposeBlock {
		return sInvalid, errors.Errorf("invalid event type %s", evt.Type())
	}
	m.ctx.round.block = nil
	proposeBlkEvt, ok := evt.(*proposeBlkEvt)
	if !ok {
		return sInvalid, errors.Wrap(ErrEvtCast, "the event is not a proposeBlkEvt")
	}
	proposer, err := m.ctx.calcProposer(proposeBlkEvt.block.Height(), m.ctx.epoch.delegates)
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when calculating the proposer")
	}
	if !m.validateProposeBlock(proposeBlkEvt.block, proposer) {
		return sAcceptPropose, nil
	}
	m.ctx.round.block = proposeBlkEvt.block
	endorseEvt, err := m.newEndorseProposalEvt(m.ctx.round.block.HashBlock())
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when generating new endorse proposal event")
	}
	endorseEvtProto := endorseEvt.toProtoMsg()
	// Notify itself
	m.produce(endorseEvt, 0)
	// Notify other delegates
	if err := m.ctx.p2p.Broadcast(m.ctx.chain.ChainID(), endorseEvtProto); err != nil {
		logger.Error().
			Err(err).
			Msg("error when broadcasting endorseEvtProto")
	}

	return m.moveToAcceptProposalEndorse()
}

func (m *cFSM) handleProposeBlockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eProposeBlockTimeout {
		return sInvalid, errors.Errorf("invalid event type %s", evt.Type())
	}
	logger.Warn().
		Str("proposer", m.ctx.round.proposer).
		Uint64("height", m.ctx.round.height).
		Msg("didn't receive the proposed block before timeout")

	return m.moveToAcceptProposalEndorse()
}

func (m *cFSM) validateEndorse(en *endorsement.Endorsement, expectedEndorseTopic endorsement.ConsensusVoteTopic) bool {
	errorLog := logger.Error().
		Uint64("expectedHeight", m.ctx.round.height).
		Uint8("expectedEndorseTopic", uint8(expectedEndorseTopic))
	vote := en.ConsensusVote()
	if vote.Topic != expectedEndorseTopic {
		errorLog.Uint8("endorseTopic", uint8(vote.Topic)).
			Msg("error when validating the endorse topic")
		return false
	}
	if vote.Height != m.ctx.round.height {
		errorLog.Uint64("height", vote.Height).
			Msg("error when validating the endorse height")
		return false
	}
	// TODO verify that the endorser is one delegate, and verify signature via endorse.VerifySignature() with pub key
	return true
}

func (m *cFSM) moveToAcceptLockEndorse() (fsm.State, error) {
	// Setup timeout for waiting for commit
	m.produce(m.newTimeoutEvt(eEndorseLockTimeout, m.ctx.round.height), m.ctx.cfg.AcceptCommitEndorseTTL)
	return sAcceptLockEndorse, nil
}

func (m *cFSM) isProposedBlock(hash []byte) bool {
	if m.ctx.round.block == nil {
		return false
	}
	blkHash := m.ctx.round.block.HashBlock()
	return bytes.Equal(hash[:], blkHash[:])
}

func (m *cFSM) handleEndorseProposalEvt(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eEndorseProposal {
		return sInvalid, errors.Errorf("invalid event type %s", evt.Type())
	}
	endorseEvt, ok := evt.(*endorseEvt)
	if !ok {
		return sInvalid, errors.Wrap(ErrEvtCast, "the event is not an endorseEvt")
	}
	endorse := endorseEvt.endorse
	vote := endorse.ConsensusVote()
	if !m.isProposedBlock(vote.BlkHash[:]) {
		return sAcceptProposalEndorse, nil
	}
	if !m.validateEndorse(endorse, endorsement.PROPOSAL) {
		return sAcceptProposalEndorse, nil
	}
	blkHash := vote.BlkHash
	endorses := m.ctx.round.proposalEndorses[blkHash]
	if endorses == nil {
		endorses = map[string]bool{}
		m.ctx.round.proposalEndorses[blkHash] = endorses
	}
	endorses[endorse.Endorser()] = true
	// if ether yes or no is true, block must exists and blkHash must be a valid one
	yes, _ := m.ctx.calcQuorum(m.ctx.round.proposalEndorses[blkHash])
	if !yes {
		// Wait for more preCommits to come
		return sAcceptProposalEndorse, nil
	}
	// Reached the agreement
	cEvt, err := m.newEndorseLockEvt(blkHash)
	if err != nil {
		return sInvalid, errors.Wrap(err, "failed to generate endorse commit event")
	}
	cEvtProto := cEvt.toProtoMsg()
	// Notify itself
	m.produce(cEvt, 0)
	// Notify other delegates
	if err := m.ctx.p2p.Broadcast(m.ctx.chain.ChainID(), cEvtProto); err != nil {
		logger.Error().
			Err(err).
			Msg("error when broadcasting commitEvtProto")
	}

	return m.moveToAcceptLockEndorse()
}

func (m *cFSM) handleEndorseProposalTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eEndorseProposalTimeout {
		return sInvalid, errors.Errorf("invalid event type %s", evt.Type())
	}
	logger.Warn().
		Uint64("height", m.ctx.round.height).
		Int("numberOfEndorses", len(m.ctx.round.proposalEndorses)).
		Msg("didn't collect enough proposal endorses before timeout")

	return m.moveToAcceptLockEndorse()
}

func (m *cFSM) handleEndorseLockEvt(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eEndorseLock {
		return sInvalid, errors.Errorf("invalid event type %s", evt.Type())
	}
	endorseEvt, ok := evt.(*endorseEvt)
	if !ok {
		return sInvalid, errors.Wrap(ErrEvtCast, "the event is not an endorseEvt")
	}
	endorse := endorseEvt.endorse
	vote := endorse.ConsensusVote()
	if !m.isProposedBlock(vote.BlkHash[:]) {
		return sAcceptLockEndorse, nil
	}
	if vote.Topic != endorsement.LOCK {
		return sAcceptLockEndorse, nil
	}
	// TODO verify that the endorse is one delegate, and verify signature via endorse.VerifySignature() with pub key
	blkHash := vote.BlkHash
	endorses := m.ctx.round.lockEndorses[blkHash]
	if endorses == nil {
		endorses = map[string]bool{}
		m.ctx.round.lockEndorses[blkHash] = endorses
	}
	endorses[endorse.Endorser()] = true
	// if either yes or no is true, block must exists and blkHash must be a valid one
	yes, no := m.ctx.calcQuorum(endorses)
	if !yes && !no {
		// Wait for more votes to come
		return sAcceptLockEndorse, nil
	}

	return m.processEndorseLock(yes && !no)
}

func (m *cFSM) handleEndorseLockTimeout(evt fsm.Event) (fsm.State, error) {
	if evt.Type() != eEndorseLockTimeout {
		return sInvalid, errors.Errorf("invalid event type %s", evt.Type())
	}
	logger.Warn().
		Uint64("height", m.ctx.round.height).
		Int("numOfLockEndorses", len(m.ctx.round.lockEndorses)).
		Msg("didn't collect enough commit endorse before timeout")

	return m.processEndorseLock(false)
}

func (m *cFSM) processEndorseLock(consensus bool) (fsm.State, error) {
	var pendingBlock *blockchain.Block
	height := m.ctx.round.height
	if consensus {
		pendingBlock = m.ctx.round.block
		logger.Info().
			Uint64("block", height).
			Msg("consensus reached")
		consensusMtc.WithLabelValues("true").Inc()
	} else {
		logger.Warn().
			Uint64("block", height).
			Bool("consensus", consensus).
			Msg("consensus did not reach")
		consensusMtc.WithLabelValues("false").Inc()
		if m.ctx.cfg.EnableDummyBlock {
			pendingBlock = m.ctx.chain.MintNewDummyBlock()
			logger.Warn().
				Uint64("block", pendingBlock.Height()).
				Msg("dummy block is generated")
		}
	}
	if pendingBlock != nil {
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
				Bool("dummy", pendingBlock.IsDummyBlock()).
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
					Bool("dummy", pendingBlock.IsDummyBlock()).
					Msg("error when broadcasting blkProto")
			}
		} else {
			logger.Error().
				Uint64("block", pendingBlock.Height()).
				Bool("dummy", pendingBlock.IsDummyBlock()).
				Msg("error when converting a block into a proto msg")
		}
	}
	m.produce(m.newCEvt(eFinishEpoch), 0)
	return sRoundStart, nil
}

func (m *cFSM) handleFinishEpochEvt(evt fsm.Event) (fsm.State, error) {
	if m.ctx.shouldHandleDKG() && m.ctx.isDKGFinished() {
		dkgPubKey, dkgPriKey, err := m.ctx.generateDKGKeyPair()
		if err != nil {
			return sInvalid, errors.Wrap(err, "error when generating DKG key pair")
		}
		m.ctx.epoch.dkgAddress.PublicKey = dkgPubKey
		m.ctx.epoch.dkgAddress.PrivateKey = dkgPriKey
	}

	epochFinished, err := m.ctx.isEpochFinished()
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when checking if the epoch is finished")
	}
	if epochFinished {
		m.produce(m.newCEvt(eRollDelegates), 0)
		return sEpochStart, nil
	}
	if err := m.produceStartRoundEvt(); err != nil {
		return sInvalid, errors.Wrapf(err, "error when producing %s", eStartRound)
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
	if m.ctx.round.block != nil {
		duration = m.ctx.clock.Now().Sub(m.ctx.round.block.Header.Timestamp())
	} else if duration, err = m.ctx.calcDurationSinceLastBlock(); err != nil {
		// Otherwise, we read it from blockchain
		return errors.Wrap(err, "error when computing the duration since last block time")

	}
	// If the proposal interval is not set (not zero), the next round will only be started after the configured duration
	// after last block's creation time, so that we could keep the constant
	if duration >= m.ctx.cfg.ProposerInterval {
		m.produce(m.newCEvt(eStartRound), 0)
	} else {
		m.produce(m.newCEvt(eStartRound), m.ctx.cfg.ProposerInterval-duration)
	}
	return nil
}

// handleBackdoorEvt takes the dst state from the event and move the FSM into it
func (m *cFSM) handleBackdoorEvt(evt fsm.Event) (fsm.State, error) {
	bEvt, ok := evt.(*backdoorEvt)
	if !ok {
		return sInvalid, errors.Wrap(ErrEvtCast, "the event is not a backdoorEvt")
	}
	return bEvt.dst, nil
}

func (m *cFSM) newCEvt(t fsm.EventType) *consensusEvt {
	return newCEvt(t, m.ctx.clock)
}

func (m *cFSM) newProposeBlkEvt(blk *blockchain.Block) *proposeBlkEvt {
	return newProposeBlkEvt(blk, m.ctx.clock)
}

func (m *cFSM) newProposeBlkEvtFromProposePb(pb *iproto.ProposePb) (*proposeBlkEvt, error) {
	pbEvt := m.newProposeBlkEvt(nil)
	if err := pbEvt.fromProtoMsg(pb); err != nil {
		return nil, errors.Wrap(err, "error when casting a proto msg to proposeBlkEvt")
	}
	return pbEvt, nil
}

func (m *cFSM) newEndorseEvtWithEndorsePb(ePb *iproto.EndorsePb) (*endorseEvt, error) {
	en, err := endorsement.FromProtoMsg(ePb)
	if err != nil {
		return nil, errors.Wrap(err, "error when casting a proto msg to endorse")
	}
	return newEndorseEvtWithEndorse(en, m.ctx.clock), nil
}

func (m *cFSM) newEndorseProposalEvt(blkHash hash.Hash32B) (*endorseEvt, error) {
	return newEndorseEvt(endorsement.PROPOSAL, blkHash, m.ctx.round.height, m.ctx.addr, m.ctx.clock)
}

func (m *cFSM) newEndorseLockEvt(blkHash hash.Hash32B) (*endorseEvt, error) {
	return newEndorseEvt(endorsement.LOCK, blkHash, m.ctx.round.height, m.ctx.addr, m.ctx.clock)
}

func (m *cFSM) newTimeoutEvt(t fsm.EventType, height uint64) *timeoutEvt {
	return newTimeoutEvt(t, m.ctx.clock)
}

func (m *cFSM) newBackdoorEvt(dst fsm.State) *backdoorEvt {
	return newBackdoorEvt(dst, m.ctx.clock)
}

func verifyDKGSignature(blk *blockchain.Block, seedByte []byte) error {
	return crypto.BLS.Verify(blk.Header.DKGPubkey, seedByte, blk.Header.DKGBlockSig)
}
