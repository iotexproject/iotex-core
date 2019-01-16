// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"time"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-fsm"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/consensusfsm"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/proto"
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

const sigSize = 5 // number of uint32s in BLS sig

func init() {
	prometheus.MustRegister(timeSlotMtc)
	prometheus.MustRegister(blockIntervalMtc)
}

var (
	// ErrNewRollDPoS indicates the error of constructing RollDPoS
	ErrNewRollDPoS = errors.New("error when constructing RollDPoS")
	// ErrZeroDelegate indicates seeing 0 delegates in the network
	ErrZeroDelegate = errors.New("zero delegates in the network")
)

var (
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

type endorsementWrapper struct {
	*endorsement.Endorsement
}

func (ew *endorsementWrapper) Hash() []byte {
	return ew.ConsensusVote().BlkHash
}

func (ew *endorsementWrapper) Height() uint64 {
	return ew.ConsensusVote().Height
}

func (ew *endorsementWrapper) Round() uint32 {
	return ew.ConsensusVote().Round
}

func (ew *endorsementWrapper) Topic() endorsement.ConsensusVoteTopic {
	return ew.ConsensusVote().Topic
}

func verifyDKGSignature(blk *block.Block, seedByte []byte) error {
	return crypto.BLS.Verify(blk.DKGPubkey(), seedByte, blk.DKGSignature())
}

// RollDPoS is Roll-DPoS consensus main entrance
type RollDPoS struct {
	cfsm *consensusfsm.ConsensusFSM
	ctx  *rollDPoSCtx
}

// Start starts RollDPoS consensus
func (r *RollDPoS) Start(ctx context.Context) error {
	if err := r.cfsm.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting the consensus FSM")
	}
	r.cfsm.ProducePrepareEvent(r.ctx.cfg.Delay)
	return nil
}

// Stop stops RollDPoS consensus
func (r *RollDPoS) Stop(ctx context.Context) error {
	return errors.Wrap(r.cfsm.Stop(ctx), "error when stopping the consensus FSM")
}

// HandleConsensusMsg handles incoming consensus message
func (r *RollDPoS) HandleConsensusMsg(msg *iproto.ConsensusPb) error {
	chainHeight := r.ctx.chain.TipHeight()
	if msg.Height <= chainHeight {
		log.L().Debug(
			"old consensus message",
			zap.Uint64("chainHeight", chainHeight),
			zap.Uint64("msgHeight", msg.Height),
		)
		return nil
	}
	data := msg.Data
	switch msg.Type {
	case iproto.ConsensusPb_PROPOSAL:
		block := &block.Block{}
		if err := block.Deserialize(data); err != nil {
			return errors.Wrap(err, "failed to deserialize block")
		}
		// TODO: add proof of lock
		if msg.Height != block.Height() {
			return errors.Errorf(
				"block height %d is not the same as consensus message height",
				block.Height(),
				msg.Height,
			)
		}
		if !block.VerifySignature() {
			return errors.Errorf("invalid block signature")
		}
		r.cfsm.ProduceReceiveBlockEvent(&blockWrapper{block, msg.Round})
	case iproto.ConsensusPb_ENDORSEMENT:
		en := &endorsement.Endorsement{}
		if err := en.Deserialize(data); err != nil {
			return errors.Wrap(err, "error when deserializing a msg to endorsement")
		}
		ew := &endorsementWrapper{en}
		if ew.Height() != msg.Height {
			return errors.Errorf(
				"endorsement height %d is not the same as consensus message height",
				ew.Height(),
				msg.Height,
			)
		}
		if !en.VerifySignature() {
			return errors.Errorf("invalid endorsement signature")
		}
		switch ew.Topic() {
		case endorsement.PROPOSAL:
			r.cfsm.ProduceReceiveProposalEndorsementEvent(ew)
		case endorsement.LOCK:
			r.cfsm.ProduceReceiveLockEndorsementEvent(ew)
		case endorsement.COMMIT:
			r.cfsm.ProduceReceivePreCommitEndorsementEvent(ew)
		}
	default:
		return errors.Errorf("Invalid consensus message type %s", msg.Type)
	}
	return nil
}

