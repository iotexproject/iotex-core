// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// EndorsedConsensusMessage is an endorsement on a consensus document. Exactly
// one of endorsement or blsEndorsement is set for any given message: COMMIT
// votes after BLS aggregation is activated carry blsEndorsement; PROPOSAL,
// LOCK, and pre-fork COMMIT votes carry endorsement.
type EndorsedConsensusMessage struct {
	height         uint64
	message        endorsement.Document
	endorsement    *endorsement.Endorsement
	blsEndorsement *endorsement.BLSEndorsement
}

// NewEndorsedConsensusMessage creates an EndorsedConsensusMessage carrying an
// ECDSA endorsement.
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

// NewBLSEndorsedConsensusMessage creates an EndorsedConsensusMessage carrying
// a BLS endorsement. Used for COMMIT-stage votes once BLS aggregation is
// activated (IIP-52).
func NewBLSEndorsedConsensusMessage(
	height uint64,
	message endorsement.Document,
	blsEndorsement *endorsement.BLSEndorsement,
) *EndorsedConsensusMessage {
	return &EndorsedConsensusMessage{
		height:         height,
		message:        message,
		blsEndorsement: blsEndorsement,
	}
}

// Document returns the endorsed consensus message.
func (ecm *EndorsedConsensusMessage) Document() endorsement.Document {
	return ecm.message
}

// Endorsement returns the ECDSA endorsement, or nil if this message carries a
// BLS endorsement instead.
func (ecm *EndorsedConsensusMessage) Endorsement() *endorsement.Endorsement {
	return ecm.endorsement
}

// BLSEndorsement returns the BLS endorsement, or nil if this message carries
// an ECDSA endorsement instead.
func (ecm *EndorsedConsensusMessage) BLSEndorsement() *endorsement.BLSEndorsement {
	return ecm.blsEndorsement
}

// IsBLS reports whether this message carries a BLS endorsement.
func (ecm *EndorsedConsensusMessage) IsBLS() bool {
	return ecm.blsEndorsement != nil
}

// Height returns the height of this message.
func (ecm *EndorsedConsensusMessage) Height() uint64 {
	return ecm.height
}

// Proto converts the endorsed consensus message to its protobuf form.
func (ecm *EndorsedConsensusMessage) Proto() (*iotextypes.ConsensusMessage, error) {
	if ecm.endorsement != nil && ecm.blsEndorsement != nil {
		return nil, errors.New("endorsed consensus message has both endorsement and bls_endorsement set")
	}
	cmsg := &iotextypes.ConsensusMessage{
		Height: ecm.height,
	}
	switch {
	case ecm.endorsement != nil:
		cmsg.Endorsement = ecm.endorsement.Proto()
	case ecm.blsEndorsement != nil:
		cmsg.BlsEndorsement = ecm.blsEndorsement.Proto()
	default:
		return nil, errors.New("endorsed consensus message has no endorsement")
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

// LoadProto creates an endorsement message from protobuf message.
func (ecm *EndorsedConsensusMessage) LoadProto(msg *iotextypes.ConsensusMessage, deserializer *block.Deserializer) error {
	if msg.GetEndorsement() != nil && msg.GetBlsEndorsement() != nil {
		return errors.New("consensus message has both endorsement and bls_endorsement set")
	}
	switch {
	case msg.GetVote() != nil:
		vote := &ConsensusVote{}
		if err := vote.LoadProto(msg.GetVote()); err != nil {
			return err
		}
		ecm.message = vote
	case msg.GetBlockProposal() != nil:
		proposal := &blockProposal{}
		if err := proposal.LoadProto(msg.GetBlockProposal(), deserializer); err != nil {
			return err
		}
		ecm.message = proposal
	default:
		return errors.New("unknown message")
	}
	ecm.height = msg.Height
	switch {
	case msg.GetEndorsement() != nil:
		ecm.endorsement = &endorsement.Endorsement{}
		ecm.blsEndorsement = nil
		return ecm.endorsement.LoadProto(msg.GetEndorsement())
	case msg.GetBlsEndorsement() != nil:
		ecm.blsEndorsement = &endorsement.BLSEndorsement{}
		ecm.endorsement = nil
		return ecm.blsEndorsement.LoadProto(msg.GetBlsEndorsement())
	default:
		return errors.New("consensus message has no endorsement")
	}
}
