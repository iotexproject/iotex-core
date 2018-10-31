// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"github.com/pkg/errors"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/proto"
)

// ConsensusVoteTopic defines the topic of an consensus vote
type ConsensusVoteTopic uint8

const (
	// PROPOSAL stands for an consensus vote to endorse a block proposal
	PROPOSAL ConsensusVoteTopic = 0
	// LOCK stands for an consensus vote to endorse a lock on a proposed block
	LOCK ConsensusVoteTopic = 1
	// COMMIT stands for an consensus vote to endorse a block commit
	COMMIT ConsensusVoteTopic = 2
)

// ConsensusVote is a vote on a given topic for a block on a specific height
type ConsensusVote struct {
	BlkHash hash.Hash32B
	Height  uint64
	Round   uint32
	Topic   ConsensusVoteTopic
}

// NewConsensusVote creates a consensus vote
func NewConsensusVote(blkHash hash.Hash32B, height uint64, round uint32, topic ConsensusVoteTopic) *ConsensusVote {
	return &ConsensusVote{
		blkHash,
		height,
		round,
		topic,
	}
}

// Hash returns a Hash32B for the consensus vote
func (en *ConsensusVote) Hash() hash.Hash32B {
	stream := byteutil.Uint64ToBytes(en.Height)
	stream = append(stream, uint8(en.Topic))
	stream = append(stream, byteutil.Uint32ToBytes(en.Round)...)
	stream = append(stream, en.BlkHash[:]...)

	return blake2b.Sum256(stream)
}

// Endorsement is a stamp on a consensus vote
type Endorsement struct {
	object         *ConsensusVote
	endorser       string
	endorserPubkey keypair.PublicKey
	signature      []byte
}

// NewEndorsement creates an Endorsement for an consensus vote
func NewEndorsement(object *ConsensusVote, endorser *iotxaddress.Address) *Endorsement {
	hash := object.Hash()
	return &Endorsement{
		object:         object,
		endorser:       endorser.RawAddress,
		endorserPubkey: endorser.PublicKey,
		signature:      crypto.EC283.Sign(endorser.PrivateKey, hash[:]),
	}
}

// ConsensusVote returns the Object of the endorse for signature
func (en *Endorsement) ConsensusVote() *ConsensusVote {
	return en.object
}

// Endorser returns the endorser of this endorsement
func (en *Endorsement) Endorser() string {
	return en.endorser
}

// EndorserPublicKey returns the public key of the endorser of this endorsement
func (en *Endorsement) EndorserPublicKey() keypair.PublicKey {
	return en.endorserPubkey
}

// Signature returns the signature of this endorsement
func (en *Endorsement) Signature() []byte {
	return en.signature
}

// VerifySignature verifies that the endorse with pubkey
func (en *Endorsement) VerifySignature() bool {
	hash := en.object.Hash()
	return crypto.EC283.Verify(en.endorserPubkey, hash[:], en.signature)
}

// ToProtoMsg converts an endorsement to endorse proto
func (en *Endorsement) ToProtoMsg() *iproto.EndorsePb {
	vote := en.ConsensusVote()
	var topic iproto.EndorsePb_ConsensusVoteTopic
	switch vote.Topic {
	case PROPOSAL:
		topic = iproto.EndorsePb_PROPOSAL
	case LOCK:
		topic = iproto.EndorsePb_LOCK
	case COMMIT:
		topic = iproto.EndorsePb_COMMIT
	default:
		logger.Error().Msgf("Endorsement object is of the wrong topic")
		return nil
	}
	pubkey := en.EndorserPublicKey()
	return &iproto.EndorsePb{
		Height:         vote.Height,
		Round:          vote.Round,
		BlockHash:      vote.BlkHash[:],
		Topic:          topic,
		Endorser:       en.Endorser(),
		EndorserPubKey: pubkey[:],
		Decision:       true,
		Signature:      en.Signature(),
	}
}

// FromProtoMsg creates an endorsement from endorsePb
func FromProtoMsg(endorsePb *iproto.EndorsePb) (*Endorsement, error) {
	var topic ConsensusVoteTopic
	switch endorsePb.Topic {
	case iproto.EndorsePb_PROPOSAL:
		topic = PROPOSAL
	case iproto.EndorsePb_LOCK:
		topic = LOCK
	case iproto.EndorsePb_COMMIT:
		topic = COMMIT
	default:
		return nil, errors.New("Invalid topic")
	}
	vote := NewConsensusVote(
		byteutil.BytesTo32B(endorsePb.BlockHash),
		endorsePb.Height,
		endorsePb.Round,
		topic,
	)
	pubKey, err := keypair.BytesToPublicKey(endorsePb.EndorserPubKey)
	if err != nil {
		logger.Error().
			Err(err).
			Bytes("endorserPubKey", endorsePb.EndorserPubKey).
			Msg("error when constructing endorse from proto message")
		return nil, err
	}
	return &Endorsement{
		object:         vote,
		endorser:       endorsePb.Endorser,
		endorserPubkey: pubKey,
		signature:      endorsePb.Signature,
	}, nil
}
