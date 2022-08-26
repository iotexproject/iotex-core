// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package consensus

import (
	blake2b "github.com/minio/blake2b-simd"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// VoteTopic defines the topic of an consensus vote
type VoteTopic uint8

const (
	// PROPOSAL stands for an consensus vote to endorse a block proposal
	PROPOSAL VoteTopic = 0
	// LOCK stands for an consensus vote to endorse a lock on a proposed block
	LOCK VoteTopic = 1
	// COMMIT stands for an consensus vote to endorse a block commit
	COMMIT VoteTopic = 2
)

// Vote is a vote on a given topic for a block on a specific height
type Vote struct {
	blkHash []byte
	topic   VoteTopic
}

// NewVote creates a consensus vote
func NewVote(
	blkHash []byte,
	topic VoteTopic,
) *Vote {
	return &Vote{
		blkHash: blkHash,
		topic:   topic,
	}
}

// BlockHash returns the block hash of the consensus vote
func (v *Vote) BlockHash() []byte {
	retval := make([]byte, len(v.blkHash))
	copy(retval, v.blkHash)

	return retval
}

// Topic returns the topic of the consensus vote
func (v *Vote) Topic() VoteTopic {
	return v.topic
}

// Proto converts to a protobuf message
func (v *Vote) Proto() (*iotextypes.ConsensusVote, error) {
	var topic iotextypes.ConsensusVote_Topic
	switch v.topic {
	case PROPOSAL:
		topic = iotextypes.ConsensusVote_PROPOSAL
	case LOCK:
		topic = iotextypes.ConsensusVote_LOCK
	case COMMIT:
		topic = iotextypes.ConsensusVote_COMMIT
	default:
		return nil, errors.Errorf("unsupported topic %d", v.topic)
	}
	hash := make([]byte, len(v.blkHash))
	copy(hash, v.blkHash)
	return &iotextypes.ConsensusVote{
		BlockHash: hash,
		Topic:     topic,
	}, nil
}

// LoadProto loads from a protobuf message
func (v *Vote) LoadProto(msg *iotextypes.ConsensusVote) error {
	switch msg.Topic {
	case iotextypes.ConsensusVote_PROPOSAL:
		v.topic = PROPOSAL
	case iotextypes.ConsensusVote_LOCK:
		v.topic = LOCK
	case iotextypes.ConsensusVote_COMMIT:
		v.topic = COMMIT
	default:
		return errors.Errorf("invalid topic %d", msg.Topic)
	}
	v.blkHash = make([]byte, len(msg.BlockHash))
	copy(v.blkHash, msg.BlockHash)
	return nil
}

// Hash returns the hash of this vote
func (v *Vote) Hash() ([]byte, error) {
	msg, err := v.Proto()
	if err != nil {
		return nil, err
	}
	ser, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	hash := blake2b.Sum256(ser)
	return hash[:], nil
}
