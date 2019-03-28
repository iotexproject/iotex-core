// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

var (
	timeSlotMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_time_slot",
			Help: "Consensus time slot",
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
)

func init() {
	prometheus.MustRegister(timeSlotMtc)
	prometheus.MustRegister(blockIntervalMtc)
}

var (
	// ErrNewRollDPoS indicates the error of constructing RollDPoS
	ErrNewRollDPoS = errors.New("error when constructing RollDPoS")
	// ErrZeroDelegate indicates seeing 0 delegates in the network
	ErrZeroDelegate = errors.New("zero delegates in the network")
	// ErrNotEnoughCandidates indicates there are not enough candidates from the candidate pool
	ErrNotEnoughCandidates = errors.New("Candidate pool does not have enough candidates")
)

type blockWrapper struct {
	*block.Block

	round uint32
}

func (bw *blockWrapper) Hash() []byte {
	hash := bw.HashBlock()

	return hash[:]
}

func (bw *blockWrapper) Endorser() string {
	return bw.ProducerAddress()
}

func (bw *blockWrapper) Round() uint32 {
	return bw.round
}

// RollDPoS is Roll-DPoS consensus main entrance
type RollDPoS struct {
	cfsm  *consensusfsm.ConsensusFSM
	ctx   *rollDPoSCtx
	ready chan interface{}
}

// Start starts RollDPoS consensus
func (r *RollDPoS) Start(ctx context.Context) error {
	if err := r.cfsm.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting the consensus FSM")
	}
	if _, err := r.cfsm.BackToPrepare(r.ctx.cfg.Delay); err != nil {
		return err
	}
	close(r.ready)
	return nil
}

// Stop stops RollDPoS consensus
func (r *RollDPoS) Stop(ctx context.Context) error {
	return errors.Wrap(r.cfsm.Stop(ctx), "error when stopping the consensus FSM")
}