// ValidateBlockFooter validates the signatures in the block footer
func (r *RollDPoS) ValidateBlockFooter(blk *block.Block) error {
	epoch, err := r.ctx.epochCtxByHeight(blk.Height())
	if err != nil {
		return err
	}
	proposer, err := r.ctx.getProposer(blk.Height(), blk.Round(), epoch.delegates)
	if err != nil {
		return err
	}
	if proposer != blk.ProducerAddress() {
		return errors.Errorf(
			"block proposer %s is invalid, %s expected",
			blk.ProducerAddress(),
			proposer,
		)
	}
	if 3*blk.NumOfDelegateEndorsements(epoch.delegates) <= 2*len(epoch.delegates) {
		return errors.New("insufficient endorsements from delegates")
	}

	return nil
}

// Metrics returns RollDPoS consensus metrics
func (r *RollDPoS) Metrics() (scheme.ConsensusMetrics, error) {
	var metrics scheme.ConsensusMetrics
	height := r.ctx.chain.TipHeight()
	epoch, err := r.ctx.epochCtxByHeight(height + 1)
	if err != nil {
		return metrics, errors.Wrap(err, "error when calculating epoch")
	}
	// Compute block producer
	_, producer, err := r.ctx.calcProposer(height+1, epoch.delegates, time.Duration(0))
	if err != nil {
		return metrics, errors.Wrap(err, "error when calculating the block producer")
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
		LatestEpoch:         epoch.num,
		LatestHeight:        height,
		LatestDelegates:     epoch.delegates,
		LatestBlockProducer: producer,
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
	cfg config.RollDPoS
	// TODO: we should use keystore in the future
	encodedAddr            string
	pubKey                 keypair.PublicKey
	priKey                 keypair.PrivateKey
	chain                  blockchain.Blockchain
	actPool                actpool.ActPool
	broadcastHandler       scheme.Broadcast
	clock                  clock.Clock
	rootChainAPI           explorer.Explorer
	candidatesByHeightFunc CandidatesByHeightFunc
}

// NewRollDPoSBuilder instantiates a Builder instance
func NewRollDPoSBuilder() *Builder {
	return &Builder{}
}

// SetConfig sets RollDPoS config
func (b *Builder) SetConfig(cfg config.RollDPoS) *Builder {
	b.cfg = cfg
	return b
}

// SetAddr sets the address and key pair for signature
func (b *Builder) SetAddr(encodedAddr string) *Builder {
	b.encodedAddr = encodedAddr
	return b
}

// SetPubKey sets the public key
func (b *Builder) SetPubKey(pubKey keypair.PublicKey) *Builder {
	b.pubKey = pubKey
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

// SetCandidatesByHeightFunc sets candidatesByHeightFunc, which is only used by tests
func (b *Builder) SetCandidatesByHeightFunc(
	candidatesByHeightFunc CandidatesByHeightFunc,
) *Builder {
	b.candidatesByHeightFunc = candidatesByHeightFunc
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
	ctx := rollDPoSCtx{
		cfg:                    b.cfg,
		encodedAddr:            b.encodedAddr,
		pubKey:                 b.pubKey,
		priKey:                 b.priKey,
		chain:                  b.chain,
		actPool:                b.actPool,
		broadcastHandler:       b.broadcastHandler,
		clock:                  b.clock,
		rootChainAPI:           b.rootChainAPI,
		candidatesByHeightFunc: b.candidatesByHeightFunc,
	}
	cfsm, err := consensusfsm.NewConsensusFSM(b.cfg.FSM, &ctx, b.clock)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing the consensus FSM")
	}
	return &RollDPoS{
		cfsm: cfsm,
		ctx:  &ctx,
	}, nil
}
