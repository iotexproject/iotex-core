// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"bytes"
	"slices"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

// ErrInsufficientEndorsements represents the error that not enough endorsements
var ErrInsufficientEndorsements = errors.New("Insufficient endorsements")

type status int

const (
	_open status = iota
	_locked
	_unlocked
)

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	epochNum             uint64
	epochStartHeight     uint64
	nextEpochStartHeight uint64
	numOfDelegates       uint64
	delegates            []string
	proposers            []string

	height             uint64
	roundNum           uint32
	prevHash           hash.Hash256
	proposer           string
	roundStartTime     time.Time
	nextRoundStartTime time.Time

	blockInLock []byte
	proofOfLock []*endorsement.Endorsement
	status      status
	// lockRound is the round of the polka that produced blockInLock; only
	// meaningful while status == _locked
	lockRound uint32
	// enforceLock binds this node's endorsements to its round lock
	enforceLock bool
	// refusedProposal tells whether a proposal conflicting with the lock was
	// already refused (and its block kept) in this round
	refusedProposal bool
	eManager        *endorsementManager
}

func (ctx *roundCtx) Log(l *zap.Logger) *zap.Logger {
	return l.With(
		zap.Uint64("height", ctx.height),
		zap.Uint64("epoch", ctx.epochNum),
		zap.Uint32("round", ctx.roundNum),
		zap.String("proposer", ctx.proposer),
		log.Hex("prevHash", ctx.prevHash[:]),
	)
}

func (ctx *roundCtx) LogWithStats(l *zap.Logger) *zap.Logger {
	return ctx.eManager.Log(ctx.Log(l), ctx.delegates)
}

func (ctx *roundCtx) EpochNum() uint64 {
	return ctx.epochNum
}

func (ctx *roundCtx) EpochStartHeight() uint64 {
	return ctx.epochStartHeight
}

func (ctx *roundCtx) NextEpochStartHeight() uint64 {
	return ctx.nextEpochStartHeight
}

func (ctx *roundCtx) StartTime() time.Time {
	return ctx.roundStartTime
}

func (ctx *roundCtx) NextRoundStartTime() time.Time {
	return ctx.nextRoundStartTime
}

func (ctx *roundCtx) Height() uint64 {
	return ctx.height
}

func (ctx *roundCtx) Number() uint32 {
	return ctx.roundNum
}

func (ctx *roundCtx) PrevHash() hash.Hash256 {
	return ctx.prevHash
}

func (ctx *roundCtx) Proposer() string {
	return ctx.proposer
}

func (ctx *roundCtx) Delegates() []string {
	return ctx.delegates
}

func (ctx *roundCtx) Proposers() []string {
	return ctx.proposers
}

func (ctx *roundCtx) IsDelegate(addr string) bool {
	return slices.Contains(ctx.delegates, addr)
}

func (ctx *roundCtx) Block(blkHash []byte) *block.Block {
	return ctx.block(blkHash)
}

func (ctx *roundCtx) Endorsements(blkHash []byte, topics []ConsensusVoteTopic) []*endorsement.Endorsement {
	return ctx.endorsements(blkHash, topics)
}

func (ctx *roundCtx) IsLocked() bool {
	return ctx.status == _locked
}

func (ctx *roundCtx) IsUnlocked() bool {
	return ctx.status == _unlocked
}

func (ctx *roundCtx) ReadyToCommit(addrs []string) []*EndorsedConsensusMessage {
	c := ctx.eManager.CollectionByBlockHash(ctx.blockInLock)
	if c == nil {
		return nil
	}
	blk := c.Block()
	if blk == nil {
		return nil
	}
	blkHash := blk.HashBlock()
	msgs := make([]*EndorsedConsensusMessage, 0)
	for _, addr := range addrs {
		en := c.Endorsement(addr, COMMIT)
		if en == nil {
			continue
		}
		msgs = append(msgs, NewEndorsedConsensusMessage(
			blk.Height(),
			NewConsensusVote(blkHash[:], COMMIT),
			en,
		))
	}

	return msgs
}

