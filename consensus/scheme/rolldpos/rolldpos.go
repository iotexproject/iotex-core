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
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
)

var (
	// ErrNewRollDPoS indicates the error of constructing RollDPoS
	ErrNewRollDPoS = errors.New("error when constructing RollDPoS")
	// ErrZeroDelegate indicates seeing 0 delegates in the network
	ErrZeroDelegate = errors.New("zero delegates in the network")
)

type rollDPoSCtx struct {
	cfg     config.RollDPoS
	addr    *iotxaddress.Address
	chain   blockchain.Blockchain
	actPool actpool.ActPool
	p2p     network.Overlay
	epoch   epochCtx
	round   roundCtx
	clock   clock.Clock
	// candidatesByHeightFunc is only used for testing purpose
	candidatesByHeightFunc func(uint64) ([]*state.Candidate, error)
	sync                   blocksync.BlockSync
}

var (
	// ErrNotEnoughCandidates indicates there are not enough candidates from the candidate pool
	ErrNotEnoughCandidates = errors.New("Candidate pool does not have enough candidates")
)

// rollingDelegates will only allows the delegates chosen for given epoch to enter the epoch
func (ctx *rollDPoSCtx) rollingDelegates(epochNum uint64) ([]string, error) {
	numDlgs := ctx.cfg.NumDelegates
	height := uint64(numDlgs) * uint64(ctx.cfg.NumSubEpochs) * (epochNum - 1)
	var candidates []*state.Candidate
	var err error
	if ctx.candidatesByHeightFunc != nil {
		// Test only
		candidates, err = ctx.candidatesByHeightFunc(height)
	} else {
		candidates, err = ctx.chain.CandidatesByHeight(height)
	}
	if err != nil {
		return []string{}, errors.Wrap(err, "error when getting delegates from the candidate pool")
	}
	if len(candidates) < int(numDlgs) {
		return []string{}, errors.Wrapf(ErrNotEnoughCandidates, "only %d delegates from the candidate pool", len(candidates))
	}
	candidates = candidates[:numDlgs]

	var delegates []string
	for _, candidate := range candidates {
		delegates = append(delegates, candidate.Address)
	}

	return delegates, nil
}

// calcEpochNum calculates the epoch ordinal number and the epoch start height offset, which is based on the height of
// the next block to be produced
func (ctx *rollDPoSCtx) calcEpochNumAndHeight() (uint64, uint64, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return 0, 0, err
	}
	numDlgs := ctx.cfg.NumDelegates
	if err != nil {
		return 0, 0, err
	}
	subEpochNum := ctx.getNumSubEpochs()
	epochNum := height/(uint64(numDlgs)*uint64(subEpochNum)) + 1
	epochHeight := uint64(numDlgs)*uint64(subEpochNum)*(epochNum-1) + 1
	return epochNum, epochHeight, nil
}

// generateDKG generates a pseudo DKG bytes
func (ctx *rollDPoSCtx) generateDKG() (hash.DKGHash, error) {
	var dkg hash.DKGHash
	// TODO: fill the logic to generate DKG
	return dkg, nil
}

// getNumSubEpochs returns max(configured number, 1)
func (ctx *rollDPoSCtx) getNumSubEpochs() uint {
	num := uint(1)
	if ctx.cfg.NumSubEpochs > 0 {
		num = ctx.cfg.NumSubEpochs
	}
	return num
}

// rotatedProposer will rotate among the delegates to choose the proposer. It is pseudo order based on the position
// in the delegate list and the block height
func (ctx *rollDPoSCtx) rotatedProposer() (string, uint64, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return "", 0, err
	}
	// Next block height
	height++
	proposer, err := ctx.calcProposer(height, ctx.epoch.delegates)
	return proposer, height, err
}

// calcProposer calculates the proposer for the block at a given height
func (ctx *rollDPoSCtx) calcProposer(height uint64, delegates []string) (string, error) {
	numDelegates := len(delegates)
	if numDelegates == 0 {
		return "", ErrZeroDelegate
	}
	return delegates[(height)%uint64(numDelegates)], nil
}

