// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"encoding/hex"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/blake2b"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/crypto/key"
	"github.com/iotexproject/iotex-core/pkg/hash"
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
	endorserPubkey []byte
	signature      []byte
}

// NewEndorsement creates an Endorsement for an consensus vote
func NewEndorsement(object *ConsensusVote, endorserPubKey, endorserPriKey []byte, endorserAddr string) *Endorsement {
	hash := object.Hash()

	sk, err := key.NewPrivateKeyFromBytes(endorserPriKey)
	if err != nil {
		log.L().Error(errors.Wrapf(err, key.ErrInvalidKey.Error()).Error())
		return nil
	}
	sig, err := sk.Sign(hash[:])
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
func (en *Endorsement) EndorserPublicKey() []byte {
	return en.endorserPubkey
}

// Signature returns the signature of this endorsement
func (en *Endorsement) Signature() []byte {
	return en.signature
}

// VerifySignature verifies that the endorse with pubkey
func (en *Endorsement) VerifySignature() bool {
	hash := en.object.Hash()

	pk, err := key.NewPublicKeyFromBytes(en.endorserPubkey)
	if err != nil {
		return false
	}
	return pk.Verify(hash[:], en.signature[:action.SignatureLength-1])
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
		EndorserPubKey: pubkey,
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
	en.object = vote
	en.endorser = endorsePb.Endorser
	en.endorserPubkey = endorsePb.EndorserPubKey
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

// MarshalLogObject marshals the endorsement to a zap object
func (en *Endorsement) MarshalLogObject(oe zapcore.ObjectEncoder) error {
	oe.AddUint8("topic", uint8(en.object.Topic))
	oe.AddString("warrantee", hex.EncodeToString(en.object.BlkHash))
	oe.AddUint64("height", en.object.Height)
	oe.AddUint32("round", en.object.Round)
	oe.AddString("endorser", en.endorser)

	return nil
}