// HandleConsensusMsg handles incoming consensus message
func (r *RollDPoS) HandleConsensusMsg(msg *iotextypes.ConsensusMessage) error {
	<-r.ready
	consensusHeight := r.ctx.Height()
	switch {
	case consensusHeight == 0:
		log.L().Debug("consensus component is not ready yet")
		return nil
	case msg.Height < consensusHeight:
		log.L().Debug(
			"old consensus message",
			zap.Uint64("consensusHeight", consensusHeight),
			zap.Uint64("msgHeight", msg.Height),
		)
		return nil
	case msg.Height > consensusHeight+1:
		log.L().Debug(
			"future consensus message",
			zap.Uint64("consensusHeight", consensusHeight),
			zap.Uint64("msgHeight", msg.Height),
		)
		return nil
	}
	endorsedMessage := &EndorsedConsensusMessage{}
	if err := endorsedMessage.LoadProto(msg); err != nil {
		return errors.Wrapf(err, "failed to decode endorsed consensus message")
	}
	if !endorsement.VerifyEndorsedDocument(endorsedMessage) {
		return errors.New("failed to verify signature in endorsement")
	}
	en := endorsedMessage.Endorsement()
	switch consensusMessage := endorsedMessage.Document().(type) {
	case *blockProposal:
		if err := r.ctx.VerifyBlockProposal(endorsedMessage.Height(), consensusMessage, en); err != nil {
			return errors.Wrap(err, "failed to verify block proposal")
		}
		r.cfsm.ProduceReceiveBlockEvent(endorsedMessage)
		return nil
	case *ConsensusVote:
		if err := r.ctx.VerifyVote(endorsedMessage.Height(), consensusMessage, en); err != nil {
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
	round, err := r.ctx.RoundCalc().NewRound(blk.Height(), blk.Timestamp())
	if err != nil {
		return err
	}
	if round.Proposer() != blk.ProducerAddress() {
		return errors.Errorf(
			"block proposer %s is invalid, %s expected",
			blk.ProducerAddress(),
			round.proposer,
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
	round, err := r.ctx.RoundCalc().NewRound(height+1, r.ctx.clock.Now())
	if err != nil {
		return metrics, errors.Wrap(err, "error when calculating round")
	}
	// Get all candidates
	candidates, err := r.ctx.chain.CandidatesByHeight(height)
	if err != nil {
		return metrics, errors.Wrap(err, "error when getting all candidates")
	}
	candidateAddresses := make([]string, len(candidates))
	for i, c := range candidates {
		candidateAddresses[i] = c.Address
	}

	return scheme.ConsensusMetrics{
		LatestEpoch:         round.EpochNum(),
		LatestHeight:        height,
		LatestDelegates:     round.Delegates(),
		LatestBlockProducer: r.ctx.round.proposer,
		Candidates:          candidateAddresses,
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

// Builder is the builder for RollDPoS
type Builder struct {
	cfg config.Config
	// TODO: we should use keystore in the future
	encodedAddr            string
	priKey                 keypair.PrivateKey
	chain                  blockchain.Blockchain
	actPool                actpool.ActPool
	broadcastHandler       scheme.Broadcast
	clock                  clock.Clock
	rootChainAPI           explorer.Explorer
	rp                     *rolldpos.Protocol
	candidatesByHeightFunc CandidatesByHeightFunc
}

// NewRollDPoSBuilder instantiates a Builder instance
func NewRollDPoSBuilder() *Builder {
	return &Builder{}
}

// SetConfig sets config
func (b *Builder) SetConfig(cfg config.Config) *Builder {
	b.cfg = cfg
	return b
}

// SetAddr sets the address and key pair for signature
func (b *Builder) SetAddr(encodedAddr string) *Builder {
	b.encodedAddr = encodedAddr
	return b
}

// SetPriKey sets the private key
func (b *Builder) SetPriKey(priKey keypair.PrivateKey) *Builder {
	b.priKey = priKey
	return b
}

// SetBlockchain sets the blockchain APIs
func (b *Builder) SetBlockchain(chain blockchain.Blockchain) *Builder {
	b.chain = chain
	return b
}

// SetActPool sets the action pool APIs
func (b *Builder) SetActPool(actPool actpool.ActPool) *Builder {
	b.actPool = actPool
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

// SetRootChainAPI sets root chain API
func (b *Builder) SetRootChainAPI(api explorer.Explorer) *Builder {
	b.rootChainAPI = api
	return b
}

// SetCandidatesByHeightFunc sets candidatesByHeightFunc
func (b *Builder) SetCandidatesByHeightFunc(
	candidatesByHeightFunc CandidatesByHeightFunc,
) *Builder {
	b.candidatesByHeightFunc = candidatesByHeightFunc
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
	if b.actPool == nil {
		return nil, errors.Wrap(ErrNewRollDPoS, "action pool APIs is nil")
	}
	if b.broadcastHandler == nil {
		return nil, errors.Wrap(ErrNewRollDPoS, "broadcast callback is nil")
	}
	if b.clock == nil {
		b.clock = clock.New()
	}
	ctx := newRollDPoSCtx(
		b.cfg.Consensus.RollDPoS,
		b.cfg.Genesis.Blockchain.BlockInterval,
		b.cfg.Consensus.RollDPoS.ToleratedOvertime,
		b.cfg.Genesis.TimeBasedRotation,
		b.rootChainAPI,
		b.chain,
		b.actPool,
		b.rp,
		b.broadcastHandler,
		b.candidatesByHeightFunc,
		b.encodedAddr,
		b.priKey,
		b.clock,
	)
	cfsm, err := consensusfsm.NewConsensusFSM(b.cfg.Consensus.RollDPoS.FSM, ctx, b.clock)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing the consensus FSM")
	}
	return &RollDPoS{
		cfsm:  cfsm,
		ctx:   ctx,
		ready: make(chan interface{}),
	}, nil
}
