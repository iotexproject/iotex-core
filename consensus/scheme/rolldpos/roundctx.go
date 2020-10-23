// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/endorsement"
)

// ErrInsufficientEndorsements represents the error that not enough endorsements
var ErrInsufficientEndorsements = errors.New("Insufficient endorsements")

type status int

const (
	open status = iota
	locked
	unlocked
)

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	epochNum             uint64
	epochStartHeight     uint64
	nextEpochStartHeight uint64
	delegates            []string

	height             uint64
	roundNum           uint32
	proposer           string
	roundStartTime     time.Time
	nextRoundStartTime time.Time

	blockInLock []byte
	proofOfLock []*endorsement.Endorsement
	status      status
	eManager    *endorsementManager
}

func (ctx *roundCtx) Log(l *zap.Logger) *zap.Logger {
	return l.With(
		zap.Uint64("height", ctx.height),
		zap.Uint64("epoch", ctx.epochNum),
		zap.Uint32("round", ctx.roundNum),
		zap.String("proposer", ctx.proposer),
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

func (ctx *roundCtx) Proposer() string {
	return ctx.proposer
}

func (ctx *roundCtx) Delegates() []string {
	return ctx.delegates
}

func (ctx *roundCtx) IsDelegate(addr string) bool {
	for _, d := range ctx.delegates {
		if addr == d {
			return true
		}
	}

	return false
}

func (ctx *roundCtx) Block(blkHash []byte) *block.Block {
	return ctx.block(blkHash)
}

func (ctx *roundCtx) Endorsements(blkHash []byte, topics []ConsensusVoteTopic) []*endorsement.Endorsement {
	return ctx.endorsements(blkHash, topics)
}

func (ctx *roundCtx) IsLocked() bool {
	return ctx.status == locked
}

func (ctx *roundCtx) IsUnlocked() bool {
	return ctx.status == unlocked
}

func (ctx *roundCtx) ReadyToCommit(addr string) *EndorsedConsensusMessage {
	c := ctx.eManager.CollectionByBlockHash(ctx.blockInLock)
	if c == nil {
		return nil
	}
	en := c.Endorsement(addr, COMMIT)
	if en == nil {
		return nil
	}
	blk := c.Block()
	if blk == nil {
		return nil
	}
	blkHash := blk.HashBlock()
	return NewEndorsedConsensusMessage(
		blk.Height(),
		NewConsensusVote(blkHash[:], COMMIT),
		en,
	)
}

func (ctx *roundCtx) HashOfBlockInLock() []byte {
	return ctx.blockInLock
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
	if len(blockHash) != 0 && ctx.status == locked {
		return nil
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
		ctx.status = unlocked
	} else {
		ctx.status = locked
	}
	ctx.blockInLock = blockHash
	ctx.proofOfLock = endorsements

	return nil
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
	return 3*len(endorsements) > 2*len(ctx.delegates)
}

func (ctx *roundCtx) block(blkHash []byte) *block.Block {
	c := ctx.eManager.CollectionByBlockHash(blkHash)
	if c == nil {
		return nil
	}
	return c.Block()
}
