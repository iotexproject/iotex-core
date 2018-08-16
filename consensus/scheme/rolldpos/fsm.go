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
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/crypto"
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
	sAcceptPropose fsm.State = "S_ACCEPT_PROPOSE"
	sAcceptPrevote fsm.State = "S_ACCEPT_PREVOTE"
	sAcceptVote    fsm.State = "S_ACCEPT_VOTE"

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
		sAcceptPrevote,
		sAcceptVote,
	}
)

// iConsensusEvt is the interface of all events for the consensusEvt FSM
type iConsensusEvt interface {
	fsm.Event
	timestamp() time.Time
	// TODO: we need to add height or some other ctx to identify which consensus round the event is associated to
}

// protoMsg is the interface of all events that could convert from/to protobuf messages
type protoMsg interface {
	toProtoMsg() (*iproto.ViewChangeMsg, error)
	fromProtoMsg(msg *iproto.ViewChangeMsg) error
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
	return nil, errors.Wrap(ErrEvtConvert, "converting to the proto message is not implemented")
}

func (e *consensusEvt) fromProtoMsg(pMsg *iproto.ViewChangeMsg) error {
	return errors.Wrap(ErrEvtConvert, "converting from the proto message is not implemented")
}

type proposeBlkEvt struct {
	consensusEvt
	block    *blockchain.Block
	proposer string
}

func newProposeBlkEvt(block *blockchain.Block, proposer string, c clock.Clock) *proposeBlkEvt {
	return &proposeBlkEvt{
		consensusEvt: *newCEvt(eProposeBlock, c),
		block:        block,
		proposer:     proposer,
	}
}

func (e *proposeBlkEvt) toProtoMsg() (*iproto.ViewChangeMsg, error) {
	return &iproto.ViewChangeMsg{
		Vctype:     iproto.ViewChangeMsg_PROPOSE,
		Block:      e.block.ConvertToBlockPb(),
		SenderAddr: e.proposer,
	}, nil
}

func (e *proposeBlkEvt) fromProtoMsg(pMsg *iproto.ViewChangeMsg) error {
	if pMsg.Vctype != iproto.ViewChangeMsg_PROPOSE {
		return errors.Wrapf(ErrEvtConvert, "pMsg Vctype is %d", pMsg.Vctype)
	}
	if pMsg.Block != nil {
		e.block = &blockchain.Block{}
		e.block.ConvertFromBlockPb(pMsg.Block)
	}
	e.proposer = pMsg.SenderAddr
	return nil
}

type voteEvt struct {
	consensusEvt
	blkHash  hash.Hash32B
	decision bool
	voter    string
}

func newVoteEvt(t fsm.EventType, blkHash hash.Hash32B, decision bool, voter string, c clock.Clock) *voteEvt {
	if t != ePrevote && t != eVote {
		return nil
	}
	return &voteEvt{
		consensusEvt: *newCEvt(t, c),
		blkHash:      blkHash,
		decision:     decision,
		voter:        voter,
	}
}

func (e *voteEvt) toProtoMsg() (*iproto.ViewChangeMsg, error) {
	var vctype iproto.ViewChangeMsg_ViewChangeType
	switch e.t {
	case ePrevote:
		vctype = iproto.ViewChangeMsg_PREVOTE
	case eVote:
		vctype = iproto.ViewChangeMsg_VOTE
	}
	return &iproto.ViewChangeMsg{
		Vctype:     vctype,
		BlockHash:  e.blkHash[:],
		SenderAddr: e.voter,
		Decision:   e.decision,
	}, nil
}

