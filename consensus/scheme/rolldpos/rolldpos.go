// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"time"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-fsm"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/v2/consensus/scheme"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/endorsement"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
		FSM               consensusfsm.ConsensusTiming `yaml:"fsm"`
		ToleratedOvertime time.Duration                `yaml:"toleratedOvertime"`
		Delay             time.Duration                `yaml:"delay"`
		ConsensusDBPath   string                       `yaml:"consensusDBPath"`
	}
)

// DefaultConfig is the default config
var DefaultConfig = Config{
	FSM: consensusfsm.ConsensusTiming{
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

// RollDPoS is Roll-DPoS consensus main entrance
type RollDPoS struct {
	cfsm       *consensusfsm.ConsensusFSM
	ctx        RDPoSCtx
	startDelay time.Duration
	ready      chan interface{}
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
	if err := endorsedMessage.LoadProto(msg, r.ctx.BlockDeserializer()); err != nil {
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
	roundCalc := r.ctx.RoundCalculator().Fork(r.ctx.Chain())
	round, err := roundCalc.NewRound(height, r.ctx.BlockInterval(height), blk.Timestamp(), nil)
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
	height := r.ctx.Chain().TipHeight()
	round, err := r.ctx.RoundCalculator().NewRound(height+1, r.ctx.BlockInterval(height), r.ctx.Clock().Now(), nil)
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

type (
	// BuilderConfig returns the configuration of the builder
	BuilderConfig struct {
		Chain              blockchain.Config
		Consensus          Config
		Scheme             string
		DardanellesUpgrade consensusfsm.DardanellesUpgrade
		WakeUpgrade        consensusfsm.WakeUpgrade
		DB                 db.Config
		Genesis            genesis.Genesis
		SystemActive       bool
	}

	// Builder is the builder for rollDPoS
	Builder struct {
		cfg BuilderConfig
		// TODO: we should use keystore in the future
		encodedAddr       string
		priKey            []crypto.PrivateKey
		chain             ChainManager
		blockDeserializer *block.Deserializer
		broadcastHandler  scheme.Broadcast
		clock             clock.Clock
		// TODO: explorer dependency deleted at #1085, need to add api params
		rp                   *rolldpos.Protocol
		delegatesByEpochFunc NodesSelectionByEpochFunc
		proposersByEpochFunc NodesSelectionByEpochFunc
	}
)

// NewRollDPoSBuilder instantiates a Builder instance
func NewRollDPoSBuilder() *Builder {
	return &Builder{}
}

// SetConfig sets config
func (b *Builder) SetConfig(cfg BuilderConfig) *Builder {
	b.cfg = cfg
	return b
}

// SetPriKey sets the private key
func (b *Builder) SetPriKey(priKeys ...crypto.PrivateKey) *Builder {
	b.priKey = priKeys
	return b
}

// SetChainManager sets the blockchain APIs
func (b *Builder) SetChainManager(chain ChainManager) *Builder {
	b.chain = chain
	return b
}

// SetBlockDeserializer set block deserializer
func (b *Builder) SetBlockDeserializer(deserializer *block.Deserializer) *Builder {
	b.blockDeserializer = deserializer
	return b
}

// SetBroadcast sets the broadcast callback
func (b *Builder) SetBroadcast(broadcastHandler scheme.Broadcast) *Builder {
	b.broadcastHandler = broadcastHandler
	return b
}

// SetClock sets the clock
func (b *Builder) SetClock(clock clock.Clock) *Builder {
	b.clock = clock
	return b
}

// SetDelegatesByEpochFunc sets delegatesByEpochFunc
func (b *Builder) SetDelegatesByEpochFunc(
	delegatesByEpochFunc NodesSelectionByEpochFunc,
) *Builder {
	b.delegatesByEpochFunc = delegatesByEpochFunc
	return b
}

// SetProposersByEpochFunc sets proposersByEpochFunc
func (b *Builder) SetProposersByEpochFunc(
	proposersByEpochFunc NodesSelectionByEpochFunc,
) *Builder {
	b.proposersByEpochFunc = proposersByEpochFunc
	return b
}

// RegisterProtocol sets the rolldpos protocol
func (b *Builder) RegisterProtocol(rp *rolldpos.Protocol) *Builder {
	b.rp = rp
	return b
}

// Build builds a RollDPoS consensus module
func (b *Builder) Build() (*RollDPoS, error) {
	if b.chain == nil {
		return nil, errors.Wrap(ErrNewRollDPoS, "blockchain APIs is nil")
	}
	if b.broadcastHandler == nil {
		return nil, errors.Wrap(ErrNewRollDPoS, "broadcast callback is nil")
	}
	if b.clock == nil {
		b.clock = clock.New()
	}
	b.cfg.DB.DbPath = b.cfg.Consensus.ConsensusDBPath
	ctx, err := NewRollDPoSCtx(
		consensusfsm.NewConsensusConfig(b.cfg.Consensus.FSM, b.cfg.DardanellesUpgrade, b.cfg.WakeUpgrade, b.cfg.Genesis, b.cfg.Consensus.Delay),
		b.cfg.DB,
		b.cfg.SystemActive,
		b.cfg.Consensus.ToleratedOvertime,
		b.cfg.Genesis.TimeBasedRotation,
		b.chain,
		b.blockDeserializer,
		b.rp,
		b.broadcastHandler,
		b.delegatesByEpochFunc,
		b.proposersByEpochFunc,
		b.priKey,
		b.clock,
		b.cfg.Genesis.BeringBlockHeight,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing consensus context")
	}
	cfsm, err := consensusfsm.NewConsensusFSM(ctx, b.clock)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing the consensus FSM")
	}
	return &RollDPoS{
		cfsm:       cfsm,
		ctx:        ctx,
		startDelay: b.cfg.Consensus.Delay,
		ready:      make(chan interface{}),
	}, nil
}