func (ctx *roundCtx) HashOfBlockInLock() []byte {
	return ctx.blockInLock
}

func (ctx *roundCtx) LockRound() uint32 {
	return ctx.lockRound
}

// lockOn locks this node on blkHash as of the given round, backed by pol.
// It is the single entry point for (re)locking. The lock is persisted before
// it takes effect, so a node never acts on a lock it could forget.
func (ctx *roundCtx) lockOn(blkHash []byte, round uint32, pol []*endorsement.Endorsement) error {
	if err := ctx.persistLock(_locked, blkHash, round, pol); err != nil {
		return errors.Wrap(err, "failed to persist round lock")
	}
	ctx.status = _locked
	ctx.blockInLock = blkHash
	ctx.lockRound = round
	ctx.proofOfLock = pol
	return nil
}

// unlock records a polka on the empty block (proof of unlock)
func (ctx *roundCtx) unlock(pol []*endorsement.Endorsement) error {
	if ctx.status != _unlocked {
		if err := ctx.persistLock(_unlocked, nil, 0, pol); err != nil {
			return errors.Wrap(err, "failed to persist round unlock")
		}
	}
	ctx.status = _unlocked
	ctx.blockInLock = nil
	ctx.lockRound = 0
	ctx.proofOfLock = pol
	return nil
}

func (ctx *roundCtx) persistLock(st status, blkHash []byte, round uint32, pol []*endorsement.Endorsement) error {
	if !ctx.enforceLock || ctx.eManager == nil {
		return nil
	}
	return ctx.eManager.SetLock(&lockState{
		height:      ctx.height,
		round:       round,
		status:      st,
		blkHash:     blkHash,
		proofOfLock: pol,
	})
}

// hasValidProofOfLock reports whether the proof of lock held for blockInLock
// is a quorum that other nodes will accept when this node re-proposes it. A
// lock taken on 2/3 LOCK (NewPreCommitEndorsement) may hold a weaker proof.
func (ctx *roundCtx) hasValidProofOfLock() bool {
	return ctx.isMajority(ctx.proofOfLock)
}

// restoreLock re-applies a lock persisted before a restart, if it belongs to
// this round's height and the locked block is still known
func (ctx *roundCtx) restoreLock(ls *lockState) bool {
	if ls == nil || ls.height != ctx.height {
		return false
	}
	if ls.round > ctx.roundNum {
		// the clock stepped back across the restart; keep the lock rather
		// than fail open, as of the current round at the latest
		log.L().Warn("persisted round lock is ahead of the current round",
			zap.Uint32("lockRound", ls.round), zap.Uint32("round", ctx.roundNum))
		ls.round = ctx.roundNum
	}
	switch ls.status {
	case _locked:
		if len(ls.blkHash) == 0 || ctx.block(ls.blkHash) == nil {
			return false
		}
	case _unlocked:
	default:
		return false
	}
	ctx.status = ls.status
	ctx.blockInLock = ls.blkHash
	ctx.lockRound = ls.round
	ctx.proofOfLock = ls.proofOfLock
	return true
}

func (ctx *roundCtx) ProofOfLock() []*endorsement.Endorsement {
	return ctx.proofOfLock
}

func (ctx *roundCtx) IsStale(height uint64, num uint32, data interface{}) bool {
	switch {
	case height < ctx.height:
		return true
	case height > ctx.height:
		return false
	case num >= ctx.roundNum:
		return false
	default:
		msg, ok := data.(*EndorsedConsensusMessage)
		if !ok {
			return true
		}
		vote, ok := msg.Document().(*ConsensusVote)
		if !ok {
			return true
		}

		return vote.Topic() != COMMIT
	}
}

