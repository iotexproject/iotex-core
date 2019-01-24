// Copyright (c) 2018 IoTeX
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
	"github.com/iotexproject/iotex-core/proto"
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
	round            roundCtx
	clock            clock.Clock
	rootChainAPI     explorer.Explorer
	// candidatesByHeightFunc is only used for testing purpose
	candidatesByHeightFunc CandidatesByHeightFunc
	mutex                  sync.RWMutex
}

func (ctx *rollDPoSCtx) Prepare() (time.Duration, error) {
	ctx.mutex.Lock()
	defer ctx.mutex.Unlock()
	height := ctx.chain.TipHeight()
	epochNum := uint64(0)
	if ctx.epoch != nil {
		epochNum = ctx.epoch.num
	}
	if epochNum < getEpochNum(height+1, ctx.cfg.NumDelegates, ctx.cfg.NumSubEpochs) {
		epoch, err := ctx.epochCtxByHeight(height + 1)
		if err != nil {
			return ctx.cfg.DelegateInterval, err
		}
		ctx.epoch = epoch
	}
	// If the current node is the delegate, move to the next state
	if !ctx.isDelegate() {
		return ctx.cfg.DelegateInterval, nil
	}
	ctx.Logger().Info("current node is the delegate", zap.Uint64("epoch", ctx.epoch.num))
	waitDuration, err := ctx.calcWaitDuration()
	if err != nil {
		return waitDuration, err
	}
	subEpochNum, err := ctx.calcSubEpochNum()
	if err != nil {
		return waitDuration, err
	}
	ctx.epoch.subEpochNum = subEpochNum

	proposer, height, round, err := ctx.rotatedProposer(waitDuration)
	if err != nil {
		return waitDuration, err
	}
	if ctx.round.height != height {
		ctx.round = roundCtx{
			height:          height,
			endorsementSets: make(map[string]*endorsement.Set),
		}
	} else {
		for _, s := range ctx.round.endorsementSets {
			s.DeleteEndorsements(
				map[endorsement.ConsensusVoteTopic]bool{
					endorsement.PROPOSAL: true,
					endorsement.LOCK:     true,
				},
				round,
			)
		}
	}
	ctx.round.number = round
	ctx.round.proposer = proposer
	ctx.round.timestamp = ctx.clock.Now()

	return waitDuration, nil
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
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	ctx.Logger().Info("consensus reached", zap.Uint64("blockHeight", ctx.round.height))
	pendingBlock := ctx.round.block
	if err := pendingBlock.Block.Finalize(
		ctx.round.proofOfLock,
		ctx.round.number,
		ctx.clock.Now(),
	); err != nil {
		ctx.Logger().Panic("failed to add endorsements to block", zap.Error(err))
	}
	// Commit and broadcast the pending block
	if err := ctx.chain.CommitBlock(pendingBlock.Block); err != nil {
		// TODO: review the error handling logic (panic?)
		ctx.Logger().Panic("error when committing a block", zap.Error(err))
	}
	// Remove transfers in this block from ActPool and reset ActPool state
	ctx.actPool.Reset()
	// Broadcast the committed block to the network
	if blkProto := pendingBlock.ConvertToBlockPb(); blkProto != nil {
		if err := ctx.broadcastHandler(blkProto); err != nil {
			ctx.Logger().Error(
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
		ctx.Logger().Panic(
			"error when converting a block into a proto msg",
			zap.Uint64("block", pendingBlock.Height()),
		)
	}
}

func (ctx *rollDPoSCtx) MintBlock() (consensusfsm.Endorsement, error) {
	ctx.mutex.RLock()
	defer ctx.mutex.RUnlock()
	blk := ctx.round.block
	if blk == nil {
		actionMap := ctx.actPool.PendingActionMap()
		ctx.Logger().Debug(
			"pick actions from the action pool",
			zap.Int("action", len(actionMap)),
		)
		b, err := ctx.chain.MintNewBlock(actionMap, ctx.pubKey, ctx.priKey, ctx.encodedAddr)
		if err != nil {
			return nil, err
		}
		blk = &blockWrapper{Block: b, round: ctx.round.number}
	} else {
		// TODO: when time rotation is enabled, proof of lock should be checked
		blk.round = ctx.round.number
	}
	ctx.Logger().Info(
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
		containCoinbase := true
		if err := ctx.chain.ValidateBlock(blk.Block, containCoinbase); err != nil {
			return nil, errors.Wrapf(err, "error when validating the proposed block")
		}
	}
	ctx.round.block = blk

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
	if err := ctx.broadcastHandler(&iproto.ConsensusPb{
		Height:    ctx.round.height,
		Round:     ctx.round.number,
		Type:      iproto.ConsensusPb_PROPOSAL,
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
	if err := ctx.broadcastHandler(&iproto.ConsensusPb{
		Height:    ctx.round.height,
		Round:     ctx.round.number,
		Type:      iproto.ConsensusPb_ENDORSEMENT,
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

func (ctx *rollDPoSCtx) calcWaitDuration() (time.Duration, error) {
	// TODO: Update wait duration calculation algorithm
	// If the proposal interval is not set (not zero), the next round will only be started after the configured duration
	// after last block's creation time, so that we could keep the constant
	waitDuration := time.Duration(0)
	// If we have the cached last block, we get the timestamp from it
	duration, err := ctx.calcDurationSinceLastBlock()
	if err != nil {
		return waitDuration, err
	}
	interval := ctx.cfg.FSM.ProposerInterval
	if interval > 0 {
		waitDuration = (interval - (duration % interval)) % interval
	}

	return waitDuration, nil
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
	return ctx.Logger().With(
		zap.Int("numProposals", numProposals),
		zap.Int("numLocks", numLocks),
		zap.Int("numCommits", numCommits),
	)
}

func (ctx *rollDPoSCtx) newEndorsement(topic endorsement.ConsensusVoteTopic) (consensusfsm.Endorsement, error) {
	endorsement := endorsement.NewEndorsement(
		endorsement.NewConsensusVote(
			ctx.round.block.Hash(),
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
		ctx.Logger().Error("block is nil")
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
		return errors.Errorf("invalid consensus topic %s", vote.Topic)
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

// calcSubEpochNum calculates the sub-epoch ordinal number
func (ctx *rollDPoSCtx) calcSubEpochNum() (uint64, error) {
	height := ctx.chain.TipHeight() + 1
	if height < ctx.epoch.height {
		return 0, errors.New("Tip height cannot be less than epoch height")
	}
	numDlgs := ctx.cfg.NumDelegates
	subEpochNum := (height - ctx.epoch.height) / uint64(numDlgs)
	return subEpochNum, nil
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
func (ctx *rollDPoSCtx) rotatedProposer(offsetDuration time.Duration) (
	proposer string,
	height uint64,
	round uint32,
	err error,
) {
	// Next block height
	height = ctx.chain.TipHeight() + 1
	round, proposer, err = ctx.calcProposer(height, ctx.epoch.delegates, offsetDuration)

	return proposer, height, round, err
}

// calcProposer calculates the proposer for the block at a given height
func (ctx *rollDPoSCtx) calcProposer(
	height uint64,
	delegates []string,
	offsetDuration time.Duration,
) (uint32, string, error) {
	numDelegates := len(delegates)
	if numDelegates == 0 {
		return 0, "", ErrZeroDelegate
	}
	timeSlotIndex := uint32(0)
	if ctx.cfg.FSM.ProposerInterval > 0 {
		duration, err := ctx.calcDurationSinceLastBlock()
		if err != nil || duration < 0 {
			if !ctx.cfg.TimeBasedRotation {
				return 0, delegates[(height)%uint64(numDelegates)], nil
			}
			return 0, "", errors.Wrap(err, "error when computing the duration since last block time")
		}
		duration += offsetDuration
		if duration > ctx.cfg.FSM.ProposerInterval {
			timeSlotIndex = uint32(duration/ctx.cfg.FSM.ProposerInterval) - 1
		}
	}
	if !ctx.cfg.TimeBasedRotation {
		return timeSlotIndex, delegates[(height)%uint64(numDelegates)], nil
	}
	// TODO: should downgrade to debug level in the future
	ctx.Logger().Info("calculate time slot offset", zap.Uint32("slot", timeSlotIndex))
	timeSlotMtc.WithLabelValues().Set(float64(timeSlotIndex))
	return timeSlotIndex, delegates[(height+uint64(timeSlotIndex))%uint64(numDelegates)], nil
}

// calcDurationSinceLastBlock returns the duration since last block time
func (ctx *rollDPoSCtx) calcDurationSinceLastBlock() (time.Duration, error) {
	height := ctx.chain.TipHeight()
	blk, err := ctx.chain.GetBlockByHeight(height)
	if err != nil {
		return 0, errors.Wrapf(err, "error when getting the block at height: %d", height)
	}
	return ctx.clock.Now().Sub(time.Unix(blk.Header.Timestamp(), 0)), nil
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
