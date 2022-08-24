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
	blkHash []byte
	topic   ConsensusVoteTopic
}

// NewConsensusVote creates a consensus vote
func NewConsensusVote(
	blkHash []byte,
	topic ConsensusVoteTopic,
) *ConsensusVote {
	return &ConsensusVote{
		blkHash: blkHash,
		topic:   topic,
	}
}

// BlockHash returns the block hash of the consensus vote
func (v *ConsensusVote) BlockHash() []byte {
	retval := make([]byte, len(v.blkHash))
	copy(retval, v.blkHash)

	return retval
}

// Topic returns the topic of the consensus vote
func (v *ConsensusVote) Topic() ConsensusVoteTopic {
	return v.topic
}

// Proto converts to a protobuf message
func (v *ConsensusVote) Proto() (*iotextypes.ConsensusVote, error) {
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
func (v *ConsensusVote) LoadProto(msg *iotextypes.ConsensusVote) error {
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
func (v *ConsensusVote) Hash() ([]byte, error) {
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
