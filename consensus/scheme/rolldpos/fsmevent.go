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
	"github.com/iotexproject/iotex-core/pkg/hash"
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
	block *blockchain.Block
}

func newProposeBlkEvt(block *blockchain.Block, round uint32, c clock.Clock) *proposeBlkEvt {
	return &proposeBlkEvt{
		consensusEvt: *newCEvt(eProposeBlock, block.Height(), round, c),
		block:        block,
	}
}

func (e *proposeBlkEvt) toProtoMsg() *iproto.ProposePb {
	return &iproto.ProposePb{
		Block:    e.block.ConvertToBlockPb(),
		Proposer: e.block.ProducerAddress(),
	}
}

func newProposeBlkEvtFromProtoMsg(pMsg *iproto.ProposePb, c clock.Clock) *proposeBlkEvt {
	if pMsg.Block == nil {
		return nil
	}
	block := &blockchain.Block{}
	block.ConvertFromBlockPb(pMsg.Block)

	return newProposeBlkEvt(block, pMsg.Round, c)
}

type endorseEvt struct {
	consensusEvt
	endorse *endorsement.Endorsement
}

func newEndorseEvt(topic endorsement.ConsensusVoteTopic, blkHash hash.Hash32B, height uint64, round uint32, endorser *iotxaddress.Address, c clock.Clock) (*endorseEvt, error) {
	endorse := endorsement.NewEndorsement(endorsement.NewConsensusVote(blkHash, height, round, topic), endorser)

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
