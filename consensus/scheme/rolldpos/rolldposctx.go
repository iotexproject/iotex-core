// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	fsm "github.com/iotexproject/go-fsm"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	timeSlotMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_round",
			Help: "Consensus round",
		},
		[]string{},
	)

	blockIntervalMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_block_interval",
			Help: "Consensus block interval",
		},
		[]string{},
	)

	consensusDurationMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_elapse_time",
			Help: "Consensus elapse time.",
		},
		[]string{},
	)

	consensusHeightMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_height",
			Help: "Consensus height",
		},
		[]string{},
	)
)

func init() {
	prometheus.MustRegister(timeSlotMtc)
	prometheus.MustRegister(blockIntervalMtc)
	prometheus.MustRegister(consensusDurationMtc)
	prometheus.MustRegister(consensusHeightMtc)
}

// DelegatesByEpochFunc defines a function to overwrite candidates
type DelegatesByEpochFunc func(uint64) ([]string, error)
type rollDPoSCtx struct {
	consensusfsm.ConsensusConfig

	// TODO: explorer dependency deleted at #1085, need to add api params here
	chain             ChainManager
	broadcastHandler  scheme.Broadcast
	roundCalc         *roundCalculator
	eManagerDB        db.KVStore
	toleratedOvertime time.Duration

	encodedAddr string
	priKey      crypto.PrivateKey
	round       *roundCtx
	clock       clock.Clock
	active      bool
	mutex       sync.RWMutex
}

func newRollDPoSCtx(
	cfg consensusfsm.ConsensusConfig,
	consensusDBConfig config.DB,
	active bool,
	toleratedOvertime time.Duration,
	timeBasedRotation bool,
	chain ChainManager,
	rp *rolldpos.Protocol,
	broadcastHandler scheme.Broadcast,
	delegatesByEpochFunc DelegatesByEpochFunc,
	encodedAddr string,
	priKey crypto.PrivateKey,
	clock clock.Clock,
	beringHeight uint64,
) (*rollDPoSCtx, error) {
	if chain == nil {
		return nil, errors.New("chain cannot be nil")
	}
	if rp == nil {
		return nil, errors.New("roll dpos protocol cannot be nil")
	}
	if clock == nil {
		return nil, errors.New("clock cannot be nil")
	}
	if delegatesByEpochFunc == nil {
		return nil, errors.New("delegates by epoch function cannot be nil")
	}
	if cfg.AcceptBlockTTL(0)+cfg.AcceptProposalEndorsementTTL(0)+cfg.AcceptLockEndorsementTTL(0)+cfg.CommitTTL(0) > cfg.BlockInterval(0) {
		return nil, errors.Errorf(
			"invalid ttl config, the sum of ttls should be equal to block interval. acceptBlockTTL %d, acceptProposalEndorsementTTL %d, acceptLockEndorsementTTL %d, commitTTL %d, blockInterval %d",
			cfg.AcceptBlockTTL(0),
			cfg.AcceptProposalEndorsementTTL(0),
			cfg.AcceptLockEndorsementTTL(0),
			cfg.CommitTTL(0),
			cfg.BlockInterval(0),
		)
	}
	var eManagerDB db.KVStore
	if len(consensusDBConfig.DbPath) > 0 {
		eManagerDB = db.NewBoltDB(consensusDBConfig)
	}
	roundCalc := &roundCalculator{
		delegatesByEpochFunc: delegatesByEpochFunc,
		chain:                chain,
		rp:                   rp,
		timeBasedRotation:    timeBasedRotation,
		beringHeight:         beringHeight,
	}
	return &rollDPoSCtx{
		ConsensusConfig:   cfg,
		active:            active,
		encodedAddr:       encodedAddr,
		priKey:            priKey,
		chain:             chain,
		broadcastHandler:  broadcastHandler,
		clock:             clock,
		roundCalc:         roundCalc,
		eManagerDB:        eManagerDB,
		toleratedOvertime: toleratedOvertime,
	}, nil
}

func (ctx *rollDPoSCtx) Start(c context.Context) (err error) {
	var eManager *endorsementManager
	if ctx.eManagerDB != nil {
		if err := ctx.eManagerDB.Start(c); err != nil {
			return errors.Wrap(err, "Error when starting the collectionDB")
		}
		eManager, err = newEndorsementManager(ctx.eManagerDB)
	}
	ctx.round, err = ctx.roundCalc.NewRoundWithToleration(0, ctx.BlockInterval(0), ctx.clock.Now(), eManager, ctx.toleratedOvertime)

	return err
}

