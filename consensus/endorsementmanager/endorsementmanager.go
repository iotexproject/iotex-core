// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package endorsementmanager

import (
	"encoding/hex"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/consensus/endorsementmanager/endorsementpb"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	eManagerNS = "edm"
)

var (
	// ErrExpiredEndorsement indicates that the endorsement is expired
	ErrExpiredEndorsement = errors.New("the endorsement has been replaced or expired")
	statusKey             = []byte("status")
)

//EndorsedByMajorityFunc defines a function to give an information of consensus status
type EndorsedByMajorityFunc func(blockHash []byte, topics []ConsensusVoteTopic) bool

// EndorsementManager defines the endorsement manager
type EndorsementManager struct {
	isMajorityFunc EndorsedByMajorityFunc
	eManagerDB     db.KVStore
	collections    map[string]*BlockEndorsementCollection
}

// NewEndorsementManager creates a new endorsement manager
func NewEndorsementManager(eManagerDB db.KVStore) (*EndorsementManager, error) {
	if eManagerDB == nil {
		return &EndorsementManager{
			eManagerDB:  nil,
			collections: map[string]*BlockEndorsementCollection{},
		}, nil
	}
	bytes, err := eManagerDB.Get(eManagerNS, statusKey)
	switch errors.Cause(err) {
	case nil:
		// Get from DB
		manager := &EndorsementManager{eManagerDB: eManagerDB}
		managerProto := &endorsementpb.EndorsementManager{}
		if err = proto.Unmarshal(bytes, managerProto); err != nil {
			return nil, err
		}
		if err = manager.fromProto(managerProto); err != nil {
			return nil, err
		}
		manager.eManagerDB = eManagerDB
		return manager, nil
	case db.ErrNotExist:
		// If DB doesn't have any information
		log.L().Info("First initializing DB")
		return &EndorsementManager{
			eManagerDB:  eManagerDB,
			collections: map[string]*BlockEndorsementCollection{},
		}, nil
	default:
		return nil, err
	}
}

// PutEndorsementManagerToDB stores endorsement manager to database
func (m *EndorsementManager) PutEndorsementManagerToDB() error {
	managerProto, err := m.toProto()
	if err != nil {
		return err
	}
	valBytes, err := proto.Marshal(managerProto)
	if err != nil {
		return err
	}
	err = m.eManagerDB.Put(eManagerNS, statusKey, valBytes)
	if err != nil {
		return err
	}
	return nil
}

// SetIsMajorityFunc sets IsMajority function
func (m *EndorsementManager) SetIsMajorityFunc(isMajorityFunc EndorsedByMajorityFunc) {
	m.isMajorityFunc = isMajorityFunc
	return
}

// CollectionByBlockHash gets block endorsement collection by block hash
func (m *EndorsementManager) CollectionByBlockHash(blkHash []byte) *BlockEndorsementCollection {
	encodedBlockHash := encodeToString(blkHash)
	collections, exists := m.collections[encodedBlockHash]
	if !exists {
		return nil
	}
	return collections
}

// Size gets the size of collection
func (m *EndorsementManager) Size() int {
	return len(m.collections)
}

// SizeWithBlock gets the size of collection in terms of number of blocks
func (m *EndorsementManager) SizeWithBlock() int {
	size := 0
	for _, c := range m.collections {
		if c.Block() != nil {
			size++
		}
	}
	return size
}

// RegisterBlock registers a block
func (m *EndorsementManager) RegisterBlock(blk *block.Block) error {
	blkHash := blk.HashBlock()
	encodedBlockHash := encodeToString(blkHash[:])
	if c, exists := m.collections[encodedBlockHash]; exists {
		return c.SetBlock(blk)
	}
	m.collections[encodedBlockHash] = newBlockEndorsementCollection(blk)

	if m.eManagerDB != nil {
		return m.PutEndorsementManagerToDB()
	}
	return nil
}

// AddVoteEndorsement adds a vote for endorsement
func (m *EndorsementManager) AddVoteEndorsement(
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	var beforeVote, afterVote bool
	if m.isMajorityFunc != nil {
		beforeVote = m.isMajorityFunc(vote.BlockHash(), []ConsensusVoteTopic{vote.Topic()})
	}
	encoded := encodeToString(vote.BlockHash())
	c, exists := m.collections[encoded]
	if !exists {
		c = newBlockEndorsementCollection(nil)
	}
	if err := c.AddEndorsement(vote.Topic(), en); err != nil {
		return err
	}
	m.collections[encoded] = c

	if m.eManagerDB != nil && m.isMajorityFunc != nil {
		afterVote = m.isMajorityFunc(vote.BlockHash(), []ConsensusVoteTopic{vote.Topic()})
		if !beforeVote && afterVote {
			//put into DB only it changes the status of consensus
			return m.PutEndorsementManagerToDB()
		}
	}
	return nil
}

// Cleanup cleans up block endorsement collection
func (m *EndorsementManager) Cleanup(timestamp time.Time) error {
	if !timestamp.IsZero() {
		for encoded, c := range m.collections {
			m.collections[encoded] = c.Cleanup(timestamp)
		}
	} else {
		m.collections = map[string]*BlockEndorsementCollection{}
	}
	if m.eManagerDB != nil {
		return m.PutEndorsementManagerToDB()
	}
	return nil
}

