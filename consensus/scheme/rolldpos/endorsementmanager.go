// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
)

var (
	// ErrExpiredEndorsement indicates that the endorsement is expired
	ErrExpiredEndorsement = errors.New("the endorsement has been replaced or expired")
	collectionDB          db.KVStore
	cfg                   = config.Default.DB
	nameSpace             = "endorsement"
)

func init() {
	path := "endorsement.bolt"
	cfg.DbPath = path
	collectionDB = db.NewOnDiskDB(cfg)
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
	managerKey  []byte
	collections map[string]*blockEndorsementCollection
}

func (ee *endorserEndorsementCollection) fromProto(endorserPro *EndorserEndorsementCollection) (string, error) {
	ee = &endorserEndorsementCollection{}
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

func (bc *blockEndorsementCollection) fromProto(blockPro *BlockEndorsementCollection) (string, error) {
	bc = &blockEndorsementCollection{}
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
		var ee *endorserEndorsementCollection
		endorser, err := ee.fromProto(endorsement)
		if err != nil {
			return "", err
		}
		bc.endorsers[endorser] = ee
	}
	return encodedBlockHash, nil
}

func (m *endorsementManager) fromProto(managerPro *ManagerCollection) error {
	m = &endorsementManager{}
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
	m.managerKey = m.managerKey
	return nil
}

func (ee *endorserEndorsementCollection) toProto(endorser string) (*EndorserEndorsementCollection, error) {
	eeProto := &EndorserEndorsementCollection{}
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

func (bc *blockEndorsementCollection) toProto() (*BlockEndorsementCollection, error) {
	bcProto := &BlockEndorsementCollection{}
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

func (m *endorsementManager) toProto() (*ManagerCollection, error) {
	mc := &ManagerCollection{}
	for _, block := range m.collections {
		ioBlockEndorsement, err := block.toProto()
		if err != nil {
			return nil, err
		}
		mc.BlockEndorsements = append(mc.BlockEndorsements, ioBlockEndorsement)
	}
	return mc, nil
}

func newEndorsementManager() *endorsementManager {
	timeKey := time.Now().UnixNano()
	timeBytes := []byte(strconv.FormatInt(timeKey, 10))
	em := &endorsementManager{
		managerKey:  timeBytes,
		collections: map[string]*blockEndorsementCollection{},
	}
	managerCollection, err := em.toProto()
	if err != nil {
		fmt.Println(err)
		return nil
	}
	mapBytes, err := proto.Marshal(managerCollection)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	collectionDB.Put(nameSpace, timeBytes, mapBytes)
	return em
}

func (m *endorsementManager) CollectionByBlockHash(blkHash []byte) *blockEndorsementCollection {
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	managerCollection := &ManagerCollection{}
	err = proto.Unmarshal(collectionBytes, managerCollection)
	err = m.fromProto(managerCollection)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	encodedBlockHash := encodeToString(blkHash)
	c, exists := m.collections[encodedBlockHash]
	if !exists {
		return nil
	}
	fmt.Println(c)
	return c
}

func (m *endorsementManager) Size() int {
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	if err != nil {
		return 0
	}
	managerCollection := &ManagerCollection{}
	err = proto.Unmarshal(collectionBytes, managerCollection)
	err = m.fromProto(managerCollection)
	if err != nil {
		return 0
	}
	return len(m.collections)
}

func (m *endorsementManager) SizeWithBlock() int {
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	if err != nil {
		return 0
	}
	managerCollection := &ManagerCollection{}
	err = proto.Unmarshal(collectionBytes, managerCollection)
	err = m.fromProto(managerCollection)
	if err != nil {
		return 0
	}
	size := 0
	for _, c := range m.collections {
		if c.Block() != nil {
			size++
		}
	}
	return size
}

func (m *endorsementManager) RegisterBlock(blk *block.Block) error {
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	if err != nil {
		return err
	}
	managerCollection := &ManagerCollection{}
	err = proto.Unmarshal(collectionBytes, managerCollection)
	err = m.fromProto(managerCollection)
	if err != nil {
		return err
	}
	blkHash := blk.HashBlock()
	encodedBlockHash := encodeToString(blkHash[:])
	if c, exists := m.collections[encodedBlockHash]; exists {
		return c.SetBlock(blk)
	}
	m.collections[encodedBlockHash] = newBlockEndorsementCollection(blk)
	mc, err := m.toProto()
	if err != nil {
		return err
	}
	mapBytes, err := proto.Marshal(mc)
	if err != nil {
		fmt.Println(err)
		return err
	}
	collectionDB.Put(nameSpace, m.managerKey, mapBytes)
	return nil
}

func (m *endorsementManager) AddVoteEndorsement(
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	if err != nil {
		return err
	}
	managerCollection := &ManagerCollection{}
	err = proto.Unmarshal(collectionBytes, managerCollection)
	err = m.fromProto(managerCollection)
	if err != nil {
		return err
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
	mc, err := m.toProto()
	if err != nil {
		return err
	}
	mapBytes, err := proto.Marshal(mc)
	if err != nil {
		fmt.Println(err)
		return err
	}
	collectionDB.Put(nameSpace, m.managerKey, mapBytes)
	return nil
}

func (m *endorsementManager) Cleanup(timestamp time.Time) *endorsementManager {
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	managerCollection := &ManagerCollection{}
	err = proto.Unmarshal(collectionBytes, managerCollection)
	err = m.fromProto(managerCollection)
	if err != nil {
		fmt.Println(err)
		return nil
	}
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