func (ctx *rollDPoSCtx) Stop(c context.Context) error {
	if ctx.eManagerDB != nil {
		return ctx.eManagerDB.Stop(c)
	}
	return nil
}

// CheckVoteEndorser checks if the endorsement's endorser is a valid delegate at the given height
func (ctx *rollDPoSCtx) CheckVoteEndorser(
	height uint64,
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	endorserAddr, err := address.FromBytes(en.Endorser().Hash())
	if err != nil {
		return err
	}
	if !ctx.roundCalc.IsDelegate(endorserAddr.String(), height) {
		return errors.Errorf("%s is not delegate of the corresponding round", endorserAddr)
	}

	return nil
}

// CheckBlockProposer checks the block proposal
func (ctx *rollDPoSCtx) CheckBlockProposer(
	height uint64,
	proposal *blockProposal,
	en *endorsement.Endorsement,
) error {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	if height != proposal.block.Height() {
		return errors.Errorf(
			"block height %d different from expected %d",
			proposal.block.Height(),
			height,
		)
	}
	endorserAddr, err := address.FromBytes(en.Endorser().Hash())
	if err != nil {
		return err
	}
	if proposer := ctx.roundCalc.Proposer(height, ctx.BlockInterval(height), en.Timestamp()); proposer != endorserAddr.String() {
		return errors.Errorf(
			"%s is not proposer of the corresponding round, %s expected",
			endorserAddr.String(),
			proposer,
		)
	}
	proposerAddr := proposal.ProposerAddress()
	if ctx.roundCalc.Proposer(height, ctx.BlockInterval(height), proposal.block.Timestamp()) != proposerAddr {
		return errors.Errorf("%s is not proposer of the corresponding round", proposerAddr)
	}
	if !proposal.block.VerifySignature() {
		return errors.Errorf("invalid block signature")
	}
	if proposerAddr != endorserAddr.String() {
		round, err := ctx.roundCalc.NewRound(height, ctx.BlockInterval(height), en.Timestamp(), nil)
		if err != nil {
			return err
		}
		if err := round.AddBlock(proposal.block); err != nil {
			return err
		}
		blkHash := proposal.block.HashBlock()
		for _, e := range proposal.proofOfLock {
			if err := round.AddVoteEndorsement(
				NewConsensusVote(blkHash[:], PROPOSAL),
				e,
			); err == nil {
				continue
			}
			if err := round.AddVoteEndorsement(
				NewConsensusVote(blkHash[:], COMMIT),
				e,
			); err != nil {
				return err
			}
		}
		if !round.EndorsedByMajority(blkHash[:], []ConsensusVoteTopic{PROPOSAL, COMMIT}) {
			return errors.Wrap(ErrInsufficientEndorsements, "failed to verify proof of lock")
		}
	}
	return nil
}

func (ctx *rollDPoSCtx) RoundCalc() *roundCalculator {
	return ctx.roundCalc
}

/////////////////////////////////////
// Context of consensusFSM interfaces
/////////////////////////////////////

func (ctx *rollDPoSCtx) NewConsensusEvent(
	eventType fsm.EventType,
	data interface{},
) *consensusfsm.ConsensusEvent {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.newConsensusEvent(eventType, data)
}

func (ctx *rollDPoSCtx) NewBackdoorEvt(
	dst fsm.State,
) *consensusfsm.ConsensusEvent {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.newConsensusEvent(consensusfsm.BackdoorEvent, dst)
}

func (ctx *rollDPoSCtx) Logger() *zap.Logger {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.logger()
}

func (ctx *rollDPoSCtx) Prepare() error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	height := ctx.chain.TipHeight() + 1
	newRound, err := ctx.roundCalc.UpdateRound(ctx.round, height, ctx.BlockInterval(height), ctx.clock.Now(), ctx.toleratedOvertime)
	if err != nil {
		return err
	}
	ctx.logger().Debug(
		"new round",
		zap.Uint64("height", newRound.height),
		zap.String("ts", ctx.clock.Now().String()),
		zap.Uint64("epoch", newRound.epochNum),
		zap.Uint64("epochStartHeight", newRound.epochStartHeight),
		zap.Uint32("round", newRound.roundNum),
		zap.String("roundStartTime", newRound.roundStartTime.String()),
	)
	ctx.round = newRound
	consensusHeightMtc.WithLabelValues().Set(float64(ctx.round.height))
	timeSlotMtc.WithLabelValues().Set(float64(ctx.round.roundNum))
	return nil
}

