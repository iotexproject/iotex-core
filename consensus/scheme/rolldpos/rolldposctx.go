// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"bytes"
	"encoding/hex"
	"sync"
	"time"

	"github.com/facebookgo/clock"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotexrpc"
	"github.com/iotexproject/iotex-core/state"
)

// CandidatesByHeightFunc defines a function to overwrite candidates
type CandidatesByHeightFunc func(uint64) ([]*state.Candidate, error)

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	height          uint64
	number          uint32
	proofOfLock     *endorsement.Set
	timestamp       time.Time
	block           *blockWrapper
	endorsementSets map[string]*endorsement.Set
	proposer        string
}

type rollDPoSCtx struct {
	cfg              config.RollDPoS
	encodedAddr      string
	pubKey           keypair.PublicKey
	priKey           keypair.PrivateKey
	chain            blockchain.Blockchain
	actPool          actpool.ActPool
	broadcastHandler scheme.Broadcast
	epoch            *epochCtx
	round            *roundCtx
	clock            clock.Clock
	rootChainAPI     explorer.Explorer
	// candidatesByHeightFunc is only used for testing purpose
	candidatesByHeightFunc CandidatesByHeightFunc
	mutex                  sync.RWMutex
}

func (ctx *rollDPoSCtx) Prepare() (time.Duration, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	height := ctx.chain.TipHeight() + 1
	if err := ctx.updateEpoch(height); err != nil {
		return ctx.cfg.DelegateInterval, err
	}
	// If the current node is the delegate, move to the next state
	if !ctx.isDelegate() {
		log.L().Info(
			"current node is not a delegate",
			zap.Uint64("epoch", ctx.epoch.num),
			zap.Uint64("height", height),
		)
		return ctx.cfg.DelegateInterval, nil
	}
	log.L().Info(
		"current node is a delegate",
		zap.Uint64("epoch", ctx.epoch.num),
		zap.Uint64("height", height),
	)
	if err := ctx.updateSubEpochNum(height); err != nil {
		return ctx.cfg.DelegateInterval, err
	}
	if err := ctx.updateRound(height); err != nil {
		return ctx.cfg.DelegateInterval, err
	}

	return ctx.round.timestamp.Sub(ctx.clock.Now()), nil
}

func (ctx *rollDPoSCtx) ReadyToCommit() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	if ctx.round.proofOfLock == nil {
		return false
	}
	return ctx.hasEnoughEndorsements(
		ctx.round.proofOfLock.BlockHash(),
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.COMMIT: true,
		},
	)
}

func (ctx *rollDPoSCtx) OnConsensusReached() {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	ctx.logger().Info("consensus reached", zap.Uint64("blockHeight", ctx.round.height))
	pendingBlock := ctx.round.block
	if err := pendingBlock.Block.Finalize(
		ctx.round.proofOfLock,
		ctx.clock.Now(),
	); err != nil {
		ctx.logger().Panic("failed to add endorsements to block", zap.Error(err))
	}
	// Commit and broadcast the pending block
	if err := ctx.chain.CommitBlock(pendingBlock.Block); err != nil {
		// TODO: review the error handling logic (panic?)
		ctx.logger().Panic("error when committing a block", zap.Error(err))
	}
	// Remove transfers in this block from ActPool and reset ActPool state
	ctx.actPool.Reset()
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
		if ctx.round.proposer == ctx.encodedAddr && ctx.chain.ChainAddress() != "" {
			putBlockToParentChain(ctx.rootChainAPI, ctx.chain.ChainAddress(), ctx.pubKey, ctx.priKey, ctx.encodedAddr, pendingBlock.Block)
		}
	} else {
		ctx.logger().Panic(
			"error when converting a block into a proto msg",
			zap.Uint64("block", pendingBlock.Height()),
		)
	}
}

