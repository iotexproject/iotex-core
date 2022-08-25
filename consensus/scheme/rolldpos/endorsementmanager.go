// Copyright (c) 2019 IoTeX Foundation
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
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos/endorsementpb"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	_eManagerNS = "edm"
)

var (
	// ErrExpiredEndorsement indicates that the endorsement is expired
	ErrExpiredEndorsement = errors.New("the endorsement has been replaced or expired")
	_statusKey            = []byte("status")
)

// EndorsedByMajorityFunc defines a function to give an information of consensus status
type EndorsedByMajorityFunc func(blockHash []byte, topics []ConsensusVoteTopic) bool

type endorserEndorsementCollection struct {
	endorsements map[ConsensusVoteTopic]*endorsement.Endorsement
}

func newEndorserEndorsementCollection() *endorserEndorsementCollection {
	return &endorserEndorsementCollection{
		endorsements: map[ConsensusVoteTopic]*endorsement.Endorsement{},
	}
}

func (ee *endorserEndorsementCollection) fromProto(endorserPro *endorsementpb.EndorserEndorsementCollection) error {
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

func (ee *endorserEndorsementCollection) toProto(endorser string) (*endorsementpb.EndorserEndorsementCollection, error) {
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

func (bc *blockEndorsementCollection) fromProto(blockPro *endorsementpb.BlockEndorsementCollection, deserializer *block.Deserializer) error {
	bc.endorsers = make(map[string]*endorserEndorsementCollection)
	if blockPro.Blk == nil {
		bc.blk = nil
	} else {
		blk, err := deserializer.FromBlockProto(blockPro.Blk)
		if err != nil {
			return err
		}
		bc.blk = blk
	}
	for _, endorsement := range blockPro.BlockMap {
		ee := &endorserEndorsementCollection{}
		if err := ee.fromProto(endorsement); err != nil {
			return err
		}
		bc.endorsers[endorsement.Endorser] = ee
	}
	return nil
}

func (bc *blockEndorsementCollection) toProto() (*endorsementpb.BlockEndorsementCollection, error) {
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

func (bc *blockEndorsementCollection) Endorsement(
	endorser string,
	topic ConsensusVoteTopic,
) *endorsement.Endorsement {
	ee, exists := bc.endorsers[endorser]
	if !exists {
		return nil
	}
	return ee.Endorsement(topic)
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
	isMajorityFunc  EndorsedByMajorityFunc
	eManagerDB      db.KVStore
	collections     map[string]*blockEndorsementCollection
	cachedMintedBlk *block.Block
}

func newEndorsementManager(eManagerDB db.KVStore, deserializer *block.Deserializer) (*endorsementManager, error) {
	if eManagerDB == nil {
		return &endorsementManager{
			eManagerDB:      nil,
			collections:     map[string]*blockEndorsementCollection{},
			cachedMintedBlk: nil,
		}, nil
	}
	bytes, err := eManagerDB.Get(_eManagerNS, _statusKey)
	switch errors.Cause(err) {
	case nil:
		// Get from DB
		manager := &endorsementManager{eManagerDB: eManagerDB}
		managerProto := &endorsementpb.EndorsementManager{}
		if err = proto.Unmarshal(bytes, managerProto); err != nil {
			return nil, err
		}
		if err = manager.fromProto(managerProto, deserializer); err != nil {
			return nil, err
		}
		manager.eManagerDB = eManagerDB
		return manager, nil
	case db.ErrNotExist:
		// If DB doesn't have any information
		log.L().Info("First initializing DB")
		return &endorsementManager{
			eManagerDB:      eManagerDB,
			collections:     map[string]*blockEndorsementCollection{},
			cachedMintedBlk: nil,
		}, nil
	default:
		return nil, err
	}
}

func (m *endorsementManager) PutEndorsementManagerToDB() error {
	managerProto, err := m.toProto()
	if err != nil {
		return err
	}
	valBytes, err := proto.Marshal(managerProto)
	if err != nil {
		return err
	}
	err = m.eManagerDB.Put(_eManagerNS, _statusKey, valBytes)
	if err != nil {
		return err
	}
	return nil
}

func (m *endorsementManager) SetIsMarjorityFunc(isMajorityFunc EndorsedByMajorityFunc) {
	m.isMajorityFunc = isMajorityFunc
}

func (m *endorsementManager) fromProto(managerPro *endorsementpb.EndorsementManager, deserializer *block.Deserializer) error {
	m.collections = make(map[string]*blockEndorsementCollection)
	for i, block := range managerPro.BlockEndorsements {
		bc := &blockEndorsementCollection{}
		if err := bc.fromProto(block, deserializer); err != nil {
			return err
		}
		m.collections[managerPro.BlkHash[i]] = bc
	}
	if managerPro.CachedMintedBlk != nil {
		blk, err := deserializer.FromBlockProto(managerPro.CachedMintedBlk)
		if err != nil {
			return err
		}
		m.cachedMintedBlk = blk
	}
	return nil
}

func (m *endorsementManager) toProto() (*endorsementpb.EndorsementManager, error) {
	mc := &endorsementpb.EndorsementManager{}
	for encodedBlockHash, block := range m.collections {
		ioBlockEndorsement, err := block.toProto()
		if err != nil {
			return nil, err
		}
		mc.BlkHash = append(mc.BlkHash, encodedBlockHash)
		mc.BlockEndorsements = append(mc.BlockEndorsements, ioBlockEndorsement)
	}
	if m.cachedMintedBlk != nil {
		mc.CachedMintedBlk = m.cachedMintedBlk.ConvertToBlockPb()
	}
	return mc, nil
}

func (m *endorsementManager) CollectionByBlockHash(blkHash []byte) *blockEndorsementCollection {
	encodedBlockHash := encodeToString(blkHash)
	collections, exists := m.collections[encodedBlockHash]
	if !exists {
		return nil
	}
	return collections
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

	if m.eManagerDB != nil {
		return m.PutEndorsementManagerToDB()
	}
	return nil
}

func (m *endorsementManager) AddVoteEndorsement(
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

func (m *endorsementManager) SetMintedBlock(blk *block.Block) error {
	m.cachedMintedBlk = blk
	if m.eManagerDB != nil {
		return m.PutEndorsementManagerToDB()
	}
	return nil
}

func (m *endorsementManager) CachedMintedBlock() *block.Block {
	return m.cachedMintedBlk
}

func (m *endorsementManager) Cleanup(timestamp time.Time) error {
	if !timestamp.IsZero() {
		for encoded, c := range m.collections {
			m.collections[encoded] = c.Cleanup(timestamp)
		}
	} else {
		m.collections = map[string]*blockEndorsementCollection{}
	}
	if m.cachedMintedBlk != nil {
		if timestamp.IsZero() || m.cachedMintedBlk.Timestamp().Before(timestamp) {
			// in case that the cached minted block is outdated, clean up
			m.cachedMintedBlk = nil
		}
	}
	if m.eManagerDB != nil {
		return m.PutEndorsementManagerToDB()
	}
	return nil
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
