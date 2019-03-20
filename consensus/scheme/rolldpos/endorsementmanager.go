// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/endorsement"
)

var (
	// ErrExpiredEndorsement indicates that the endorsement is expired
	ErrExpiredEndorsement = errors.New("the endorsement has been replaced or expired")
)

type endorserEndorsementCollection struct {
	endorsements map[ConsensusVoteTopic]*endorsement.Endorsement
}

func newEndorserEndorsementCollection() *endorserEndorsementCollection {
	return &endorserEndorsementCollection{
		endorsements: map[ConsensusVoteTopic]*endorsement.Endorsement{},
	}
}

func (ee *endorserEndorsementCollection) AddEndorsement(
	topic ConsensusVoteTopic,
	en *endorsement.Endorsement,
) error {
	if e, exists := ee.endorsements[topic]; exists {
		if e.Timestamp().After(en.Timestamp()) {
			return ErrExpiredEndorsement
		}
	}
	ee.endorsements[topic] = en

	return nil
}

func (ee *endorserEndorsementCollection) Cleanup(timestamp time.Time) *endorserEndorsementCollection {
	cleaned := newEndorserEndorsementCollection()
	if e, exists := ee.endorsements[PROPOSAL]; exists {
		if !e.Timestamp().Before(timestamp) {
			cleaned.endorsements[PROPOSAL] = e
		}
	}
	if e, exists := ee.endorsements[LOCK]; exists {
		if !e.Timestamp().Before(timestamp) {
			cleaned.endorsements[LOCK] = e
		}
	}
	if e, exists := ee.endorsements[COMMIT]; exists {
		cleaned.endorsements[COMMIT] = e
	}
	return cleaned
}

func (ee *endorserEndorsementCollection) Endorsement(
	topic ConsensusVoteTopic,
) *endorsement.Endorsement {
	return ee.endorsements[topic]
}

type blockEndorsementCollection struct {
	blk       *block.Block
	endorsers map[string]*endorserEndorsementCollection
}

func newBlockEndorsementCollection(blk *block.Block) *blockEndorsementCollection {
	return &blockEndorsementCollection{
		blk:       blk,
		endorsers: map[string]*endorserEndorsementCollection{},
	}
}

func (bc *blockEndorsementCollection) SetBlock(blk *block.Block) error {
	bc.blk = blk
	return nil
}

func (bc *blockEndorsementCollection) Block() *block.Block {
	return bc.blk
}

func (bc *blockEndorsementCollection) AddEndorsement(
	topic ConsensusVoteTopic,
	en *endorsement.Endorsement,
) error {
	endorser := en.Endorser().HexString()
	ee, exists := bc.endorsers[endorser]
	if !exists {
		ee = newEndorserEndorsementCollection()
	}
	if err := ee.AddEndorsement(topic, en); err != nil {
		return err
	}
	bc.endorsers[endorser] = ee

	return nil
}

func (bc *blockEndorsementCollection) Cleanup(timestamp time.Time) *blockEndorsementCollection {
	cleaned := newBlockEndorsementCollection(bc.blk)
	for endorser, ee := range bc.endorsers {
		cleaned.endorsers[endorser] = ee.Cleanup(timestamp)
	}
	return cleaned
}

func (bc *blockEndorsementCollection) Endorsements(
	topics []ConsensusVoteTopic,
) []*endorsement.Endorsement {
	endorsements := []*endorsement.Endorsement{}
	for _, ee := range bc.endorsers {
		for _, topic := range topics {
			if en := ee.Endorsement(topic); en != nil {
				endorsements = append(endorsements, en)
				break
			}
		}
	}
	return endorsements
}

type endorsementManager struct {
	collections map[string]*blockEndorsementCollection
}

func newEndorsementManager() *endorsementManager {
	return &endorsementManager{
		collections: map[string]*blockEndorsementCollection{},
	}
}

func (m *endorsementManager) CollectionByBlockHash(blkHash []byte) *blockEndorsementCollection {
	encodedBlockHash := encodeToString(blkHash)
	c, exists := m.collections[encodedBlockHash]
	if !exists {
		return nil
	}
	return c
}

func (m *endorsementManager) Size() int {
	return len(m.collections)
}

func (m *endorsementManager) SizeWithBlock() int {
	size := 0
	for _, c := range m.collections {
		if c.Block() != nil {
			size++
		}
	}
	return size
}

func (m *endorsementManager) RegisterBlock(blk *block.Block) error {
	blkHash := blk.HashBlock()
	encodedBlockHash := encodeToString(blkHash[:])
	if c, exists := m.collections[encodedBlockHash]; exists {
		return c.SetBlock(blk)
	}
	m.collections[encodedBlockHash] = newBlockEndorsementCollection(blk)

	return nil
}

func (m *endorsementManager) AddVoteEndorsement(
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	encoded := encodeToString(vote.BlockHash())
	c, exists := m.collections[encoded]
	if !exists {
		c = newBlockEndorsementCollection(nil)
	}
	if err := c.AddEndorsement(vote.Topic(), en); err != nil {
		return err
	}
	m.collections[encoded] = c

	return nil
}

func (m *endorsementManager) Cleanup(timestamp time.Time) *endorsementManager {
	cleaned := newEndorsementManager()
	for encoded, c := range m.collections {
		cleaned.collections[encoded] = c.Cleanup(timestamp)
	}

	return cleaned
}

func (m *endorsementManager) Log(
	logger *zap.Logger,
	delegates []string,
) *zap.Logger {
	for encoded, c := range m.collections {
		proposalEndorsements := c.Endorsements(
			[]ConsensusVoteTopic{PROPOSAL},
		)
		lockEndorsements := c.Endorsements(
			[]ConsensusVoteTopic{LOCK},
		)
		commitEndorsments := c.Endorsements(
			[]ConsensusVoteTopic{COMMIT},
		)
		return logger.With(
			zap.Int("numProposals:"+encoded, len(proposalEndorsements)),
			zap.Int("numLocks:"+encoded, len(lockEndorsements)),
			zap.Int("numCommits:"+encoded, len(commitEndorsments)),
		)
	}
	return logger
}

func encodeToString(hash []byte) string {
	return hex.EncodeToString(hash)
}