func (ctx *rollDPoSCtx) MintBlock() (consensusfsm.Endorsement, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	blk := ctx.round.block
	if blk == nil {
		actionMap := ctx.actPool.PendingActionMap()
		log.L().Debug("Pick actions from the action pool.", zap.Int("action", len(actionMap)))
		b, err := ctx.chain.MintNewBlock(
			actionMap,
			ctx.pubKey,
			ctx.priKey,
			ctx.encodedAddr,
			ctx.round.timestamp.Unix(),
		)
		if err != nil {
			return nil, err
		}
		blk = &blockWrapper{Block: b, round: ctx.round.number}
	} else {
		// TODO: when time rotation is enabled, proof of lock should be checked
		blk.round = ctx.round.number
	}
	ctx.logger().Info(
		"minted a new block",
		zap.Uint64("height", blk.Height()),
		zap.Int("actions", len(blk.Actions)),
	)

	return blk, nil
}

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

func (ctx *rollDPoSCtx) logger() *zap.Logger {
	return log.L().With(
		zap.Uint64("height", ctx.round.height),
		zap.Uint32("round", ctx.round.number),
		zap.String("proposer", ctx.round.proposer),
	)
}

func (ctx *rollDPoSCtx) LoggerWithStats() *zap.Logger {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.loggerWithStats()
}

func (ctx *rollDPoSCtx) NewProposalEndorsement(en consensusfsm.Endorsement) (consensusfsm.Endorsement, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	if en != nil {
		blk, ok := en.(*blockWrapper)
		if !ok {
			return nil, errors.New("invalid block")
		}
		// TODO: delete the following check
		if blk.Height() != ctx.round.height {
			return nil, errors.Errorf(
				"unexpected block height %d, %d expected",
				blk.Height(),
				ctx.round.height,
			)
		}
		producer := blk.Endorser()
		expectedProposer := ctx.round.proposer
		if producer == "" || producer != expectedProposer {
			return nil, errors.Errorf(
				"unexpected block proposer %s, %s expected",
				producer,
				ctx.round.proposer,
			)
		}
		if producer != ctx.round.proposer || blk.WorkingSet == nil {
			if err := ctx.chain.ValidateBlock(blk.Block); err != nil {
				return nil, errors.Wrapf(err, "error when validating the proposed block")
			}
		}
		ctx.round.block = blk
	}

	return ctx.newEndorsement(endorsement.PROPOSAL)
}

func (ctx *rollDPoSCtx) HasReceivedBlock() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.round.block != nil
}

func (ctx *rollDPoSCtx) NewLockEndorsement() (consensusfsm.Endorsement, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.newEndorsement(endorsement.LOCK)
}

func (ctx *rollDPoSCtx) NewPreCommitEndorsement() (consensusfsm.Endorsement, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.newEndorsement(endorsement.COMMIT)
}

func (ctx *rollDPoSCtx) AddProposalEndorsement(en consensusfsm.Endorsement) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	expectedTopics := map[endorsement.ConsensusVoteTopic]bool{
		endorsement.PROPOSAL: true,
		endorsement.COMMIT:   true, // commit endorse is counted as one proposal endorse
	}
	if err := ctx.processEndorsement(en, expectedTopics); err != nil {
		return err
	}

	hash := en.Hash()
	if ctx.hasEnoughEndorsements(hash, expectedTopics) {
		// TODO: handle the case of multiple prooves of lock
		ctx.round.proofOfLock = ctx.round.endorsementSets[hex.EncodeToString(hash)]
	}

	return nil
}

func (ctx *rollDPoSCtx) IsLocked() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.round.proofOfLock != nil && ctx.round.proofOfLock.BlockHash() != nil
}

func (ctx *rollDPoSCtx) AddLockEndorsement(en consensusfsm.Endorsement) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	return ctx.processEndorsement(
		en,
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.LOCK:   true,
			endorsement.COMMIT: true, // commit endorse is counted as one proposal endorse
		},
	)
}

func (ctx *rollDPoSCtx) ReadyToPreCommit() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	if ctx.round.proofOfLock == nil {
		return false
	}

	return ctx.hasEnoughEndorsements(
		ctx.round.proofOfLock.BlockHash(),
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.LOCK:   true,
			endorsement.COMMIT: true, // commit endorse is counted as one proposal endorse
		},
	)
}

func (ctx *rollDPoSCtx) AddPreCommitEndorsement(en consensusfsm.Endorsement) error {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()

	return ctx.processEndorsement(
		en,
		map[endorsement.ConsensusVoteTopic]bool{
			endorsement.COMMIT: true, // commit endorse is counted as one proposal endorse
		},
	)
}

