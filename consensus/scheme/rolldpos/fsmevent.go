// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"time"

	"github.com/facebookgo/clock"
	fsm "github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/proto"
)

// iConsensusEvt is the interface of all events for the consensusEvt FSM
type iConsensusEvt interface {
	fsm.Event
	height() uint64
	round() uint32
	timestamp() time.Time
	// TODO: we need to add height or some other ctx to identify which consensus round the event is associated to
}

type consensusEvt struct {
	h  uint64
	r  uint32
	t  fsm.EventType
	ts time.Time
}

func newCEvt(t fsm.EventType, height uint64, round uint32, c clock.Clock) *consensusEvt {
	return &consensusEvt{
		h:  height,
		r:  round,
		t:  t,
		ts: c.Now(),
	}
}

func (e *consensusEvt) height() uint64 { return e.h }

func (e *consensusEvt) round() uint32 { return e.r }

func (e *consensusEvt) Type() fsm.EventType { return e.t }

func (e *consensusEvt) timestamp() time.Time { return e.ts }

type proposeBlkEvt struct {
	consensusEvt
	block     Block
	lockProof *endorsement.Set
}

func newProposeBlkEvt(
	block Block,
	lockProof *endorsement.Set,
	round uint32,
	c clock.Clock,
) *proposeBlkEvt {
	return &proposeBlkEvt{
		consensusEvt: *newCEvt(eProposeBlock, block.Height(), round, c),
		block:        block,
		lockProof:    lockProof,
	}
}

func (e *proposeBlkEvt) toProtoMsg() *iproto.ProposePb {
	var lockProof *iproto.EndorsementSet
	if e.lockProof != nil {
		lockProof = e.lockProof.ToProto()
	}
	data, _ := e.block.Serialize()

	return &iproto.ProposePb{
		Height:    e.block.Height(),
		Hash:      e.block.Hash(),
		Block:     data,
		LockProof: lockProof,
		Round:     e.r,
		Proposer:  e.block.Proposer(),
	}
}

func newProposeBlkEvtFromProtoMsg(pMsg *iproto.ProposePb, c clock.Clock) *proposeBlkEvt {
	if pMsg.Block == nil {
		return nil
	}
	block := &blockchain.Block{}
	if err := block.Deserialize(pMsg.Block); err != nil {
		logger.Error().Err(err).Msg("failed to deserialize block")
		return nil
	}
	var lockProof *endorsement.Set
	if pMsg.LockProof != nil {
		lockProof = &endorsement.Set{}
		err := lockProof.FromProto(pMsg.LockProof)
		if err != nil {
			logger.Error().Err(err).Msg("failed to generate proposeBlkEvt from protobuf")
			return nil
		}
	}

	return newProposeBlkEvt(&blockWrapper{block, pMsg.Round}, lockProof, pMsg.Round, c)
}

type endorseEvt struct {
	consensusEvt
	endorse *endorsement.Endorsement
}

func newEndorseEvt(
	topic endorsement.ConsensusVoteTopic,
	blkHash []byte,
	height uint64,
	round uint32,
	endorser *iotxaddress.Address,
	c clock.Clock,
) *endorseEvt {
	endorse := endorsement.NewEndorsement(endorsement.NewConsensusVote(blkHash, height, round, topic), endorser)

	return newEndorseEvtWithEndorse(endorse, c)
}

func newEndorseEvtWithEndorse(endorse *endorsement.Endorsement, c clock.Clock) *endorseEvt {
	vote := endorse.ConsensusVote()
	var eventType fsm.EventType
	switch vote.Topic {
	case endorsement.PROPOSAL:
		eventType = eEndorseProposal
	case endorsement.LOCK:
		eventType = eEndorseLock
	case endorsement.COMMIT:
		eventType = eEndorseCommit
	}
	return &endorseEvt{
		consensusEvt: *newCEvt(eventType, vote.Height, vote.Round, c),
		endorse:      endorse,
	}
}

func (en *endorseEvt) toProtoMsg() *iproto.EndorsePb {
	return en.endorse.ToProtoMsg()
}

type timeoutEvt struct {
	consensusEvt
}

func newTimeoutEvt(t fsm.EventType, height uint64, round uint32, c clock.Clock) *timeoutEvt {
	return &timeoutEvt{
		consensusEvt: *newCEvt(t, height, round, c),
	}
}

// backdoorEvt is used for testing purpose to set the consensusEvt FSM to any particular state
type backdoorEvt struct {
	consensusEvt
	dst fsm.State
}

func newBackdoorEvt(dst fsm.State, height uint64, round uint32, c clock.Clock) *backdoorEvt {
	return &backdoorEvt{
		consensusEvt: *newCEvt(eBackdoor, height, round, c),
		dst:          dst,
	}
}