func (e *voteEvt) fromProtoMsg(pMsg *iproto.ViewChangeMsg) error {
	if e.t == ePrevote && !(pMsg.Vctype == iproto.ViewChangeMsg_PREVOTE) {
		return errors.Wrapf(ErrEvtConvert, "pMsg Vctype is %d, it doesn't match %s", pMsg.Vctype, ePrevote)
	}
	if e.t == eVote && !(pMsg.Vctype == iproto.ViewChangeMsg_VOTE) {
		return errors.Wrapf(ErrEvtConvert, "pMsg Vctype is %d, it doesn't match %s", pMsg.Vctype, eVote)
	}
	copy(e.blkHash[:], pMsg.BlockHash)
	e.voter = pMsg.SenderAddr
	e.decision = pMsg.Decision
	return nil
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
		AddStates(sDKGGeneration, sRoundStart, sInitPropose, sAcceptPropose, sAcceptPrevote, sAcceptVote).
		AddTransition(sEpochStart, eRollDelegates, cm.handleRollDelegatesEvt, []fsm.State{sEpochStart, sDKGGeneration}).
		AddTransition(sDKGGeneration, eGenerateDKG, cm.handleGenerateDKGEvt, []fsm.State{sRoundStart}).
		AddTransition(sRoundStart, eStartRound, cm.handleStartRoundEvt, []fsm.State{sInitPropose, sAcceptPropose}).
		AddTransition(sInitPropose, eInitBlock, cm.handleInitBlockEvt, []fsm.State{sAcceptPropose}).
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
						if time.Since(evt.timestamp()) <= m.ctx.cfg.UnmatchedEventTTL {
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
		// Even if error happens, we still need to schedule next check of delegate to tolerate transit error
		m.produce(m.newCEvt(eRollDelegates), m.ctx.cfg.DelegateInterval)
		return sInvalid, errors.Wrap(
			err,
			"error when determining the epoch ordinal number and start height offset",
		)
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
	dkg, err := m.ctx.generateDKG()
	if err != nil {
		return sInvalid, err
	}
	m.ctx.epoch.dkg = dkg
	if err := m.produceStartRoundEvt(); err != nil {
		return sInvalid, errors.Wrapf(err, "error when producing %s", eStartRound)
	}
	return sRoundStart, nil
}

func (m *cFSM) handleStartRoundEvt(_ fsm.Event) (fsm.State, error) {
	proposer, height, err := m.ctx.rotatedProposer()
	if err != nil {
		logger.Error().
			Err(err).
			Msg("error when getting the proposer")
		return sInvalid, err
	}
	m.ctx.round = roundCtx{
		height:    height,
		timestamp: m.ctx.clock.Now(),
		prevotes:  make(map[string]bool),
		votes:     make(map[string]bool),
		proposer:  proposer,
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
	proposeBlkEvtProto, err := proposeBlkEvt.toProtoMsg()
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when converting a proposeBlkEvt into a proto msg")
	}
	// Notify itself
	m.produce(proposeBlkEvt, 0)
	// Notify other delegates
	if err := m.ctx.p2p.Broadcast(proposeBlkEvtProto); err != nil {
		logger.Error().
			Err(err).
			Msg("error when broadcasting proposeBlkEvt")
	}
	return sAcceptPropose, nil
}

func (m *cFSM) handleProposeBlockEvt(evt fsm.Event) (fsm.State, error) {
	received := true
	validated := true
	m.ctx.round.block = nil
	switch evt.Type() {
	case eProposeBlock:
		proposeBlkEvt, ok := evt.(*proposeBlkEvt)
		if !ok {
			return sInvalid, errors.Wrap(ErrEvtCast, "the event is not a proposeBlkEvt")
		}
		// If the block is self proposed, skip validation
		if proposeBlkEvt.proposer != m.ctx.addr.RawAddress {
			blkHash := proposeBlkEvt.block.HashBlock()
			if err := m.ctx.chain.ValidateBlock(proposeBlkEvt.block); err != nil {
				logger.Error().
					Str("proposer", proposeBlkEvt.proposer).
					Uint64("block", proposeBlkEvt.block.Height()).
					Str("hash", hex.EncodeToString(blkHash[:])).
					Err(err).
					Msg("error when validating the block")
				validated = false
			}
			// Verify dkg signature
			if len(proposeBlkEvt.block.Header.DKGPubkey) > 0 && len(proposeBlkEvt.block.Header.DKGBlockSig) > 0 {
				if err := verifyDKGSignature(proposeBlkEvt.block, m.ctx.epoch.seed); err != nil {
					logger.Error().
						Str("proposer", proposeBlkEvt.proposer).
						Uint64("block", proposeBlkEvt.block.Height()).
						Str("hash", hex.EncodeToString(blkHash[:])).
						Err(err).
						Msg("Failed to verify the DKG signature")
					validated = false
				}
			}
		}
		m.ctx.round.block = proposeBlkEvt.block
	case eProposeBlockTimeout:
		received = false
		validated = false
		logger.Warn().
			Str("proposer", m.ctx.round.proposer).
			Uint64("height", m.ctx.round.height).
			Msg("didn't receive the proposed block before timeout")
	}

	if received {
		prevoteEvt := m.newPrevoteEvt(m.ctx.round.block.HashBlock(), validated)
		prevoteEvtProto, err := prevoteEvt.toProtoMsg()
		if err != nil {
			return sInvalid, errors.Wrap(err, "error when converting a prevoteEvt into a proto msg")
		}
		// Notify itself
		m.produce(prevoteEvt, 0)
		// Notify other delegates
		if err := m.ctx.p2p.Broadcast(prevoteEvtProto); err != nil {
			logger.Error().
				Err(err).
				Msg("error when broadcasting prevoteEvtProto")
		}
	}
	// Setup timeout for waiting for prevote
	m.produce(m.newTimeoutEvt(ePrevoteTimeout, m.ctx.round.height), m.ctx.cfg.AcceptPrevoteTTL)
	return sAcceptPrevote, nil
}

func (m *cFSM) handlePrevoteEvt(evt fsm.Event) (fsm.State, error) {
	var vEvt *voteEvt
	switch evt.Type() {
	case ePrevote:
		prevoteEvt, ok := evt.(*voteEvt)
		if !ok {
			return sInvalid, errors.Wrap(ErrEvtCast, "the event is not a voteEvt")
		}
		var blkHash hash.Hash32B
		if m.ctx.round.block != nil {
			blkHash = m.ctx.round.block.HashBlock()
		}
		if bytes.Equal(blkHash[:], prevoteEvt.blkHash[:]) {
			m.ctx.round.prevotes[prevoteEvt.voter] = prevoteEvt.decision
		}
		// if ether yes or no is true, block must exists and blkHash must be a valid one
		yes, no := m.ctx.calcQuorum(m.ctx.round.prevotes)
		if yes {
			vEvt = m.newVoteEvt(blkHash, true)
		} else if no {
			vEvt = m.newVoteEvt(blkHash, false)
		}
		if vEvt == nil {
			// Wait for more prevotes to come
			return sAcceptPrevote, nil
		}
		// Reached the agreement
	case ePrevoteTimeout:
		if m.ctx.round.block != nil {
			vEvt = m.newVoteEvt(m.ctx.round.block.HashBlock(), false)
		}
		logger.Warn().
			Uint64("height", m.ctx.round.height).
			Int("prevotes", len(m.ctx.round.prevotes)).
			Msg("didn't collect enough prevotes before timeout")
	}
	if vEvt != nil {
		vEvtProto, err := vEvt.toProtoMsg()
		if err != nil {
			return sInvalid, errors.Wrap(err, "error when converting a voteEvt into a proto msg")
		}
		// Notify itself
		m.produce(vEvt, 0)
		// Notify other delegates
		if err := m.ctx.p2p.Broadcast(vEvtProto); err != nil {
			logger.Error().
				Err(err).
				Msg("error when broadcasting voteEvtProto")
		}
	}
	// Setup timeout for waiting for vote
	m.produce(m.newTimeoutEvt(eVoteTimeout, m.ctx.round.height), m.ctx.cfg.AcceptVoteTTL)
	return sAcceptVote, nil
}

func (m *cFSM) handleVoteEvt(evt fsm.Event) (fsm.State, error) {
	consensus := false
	timeout := false
	disagreement := false
	switch evt.Type() {
	case eVote:
		voteEvt, ok := evt.(*voteEvt)
		if !ok {
			return sInvalid, errors.Wrap(ErrEvtCast, "the event is not a voteEvt")
		}
		var blkHash hash.Hash32B
		if m.ctx.round.block != nil {
			blkHash = m.ctx.round.block.HashBlock()
		}
		if bytes.Equal(blkHash[:], voteEvt.blkHash[:]) {
			m.ctx.round.votes[voteEvt.voter] = voteEvt.decision
		}
		// if ether yes or no is true, block must exists and blkHash must be a valid one
		yes, no := m.ctx.calcQuorum(m.ctx.round.votes)
		if yes {
			consensus = true
		} else if no {
			disagreement = true
		} else {
			// Wait for more votes to come
			return sAcceptVote, nil
		}
	case eVoteTimeout:
		consensus = false
		timeout = true
		logger.Warn().
			Uint64("height", m.ctx.round.height).
			Int("votes", len(m.ctx.round.votes)).
			Msg("didn't collect enough votes before timeout")
	}
	var pendingBlock *blockchain.Block
	if consensus {
		pendingBlock = m.ctx.round.block
		logger.Info().
			Uint64("block", pendingBlock.Height()).
			Msg("consensus reached")
	} else {
		var height uint64
		if m.ctx.round.block != nil {
			height = m.ctx.round.block.Height()
		}
		logger.Warn().
			Uint64("block", height).
			Bool("timeout", timeout).
			Bool("disagreement", disagreement).
			Msg("consensus did not reach")
		pendingBlock = m.ctx.chain.MintNewDummyBlock()
		logger.Warn().
			Uint64("block", pendingBlock.Height()).
			Msg("dummy block is generated")
	}
	// Commit and broadcast the pending block
	var err error
	if err = m.ctx.chain.CommitBlock(pendingBlock); err == nil {
		// Remove transfers in this block from ActPool and reset ActPool state
		m.ctx.actPool.Reset()
	}
	if err != nil {
		logger.Error().
			Err(err).
			Uint64("block", pendingBlock.Height()).
			Bool("dummy", pendingBlock.IsDummyBlock()).
			Msg("error when committing a block")
	} else {
		// Broadcast the committed block to the network
		if blkProto := pendingBlock.ConvertToBlockPb(); blkProto != nil {
			if err := m.ctx.p2p.Broadcast(blkProto); err != nil {
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
	finished, err := m.ctx.isEpochFinished()
	if err != nil {
		return sInvalid, errors.Wrap(err, "error when checking if the epoch is finished")
	}
	if finished {
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
	var duration time.Duration
	// If we have the cached last block, we get the timestamp from it
	if m.ctx.round.block != nil {
		duration = time.Since(m.ctx.round.block.Header.Timestamp())
	}
	// Otherwise, we read it from blockchain
	duration, err := m.ctx.calcDurationSinceLastBlock()
	if err != nil {
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
	return newProposeBlkEvt(blk, m.ctx.addr.RawAddress, m.ctx.clock)
}

func (m *cFSM) newPrevoteEvt(blkHash hash.Hash32B, decision bool) *voteEvt {
	return newVoteEvt(ePrevote, blkHash, decision, m.ctx.addr.RawAddress, m.ctx.clock)
}

func (m *cFSM) newVoteEvt(blkHash hash.Hash32B, decision bool) *voteEvt {
	return newVoteEvt(eVote, blkHash, decision, m.ctx.addr.RawAddress, m.ctx.clock)
}

func (m *cFSM) newTimeoutEvt(t fsm.EventType, height uint64) *timeoutEvt {
	return newTimeoutEvt(t, m.ctx.clock)
}

func (m *cFSM) newBackdoorEvt(dst fsm.State) *backdoorEvt {
	return newBackdoorEvt(dst, m.ctx.clock)
}

func (m *cFSM) updateSeed() ([]byte, error) {
	numDlgs := m.ctx.cfg.NumDelegates
	epochNum, epochHeight, err := m.ctx.calcEpochNumAndHeight()
	if err != nil {
		return []byte{}, errors.Wrap(err, "Failed to do decode seed")
	}
	if epochNum <= 1 {
		return []byte{}, nil
	}
	selectedID := make([][]uint8, 0)
	selectedSig := make([][]byte, 0)
	selectedPK := make([][]byte, 0)
	endHeight := epochHeight - 1
	startHeight := uint64(numDlgs)*uint64(m.ctx.cfg.NumSubEpochs)*(epochNum-2) + 1
	for i := startHeight; i <= endHeight && len(selectedID) < crypto.Degree+1; i++ {
		blk, err := m.ctx.chain.GetBlockByHeight(i)
		if err != nil {
			continue
		}
		if len(blk.Header.DKGID) > 0 && len(blk.Header.DKGPubkey) > 0 && len(blk.Header.DKGBlockSig) > 0 {
			selectedID = append(selectedID, blk.Header.DKGID)
			selectedSig = append(selectedSig, blk.Header.DKGBlockSig)
			selectedPK = append(selectedPK, blk.Header.DKGPubkey)
		}
	}

	if len(selectedID) < crypto.Degree+1 {
		return []byte{}, errors.New("DKG signature/pubic key is not enough to aggregate")
	}

	aggregateSig, err := crypto.BLS.SignAggregate(selectedID, selectedSig)
	if err != nil {
		return []byte{}, errors.Wrap(err, "Failed to generate aggregate signature to update Seed")
	}
	if err = crypto.BLS.VerifyAggregate(selectedID, selectedPK, m.ctx.epoch.seed, aggregateSig); err != nil {
		return []byte{}, errors.Wrap(err, "Failed to verify aggregate signature to update Seed")
	}
	return aggregateSig, nil
}

func verifyDKGSignature(blk *blockchain.Block, seedByte []byte) error {
	if err := crypto.BLS.Verify(blk.Header.DKGPubkey, seedByte, blk.Header.DKGBlockSig); err != nil {
		return err
	}
	return nil
}