func (ctx *rollDPoSCtx) BroadcastBlockProposal(block consensusfsm.Endorsement) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	data, err := block.Serialize()
	if err != nil {
		ctx.loggerWithStats().Panic("Failed to serialize block", zap.Error(err))
	}
	if err := ctx.broadcastHandler(&iotexrpc.ConsensusPb{
		Height:    ctx.round.height,
		Round:     ctx.round.number,
		Type:      iotexrpc.ConsensusPb_PROPOSAL,
		Data:      data,
		Timestamp: &timestamp.Timestamp{Seconds: ctx.clock.Now().Unix()},
	}); err != nil {
		ctx.loggerWithStats().Error("fail to broadcast block", zap.Error(err))
	}
}

func (ctx *rollDPoSCtx) BroadcastEndorsement(en consensusfsm.Endorsement) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	data, err := en.Serialize()
	if err != nil {
		ctx.loggerWithStats().Panic("Failed to serialize endorsement", zap.Error(err))
	}
	if err := ctx.broadcastHandler(&iotexrpc.ConsensusPb{
		Height:    ctx.round.height,
		Round:     ctx.round.number,
		Type:      iotexrpc.ConsensusPb_ENDORSEMENT,
		Data:      data,
		Timestamp: &timestamp.Timestamp{Seconds: ctx.clock.Now().Unix()},
	}); err != nil {
		ctx.loggerWithStats().Error("fail to broadcast endorsement", zap.Error(err))
	}
}

func (ctx *rollDPoSCtx) IsStaleEvent(evt *consensusfsm.ConsensusEvent) bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return evt.Height() < ctx.round.height ||
		evt.Height() == ctx.round.height && evt.Round() < ctx.round.number
}

func (ctx *rollDPoSCtx) IsFutureEvent(evt *consensusfsm.ConsensusEvent) bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return evt.Height() > ctx.round.height ||
		evt.Height() == ctx.round.height && evt.Round() > ctx.round.number
}

func (ctx *rollDPoSCtx) IsStaleUnmatchedEvent(evt *consensusfsm.ConsensusEvent) bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.clock.Now().Sub(evt.Timestamp()) > ctx.cfg.FSM.UnmatchedEventTTL
}

func (ctx *rollDPoSCtx) IsDelegate() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.isDelegate()
}

func (ctx *rollDPoSCtx) IsProposer() bool {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()

	return ctx.round.proposer == ctx.encodedAddr
}

func (ctx *rollDPoSCtx) Height() uint64 {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	if ctx.round == nil {
		return 0
	}

	return ctx.round.height
}

///////////////////////////////////////////
// private functions
///////////////////////////////////////////

func (ctx *rollDPoSCtx) hasEnoughEndorsements(
	hash []byte,
	expectedTopics map[endorsement.ConsensusVoteTopic]bool,
) bool {
	set, ok := ctx.round.endorsementSets[hex.EncodeToString(hash)]
	if !ok {
		return false
	}
	validNum := set.NumOfValidEndorsements(
		expectedTopics,
		ctx.epoch.delegates,
	)
	numDelegates := len(ctx.epoch.delegates)
	return numDelegates >= 4 && validNum > numDelegates*2/3 ||
		numDelegates < 4 && validNum >= numDelegates
}

func (ctx *rollDPoSCtx) newConsensusEvent(
	eventType fsm.EventType,
	data interface{},
) *consensusfsm.ConsensusEvent {
	return consensusfsm.NewConsensusEvent(
		eventType,
		data,
		ctx.round.height,
		ctx.round.number,
		ctx.clock.Now(),
	)
}

func (ctx *rollDPoSCtx) loggerWithStats() *zap.Logger {
	numProposals := 0
	numLocks := 0
	numCommits := 0
	if ctx.round.block != nil {
		blkHashHex := hex.EncodeToString(ctx.round.block.Hash())
		endorsementSet := ctx.round.endorsementSets[blkHashHex]
		if endorsementSet != nil {
			numProposals = endorsementSet.NumOfValidEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.PROPOSAL: true,
					endorsement.COMMIT:   true,
				},
				ctx.epoch.delegates,
			)
			numLocks = endorsementSet.NumOfValidEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.LOCK:   true,
					endorsement.COMMIT: true,
				},
				ctx.epoch.delegates,
			)
			numCommits = endorsementSet.NumOfValidEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.COMMIT: true,
				},
				ctx.epoch.delegates,
			)
		}
	}
	return ctx.logger().With(
		zap.Int("numProposals", numProposals),
		zap.Int("numLocks", numLocks),
		zap.Int("numCommits", numCommits),
	)
}

