// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"time"

	"github.com/iotexproject/go-fsm"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	// ErrNewRollDPoS indicates the error of constructing RollDPoS
	ErrNewRollDPoS = errors.New("error when constructing RollDPoS")
	// ErrZeroDelegate indicates seeing 0 delegates in the network
	ErrZeroDelegate = errors.New("zero delegates in the network")
	// ErrNotEnoughCandidates indicates there are not enough candidates from the candidate pool
	ErrNotEnoughCandidates = errors.New("Candidate pool does not have enough candidates")
)

type (
	// Config is the config struct for RollDPoS consensus package
	Config struct {
		FSM               consensusfsm.Timing `yaml:"fsm"`
		ToleratedOvertime time.Duration       `yaml:"toleratedOvertime"`
		Delay             time.Duration       `yaml:"delay"`
		ConsensusDBPath   string              `yaml:"consensusDBPath"`
	}

	// ChainManager defines the blockchain interface
	ChainManager interface {
		// BlockProposeTime return propose time by height
		BlockProposeTime(uint64) (time.Time, error)
		// BlockCommitTime return commit time by height
		BlockCommitTime(uint64) (time.Time, error)
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(timestamp time.Time) (*block.Block, error)
		// CommitBlock validates and appends a block to the chain
		CommitBlock(blk *block.Block) error
		// ValidateBlock validates a new block before adding it to the blockchain
		ValidateBlock(blk *block.Block) error
		// TipHeight returns tip block's height
		TipHeight() uint64
		// ChainAddress returns chain address on parent chain, the root chain return empty.
		ChainAddress() string
	}

	chainManager struct {
		bc blockchain.Blockchain
	}
)

// DefaultConfig is the default config
var DefaultConfig = Config{
	FSM: consensusfsm.Timing{
		UnmatchedEventTTL:            3 * time.Second,
		UnmatchedEventInterval:       100 * time.Millisecond,
		AcceptBlockTTL:               4 * time.Second,
		AcceptProposalEndorsementTTL: 2 * time.Second,
		AcceptLockEndorsementTTL:     2 * time.Second,
		CommitTTL:                    2 * time.Second,
		EventChanSize:                10000,
	},
	ToleratedOvertime: 2 * time.Second,
	Delay:             5 * time.Second,
	ConsensusDBPath:   "/var/data/consensus.db",
}

// NewChainManager creates a chain manager
func NewChainManager(bc blockchain.Blockchain) ChainManager {
	return &chainManager{
		bc: bc,
	}
}

// BlockProposeTime return propose time by height
func (cm *chainManager) BlockProposeTime(height uint64) (time.Time, error) {
	if height == 0 {
		return time.Unix(cm.bc.Genesis().Timestamp, 0), nil
	}
	header, err := cm.bc.BlockHeaderByHeight(height)
	if err != nil {
		return time.Time{}, errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return header.Timestamp(), nil
}

// BlockCommitTime return commit time by height
func (cm *chainManager) BlockCommitTime(height uint64) (time.Time, error) {
	footer, err := cm.bc.BlockFooterByHeight(height)
	if err != nil {
		return time.Time{}, errors.Wrapf(
			err, "error when getting the block at height: %d",
			height,
		)
	}
	return footer.CommitTime(), nil
}

// MintNewBlock creates a new block with given actions
func (cm *chainManager) MintNewBlock(timestamp time.Time) (*block.Block, error) {
	return cm.bc.MintNewBlock(timestamp)
}

// CommitBlock validates and appends a block to the chain
func (cm *chainManager) CommitBlock(blk *block.Block) error {
	return cm.bc.CommitBlock(blk)
}

// ValidateBlock validates a new block before adding it to the blockchain
func (cm *chainManager) ValidateBlock(blk *block.Block) error {
	return cm.bc.ValidateBlock(blk)
}

// TipHeight returns tip block's height
func (cm *chainManager) TipHeight() uint64 {
	return cm.bc.TipHeight()
}

// ChainAddress returns chain address on parent chain, the root chain return empty.
func (cm *chainManager) ChainAddress() string {
	return cm.bc.ChainAddress()
}

// RollDPoS is Roll-DPoS consensus main entrance
type RollDPoS struct {
	cfsm       *consensusfsm.ConsensusFSM
	ctx        *RollDPoSCtx
	startDelay time.Duration
	ready      chan interface{}
}

// NewRollDPoS returns a new instance of RollDPoS
func NewRollDPoS(cfsm *consensusfsm.ConsensusFSM, ctx *RollDPoSCtx, delay time.Duration) *RollDPoS {
	return &RollDPoS{
		cfsm:       cfsm,
		ctx:        ctx,
		startDelay: delay,
		ready:      make(chan interface{}),
	}
}

// Start starts RollDPoS consensus
func (r *RollDPoS) Start(ctx context.Context) error {
	if err := r.ctx.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting the roll dpos context")
	}
	if err := r.cfsm.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting the consensus FSM")
	}
	if _, err := r.cfsm.BackToPrepare(r.startDelay); err != nil {
		return err
	}
	close(r.ready)
	return nil
}

// Stop stops RollDPoS consensus
func (r *RollDPoS) Stop(ctx context.Context) error {
	if err := r.cfsm.Stop(ctx); err != nil {
		return errors.Wrap(err, "error when stopping the consensus FSM")
	}
	return errors.Wrap(r.ctx.Stop(ctx), "error when stopping the roll dpos context")
}

