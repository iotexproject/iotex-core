// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/binary"
	"encoding/hex"
	"time"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/consensus/scheme/rolldpos/endorsementpb"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

const (
	_eManagerNS = "edm"
)

var (
	// ErrExpiredEndorsement indicates that the endorsement is expired
	ErrExpiredEndorsement = errors.New("the endorsement has been replaced or expired")
	_statusKey            = []byte("status")
	// _lockStateKey stores the round lock separately from _statusKey so that
	// older binaries, which never read it, are unaffected by its presence.
	_lockStateKey = []byte("lockState")
	// _signedVotesKey stores the last vote this node signed per topic, so that
	// a restart within a round cannot make it sign a different block in the
	// same (height, round, topic)
	_signedVotesKey = []byte("signedVotes")
)

const (
	_lockStateVersion   = byte(1)
	_signedVotesVersion = byte(1)
)

// signedVote is the last vote signed by this node for a topic
type signedVote struct {
	height  uint64
	round   uint32
	blkHash []byte
}

func serializeSignedVotes(votes map[ConsensusVoteTopic]*signedVote) ([]byte, error) {
	buf := []byte{_signedVotesVersion, byte(len(votes))}
	for _, topic := range []ConsensusVoteTopic{PROPOSAL, LOCK, COMMIT} {
		sv, ok := votes[topic]
		if !ok {
			continue
		}
		if len(sv.blkHash) > 255 {
			return nil, errors.Errorf("invalid block hash length %d", len(sv.blkHash))
		}
		buf = append(buf, byte(topic))
		buf = binary.BigEndian.AppendUint64(buf, sv.height)
		buf = binary.BigEndian.AppendUint32(buf, sv.round)
		buf = append(buf, byte(len(sv.blkHash)))
		buf = append(buf, sv.blkHash...)
	}
	return buf, nil
}

func deserializeSignedVotes(buf []byte) (map[ConsensusVoteTopic]*signedVote, error) {
	if len(buf) < 2 || buf[0] != _signedVotesVersion {
		return nil, errors.New("invalid signed votes encoding")
	}
	votes := map[ConsensusVoteTopic]*signedVote{}
	n, buf := int(buf[1]), buf[2:]
	for i := 0; i < n; i++ {
		if len(buf) < 14 {
			return nil, errors.New("invalid signed votes encoding")
		}
		topic := ConsensusVoteTopic(buf[0])
		sv := &signedVote{
			height: binary.BigEndian.Uint64(buf[1:9]),
			round:  binary.BigEndian.Uint32(buf[9:13]),
		}
		hashLen := int(buf[13])
		if len(buf) < 14+hashLen {
			return nil, errors.New("invalid signed votes encoding")
		}
		sv.blkHash = append([]byte{}, buf[14:14+hashLen]...)
		votes[topic] = sv
		buf = buf[14+hashLen:]
	}
	if len(buf) != 0 {
		return nil, errors.New("invalid signed votes encoding")
	}
	return votes, nil
}

// lockState is the persisted round lock of a height. It survives restarts so
// that a node does not forget a lock it has already acted on.
type lockState struct {
	height      uint64
	round       uint32
	status      status
	blkHash     []byte
	proofOfLock []*endorsement.Endorsement
}

func (ls *lockState) serialize() ([]byte, error) {
	if len(ls.blkHash) > 255 {
		return nil, errors.Errorf("invalid block hash length %d", len(ls.blkHash))
	}
	footer := &iotextypes.BlockFooter{}
	for _, en := range ls.proofOfLock {
		footer.Endorsements = append(footer.Endorsements, en.Proto())
	}
	pol, err := proto.Marshal(footer)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 0, 15+len(ls.blkHash)+len(pol))
	buf = append(buf, _lockStateVersion)
	buf = binary.BigEndian.AppendUint64(buf, ls.height)
	buf = binary.BigEndian.AppendUint32(buf, ls.round)
	buf = append(buf, byte(ls.status), byte(len(ls.blkHash)))
	buf = append(buf, ls.blkHash...)
	return append(buf, pol...), nil
}

func deserializeLockState(buf []byte) (*lockState, error) {
	if len(buf) < 15 || buf[0] != _lockStateVersion {
		return nil, errors.New("invalid lock state encoding")
	}
	ls := &lockState{
		height: binary.BigEndian.Uint64(buf[1:9]),
		round:  binary.BigEndian.Uint32(buf[9:13]),
		status: status(buf[13]),
	}
	hashLen := int(buf[14])
	if len(buf) < 15+hashLen {
		return nil, errors.New("invalid lock state encoding")
	}
	ls.blkHash = append([]byte{}, buf[15:15+hashLen]...)
	footer := &iotextypes.BlockFooter{}
	if err := proto.Unmarshal(buf[15+hashLen:], footer); err != nil {
		return nil, err
	}
	for _, enPb := range footer.Endorsements {
		en := &endorsement.Endorsement{}
		if err := en.LoadProto(enPb); err != nil {
			return nil, err
		}
		ls.proofOfLock = append(ls.proofOfLock, en)
	}
	return ls, nil
}

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
		eeProto.Endorsements = append(eeProto.Endorsements, endorse.Proto())
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
	lock            *lockState
	// lockStored tells whether _lockStateKey may exist in eManagerDB, so that
	// nodes that never lock (e.g. enforcement disabled) never touch it
	lockStored bool
	// signedVotes is the last vote signed per topic; one slot per topic
	signedVotes map[ConsensusVoteTopic]*signedVote
	// signedVotesLoaded tells whether a signed votes record was read at
	// startup, i.e. this node's votes before the start are known
	signedVotesLoaded bool
	// lockUnreadable tells whether a persisted round lock existed at startup
	// but could not be read
	lockUnreadable bool
	// keepUnreadableLock makes a new-height Cleanup keep an unreadable lock
	// record; set for the first height after the start only
	keepUnreadableLock bool
	// lockLostHeight is a height whose persisted round lock could not be
	// restored after a restart, or 0; lockLostRound is the last round there
	// this node sits out
	lockLostHeight uint64
	lockLostRound  uint32
}

