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
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/zjshen14/go-fsm"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blocksync"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus/scheme"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/network"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/state"
)

var (
	timeSlotMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_consensus_time_slot",
			Help: "Consensus time slot",
		},
		[]string{},
	)
)

const sigSize = 5 // number of uint32s in BLS sig

func init() {
	prometheus.MustRegister(timeSlotMtc)
}

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

	var candidatesAddress []string
	for _, candidate := range candidates {
		candidatesAddress = append(candidatesAddress, candidate.Address)
	}
	crypto.SortCandidates(candidatesAddress, epochNum, ctx.epoch.seed)

	return candidatesAddress[:numDlgs], nil
}

// calcEpochNum calculates the epoch ordinal number and the epoch start height offset, which is based on the height of
// the next block to be produced
func (ctx *rollDPoSCtx) calcEpochNumAndHeight() (uint64, uint64, error) {
	height := ctx.chain.TipHeight()
	numDlgs := ctx.cfg.NumDelegates
	numSubEpochs := ctx.getNumSubEpochs()
	epochNum := height/(uint64(numDlgs)*uint64(numSubEpochs)) + 1
	epochHeight := uint64(numDlgs)*uint64(numSubEpochs)*(epochNum-1) + 1
	return epochNum, epochHeight, nil
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

// shouldHandleDKG indicates whether a node is in DKG stage
func (ctx *rollDPoSCtx) shouldHandleDKG() bool {
	if !ctx.cfg.EnableDKG {
		return false
	}
	return ctx.epoch.subEpochNum == 0
}

// generateDKGSecrets generates DKG secrets and witness
func (ctx *rollDPoSCtx) generateDKGSecrets() ([][]uint32, [][]byte, error) {
	idList := make([][]uint8, 0)
	for _, addr := range ctx.epoch.delegates {
		dkgID := iotxaddress.CreateID(addr)
		idList = append(idList, dkgID)
		if addr == ctx.addr.RawAddress {
			ctx.epoch.dkgAddress = iotxaddress.DKGAddress{ID: dkgID}
		}
	}
	_, secrets, witness, err := crypto.DKG.Init(crypto.DKG.SkGeneration(), idList)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate DKG Secrets and Witness")
	}
	return secrets, witness, nil
}

// TODO: numDlgs should also be configurable in BLS. For test purpose, let's make it 21.
// generateDKGKeyPair generates DKG key pair
func (ctx *rollDPoSCtx) generateDKGKeyPair() ([]byte, []uint32, error) {
	numDlgs := ctx.cfg.NumDelegates
	if numDlgs != 21 {
		return nil, nil, errors.New("Number of delegates must be 21 for test purpose")
	}
	shares := make([][]uint32, numDlgs)
	shareStatusMatrix := make([][21]bool, numDlgs)
	for i := range shares {
		shares[i] = make([]uint32, sigSize)
	}
	for i, delegate := range ctx.epoch.delegates {
		if secret, ok := ctx.epoch.committedSecrets[delegate]; ok {
			shares[i] = secret
			for j := 0; j < int(numDlgs); j++ {
				shareStatusMatrix[j][i] = true
			}
		}
	}
	_, dkgPubKey, dkgPriKey, err := crypto.DKG.KeyPairGeneration(shares, shareStatusMatrix)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate DKG key pair")
	}
	return dkgPubKey, dkgPriKey, nil
}

// getNumSubEpochs returns max(configured number, 1)
func (ctx *rollDPoSCtx) getNumSubEpochs() uint {
	num := uint(1)
	if ctx.cfg.NumSubEpochs > 0 {
		num = ctx.cfg.NumSubEpochs
	}
	if ctx.cfg.EnableDKG {
		num++
	}
	return num
}

// rotatedProposer will rotate among the delegates to choose the proposer. It is pseudo order based on the position
// in the delegate list and the block height
func (ctx *rollDPoSCtx) rotatedProposer() (string, uint64, error) {
	height := ctx.chain.TipHeight()
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
	if !ctx.cfg.TimeBasedRotation {
		return delegates[(height)%uint64(numDelegates)], nil
	}
	duration, err := ctx.calcDurationSinceLastBlock()
	if err != nil {
		return "", errors.Wrap(err, "error when computing the duration since last block time")
	}
	timeSlotIndex := int64(duration/ctx.cfg.ProposerInterval) - 1
	if timeSlotIndex < 0 {
		timeSlotIndex = 0
	}
	// TODO: should downgrade to debug level in the future
	logger.Info().Int64("slot", timeSlotIndex).Msg("calculate time slot offset")
	timeSlotMtc.WithLabelValues().Set(float64(timeSlotIndex))
	return delegates[(height+uint64(timeSlotIndex))%uint64(numDelegates)], nil
}