func (ctx *rollDPoSCtx) newEndorsement(topic endorsement.ConsensusVoteTopic) (consensusfsm.Endorsement, error) {
	var hash []byte
	if ctx.round.block != nil {
		hash = ctx.round.block.Hash()
	}
	endorsement := endorsement.NewEndorsement(
		endorsement.NewConsensusVote(
			hash,
			ctx.round.height,
			ctx.round.number,
			topic,
		),
		ctx.pubKey,
		ctx.priKey,
		ctx.encodedAddr,
	)

	return &endorsementWrapper{endorsement}, nil
}

func (ctx *rollDPoSCtx) isProposedBlock(hash []byte) bool {
	if ctx.round.block == nil {
		ctx.logger().Error("block is nil")
		return false
	}
	blkHash := ctx.round.block.Hash()

	return bytes.Equal(hash, blkHash)
}

// TODO: review the endorsement checking
func (ctx *rollDPoSCtx) isDelegateEndorsement(endorser string) bool {
	for _, delegate := range ctx.epoch.delegates {
		if delegate == endorser {
			return true
		}
	}
	return false
}

func (ctx *rollDPoSCtx) processEndorsement(
	en consensusfsm.Endorsement,
	expectedTopics map[endorsement.ConsensusVoteTopic]bool,
) error {
	endorse, ok := en.(*endorsementWrapper)
	if !ok {
		return errors.New("invalid endorsement")
	}
	vote := endorse.ConsensusVote()
	if !ctx.isProposedBlock(vote.BlkHash) {
		return errors.New("the endorsed block was not the proposed block")
	}
	if !ctx.isDelegateEndorsement(endorse.Endorser()) {
		return errors.Errorf("invalid endorser %s", endorse.Endorser())
	}
	if _, ok := expectedTopics[vote.Topic]; !ok {
		return errors.Errorf("invalid consensus topic %v", vote.Topic)
	}
	if vote.Height != ctx.round.height {
		return errors.Errorf(
			"invalid endorsement height %d, %d expected",
			vote.Height,
			ctx.round.height,
		)
	}
	blkHashHex := hex.EncodeToString(vote.BlkHash)
	endorsementSet, ok := ctx.round.endorsementSets[blkHashHex]
	if !ok {
		endorsementSet = endorsement.NewSet(vote.BlkHash)
		ctx.round.endorsementSets[blkHashHex] = endorsementSet
	}

	return endorsementSet.AddEndorsement(endorse.Endorsement)
}

// updateEpoch updates the current epoch
func (ctx *rollDPoSCtx) updateEpoch(height uint64) error {
	epochNum := uint64(0)
	if ctx.epoch != nil {
		epochNum = ctx.epoch.num
	}
	if epochNum < getEpochNum(height, ctx.cfg.NumDelegates, ctx.cfg.NumSubEpochs) {
		epoch, err := ctx.epochCtxByHeight(height)
		if err != nil {
			return err
		}
		ctx.epoch = epoch
	}
	return nil
}

// updateSubEpochNum updates the sub-epoch ordinal number
func (ctx *rollDPoSCtx) updateSubEpochNum(height uint64) error {
	if height < ctx.epoch.height {
		return errors.New("Tip height cannot be less than epoch height")
	}
	numDlgs := ctx.cfg.NumDelegates
	ctx.epoch.subEpochNum = (height - ctx.epoch.height) / uint64(numDlgs)

	return nil
}

// calcRoundNum calcuates the round number based on last block time
func (ctx *rollDPoSCtx) calcRoundNum(
	lastBlockTime time.Time,
	timestamp time.Time,
	interval time.Duration,
) (uint32, error) {
	if !lastBlockTime.Before(timestamp) {
		return 0, errors.New("time since last block cannot be less or equal to 0")
	}
	duration := timestamp.Sub(lastBlockTime)
	if duration <= interval {
		return 0, nil
	}
	overtime := duration % interval
	if overtime >= ctx.cfg.ToleratedOvertime {
		return uint32(duration / interval), nil
	}
	return uint32(duration/interval) - 1, nil
}