func newEndorsementManager(eManagerDB db.KVStore, deserializer *block.Deserializer) (*endorsementManager, error) {
	m, err := loadEndorsementManager(eManagerDB, deserializer)
	if err != nil || eManagerDB == nil {
		return m, err
	}
	switch buf, err := eManagerDB.Get(_eManagerNS, _lockStateKey); errors.Cause(err) {
	case nil:
		m.lockStored = true
		ls, err := deserializeLockState(buf)
		if err != nil {
			// an unreadable lock must not block startup; the node starts unlocked
			log.L().Error("failed to decode persisted round lock, starting unlocked", zap.Error(err))
			m.lockUnreadable = true
			break
		}
		m.lock = ls
	case db.ErrNotExist:
	default:
		m.lockStored = true
		m.lockUnreadable = true
		log.L().Error("failed to read persisted round lock, starting unlocked", zap.Error(err))
	}
	switch buf, err := eManagerDB.Get(_eManagerNS, _signedVotesKey); errors.Cause(err) {
	case nil:
		votes, err := deserializeSignedVotes(buf)
		if err != nil {
			log.L().Error("failed to decode persisted signed votes, starting without them", zap.Error(err))
			break
		}
		m.signedVotes = votes
		m.signedVotesLoaded = true
	case db.ErrNotExist:
	default:
		log.L().Error("failed to read persisted signed votes, starting without them", zap.Error(err))
	}
	return m, nil
}

func loadEndorsementManager(eManagerDB db.KVStore, deserializer *block.Deserializer) (*endorsementManager, error) {
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

// SignedVote returns the last vote this node signed for topic, or nil
func (m *endorsementManager) SignedVote(topic ConsensusVoteTopic) *signedVote {
	return m.signedVotes[topic]
}

// signingStateKnown reports whether the votes this node signed and its round
// lock before the start were read back: a signed votes record exists and no
// persisted lock was unreadable. A missing consensus DB (nil manager) knows
// nothing.
func (m *endorsementManager) signingStateKnown() bool {
	return m != nil && m.signedVotesLoaded && !m.lockUnreadable
}

// LockLost returns a height whose persisted round lock could not be restored
// after a restart (0 if none) and the last round there this node sits out
func (m *endorsementManager) LockLost() (uint64, uint32) {
	return m.lockLostHeight, m.lockLostRound
}

func (m *endorsementManager) markLockLost(height uint64, untilRound uint32) {
	m.lockLostHeight, m.lockLostRound = height, untilRound
}

// RecordSignedVote records (and persists, before it takes effect) the vote
// this node is about to sign for topic
func (m *endorsementManager) RecordSignedVote(topic ConsensusVoteTopic, sv *signedVote) error {
	votes := make(map[ConsensusVoteTopic]*signedVote, len(m.signedVotes)+1)
	for t, v := range m.signedVotes {
		votes[t] = v
	}
	votes[topic] = sv
	if m.eManagerDB != nil {
		buf, err := serializeSignedVotes(votes)
		if err != nil {
			return err
		}
		if err := m.eManagerDB.Put(_eManagerNS, _signedVotesKey, buf); err != nil {
			return err
		}
	}
	m.signedVotes = votes
	return nil
}

// Lock returns the persisted round lock, or nil if there is none
func (m *endorsementManager) Lock() *lockState {
	return m.lock
}

// SetLock records the round lock and persists it; nil clears it
func (m *endorsementManager) SetLock(ls *lockState) error {
	if m.eManagerDB != nil {
		if ls == nil {
			if err := m.eManagerDB.Delete(_eManagerNS, _lockStateKey); err != nil {
				return err
			}
		} else {
			buf, err := ls.serialize()
			if err != nil {
				return err
			}
			if err := m.eManagerDB.Put(_eManagerNS, _lockStateKey, buf); err != nil {
				return err
			}
		}
		m.lockStored = ls != nil
	}
	m.lock = ls
	return nil
}

func (m *endorsementManager) Cleanup(timestamp time.Time) error {
	if !timestamp.IsZero() {
		for encoded, c := range m.collections {
			m.collections[encoded] = c.Cleanup(timestamp)
		}
	} else {
		m.collections = map[string]*blockEndorsementCollection{}
		// a zero timestamp means a new height, where any previous lock is void;
		// an unreadable lock record is kept through the first height after
		// the start, so that another restart there still finds it
		keep := m.lock == nil && m.lockUnreadable && m.keepUnreadableLock
		if m.lock != nil || (m.lockStored && !keep) {
			if err := m.SetLock(nil); err != nil {
				return err
			}
		}
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