// mintBlock mints a new block to propose
func (ctx *rollDPoSCtx) mintBlock() (*blockchain.Block, error) {
	if ctx.shouldHandleDKG() {
		return ctx.mintSecretBlock()
	}
	return ctx.mintCommonBlock()
}

// mintSecretBlock collects DKG secret proposals and witness and creates a block to propose
func (ctx *rollDPoSCtx) mintSecretBlock() (*blockchain.Block, error) {
	secrets := ctx.epoch.secrets
	witness := ctx.epoch.witness
	if len(secrets) != len(ctx.epoch.delegates) {
		return nil, errors.New("Number of secrets does not match number of delegates")
	}
	confirmedNonce, err := ctx.chain.Nonce(ctx.addr.RawAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the confirmed nonce of secret block producer")
	}
	nonce := confirmedNonce + 1
	secretProposals := make([]*action.SecretProposal, 0)
	for i, delegate := range ctx.epoch.delegates {
		secretProposal, err := action.NewSecretProposal(nonce, ctx.addr.RawAddress, delegate, secrets[i])
		if err != nil {
			return nil, errors.Wrap(err, "failed to create the secret proposal")
		}
		secretProposals = append(secretProposals, secretProposal)
		nonce++
	}
	secretWitness, err := action.NewSecretWitness(nonce, ctx.addr.RawAddress, witness)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the secret witness")
	}
	blk, err := ctx.chain.MintNewSecretBlock(secretProposals, secretWitness, ctx.addr)
	if err != nil {
		return nil, err
	}
	logger.Info().
		Uint64("height", blk.Height()).
		Int("secretProposals", len(blk.SecretProposals)).
		Msg("minted a new secret block")
	return blk, nil
}

// mintCommonBlock picks the actions and creates a common block to propose
func (ctx *rollDPoSCtx) mintCommonBlock() (*blockchain.Block, error) {
	transfers, votes, executions, actions := ctx.actPool.PickActs()
	logger.Debug().
		Int("transfer", len(transfers)).
		Int("votes", len(votes)).
		Msg("pick actions from the action pool")
	blk, err := ctx.chain.MintNewDKGBlock(transfers, votes, executions, actions, ctx.addr, &ctx.epoch.dkgAddress,
		ctx.epoch.seed, "")
	if err != nil {
		return nil, err
	}
	logger.Info().
		Uint64("height", blk.Height()).
		Int("transfers", len(blk.Transfers)).
		Int("votes", len(blk.Votes)).
		Int("executions", len(blk.Executions)).
		Msg("minted a new block")
	return blk, nil
}

// calcDurationSinceLastBlock returns the duration since last block time
func (ctx *rollDPoSCtx) calcDurationSinceLastBlock() (time.Duration, error) {
	height := ctx.chain.TipHeight()
	blk, err := ctx.chain.GetBlockByHeight(height)
	if err != nil {
		return 0, errors.Wrapf(err, "error when getting the block at height: %d", height)
	}
	return ctx.clock.Now().Sub(blk.Header.Timestamp()), nil
}

// calcQuorum calculates if more than 2/3 vote yes or no including self's vote
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
	return yes >= numDelegates*2/3+1, no >= numDelegates*1/3
}

// isEpochFinished checks the epoch is finished or not
func (ctx *rollDPoSCtx) isEpochFinished() (bool, error) {
	height := ctx.chain.TipHeight()
	// if the height of the last committed block is already the last one should be minted from this epochStart, go back
	// to epochStart start
	if height >= ctx.epoch.height+uint64(uint(len(ctx.epoch.delegates))*ctx.epoch.numSubEpochs)-1 {
		return true, nil
	}
	return false, nil
}

