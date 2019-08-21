// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme/rolldpos/endorsementpb"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
)

var (
	// ErrExpiredEndorsement indicates that the endorsement is expired
	ErrExpiredEndorsement = errors.New("the endorsement has been replaced or expired")
	cfg                   = config.Default.DB
	namespace             = "endorsementManager"
	key                   = []byte("managerKey")
	collectionDB          db.KVStore
)

func init() {
	path := "consensus-status.bolt"
	cfg.DbPath = path
	collectionDB = db.NewBoltDB(cfg)
	ctx := context.Background()
	err := collectionDB.Start(ctx)
	if err != nil {
		return
	}
}

type endorserEndorsementCollection struct {
	endorsements map[ConsensusVoteTopic]*endorsement.Endorsement
}

func newEndorserEndorsementCollection() *endorserEndorsementCollection {
	return &endorserEndorsementCollection{
		endorsements: map[ConsensusVoteTopic]*endorsement.Endorsement{},
	}
}

func (ee *endorserEndorsementCollection) fromProto(endorserPro *endorsementpb.EndorserEndorsementCollection) (string, error) {
	ee.endorsements = make(map[ConsensusVoteTopic]*endorsement.Endorsement)
	for index := range endorserPro.Topics {
		endorse := &endorsement.Endorsement{}
		err := endorse.LoadProto(endorserPro.Endorsements[index])
		if err != nil {
			return "", err
		}
		ee.endorsements[ConsensusVoteTopic(endorserPro.Topics[index])] = endorse
	}
	return endorserPro.Endorser, nil
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

func (bc *blockEndorsementCollection) fromProto(blockPro *endorsementpb.BlockEndorsementCollection) (string, error) {
	bc.endorsers = make(map[string]*endorserEndorsementCollection)
	blk := &block.Block{}
	if blockPro.Blk != nil {
		err := blk.ConvertFromBlockPb(blockPro.Blk)
		if err != nil {
			return "", err
		}
	}
	bc.blk = blk
	blkHash := blk.HashBlock()
	encodedBlockHash := encodeToString(blkHash[:])
	for _, endorsement := range blockPro.BlockMap {
		ee := &endorserEndorsementCollection{}
		endorser, err := ee.fromProto(endorsement)
		if err != nil {
			return "", err
		}
		bc.endorsers[endorser] = ee
	}
	return encodedBlockHash, nil
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
	collections map[string]*blockEndorsementCollection
}

func newEndorsementManager() *endorsementManager {
	bytes, err := collectionDB.Get(namespace, key)
	if err != nil {
		fmt.Println(err)
		//If DB doesn't have any information
		return &endorsementManager{
			collections: map[string]*blockEndorsementCollection{},
		}
	}
	//Get from DB
	manager := &endorsementManager{}
	//manager.Deserialize(bytes)
	managerProto := &endorsementpb.EndorsementManager{}
	err = proto.Unmarshal(bytes, managerProto)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	err = manager.fromProto(managerProto)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	if manager.collections == nil {
		return manager
	}
	return manager
}

func (m *endorsementManager) fromProto(managerPro *endorsementpb.EndorsementManager) error {
	m.collections = make(map[string]*blockEndorsementCollection)
	for _, block := range managerPro.BlockEndorsements {
		bc := &blockEndorsementCollection{}
		if block.Blk == nil {
			continue
		}
		blkHash, err := bc.fromProto(block)
		if err != nil {
			return err
		}
		m.collections[blkHash] = bc

	}
	return nil
}

func (m *endorsementManager) toProto() (*endorsementpb.EndorsementManager, error) {
	mc := &endorsementpb.EndorsementManager{}
	for _, block := range m.collections {
		ioBlockEndorsement, err := block.toProto()
		if err != nil {
			return nil, err
		}
		mc.BlockEndorsements = append(mc.BlockEndorsements, ioBlockEndorsement)
	}
	return mc, nil
}

func (m *endorsementManager) CollectionByBlockHash(blkHash []byte) *blockEndorsementCollection {
	encodedBlockHash := encodeToString(blkHash)
	collections, exists := m.collections[encodedBlockHash]
	if !exists {
		return nil
	}
	fmt.Println(collections)
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

	managerProto, err := m.toProto()
	if err != nil {
		return err
	}
	valBytes, err := proto.Marshal(managerProto)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = collectionDB.Delete(namespace, key)
	if err != nil {
		return err
	}
	err = collectionDB.Put(namespace, key, valBytes)
	if err != nil {
		return err
	}
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

	managerProto, err := m.toProto()
	if err != nil {
		return err
	}
	valBytes, err := proto.Marshal(managerProto)
	if err != nil {
		fmt.Println(err)
		return err
	}
	err = collectionDB.Delete(namespace, key)
	if err != nil {
		return err
	}
	err = collectionDB.Put(namespace, key, valBytes)
	if err != nil {
		return err
	}
	return nil
}

func (m *endorsementManager) Cleanup(timestamp time.Time) error {
	cleaned := newEndorsementManager()
	for encoded, c := range m.collections {
		cleaned.collections[encoded] = c.Cleanup(timestamp)
	}
	managerProto, err := cleaned.toProto()
	if err != nil {
		return err
	}
	valBytes, err := proto.Marshal(managerProto)
	if err != nil {
		return err
	}
	err = collectionDB.Delete(namespace, key)
	if err != nil {
		return err
	}
	err = collectionDB.Put(namespace, key, valBytes)
	if err != nil {
		return err
	}
	m = cleaned
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
