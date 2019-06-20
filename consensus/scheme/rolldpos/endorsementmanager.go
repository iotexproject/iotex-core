// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

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
	managerKey []byte
}

func newEndorsementManager() *endorsementManager {
	timeKey := time.Now().UnixNano()
	timeBytes := []byte(strconv.FormatInt(timeKey, 10))
	var managerMap map[string]*blockEndorsementCollection
	mapBytes, err := getBytes(managerMap)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	collectionDB.Put(nameSpace, timeBytes, mapBytes)

	return &endorsementManager{
		managerKey: timeBytes,
	}
}

func getBytes(data interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func getInterface(bts []byte, data interface{}) error {
	buf := bytes.NewBuffer(bts)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(data)
	if err != nil {
		return err
	}
	return nil
}

func (m *endorsementManager) CollectionByBlockHash(blkHash []byte) *blockEndorsementCollection {
	var managerMap map[string]*blockEndorsementCollection
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return nil
	}
	encodedBlockHash := encodeToString(blkHash)
	c, exists := managerMap[encodedBlockHash]
	if !exists {
		return nil
	}
	return c
}

func (m *endorsementManager) Size() int {
	var managerMap map[string]*blockEndorsementCollection
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return 0
	}
	return len(managerMap)
}

func (m *endorsementManager) SizeWithBlock() int {
	var managerMap map[string]*blockEndorsementCollection
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return 0
	}
	size := 0
	for _, c := range managerMap {
		if c.Block() != nil {
			size++
		}
	}
	return size
}

func (m *endorsementManager) RegisterBlock(blk *block.Block) error {
	blkHash := blk.HashBlock()
	encodedBlockHash := encodeToString(blkHash[:])
	var managerMap map[string]*blockEndorsementCollection
	fmt.Println(m.managerKey)
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return err
	}
	if c, exists := managerMap[encodedBlockHash]; exists {
		return c.SetBlock(blk)
	}
	managerMap[encodedBlockHash] = newBlockEndorsementCollection(blk)
	mapBytes, err := getBytes(managerMap)
	if err != nil {
		return err
	}
	collectionDB.Put(nameSpace, m.managerKey, mapBytes)

	return nil
}

func (m *endorsementManager) AddVoteEndorsement(
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	var managerMap map[string]*blockEndorsementCollection
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return err
	}
	encoded := encodeToString(vote.BlockHash())
	c, exists := managerMap[encoded]
	if !exists {
		c = newBlockEndorsementCollection(nil)
	}
	if err := c.AddEndorsement(vote.Topic(), en); err != nil {
		return err
	}
	managerMap[encoded] = c
	mapBytes, err := getBytes(managerMap)
	if err != nil {
		return err
	}
	collectionDB.Put(nameSpace, m.managerKey, mapBytes)
	return nil
}

func (m *endorsementManager) Cleanup(timestamp time.Time) *endorsementManager {
	cleaned := newEndorsementManager()
	var managerMap map[string]*blockEndorsementCollection
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return nil
	}
	var cleanMap map[string]*blockEndorsementCollection
	cleanBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(cleanBytes, &managerMap)
	if err != nil {
		return nil
	}
	for encoded, c := range managerMap {
		cleanMap[encoded] = c.Cleanup(timestamp)
	}

	return cleaned
}

func (m *endorsementManager) Log(
	logger *zap.Logger,
	delegates []string,
) *zap.Logger {
	var managerMap map[string]*blockEndorsementCollection
	collectionBytes, err := collectionDB.Get(nameSpace, m.managerKey)
	err = getInterface(collectionBytes, &managerMap)
	if err != nil {
		return nil
	}
	for encoded, c := range managerMap {
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