// mintBlock picks the actions and creates an block to propose
func (ctx *rollDPoSCtx) mintBlock() (*blockchain.Block, error) {
	transfers, votes, executions := ctx.actPool.PickActs()
	logger.Debug().
		Int("transfer", len(transfers)).
		Int("votes", len(votes)).
		Msg("pick actions from the action pool")
	blk, err := ctx.chain.MintNewBlock(transfers, votes, executions, ctx.addr, "")
	if err != nil {
		logger.Error().Msg("error when minting a block")
		return nil, err
	}
	logger.Info().
		Uint64("height", blk.Height()).
		Int("transfers", len(blk.Transfers)).
		Int("votes", len(blk.Votes)).
		Msg("minted a new block")
	return blk, nil
}

// calcDurationSinceLastBlock returns the duration since last block time
func (ctx *rollDPoSCtx) calcDurationSinceLastBlock() (time.Duration, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return 0, errors.Wrap(err, "error when getting the blockchain height")
	}
	blk, err := ctx.chain.GetBlockByHeight(height)
	if err != nil {
		return 0, errors.Wrapf(err, "error when getting the block at height: %d", height)
	}
	return time.Since(blk.Header.Timestamp()), nil
}

// calcQuorum calculates if more than 2/3 vote yes or no
func (ctx *rollDPoSCtx) calcQuorum(decisions map[string]bool) (bool, bool) {
	yes := 0
	no := 0
	for _, decision := range decisions {
		if decision {
			yes++
		} else {
			no++
		}
	}
	numDelegates := len(ctx.epoch.delegates)
	return yes >= numDelegates*2/3+1, no >= numDelegates*2/3+1
}

// isEpochFinished checks the epoch is finished or not
func (ctx *rollDPoSCtx) isEpochFinished() (bool, error) {
	height, err := ctx.chain.TipHeight()
	if err != nil {
		return false, errors.Wrap(err, "error when getting the blockchain height")
	}
	// if the height of the last committed block is already the last one should be minted from this epochStart, go back
	// to epochStart start
	if height >= ctx.epoch.height+uint64(uint(len(ctx.epoch.delegates))*ctx.epoch.numSubEpochs)-1 {
		return true, nil
	}
	return false, nil
}

// epochCtx keeps the context data for the current epoch
type epochCtx struct {
	// num is the ordinal number of an epoch
	num uint64
	// height means offset for current epochStart (i.e., the height of the first block generated in this epochStart)
	height uint64
	// numSubEpochs defines number of sub-epochs/rotations will happen in an epochStart
	numSubEpochs uint
	dkg          hash.DKGHash
	delegates    []string
}

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	height    uint64
	timestamp time.Time
	block     *blockchain.Block
	prevotes  map[string]bool
	votes     map[string]bool
	proposer  string
}

// RollDPoS is Roll-DPoS consensus main entrance
type RollDPoS struct {
	cfsm *cFSM
	ctx  *rollDPoSCtx
}

// Start starts RollDPoS consensus
func (r *RollDPoS) Start(ctx context.Context) error {
	if err := r.cfsm.Start(ctx); err != nil {
		return errors.Wrap(err, "error when starting the consensus FSM")
	}
	r.cfsm.produce(r.cfsm.newCEvt(eRollDelegates), r.ctx.cfg.Delay)
	return nil
}

// Stop stops RollDPoS consensus
func (r *RollDPoS) Stop(ctx context.Context) error {
	return errors.Wrap(r.cfsm.Stop(ctx), "error when stopping the consensus FSM")
}

// Handle handles RollDPoS events coming from the network from other delegates
func (r *RollDPoS) Handle(msg proto.Message) error {
	cEvt, err := r.convertToConsensusEvt(msg)
	if err != nil {
		return errors.Wrap(err, "error when converting a proto msg to a consensus event")
	}
	r.cfsm.produce(cEvt, 0)
	return nil
}

// SetDoneStream does nothing for Noop (only used in simulator)
func (r *RollDPoS) SetDoneStream(simMsgReady chan bool) {}

// Metrics returns RollDPoS consensus metrics
func (r *RollDPoS) Metrics() (scheme.ConsensusMetrics, error) {
	var metrics scheme.ConsensusMetrics
	// Compute the epoch ordinal number
	epochNum, _, err := r.ctx.calcEpochNumAndHeight()
	if err != nil {
		return metrics, errors.Wrap(err, "error when calculating the epoch ordinal number")
	}
	// Compute delegates
	delegates, err := r.ctx.rollingDelegates(epochNum)
	if err != nil {
		return metrics, errors.Wrap(err, "error when getting the rolling delegates")
	}
	// Compute the height
	height, err := r.ctx.chain.TipHeight()
	if err != nil {
		return metrics, errors.Wrap(err, "error when getting the blockchain height")
	}
	// Compute block producer
	producer, err := r.ctx.calcProposer(height+1, delegates)
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
		LatestEpoch:         epochNum,
		LatestHeight:        height,
		LatestDelegates:     delegates,
		LatestBlockProducer: producer,
		Candidates:          candidateAddresses,
	}, nil
}

