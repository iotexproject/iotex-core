// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsement

import (
	"bytes"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
)

// Set is a collection of endorsements for block
type Set struct {
	blkHash      hash.Hash32B
	endorsements []*Endorsement
}

// AddEndorsement adds an endorsement with the right block hash and signature
func (s *Set) AddEndorsement(en *Endorsement) error {
	if !bytes.Equal(en.ConsensusVote().BlkHash[:], s.blkHash[:]) {
		return errors.New("the endorsement block hash is different from lock")
	}
	if !en.VerifySignature() {
		return errors.New("invalid signature in endorsement")
	}
	s.endorsements = append(s.endorsements, en)

	return nil
}

func (s *Set) blockHash() hash.Hash32B {
	return s.blkHash
}

func (s *Set) numOfValidEndorsements(topics []ConsensusVoteTopic, endorsers []*iotxaddress.Address) uint16 {
	topicSet := map[ConsensusVoteTopic]bool{}
	for _, topic := range topics {
		topicSet[topic] = true
	}
	endorserSet := map[keypair.PublicKey]bool{}
	for _, endorser := range endorsers {
		endorserSet[endorser.PublicKey] = true
	}
	cnt := uint16(0)
	for _, endorsement := range s.endorsements {
		if _, ok := topicSet[endorsement.ConsensusVote().Topic]; !ok {
			continue
		}
		if _, ok := endorserSet[endorsement.endorserPubkey]; !ok {
			continue
		}
		cnt++
	}

	return cnt
}