func (ctx *roundCtx) IsFuture(height uint64, num uint32) bool {
	if height > ctx.height || height == ctx.height && num > ctx.roundNum {
		return true
	}
	return false
}

func (ctx *roundCtx) EndorsedByMajority(
	blockHash []byte,
	topics []ConsensusVoteTopic,
) bool {
	return ctx.endorsedByMajority(blockHash, topics)
}

func (ctx *roundCtx) AddBlock(blk *block.Block) error {
	return ctx.eManager.RegisterBlock(blk)
}

func (ctx *roundCtx) AddVoteEndorsement(
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	if !endorsement.VerifyEndorsement(vote, en) {
		return errors.New("invalid endorsement for the vote")
	}
	if addr := en.Endorser().Address(); addr == nil || !ctx.IsDelegate(addr.String()) {
		return errors.New("invalid endorser")
	}
	blockHash := vote.BlockHash()
	// TODO: (zhi) request for block
	if len(blockHash) != 0 && ctx.block(blockHash) == nil {
		return errors.New("the corresponding block not received")
	}
	if err := ctx.eManager.AddVoteEndorsement(vote, en); err != nil {
		return err
	}
	if vote.Topic() == LOCK {
		return nil
	}
	if len(blockHash) != 0 && ctx.status == _locked {
		if !ctx.enforceLock {
			// legacy behaviour: once locked, the lock never moves
			return nil
		}
		sameBlock := bytes.Equal(blockHash, ctx.blockInLock)
		if ctx.lockRound == ctx.roundNum && !(sameBlock && !ctx.hasValidProofOfLock()) {
			// already locked in this round (a weak proof of lock on the same
			// block may still be completed below)
			if !sameBlock && ctx.isMajority(ctx.endorsements(blockHash, []ConsensusVoteTopic{PROPOSAL, COMMIT})) {
				// two polkas in one round need equivocating votes; never
				// move the lock (nor sign a second LOCK) within a round
				ctx.Log(log.Logger("consensus")).Warn(
					"conflicting polka in the round of the lock",
					log.Hex("lock", ctx.blockInLock),
					log.Hex("block", blockHash),
				)
			}
			return nil
		}
	}
	endorsements := ctx.endorsements(
		blockHash,
		[]ConsensusVoteTopic{PROPOSAL, COMMIT},
	)
	if !ctx.isMajority(endorsements) {
		return nil
	}
	if len(blockHash) == 0 {
		// TODO: (zhi) look into details of unlock
		return ctx.unlock(endorsements)
	}
	// PROPOSAL endorsements of earlier rounds are dropped at round start
	// (endorsementManager.Cleanup), so this majority is a polka of the current
	// round, which is never earlier than lockRound. A later polka is allowed to
	// move the lock, and it refreshes lockRound and proofOfLock when it is for
	// the block already in lock.
	return ctx.lockOn(blockHash, ctx.roundNum, endorsements)
}

func (ctx *roundCtx) SetMintedBlock(blk *block.Block) error {
	return ctx.eManager.SetMintedBlock(blk)
}

func (ctx *roundCtx) CachedMintedBlock() *block.Block {
	return ctx.eManager.CachedMintedBlock()
}

// private functions

func (ctx *roundCtx) endorsements(blkHash []byte, topics []ConsensusVoteTopic) []*endorsement.Endorsement {
	c := ctx.eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return []*endorsement.Endorsement{}
	}
	return c.Endorsements(topics)
}

func (ctx *roundCtx) endorsedByMajority(blockHash []byte, topics []ConsensusVoteTopic) bool {
	return ctx.isMajority(ctx.endorsements(blockHash, topics))
}

func (ctx *roundCtx) isMajority(endorsements []*endorsement.Endorsement) bool {
	return 3*len(endorsements) > 2*int(ctx.numOfDelegates)
}

func (ctx *roundCtx) block(blkHash []byte) *block.Block {
	c := ctx.eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return nil
	}
	return c.Block()
}