// NumPendingEvts returns the number of pending events
func (r *RollDPoS) NumPendingEvts() int {
	return len(r.cfsm.evtq)
}

// CurrentState returns the current state
func (r *RollDPoS) CurrentState() fsm.State {
	return r.cfsm.fsm.CurrentState()
}

func (r *RollDPoS) convertToConsensusEvt(msg proto.Message) (iConsensusEvt, error) {
	vcMsg, ok := msg.(*iproto.ViewChangeMsg)
	if !ok {
		return nil, errors.Wrap(ErrEvtCast, "error when casting a proto msg to a ViewChangeMsg")
	}
	var cEvt iConsensusEvt
	switch vcMsg.Vctype {
	case iproto.ViewChangeMsg_PROPOSE:
		pbEvt := r.cfsm.newProposeBlkEvt(nil)
		if err := pbEvt.fromProtoMsg(vcMsg); err != nil {
			return nil, errors.Wrap(err, "error when casting a proto msg to proposeBlkEvt")
		}
		cEvt = pbEvt
	case iproto.ViewChangeMsg_PREVOTE:
		var blkHash hash.Hash32B
		pvEvt := r.cfsm.newPrevoteEvt(blkHash, false)
		if err := pvEvt.fromProtoMsg(vcMsg); err != nil {
			return nil, errors.Wrap(err, "error when casting a proto msg to prevoteEvt")
		}
		cEvt = pvEvt
	case iproto.ViewChangeMsg_VOTE:
		var blkHash hash.Hash32B
		vEvt := r.cfsm.newVoteEvt(blkHash, false)
		if err := vEvt.fromProtoMsg(vcMsg); err != nil {
			return nil, errors.Wrap(err, "error when casting a proto msg to voteEvt")
		}
		cEvt = vEvt
	default:
		return nil, errors.Wrapf(ErrEvtCast, "unexpected ViewChangeMsg type %d", vcMsg.Vctype)
	}
	return cEvt, nil
}

// Builder is the builder for RollDPoS
type Builder struct {
	cfg config.RollDPoS
	// TODO: we should use keystore in the future
	addr                   *iotxaddress.Address
	chain                  blockchain.Blockchain
	actPool                actpool.ActPool
	p2p                    network.Overlay
	clock                  clock.Clock
	candidatesByHeightFunc func(uint64) ([]*state.Candidate, error)
	sync                   blocksync.BlockSync
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
func (b *Builder) SetAddr(addr *iotxaddress.Address) *Builder {
	b.addr = addr
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

// SetP2P sets the P2P APIs
func (b *Builder) SetP2P(p2p network.Overlay) *Builder {
	b.p2p = p2p
	return b
}

// SetClock sets the clock
func (b *Builder) SetClock(clock clock.Clock) *Builder {
	b.clock = clock
	return b
}

// SetCandidatesByHeightFunc sets candidatesByHeightFunc, which is only used by tests
func (b *Builder) SetCandidatesByHeightFunc(
	candidatesByHeightFunc func(uint64) ([]*state.Candidate, error),
) *Builder {
	b.candidatesByHeightFunc = candidatesByHeightFunc
	return b
}

// SetBlockSync sets block sync APIs
func (b *Builder) SetBlockSync(sync blocksync.BlockSync) *Builder {
	b.sync = sync
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
	if b.p2p == nil {
		return nil, errors.Wrap(ErrNewRollDPoS, "p2p APIs is nil")
	}
	if b.clock == nil {
		b.clock = clock.New()
	}
	ctx := rollDPoSCtx{
		cfg:     b.cfg,
		addr:    b.addr,
		chain:   b.chain,
		actPool: b.actPool,
		p2p:     b.p2p,
		clock:   b.clock,
		candidatesByHeightFunc: b.candidatesByHeightFunc,
		sync: b.sync,
	}
	cfsm, err := newConsensusFSM(&ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error when constructing the consensus FSM")
	}
	return &RollDPoS{
		cfsm: cfsm,
		ctx:  &ctx,
	}, nil
}
