// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	fsm "github.com/iotexproject/go-fsm"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"slices"

	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/consensus/scheme"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

var (
	_timeSlotMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_round",
			Help: "Consensus round",
		},
		[]string{},
	)

	_blockIntervalMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_block_interval",
			Help: "Consensus block interval",
		},
		[]string{},
	)

	_consensusDurationMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_elapse_time",
			Help: "Consensus elapse time.",
		},
		[]string{},
	)

	_consensusHeightMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_height",
			Help: "Consensus height",
		},
		[]string{},
	)
)

func init() {
	prometheus.MustRegister(_timeSlotMtc)
	prometheus.MustRegister(_blockIntervalMtc)
	prometheus.MustRegister(_consensusDurationMtc)
	prometheus.MustRegister(_consensusHeightMtc)
}

type (
	// NodesSelectionByEpochFunc defines a function to select nodes
	NodesSelectionByEpochFunc func(uint64, []byte) ([]string, error)

	// RDPoSCtx is the context of RollDPoS
	RDPoSCtx interface {
		consensusfsm.Context
		Chain() ChainManager
		BlockDeserializer() *block.Deserializer
		RoundCalculator() *roundCalculator
		Clock() clock.Clock
		CheckBlockProposer(uint64, *blockProposal, *endorsement.Endorsement) error
		CheckVoteEndorser(uint64, *ConsensusVote, *endorsement.Endorsement) error
	}

	rollDPoSCtx struct {
		consensusfsm.ConsensusConfig

		// TODO: explorer dependency deleted at #1085, need to add api params here
		chain             ChainManager
		blockDeserializer *block.Deserializer
		broadcastHandler  scheme.Broadcast
		roundCalc         *roundCalculator
		eManagerDB        db.KVStore
		toleratedOvertime time.Duration

		encodedAddrs []string
		priKeys      []crypto.PrivateKey
		round        *roundCtx
		clock        clock.Clock
		active       bool
		mutex        sync.RWMutex
	}
)

// NewRollDPoSCtx returns a context of RollDPoSCtx
func NewRollDPoSCtx(
	cfg consensusfsm.ConsensusConfig,
	consensusDBConfig db.Config,
	active bool,
	toleratedOvertime time.Duration,
	timeBasedRotation bool,
	chain ChainManager,
	blockDeserializer *block.Deserializer,
	rp *rolldpos.Protocol,
	broadcastHandler scheme.Broadcast,
	delegatesByEpochFunc NodesSelectionByEpochFunc,
	proposersByEpochFunc NodesSelectionByEpochFunc,
	priKeys []crypto.PrivateKey,
	clock clock.Clock,
	beringHeight uint64,
) (RDPoSCtx, error) {
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
	if proposersByEpochFunc == nil {
		return nil, errors.New("proposers by epoch function cannot be nil")
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
		proposersByEpochFunc: proposersByEpochFunc,
		chain:                chain,
		rp:                   rp,
		timeBasedRotation:    timeBasedRotation,
		beringHeight:         beringHeight,
	}
	encodedAddrs := make([]string, 0, len(priKeys))
	for _, pk := range priKeys {
		encodedAddrs = append(encodedAddrs, pk.PublicKey().Address().String())
	}
	return &rollDPoSCtx{
		ConsensusConfig:   cfg,
		active:            active,
		encodedAddrs:      encodedAddrs,
		priKeys:           priKeys,
		chain:             chain,
		blockDeserializer: blockDeserializer,
		broadcastHandler:  broadcastHandler,
		clock:             clock,
		roundCalc:         roundCalc,
		eManagerDB:        eManagerDB,
		toleratedOvertime: toleratedOvertime,
	}, nil
}