// Log creates logs for endorsements
func (m *EndorsementManager) Log(
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

func (m *EndorsementManager) fromProto(managerPro *endorsementpb.EndorsementManager) error {
	m.collections = make(map[string]*BlockEndorsementCollection)
	for i, block := range managerPro.BlockEndorsements {
		bc := &BlockEndorsementCollection{}
		if err := bc.fromProto(block); err != nil {
			return err
		}
		m.collections[managerPro.BlkHash[i]] = bc
	}
	return nil
}

func (m *EndorsementManager) toProto() (*endorsementpb.EndorsementManager, error) {
	mc := &endorsementpb.EndorsementManager{}
	for encodedBlockHash, block := range m.collections {
		ioBlockEndorsement, err := block.toProto()
		if err != nil {
			return nil, err
		}
		mc.BlkHash = append(mc.BlkHash, encodedBlockHash)
		mc.BlockEndorsements = append(mc.BlockEndorsements, ioBlockEndorsement)
	}
	return mc, nil
}

// EndorserEndorsementCollection defines the structure of endorser endorsement collection
type EndorserEndorsementCollection struct {
	endorsements map[ConsensusVoteTopic]*endorsement.Endorsement
}

func newEndorserEndorsementCollection() *EndorserEndorsementCollection {
	return &EndorserEndorsementCollection{
		endorsements: map[ConsensusVoteTopic]*endorsement.Endorsement{},
	}
}

// AddEndorsement adds an endorsement
func (ee *EndorserEndorsementCollection) AddEndorsement(
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

// Cleanup cleans up endorser endorsement collections
func (ee *EndorserEndorsementCollection) Cleanup(timestamp time.Time) *EndorserEndorsementCollection {
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

// Endorsement returns an endorsement by topic
func (ee *EndorserEndorsementCollection) Endorsement(
	topic ConsensusVoteTopic,
) *endorsement.Endorsement {
	return ee.endorsements[topic]
}

func (ee *EndorserEndorsementCollection) fromProto(endorserPro *endorsementpb.EndorserEndorsementCollection) error {
	ee.endorsements = make(map[ConsensusVoteTopic]*endorsement.Endorsement)
	for index := range endorserPro.Topics {
		endorse := &endorsement.Endorsement{}
		if err := endorse.LoadProto(endorserPro.Endorsements[index]); err != nil {
			return err
		}
		ee.endorsements[ConsensusVoteTopic(endorserPro.Topics[index])] = endorse
	}
	return nil
}

func (ee *EndorserEndorsementCollection) toProto(endorser string) (*endorsementpb.EndorserEndorsementCollection, error) {
	eeProto := &endorsementpb.EndorserEndorsementCollection{}
	eeProto.Endorser = endorser
	for topic, endorse := range ee.endorsements {
		eeProto.Topics = append(eeProto.Topics, uint32(topic))
		ioEndorsement, err := endorse.Proto()
		if err != nil {
			return nil, err
		}
		eeProto.Endorsements = append(eeProto.Endorsements, ioEndorsement)
	}
	return eeProto, nil
}

// BlockEndorsementCollection defines the structure of block endorsement collections
type BlockEndorsementCollection struct {
	blk       *block.Block
	endorsers map[string]*EndorserEndorsementCollection
}

func newBlockEndorsementCollection(blk *block.Block) *BlockEndorsementCollection {
	return &BlockEndorsementCollection{
		blk:       blk,
		endorsers: map[string]*EndorserEndorsementCollection{},
	}
}

// SetBlock sets a block
func (bc *BlockEndorsementCollection) SetBlock(blk *block.Block) error {
	bc.blk = blk
	return nil
}

// Block returns a block
func (bc *BlockEndorsementCollection) Block() *block.Block {
	return bc.blk
}

// AddEndorsement adds an endorsement
func (bc *BlockEndorsementCollection) AddEndorsement(
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

// Endorsement returns an endorsement
func (bc *BlockEndorsementCollection) Endorsement(
	endorser string,
	topic ConsensusVoteTopic,
) *endorsement.Endorsement {
	ee, exists := bc.endorsers[endorser]
	if !exists {
		return nil
	}
	return ee.Endorsement(topic)
}

// Cleanup cleans up block endorsement collection
func (bc *BlockEndorsementCollection) Cleanup(timestamp time.Time) *BlockEndorsementCollection {
	cleaned := newBlockEndorsementCollection(bc.blk)
	for endorser, ee := range bc.endorsers {
		cleaned.endorsers[endorser] = ee.Cleanup(timestamp)
	}
	return cleaned
}

// Endorsements returns endorsements
func (bc *BlockEndorsementCollection) Endorsements(
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

func (bc *BlockEndorsementCollection) fromProto(blockPro *endorsementpb.BlockEndorsementCollection) error {
	bc.endorsers = make(map[string]*EndorserEndorsementCollection)
	if blockPro.Blk == nil {
		bc.blk = nil
	} else {
		blk := &block.Block{}
		if err := blk.ConvertFromBlockPb(blockPro.Blk); err != nil {
			return err
		}
		bc.blk = blk
	}
	for _, endorsement := range blockPro.BlockMap {
		ee := &EndorserEndorsementCollection{}
		if err := ee.fromProto(endorsement); err != nil {
			return err
		}
		bc.endorsers[endorsement.Endorser] = ee
	}
	return nil
}

func (bc *BlockEndorsementCollection) toProto() (*endorsementpb.BlockEndorsementCollection, error) {
	bcProto := &endorsementpb.BlockEndorsementCollection{}
	if bc.blk != nil {
		bcProto.Blk = bc.blk.ConvertToBlockPb()
	}

	for s, endorse := range bc.endorsers {
		ioEndorsement, err := endorse.toProto(s)
		if err != nil {
			return nil, err
		}
		bcProto.BlockMap = append(bcProto.BlockMap, ioEndorsement)
	}
	return bcProto, nil
}

func encodeToString(hash []byte) string {
	return hex.EncodeToString(hash)
}