func (ctx *rollDPoSCtx) roundCtxByTime(
	epoch *epochCtx,
	height uint64,
	timestamp time.Time,
) (*roundCtx, error) {
	lastBlockTime, err := ctx.getBlockTime(height - 1)
	if err != nil {
		return nil, err
	}
	// proposer interval should be always larger than 0
	interval := ctx.cfg.DelegateInterval
	if interval <= 0 {
		ctx.logger().Panic("invalid proposer interval")
	}
	roundNum, err := ctx.calcRoundNum(lastBlockTime, timestamp, interval)
	if err != nil {
		return nil, err
	}
	timeSlotMtc.WithLabelValues().Set(float64(roundNum))
	ctx.logger().Debug("calculate time slot offset", zap.Uint32("slot", roundNum))
	proposer, err := ctx.rotatedProposer(epoch, height, roundNum)
	if err != nil {
		return nil, err
	}

	return &roundCtx{
		height:          height,
		endorsementSets: make(map[string]*endorsement.Set),
		number:          roundNum,
		proposer:        proposer,
		timestamp:       lastBlockTime.Add(time.Duration(roundNum+1) * interval),
	}, nil
}

// updateRound updates the round
func (ctx *rollDPoSCtx) updateRound(height uint64) error {
	round, err := ctx.roundCtxByTime(ctx.epoch, height, ctx.clock.Now())
	if err != nil {
		return err
	}
	if ctx.round != nil && ctx.round.height == height {
		round.block = ctx.round.block
		round.proofOfLock = ctx.round.proofOfLock
		round.endorsementSets = ctx.round.endorsementSets
		for _, s := range round.endorsementSets {
			s.DeleteEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.PROPOSAL: true,
					endorsement.LOCK:     true,
				},
				round.number,
			)
		}
	}
	ctx.round = round

	return nil
}

func (ctx *rollDPoSCtx) isDelegate() bool {
	for _, d := range ctx.epoch.delegates {
		if ctx.encodedAddr == d {
			return true
		}
	}

	return false
}

// rotatedProposer will rotate among the delegates to choose the proposer. It is pseudo order based on the position
// in the delegate list and the block height
func (ctx *rollDPoSCtx) rotatedProposer(epoch *epochCtx, height uint64, round uint32) (
	proposer string,
	err error,
) {
	delegates := epoch.delegates
	numDelegates := len(delegates)
	if numDelegates == 0 {
		return "", ErrZeroDelegate
	}
	if !ctx.cfg.TimeBasedRotation {
		return delegates[(height)%uint64(numDelegates)], nil
	}
	return delegates[(height+uint64(round))%uint64(numDelegates)], nil
}

// getBlockTime returns the duration since block time
func (ctx *rollDPoSCtx) getBlockTime(height uint64) (time.Time, error) {
	blk, err := ctx.chain.GetBlockByHeight(height)
	if err != nil {
		return time.Now(), errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return time.Unix(blk.Header.Timestamp(), 0), nil
}

func (ctx *rollDPoSCtx) getProposer(
	height uint64,
	round uint32,
	delegates []string,
) (string, error) {
	numDelegates := ctx.cfg.NumDelegates
	if int(numDelegates) != len(delegates) {
		return "", errors.New("delegates number is different from config")
	}
	if !ctx.cfg.TimeBasedRotation {
		return delegates[(height)%uint64(numDelegates)], nil
	}

	return delegates[(height+uint64(round))%uint64(numDelegates)], nil
}

func (ctx *rollDPoSCtx) epochCtxByHeight(height uint64) (*epochCtx, error) {
	f := ctx.candidatesByHeightFunc
	if f == nil {
		f = func(h uint64) ([]*state.Candidate, error) {
			return ctx.chain.CandidatesByHeight(h)
		}
	}

	return newEpochCtx(ctx.cfg.NumDelegates, ctx.cfg.NumSubEpochs, height, f)
}