func (ctx *rollDPoSCtx) Start(c context.Context) (err error) {
	if err := ctx.chain.Start(c); err != nil {
		return errors.Wrap(err, "Error when starting the chain")
	}
	var eManager *endorsementManager
	if ctx.eManagerDB != nil {
		if err := ctx.eManagerDB.Start(c); err != nil {
			return errors.Wrap(err, "Error when starting the collectionDB")
		}
		eManager, err = newEndorsementManager(ctx.eManagerDB, ctx.blockDeserializer)
		if err != nil {
			return errors.Wrap(err, "Error when creating the endorsement manager")
		}
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

func (ctx *rollDPoSCtx) Chain() ChainManager {
	return ctx.chain
}

func (ctx *rollDPoSCtx) BlockDeserializer() *block.Deserializer {
	return ctx.blockDeserializer
}

func (ctx *rollDPoSCtx) RoundCalculator() *roundCalculator {
	return ctx.roundCalc
}

func (ctx *rollDPoSCtx) Clock() clock.Clock {
	return ctx.clock
}

// CheckVoteEndorser checks if the endorsement's endorser is a valid delegate at the given height
func (ctx *rollDPoSCtx) CheckVoteEndorser(
	height uint64,
	vote *ConsensusVote,
	en *endorsement.Endorsement,
) error {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	endorserAddr := en.Endorser().Address()
	if endorserAddr == nil {
		return errors.New("failed to get address")
	}
	fork, err := ctx.chain.Fork(ctx.round.prevHash)
	if err != nil {
		return errors.Wrapf(err, "failed to get fork at block %d, hash %x", height, ctx.round.prevHash[:])
	}
	roundCalc := ctx.roundCalc.Fork(fork)
	if !roundCalc.IsDelegate(endorserAddr.String(), height) {
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
	endorserAddr := en.Endorser().Address()
	if endorserAddr == nil {
		return errors.New("failed to get address")
	}
	prevHash := proposal.block.PrevHash()
	fork, err := ctx.chain.Fork(prevHash)
	if err != nil {
		return errors.Wrapf(err, "failed to get fork at block %d, hash %x", proposal.block.Height(), prevHash[:])
	}
	roundCalc := ctx.roundCalc.Fork(fork)
	if proposer := roundCalc.Proposer(height, ctx.BlockInterval(height), en.Timestamp()); proposer != endorserAddr.String() {
		return errors.Errorf(
			"%s is not proposer of the corresponding round, %s expected",
			endorserAddr.String(),
			proposer,
		)
	}
	proposerAddr := proposal.ProposerAddress()
	if roundCalc.Proposer(height, ctx.BlockInterval(height), proposal.block.Timestamp()) != proposerAddr {
		return errors.Errorf("%s is not proposer of the corresponding round", proposerAddr)
	}
	if !proposal.block.VerifySignature() {
		return errors.Errorf("invalid block signature")
	}
	if proposerAddr != endorserAddr.String() {
		round, err := roundCalc.NewRound(height, ctx.BlockInterval(height), en.Timestamp(), nil)
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
) []*consensusfsm.ConsensusEvent {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.newConsensusEvent(eventType, data)
}

func (ctx *rollDPoSCtx) NewBackdoorEvt(
	dst fsm.State,
) []*consensusfsm.ConsensusEvent {
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
	_consensusHeightMtc.WithLabelValues().Set(float64(ctx.round.height))
	_timeSlotMtc.WithLabelValues().Set(float64(ctx.round.roundNum))
	return nil
}

func (ctx *rollDPoSCtx) HasDelegate() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.hasDelegate()
}

func (ctx *rollDPoSCtx) Proposal() (interface{}, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	var privateKey crypto.PrivateKey = nil
	proposer := ctx.round.Proposer()
	// TODO: this is to pass unit tests, remove it after the unit tests are fixed
	if proposer == "" {
		privateKey = ctx.priKeys[0]
	} else {
		for i, addr := range ctx.encodedAddrs {
			if addr == proposer {
				privateKey = ctx.priKeys[i]
				break
			}
		}
	}
	if privateKey == nil {
		return nil, nil
	}
	if ctx.round.IsLocked() {
		return ctx.endorseBlockProposal(newBlockProposal(
			ctx.round.Block(ctx.round.HashOfBlockInLock()),
			ctx.round.ProofOfLock(),
		), privateKey)
	}
	return ctx.mintNewBlock(privateKey)
}

func (ctx *rollDPoSCtx) prepareNextProposal(prevHeight uint64, prevHash hash.Hash256) error {
	var (
		height    = prevHeight + 1
		interval  = ctx.BlockInterval(height)
		startTime = ctx.round.StartTime().Add(interval)
		err       error
	)
	fork, err := ctx.chain.Fork(prevHash)
	if err != nil {
		return errors.Wrapf(err, "failed to check fork at block %d, hash %x", prevHeight, prevHash[:])
	}
	roundCalc := ctx.roundCalc.Fork(fork)
	// check if the current node is the next proposer
	nextProposer := roundCalc.Proposer(height, interval, startTime)
	idx := slices.Index(ctx.encodedAddrs, nextProposer)
	if idx < 0 {
		return nil
	}
	privateKey := ctx.priKeys[idx]
	ctx.logger().Debug("prepare next proposal", log.Hex("prevHash", prevHash[:]), zap.Uint64("height", ctx.round.height+1), zap.Time("timestamp", startTime), zap.String("nextproposer", nextProposer))
	go func() {
		blk, err := fork.MintNewBlock(startTime, privateKey, prevHash)
		if err != nil {
			ctx.logger().Error("failed to mint new block", zap.Error(err))
			return
		}
		ctx.logger().Debug("prepared a new block", zap.Uint64("height", blk.Height()), zap.Time("timestamp", blk.Timestamp()))
	}()
	return nil
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
	if !ctx.hasDelegate() && ctx.toleratedOvertime > overTime {
		time.Sleep(ctx.toleratedOvertime - overTime)
		return 0
	}
	return overTime
}

func (ctx *rollDPoSCtx) PreCommitEndorsement() interface{} {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	endorsements := ctx.round.ReadyToCommit(ctx.encodedAddrs)
	if len(endorsements) == 0 {
		// DON'T CHANGE, this is on purpose, because endorsement as nil won't result in a nil "interface {}"
		return nil
	}
	return endorsements
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
		if err := ctx.prepareNextProposal(proposal.block.Height(), blkHash); err != nil {
			ctx.loggerWithStats().Warn("failed to prepare next proposal", zap.Error(err), zap.Uint64("prevHeight", proposal.block.Height()))
		}
		ctx.loggerWithStats().Debug("accept block proposal", log.Hex("block", blockHash))
	} else if ctx.round.IsLocked() {
		blockHash = ctx.round.HashOfBlockInLock()
	} else {
		if err := ctx.prepareNextProposal(ctx.round.Height()-1, ctx.round.PrevHash()); err != nil {
			ctx.loggerWithStats().Warn("failed to prepare next proposal", zap.Error(err), zap.Uint64("prevHeight", ctx.round.Height()-1))
		}
	}
	// TODO: prepare next block if the current node will be a proposer

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
	if ctx.round.Height()%100 == 0 {
		ctx.logger().Info("consensus reached", zap.Uint64("blockHeight", ctx.round.Height()))
	}
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
		log.L().Error("error when committing the block", zap.Error(err))
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
		// TODO: explorer dependency deleted at #1085, need to call putblock related method
	} else {
		ctx.logger().Panic(
			"error when converting a block into a proto msg",
			zap.Uint64("block", pendingBlock.Height()),
		)
	}

	_consensusDurationMtc.WithLabelValues().Set(float64(time.Since(ctx.round.roundStartTime)))
	if pendingBlock.Height() > 1 {
		prevBlkProposeTime, err := ctx.chain.BlockProposeTime(pendingBlock.Height() - 1)
		if err != nil {
			ctx.logger().Error("Error when getting the previous block header.",
				zap.Error(err),
				zap.Uint64("height", pendingBlock.Height()-1),
			)
		}
		_blockIntervalMtc.WithLabelValues().Set(float64(pendingBlock.Timestamp().Sub(prevBlkProposeTime)))
	}
	return true, nil
}

func (ctx *rollDPoSCtx) encodeAndBroadcast(ecm *EndorsedConsensusMessage) error {
	msg, err := ecm.Proto()
	if err != nil {
		return err
	}
	return ctx.broadcastHandler(msg)
}

func (ctx *rollDPoSCtx) Broadcast(endorsedMsg interface{}) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	switch em := endorsedMsg.(type) {
	case *EndorsedConsensusMessage:
		if err := ctx.encodeAndBroadcast(em); err != nil {
			ctx.loggerWithStats().Error("fail to broadcast", zap.Error(err))
		}
	case []*EndorsedConsensusMessage:
		for _, e := range em {
			if err := ctx.encodeAndBroadcast(e); err != nil {
				ctx.loggerWithStats().Error("fail to broadcast", zap.Error(err))
			}
		}
	default:
		ctx.loggerWithStats().Error("invalid message type", zap.Any("message", em))
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

func (ctx *rollDPoSCtx) mintNewBlock(privateKey crypto.PrivateKey) (*EndorsedConsensusMessage, error) {
	var err error
	blk := ctx.round.CachedMintedBlock()
	if blk == nil {
		// in case that there is no cached block in eManagerDB, it mints a new block.
		blk, err = ctx.chain.MintNewBlock(ctx.round.StartTime(), privateKey, ctx.round.PrevHash())
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
	return ctx.endorseBlockProposal(newBlockProposal(blk, proofOfUnlock), privateKey)
}

func (ctx *rollDPoSCtx) hasDelegate() bool {
	if active := ctx.active; !active {
		ctx.logger().Info("current node is in standby mode")
		return false
	}
	return slices.ContainsFunc(ctx.encodedAddrs, ctx.round.IsDelegate)
}

func (ctx *rollDPoSCtx) endorseBlockProposal(proposal *blockProposal, privateKey crypto.PrivateKey) (*EndorsedConsensusMessage, error) {
	ens, err := endorsement.Endorse(proposal, ctx.round.StartTime(), privateKey)
	if err != nil {
		return nil, err
	}
	if len(ens) != 1 {
		return nil, errors.New("invalid number of endorsements")
	}

	return NewEndorsedConsensusMessage(ctx.round.Height(), proposal, ens[0]), nil
}

func (ctx *rollDPoSCtx) logger() *zap.Logger {
	return ctx.round.Log(log.Logger("consensus"))
}

func (ctx *rollDPoSCtx) newConsensusEvent(
	eventType fsm.EventType,
	data interface{},
) []*consensusfsm.ConsensusEvent {
	var msgs []*EndorsedConsensusMessage
	switch ed := data.(type) {
	case []*EndorsedConsensusMessage:
		msgs = ed
	case *EndorsedConsensusMessage:
		msgs = []*EndorsedConsensusMessage{ed}
	default:
		return []*consensusfsm.ConsensusEvent{
			consensusfsm.NewConsensusEvent(
				eventType,
				data,
				ctx.round.Height(),
				ctx.round.Number(),
				ctx.clock.Now(),
			),
		}
	}
	events := make([]*consensusfsm.ConsensusEvent, 0, len(msgs))
	for _, msg := range msgs {
		height := msg.Height()
		roundNum, _, err := ctx.roundCalc.RoundInfo(height, ctx.BlockInterval(height), msg.Endorsement().Timestamp())
		if err != nil {
			ctx.logger().Error(
				"failed to calculate round for generating consensus event",
				zap.String("eventType", string(eventType)),
				zap.Uint64("height", msg.Height()),
				zap.String("timestamp", msg.Endorsement().Timestamp().String()),
				zap.Any("msg", msg),
				zap.Error(err),
			)
			return nil
		}
		events = append(events, consensusfsm.NewConsensusEvent(
			eventType,
			msg,
			msg.Height(),
			roundNum,
			ctx.clock.Now(),
		))
	}

	return events
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
) ([]*EndorsedConsensusMessage, error) {
	vote := NewConsensusVote(
		blkHash,
		topic,
	)
	privKeys := make([]crypto.PrivateKey, 0, len(ctx.priKeys))
	for i, addr := range ctx.encodedAddrs {
		if !ctx.round.IsDelegate(addr) {
			continue
		}
		privKeys = append(privKeys, ctx.priKeys[i])
	}
	ens, err := endorsement.Endorse(vote, timestamp, privKeys...)
	if err != nil {
		return nil, err
	}
	msgs := make([]*EndorsedConsensusMessage, 0, len(ens))
	for _, en := range ens {
		msgs = append(msgs, NewEndorsedConsensusMessage(ctx.round.Height(), vote, en))
	}

	return msgs, nil
}