func (ctx *rollDPoSCtx) IsDelegate() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.isDelegate()
}

func (ctx *rollDPoSCtx) Proposal() (interface{}, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	if ctx.round.Proposer() != ctx.encodedAddr {
		return nil, nil
	}
	if ctx.round.IsLocked() {
		return ctx.endorseBlockProposal(newBlockProposal(
			ctx.round.Block(ctx.round.HashOfBlockInLock()),
			ctx.round.ProofOfLock(),
		))
	}
	return ctx.mintNewBlock()
}

func (ctx *rollDPoSCtx) WaitUntilRoundStart() time.Duration {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	now := ctx.clock.Now()
	startTime := ctx.round.StartTime()
	if now.Before(startTime) {
		time.Sleep(startTime.Sub(now))
		return 0
	}
	overTime := now.Sub(startTime)
	if !ctx.isDelegate() && ctx.toleratedOvertime > overTime {
		time.Sleep(ctx.toleratedOvertime - overTime)
		return 0
	}
	return overTime
}

func (ctx *rollDPoSCtx) PreCommitEndorsement() interface{} {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	endorsement := ctx.round.ReadyToCommit(ctx.encodedAddr)
	if endorsement == nil {
		// DON'T CHANGE, this is on purpose, because endorsement as nil won't result in a nil "interface {}"
		return nil
	}
	return endorsement
}

func (ctx *rollDPoSCtx) NewProposalEndorsement(msg interface{}) (interface{}, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	var blockHash []byte
	if msg != nil {
		ecm, ok := msg.(*EndorsedConsensusMessage)
		if !ok {
			return nil, errors.New("invalid endorsed block")
		}
		proposal, ok := ecm.Document().(*blockProposal)
		if !ok {
			return nil, errors.New("invalid endorsed block")
		}
		blkHash := proposal.block.HashBlock()
		blockHash = blkHash[:]
		if err := ctx.chain.ValidateBlock(proposal.block); err != nil {
			return nil, errors.Wrapf(err, "error when validating the proposed block")
		}
		if err := ctx.round.AddBlock(proposal.block); err != nil {
			return nil, err
		}
		ctx.loggerWithStats().Debug("accept block proposal", log.Hex("block", blockHash))
	} else if ctx.round.IsLocked() {
		blockHash = ctx.round.HashOfBlockInLock()
	}

	return ctx.newEndorsement(
		blockHash,
		PROPOSAL,
		ctx.round.StartTime().Add(ctx.AcceptBlockTTL(ctx.round.height)),
	)
}

func (ctx *rollDPoSCtx) NewLockEndorsement(
	msg interface{},
) (interface{}, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	blkHash, err := ctx.verifyVote(
		msg,
		[]ConsensusVoteTopic{PROPOSAL, COMMIT}, // commit is counted as one proposal
	)
	switch errors.Cause(err) {
	case ErrInsufficientEndorsements:
		return nil, nil
	case nil:
		if len(blkHash) != 0 {
			ctx.loggerWithStats().Debug("Locked", log.Hex("block", blkHash))
			return ctx.newEndorsement(
				blkHash,
				LOCK,
				ctx.round.StartTime().Add(
					ctx.AcceptBlockTTL(ctx.round.height)+ctx.AcceptProposalEndorsementTTL(ctx.round.height),
				),
			)
		}
		ctx.loggerWithStats().Debug("Unlocked")
	}
	return nil, err
}

func (ctx *rollDPoSCtx) NewPreCommitEndorsement(
	msg interface{},
) (interface{}, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	blkHash, err := ctx.verifyVote(
		msg,
		[]ConsensusVoteTopic{LOCK, COMMIT}, // commit endorse is counted as one lock endorse
	)
	switch errors.Cause(err) {
	case ErrInsufficientEndorsements:
		return nil, nil
	case nil:
		ctx.loggerWithStats().Debug("Ready to pre-commit")
		return ctx.newEndorsement(
			blkHash,
			COMMIT,
			ctx.round.StartTime().Add(
				ctx.AcceptBlockTTL(ctx.round.height)+ctx.AcceptProposalEndorsementTTL(ctx.round.height)+ctx.AcceptLockEndorsementTTL(ctx.round.height),
			),
		)
	default:
		return nil, err
	}
}

