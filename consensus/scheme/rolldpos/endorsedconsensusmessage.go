// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// EndorsedConsensusMessage is an endorsement on document
type EndorsedConsensusMessage struct {
	height      uint64
	message     endorsement.Document
	endorsement *endorsement.Endorsement
}

// NewEndorsedConsensusMessage creates an EndorsedConsensusMessage for an consensus vote
func NewEndorsedConsensusMessage(
	height uint64,
	message endorsement.Document,
	endorsement *endorsement.Endorsement,
) *EndorsedConsensusMessage {
	return &EndorsedConsensusMessage{
		height:      height,
		message:     message,
		endorsement: endorsement,
	}
}

// Document returns the endorsed consensus message
func (ecm *EndorsedConsensusMessage) Document() endorsement.Document {
	return ecm.message
}

// Endorsement returns the endorsement
func (ecm *EndorsedConsensusMessage) Endorsement() *endorsement.Endorsement {
	return ecm.endorsement
}

// Height returns the height of this message
func (ecm *EndorsedConsensusMessage) Height() uint64 {
	return ecm.height
}

// Proto converts an endorsement to endorse proto
func (ecm *EndorsedConsensusMessage) Proto() (*iotextypes.ConsensusMessage, error) {
	ebp, err := ecm.endorsement.Proto()
	if err != nil {
		return nil, err
	}
	cmsg := &iotextypes.ConsensusMessage{
		Height:      ecm.height,
		Endorsement: ebp,
	}
	switch message := ecm.message.(type) {
	case *ConsensusVote:
		mbp, err := message.Proto()
		if err != nil {
			return nil, err
		}
		cmsg.Msg = &iotextypes.ConsensusMessage_Vote{Vote: mbp}
	case *blockProposal:
		mbp, err := message.Proto()
		if err != nil {
			return nil, err
		}
		cmsg.Msg = &iotextypes.ConsensusMessage_BlockProposal{BlockProposal: mbp}
	default:
		return nil, errors.New("unknown consensus message type")
	}

	return cmsg, nil
}

// LoadProto creates an endorsement message from protobuf message
func (ecm *EndorsedConsensusMessage) LoadProto(ctx context.Context, msg *iotextypes.ConsensusMessage) error {
	switch {
	case msg.GetVote() != nil:
		vote := &ConsensusVote{}
		if err := vote.LoadProto(msg.GetVote()); err != nil {
			return err
		}
		ecm.message = vote
	case msg.GetBlockProposal() != nil:
		proposal := &blockProposal{}
		if err := proposal.LoadProto(ctx, msg.GetBlockProposal()); err != nil {
			return err
		}
		ecm.message = proposal
	default:
		return errors.New("unknown message")
	}
	ecm.height = msg.Height
	ecm.endorsement = &endorsement.Endorsement{}

	return ecm.endorsement.LoadProto(msg.GetEndorsement())
}