// isDKGFinished checks the DKG sub-epoch is finished or not
func (ctx *rollDPoSCtx) isDKGFinished() bool {
	height := ctx.chain.TipHeight()
	return height >= ctx.epoch.height+uint64(len(ctx.epoch.delegates))-1
}

// updateSeed returns the seed for the next epoch
func (ctx *rollDPoSCtx) updateSeed() ([]byte, error) {
	epochNum, epochHeight, err := ctx.calcEpochNumAndHeight()
	if err != nil {
		return hash.Hash256b(ctx.epoch.seed), errors.Wrap(err, "Failed to do decode seed")
	}
	if epochNum <= 1 {
		return crypto.CryptoSeed, nil
	}
	selectedID := make([][]uint8, 0)
	selectedSig := make([][]byte, 0)
	selectedPK := make([][]byte, 0)
	for h := uint64(ctx.cfg.NumDelegates)*uint64(ctx.cfg.NumSubEpochs)*(epochNum-2) + 1; h < epochHeight && len(selectedID) <= crypto.Degree; h++ {
		blk, err := ctx.chain.GetBlockByHeight(h)
		if err != nil {
			continue
		}
		if len(blk.Header.DKGID) > 0 && len(blk.Header.DKGPubkey) > 0 && len(blk.Header.DKGBlockSig) > 0 {
			selectedID = append(selectedID, blk.Header.DKGID)
			selectedSig = append(selectedSig, blk.Header.DKGBlockSig)
			selectedPK = append(selectedPK, blk.Header.DKGPubkey)
		}
	}

	if len(selectedID) <= crypto.Degree {
		return hash.Hash256b(ctx.epoch.seed), errors.New("DKG signature/pubic key is not enough to aggregate")
	}

	aggregateSig, err := crypto.BLS.SignAggregate(selectedID, selectedSig)
	if err != nil {
		return hash.Hash256b(ctx.epoch.seed), errors.Wrap(err, "Failed to generate aggregate signature to update Seed")
	}
	if err = crypto.BLS.VerifyAggregate(selectedID, selectedPK, ctx.epoch.seed, aggregateSig); err != nil {
		return hash.Hash256b(ctx.epoch.seed), errors.Wrap(err, "Failed to verify aggregate signature to update Seed")
	}
	return aggregateSig, nil
}

// epochCtx keeps the context data for the current epoch
type epochCtx struct {
	// num is the ordinal number of an epoch
	num uint64
	// height means offset for current epochStart (i.e., the height of the first block generated in this epochStart)
	height uint64
	// numSubEpochs defines number of sub-epochs/rotations will happen in an epochStart
	numSubEpochs uint
	// subEpochNum is the ordinal number of sub-epoch within the current epoch
	subEpochNum uint64
	// secrets are the dkg secrets sent from current node to other delegates
	secrets [][]uint32
	// witness is the dkg secret witness sent from current node to other delegates
	witness [][]byte
	// committedSecrets are the secret shares within the secret blocks committed by current node
	committedSecrets map[string][]uint32
	delegates        []string
	dkgAddress       iotxaddress.DKGAddress
	seed             []byte
}

// roundCtx keeps the context data for the current round and block.
type roundCtx struct {
	height           uint64
	timestamp        time.Time
	block            *blockchain.Block
	proposalEndorses map[hash.Hash32B]map[string]bool
	commitEndorses   map[hash.Hash32B]map[string]bool
	proposer         string
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

// HandleBlockPropose handles incoming block propose
func (r *RollDPoS) HandleBlockPropose(propose *iproto.ProposePb) error {
	pbEvt, err := r.cfsm.newProposeBlkEvtFromProposePb(propose)
	if err != nil {
		return errors.Wrap(err, "error when casting a proto msg to proposeBlkEvt")
	}
	r.cfsm.produce(pbEvt, 0)
	return nil
}

// HandleEndorse handles incoming endorse
func (r *RollDPoS) HandleEndorse(ePb *iproto.EndorsePb) error {
	eEvt, err := r.cfsm.newEndorseEvtWithEndorsePb(ePb)
	if err != nil {
		return errors.Wrap(err, "error when casting a proto msg to endorse")
	}
	r.cfsm.produce(eEvt, 0)
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
	height := r.ctx.chain.TipHeight()
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

	crypto.SortCandidates(candidateAddresses, epochNum, r.ctx.epoch.seed)

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