func (ctx *rollDPoSCtx) Commit(msg interface{}) (bool, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	blkHash, err := ctx.verifyVote(msg, []ConsensusVoteTopic{COMMIT})
	switch errors.Cause(err) {
	case ErrInsufficientEndorsements:
		return false, nil
	case nil:
		ctx.loggerWithStats().Debug("Ready to commit")
	default:
		return false, err
	}
	// this is redudant check for now, as we only accept endorsements of the received blocks
	pendingBlock := ctx.round.Block(blkHash)
	if pendingBlock == nil {
		return false, nil
	}
	ctx.logger().Info("consensus reached", zap.Uint64("blockHeight", ctx.round.Height()))
	if err := pendingBlock.Finalize(
		ctx.round.Endorsements(blkHash, []ConsensusVoteTopic{COMMIT}),
		ctx.round.StartTime().Add(
			ctx.AcceptBlockTTL(ctx.round.height)+ctx.AcceptProposalEndorsementTTL(ctx.round.height)+ctx.AcceptLockEndorsementTTL(ctx.round.height),
		),
	); err != nil {
		return false, errors.Wrap(err, "failed to add endorsements to block")
	}

	// Commit and broadcast the pending block
	switch err := ctx.chain.CommitBlock(pendingBlock); errors.Cause(err) {
	case blockchain.ErrInvalidTipHeight:
		return true, nil
	case nil:
		break
	default:
		return false, errors.Wrap(err, "error when committing a block")
	}
	// Broadcast the committed block to the network
	if blkProto := pendingBlock.ConvertToBlockPb(); blkProto != nil {
		if err := ctx.broadcastHandler(blkProto); err != nil {
			ctx.logger().Error(
				"error when broadcasting blkProto",
				zap.Error(err),
				zap.Uint64("block", pendingBlock.Height()),
			)
		}
		// putblock to parent chain if the current node is proposer and current chain is a sub chain
		if ctx.round.Proposer() == ctx.encodedAddr && ctx.chain.ChainAddress() != "" {
			// TODO: explorer dependency deleted at #1085, need to call putblock related method
		}
	} else {
		ctx.logger().Panic(
			"error when converting a block into a proto msg",
			zap.Uint64("block", pendingBlock.Height()),
		)
	}

	consensusDurationMtc.WithLabelValues().Set(float64(time.Since(ctx.round.roundStartTime)))
	if pendingBlock.Height() > 1 {
		prevBlkHeader, err := ctx.chain.BlockHeaderByHeight(pendingBlock.Height() - 1)
		if err != nil {
			log.L().Error("Error when getting the previous block header.",
				zap.Error(err),
				zap.Uint64("height", pendingBlock.Height()-1),
			)
		}
		blockIntervalMtc.WithLabelValues().Set(float64(pendingBlock.Timestamp().Sub(prevBlkHeader.Timestamp())))
	}
	return true, nil
}

func (ctx *rollDPoSCtx) Broadcast(endorsedMsg interface{}) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	ecm, ok := endorsedMsg.(*EndorsedConsensusMessage)
	if !ok {
		ctx.loggerWithStats().Error("invalid message type", zap.Any("message", ecm))
		return
	}
	msg, err := ecm.Proto()
	if err != nil {
		ctx.loggerWithStats().Error("failed to generate protobuf message", zap.Error(err))
		return
	}
	if err := ctx.broadcastHandler(msg); err != nil {
		ctx.loggerWithStats().Error("fail to broadcast", zap.Error(err))
	}
}

func (ctx *rollDPoSCtx) IsStaleEvent(evt *consensusfsm.ConsensusEvent) bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.round.IsStale(evt.Height(), evt.Round(), evt.Data())
}

func (ctx *rollDPoSCtx) IsFutureEvent(evt *consensusfsm.ConsensusEvent) bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.round.IsFuture(evt.Height(), evt.Round())
}

func (ctx *rollDPoSCtx) IsStaleUnmatchedEvent(evt *consensusfsm.ConsensusEvent) bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.clock.Now().Sub(evt.Timestamp()) > ctx.UnmatchedEventTTL(ctx.round.height)
}

func (ctx *rollDPoSCtx) Height() uint64 {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.round.Height()
}

func (ctx *rollDPoSCtx) Activate(active bool) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	ctx.active = active
}

func (ctx *rollDPoSCtx) Active() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.active
}