// HandleConsensusMsg handles incoming consensus message
func (r *RollDPoS) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	// Do not handle consensus message if the node is not active in consensus
	if !r.ctx.Active() {
		return nil
	}
	<-r.ready
	consensusHeight := r.ctx.Height()
	switch {
	case consensusHeight == 0:
		log.Logger("consensus").Debug("consensus component is not ready yet")
		return nil
	case msg.Height < consensusHeight:
		log.Logger("consensus").Debug(
			"old consensus message",
			zap.Uint64("consensusHeight", consensusHeight),
			zap.Uint64("msgHeight", msg.Height),
		)
		return nil
	case msg.Height > consensusHeight+1:
		log.Logger("consensus").Debug(
			"future consensus message",
			zap.Uint64("consensusHeight", consensusHeight),
			zap.Uint64("msgHeight", msg.Height),
		)
		return nil
	}
	endorsedMessage := &EndorsedConsensusMessage{}
	if err := endorsedMessage.LoadProto(msg, r.ctx.blockDeserializer); err != nil {
		return errors.Wrapf(err, "failed to decode endorsed consensus message")
	}
	if !endorsement.VerifyEndorsedDocument(endorsedMessage) {
		return errors.New("failed to verify signature in endorsement")
	}
	en := endorsedMessage.Endorsement()
	switch consensusMessage := endorsedMessage.Document().(type) {
	case *blockProposal:
		if err := r.ctx.CheckBlockProposer(endorsedMessage.Height(), consensusMessage, en); err != nil {
			return errors.Wrap(err, "failed to verify block proposal")
		}
		r.cfsm.ProduceReceiveBlockEvent(endorsedMessage)
		return nil
	case *ConsensusVote:
		if err := r.ctx.CheckVoteEndorser(endorsedMessage.Height(), consensusMessage, en); err != nil {
			return errors.Wrapf(err, "failed to verify vote")
		}
		switch consensusMessage.Topic() {
		case PROPOSAL:
			r.cfsm.ProduceReceiveProposalEndorsementEvent(endorsedMessage)
		case LOCK:
			r.cfsm.ProduceReceiveLockEndorsementEvent(endorsedMessage)
		case COMMIT:
			r.cfsm.ProduceReceivePreCommitEndorsementEvent(endorsedMessage)
		}
		return nil
	// TODO: response block by hash, requestBlock.BlockHash
	default:
		return errors.Errorf("Invalid consensus message type %+v", msg)
	}
}

// Calibrate called on receive a new block not via consensus
func (r *RollDPoS) Calibrate(height uint64) {
	r.cfsm.Calibrate(height)
}

// ValidateBlockFooter validates the signatures in the block footer
func (r *RollDPoS) ValidateBlockFooter(blk *block.Block) error {
	height := blk.Height()
	round, err := r.ctx.roundCalc.NewRound(height, r.ctx.BlockInterval(height), blk.Timestamp(), nil)
	if err != nil {
		return err
	}
	if !round.IsDelegate(blk.ProducerAddress()) {
		return errors.Errorf(
			"block proposer %s is not a valid delegate",
			blk.ProducerAddress(),
		)
	}
	if err := round.AddBlock(blk); err != nil {
		return err
	}
	blkHash := blk.HashBlock()
	for _, en := range blk.Endorsements() {
		if err := round.AddVoteEndorsement(
			NewConsensusVote(blkHash[:], COMMIT),
			en,
		); err != nil {
			return err
		}
	}
	if !round.EndorsedByMajority(blkHash[:], []ConsensusVoteTopic{COMMIT}) {
		return ErrInsufficientEndorsements
	}

	return nil
}

// Metrics returns RollDPoS consensus metrics
func (r *RollDPoS) Metrics() (scheme.ConsensusMetrics, error) {
	var metrics scheme.ConsensusMetrics
	height := r.ctx.chain.TipHeight()
	round, err := r.ctx.roundCalc.NewRound(height+1, r.ctx.BlockInterval(height), r.ctx.clock.Now(), nil)
	if err != nil {
		return metrics, errors.Wrap(err, "error when calculating round")
	}

	return scheme.ConsensusMetrics{
		LatestEpoch:         round.EpochNum(),
		LatestHeight:        height,
		LatestDelegates:     round.Delegates(),
		LatestBlockProducer: round.proposer,
	}, nil
}

// NumPendingEvts returns the number of pending events
func (r *RollDPoS) NumPendingEvts() int {
	return r.cfsm.NumPendingEvents()
}

// CurrentState returns the current state
func (r *RollDPoS) CurrentState() fsm.State {
	return r.cfsm.CurrentState()
}

// Activate activates or pauses the roll-DPoS consensus. When it is deactivated, the node will finish the current
// consensus round if it is doing the work and then return the the initial state
func (r *RollDPoS) Activate(active bool) {
	r.ctx.Activate(active)
	// reactivate cfsm if the node is reactivated
	if _, err := r.cfsm.BackToPrepare(0); err != nil {
		log.L().Panic("Failed to reactivate cfsm", zap.Error(err))
	}
}

// Active is true if the roll-DPoS consensus is active, or false if it is stand-by
func (r *RollDPoS) Active() bool {
	return r.ctx.Active() || r.cfsm.CurrentState() != consensusfsm.InitState
}
