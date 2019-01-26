// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-ethereum/crypto"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
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
	BlkHash []byte
	Height  uint64
	Round   uint32
	Topic   ConsensusVoteTopic
}

// NewConsensusVote creates a consensus vote
func NewConsensusVote(blkHash []byte, height uint64, round uint32, topic ConsensusVoteTopic) *ConsensusVote {
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
func NewEndorsement(object *ConsensusVote, endorserPubKey keypair.PublicKey, endorserPriKey keypair.PrivateKey, endorserAddr string) *Endorsement {
	hash := object.Hash()
	sig, err := crypto.Sign(hash[:], endorserPriKey)
	if err != nil {
		log.L().Error("Failed to sign endorsement.")
		return nil
	}
	return &Endorsement{
		object:         object,
		endorser:       endorserAddr,
		endorserPubkey: endorserPubKey,
		signature:      sig,
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
	return crypto.VerifySignature(keypair.PublicKeyToBytes(en.endorserPubkey), hash[:], en.signature[:64])
}

// ToProtoMsg converts an endorsement to endorse proto
func (en *Endorsement) ToProtoMsg() *iproto.Endorsement {
	vote := en.ConsensusVote()
	var topic iproto.Endorsement_ConsensusVoteTopic
	switch vote.Topic {
	case PROPOSAL:
		topic = iproto.Endorsement_PROPOSAL
	case LOCK:
		topic = iproto.Endorsement_LOCK
	case COMMIT:
		topic = iproto.Endorsement_COMMIT
	default:
		log.L().Error("Endorsement object is of the wrong topic.")
		return nil
	}
	pubkey := en.EndorserPublicKey()
	return &iproto.Endorsement{
		Height:         vote.Height,
		Round:          vote.Round,
		BlockHash:      vote.BlkHash[:],
		Topic:          topic,
		Endorser:       en.Endorser(),
		EndorserPubKey: keypair.PublicKeyToBytes(pubkey),
		Decision:       true,
		Signature:      en.Signature(),
	}
}

// Serialize converts an endorsement to bytes
func (en *Endorsement) Serialize() ([]byte, error) {
	pb := en.ToProtoMsg()
	if pb == nil {
		return nil, errors.New("error when converting to protobuf")
	}

	return proto.Marshal(pb)
}

// FromProtoMsg creates an endorsement from endorsePb
func (en *Endorsement) FromProtoMsg(endorsePb *iproto.Endorsement) error {
	var topic ConsensusVoteTopic
	switch endorsePb.Topic {
	case iproto.Endorsement_PROPOSAL:
		topic = PROPOSAL
	case iproto.Endorsement_LOCK:
		topic = LOCK
	case iproto.Endorsement_COMMIT:
		topic = COMMIT
	default:
		return errors.New("Invalid topic")
	}
	vote := NewConsensusVote(
		endorsePb.BlockHash,
		endorsePb.Height,
		endorsePb.Round,
		topic,
	)
	pubKey, err := keypair.BytesToPublicKey(endorsePb.EndorserPubKey)
	if err != nil {
		log.L().Error("Error when constructing endorse from proto message.",
			zap.Error(err),
			log.Hex("endorserPubKey", endorsePb.EndorserPubKey))
		return err
	}
	en.object = vote
	en.endorser = endorsePb.Endorser
	en.endorserPubkey = pubKey
	en.signature = endorsePb.Signature

	return nil
}

// Deserialize converts a byte array to endorsement
func (en *Endorsement) Deserialize(bs []byte) error {
	pb := iproto.Endorsement{}
	if err := proto.Unmarshal(bs, &pb); err != nil {
		return err
	}
	if err := en.FromProtoMsg(&pb); err != nil {
		return err
	}

	return nil
}