///////////////////////////////////////////
// private functions
///////////////////////////////////////////

func (ctx *rollDPoSCtx) mintNewBlock() (*EndorsedConsensusMessage, error) {
	var err error
	blk := ctx.round.CachedMintedBlock()
	if blk == nil {
		// in case that there is no cached block in eManagerDB, it mints a new block.
		blk, err = ctx.chain.MintNewBlock(ctx.round.StartTime())
		if err != nil {
			return nil, err
		}
		if err = ctx.round.SetMintedBlock(blk); err != nil {
			return nil, err
		}
	}

	var proofOfUnlock []*endorsement.Endorsement
	if ctx.round.IsUnlocked() {
		proofOfUnlock = ctx.round.ProofOfLock()
	}
	return ctx.endorseBlockProposal(newBlockProposal(blk, proofOfUnlock))
}

func (ctx *rollDPoSCtx) isDelegate() bool {
	if active := ctx.active; !active {
		ctx.logger().Info("current node is in standby mode")
		return false
	}
	return ctx.round.IsDelegate(ctx.encodedAddr)
}

func (ctx *rollDPoSCtx) endorseBlockProposal(proposal *blockProposal) (*EndorsedConsensusMessage, error) {
	en, err := endorsement.Endorse(ctx.priKey, proposal, ctx.round.StartTime())
	if err != nil {
		return nil, err
	}
	return NewEndorsedConsensusMessage(proposal.block.Height(), proposal, en), nil
}

func (ctx *rollDPoSCtx) logger() *zap.Logger {
	return ctx.round.Log(log.Logger("consensus"))
}

func (ctx *rollDPoSCtx) newConsensusEvent(
	eventType fsm.EventType,
	data interface{},
) *consensusfsm.ConsensusEvent {
	switch ed := data.(type) {
	case *EndorsedConsensusMessage:
		height := ed.Height()
		roundNum, _, err := ctx.roundCalc.RoundInfo(height, ctx.BlockInterval(height), ed.Endorsement().Timestamp())
		if err != nil {
			ctx.logger().Error(
				"failed to calculate round for generating consensus event",
				zap.String("eventType", string(eventType)),
				zap.Uint64("height", ed.Height()),
				zap.String("timestamp", ed.Endorsement().Timestamp().String()),
				zap.Any("data", data),
				zap.Error(err),
			)
			return nil
		}
		return consensusfsm.NewConsensusEvent(
			eventType,
			data,
			ed.Height(),
			roundNum,
			ctx.clock.Now(),
		)
	default:
		return consensusfsm.NewConsensusEvent(
			eventType,
			data,
			ctx.round.Height(),
			ctx.round.Number(),
			ctx.clock.Now(),
		)
	}
}

func (ctx *rollDPoSCtx) loggerWithStats() *zap.Logger {
	return ctx.round.LogWithStats(log.Logger("consensus"))
}

func (ctx *rollDPoSCtx) verifyVote(
	msg interface{},
	topics []ConsensusVoteTopic,
) ([]byte, error) {
	consensusMsg, ok := msg.(*EndorsedConsensusMessage)
	if !ok {
		return nil, errors.New("invalid msg")
	}
	vote, ok := consensusMsg.Document().(*ConsensusVote)
	if !ok {
		return nil, errors.New("invalid msg")
	}
	blkHash := vote.BlockHash()
	endorsement := consensusMsg.Endorsement()
	if err := ctx.round.AddVoteEndorsement(vote, endorsement); err != nil {
		return blkHash, err
	}
	ctx.loggerWithStats().Debug(
		"verified consensus vote",
		log.Hex("block", blkHash),
		zap.Uint8("topic", uint8(vote.Topic())),
		zap.String("endorser", endorsement.Endorser().HexString()),
	)
	if !ctx.round.EndorsedByMajority(blkHash, topics) {
		return blkHash, ErrInsufficientEndorsements
	}
	return blkHash, nil
}

func (ctx *rollDPoSCtx) newEndorsement(
	blkHash []byte,
	topic ConsensusVoteTopic,
	timestamp time.Time,
) (*EndorsedConsensusMessage, error) {
	vote := NewConsensusVote(
		blkHash,
		topic,
	)
	en, err := endorsement.Endorse(ctx.priKey, vote, timestamp)
	if err != nil {
		return nil, err
	}

	return NewEndorsedConsensusMessage(ctx.round.Height(), vote, en), nil
}
