// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

var (
	blockMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_block_metrics",

			Help: "Block metrics.",
		},
		[]string{"type"},
	)
	// ErrInvalidTipHeight is the error returned when the block height is not valid
	ErrInvalidTipHeight = errors.New("invalid tip height")
	// ErrInvalidBlock is the error returned when the block is not valid
	ErrInvalidBlock = errors.New("failed to validate the block")
	// ErrActionNonce is the error when the nonce of the action is wrong
	ErrActionNonce = errors.New("invalid action nonce")
	// ErrInsufficientGas indicates the error of insufficient gas value for data storage
	ErrInsufficientGas = errors.New("insufficient intrinsic gas value")
	// ErrBalance indicates the error of balance
	ErrBalance = errors.New("invalid balance")
)

func init() {
	prometheus.MustRegister(blockMtc)
}

type (
	// Blockchain represents the blockchain data structure and hosts the APIs to access it
	Blockchain interface {
		lifecycle.StartStopper

		// For exposing blockchain states
		// BlockHeaderByHeight return block header by height
		BlockHeaderByHeight(height uint64) (*block.Header, error)
		// BlockFooterByHeight return block footer by height
		BlockFooterByHeight(height uint64) (*block.Footer, error)
		// ChainID returns the chain ID
		ChainID() uint32
		// ChainAddress returns chain address on parent chain, the root chain return empty.
		ChainAddress() string
		// TipHash returns tip block's hash
		TipHash() hash.Hash256
		// TipHeight returns tip block's height
		TipHeight() uint64
		// Genesis returns the genesis
		Genesis() genesis.Genesis
		// Context returns current context
		Context() (context.Context, error)

		// For block operations
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(
			actionMap map[string][]action.SealedEnvelope,
			timestamp time.Time,
		) (*block.Block, error)
		// CommitBlock validates and appends a block to the chain
		CommitBlock(blk *block.Block) error
		// ValidateBlock validates a new block before adding it to the blockchain
		ValidateBlock(blk *block.Block) error

		// AddSubscriber make you listen to every single produced block
		AddSubscriber(BlockCreationSubscriber) error

		// RemoveSubscriber make you listen to every single produced block
		RemoveSubscriber(BlockCreationSubscriber) error
	}
	// BlockBuilderFactory is the factory interface of block builder
	BlockBuilderFactory interface {
		// NewBlockBuilder creates block builder
		NewBlockBuilder(context.Context, map[string][]action.SealedEnvelope, []action.SealedEnvelope) (*block.Builder, error)
	}
)

// ProductivityByEpoch returns the map of the number of blocks produced per delegate in an epoch
// TODO: move to poll protocol and implement reading current epoch meta from state factory -- now only reading current epoch productivity
func ProductivityByEpoch(ctx context.Context, bc Blockchain, epochNum uint64) (uint64, map[string]uint64, error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)

	var epochEndHeight uint64
	epochStartHeight := rp.GetEpochHeight(epochNum)
	currentEpochNum := rp.GetEpochNum(bcCtx.Tip.Height)
	if epochNum > currentEpochNum {
		return 0, nil, errors.Errorf("epoch number %d is larger than current epoch number %d", epochNum, currentEpochNum)
	}
	if epochNum == currentEpochNum {
		epochEndHeight = bcCtx.Tip.Height
	} else {
		// TODO: delete, won't happen
		epochEndHeight = rp.GetEpochLastBlockHeight(epochNum)
	}
	numBlks := epochEndHeight - epochStartHeight + 1

	activeConsensusBlockProducers, err := poll.MustGetProtocol(bcCtx.Registry).DelegatesByEpoch(ctx, epochNum)
	if err != nil {
		return 0, nil, status.Error(codes.NotFound, err.Error())
	}

	produce := make(map[string]uint64)
	for _, bp := range activeConsensusBlockProducers {
		produce[bp.Address] = 0
	}
	// TODO: because now this function is only getting current epoch data, (not history)
	// change to get from bc indexer before easter and after easter(backward compatiblility), read from state factory(cache layer)
	for i := uint64(0); i < numBlks; i++ {
		header, err := bc.BlockHeaderByHeight(epochStartHeight + i)
		if err != nil {
			return 0, nil, err
		}
		produce[header.ProducerAddress()]++
	}
	return numBlks, produce, nil
}

// blockchain implements the Blockchain interface
type blockchain struct {
	mu             sync.RWMutex // mutex to protect utk, tipHeight and tipHash
	dao            blockdao.BlockDAO
	config         config.Config
	blockValidator block.Validator
	lifecycle      lifecycle.Lifecycle
	clk            clock.Clock
	pubSubManager  PubSubManager
	timerFactory   *prometheustimer.TimerFactory

	// used by account-based model
	bbf      BlockBuilderFactory
	sf       factory.Factory
	registry *protocol.Registry
}

// ActPoolManager defines the actpool interface
type ActPoolManager interface {
	// GetActionByHash returns the pending action in pool given action's hash
	GetActionByHash(hash hash.Hash256) (action.SealedEnvelope, error)
}

// Option sets blockchain construction parameter
type Option func(*blockchain, config.Config) error

// BlockValidatorOption sets block validator
func BlockValidatorOption(blockValidator block.Validator) Option {
	return func(bc *blockchain, cfg config.Config) error {
		bc.blockValidator = blockValidator
		return nil
	}
}

// BoltDBDaoOption sets blockchain's dao with BoltDB from config.Chain.ChainDBPath
func BoltDBDaoOption() Option {
	return func(bc *blockchain, cfg config.Config) error {
		if bc.dao != nil {
			return nil
		}
		cfg.DB.DbPath = cfg.Chain.ChainDBPath // TODO: remove this after moving TrieDBPath from cfg.Chain to cfg.DB
		bc.dao = blockdao.NewBlockDAO(
			db.NewBoltDB(cfg.DB),
			nil,
			cfg.Chain.CompressBlock,
			cfg.DB,
		)
		return nil
	}
}

// InMemDaoOption sets blockchain's dao with MemKVStore
func InMemDaoOption() Option {
	return func(bc *blockchain, cfg config.Config) error {
		if bc.dao != nil {
			return nil
		}
		bc.dao = blockdao.NewBlockDAO(
			db.NewMemKVStore(),
			nil,
			cfg.Chain.CompressBlock,
			cfg.DB,
		)
		return nil
	}
}

// ClockOption overrides the default clock
func ClockOption(clk clock.Clock) Option {
	return func(bc *blockchain, conf config.Config) error {
		bc.clk = clk

		return nil
	}
}

// RegistryOption sets the blockchain with the protocol registry
func RegistryOption(registry *protocol.Registry) Option {
	return func(bc *blockchain, conf config.Config) error {
		bc.registry = registry
		return nil
	}
}

// NewBlockchain creates a new blockchain and DB instance
// TODO: replace sf with blockbuilderfactory
func NewBlockchain(cfg config.Config, dao blockdao.BlockDAO, sf factory.Factory, opts ...Option) Blockchain {
	bbf, ok := sf.(BlockBuilderFactory)
	if !ok {
		log.S().Panic("state factory didn't implement BlockBuilderFactory")
	}

	// create the Blockchain
	chain := &blockchain{
		config:        cfg,
		dao:           dao,
		bbf:           bbf,
		sf:            sf,
		clk:           clock.New(),
		pubSubManager: NewPubSub(cfg.BlockSync.BufferSize),
	}
	for _, opt := range opts {
		if err := opt(chain, cfg); err != nil {
			log.S().Panicf("Failed to execute blockchain creation option %p: %v", opt, err)
		}
	}
	timerFactory, err := prometheustimer.New(
		"iotex_blockchain_perf",
		"Performance of blockchain module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(cfg.Chain.ID), 10)},
	)
	if err != nil {
		log.L().Panic("Failed to generate prometheus timer factory.", zap.Error(err))
	}
	chain.timerFactory = timerFactory
	// Set block validator
	if err != nil {
		log.L().Panic("Failed to get block producer address.", zap.Error(err))
	}
	if chain.dao != nil {
		chain.lifecycle.Add(chain.dao)
	}
	if chain.sf != nil {
		chain.lifecycle.Add(chain.sf)
	}
	return chain
}

func (bc *blockchain) ChainID() uint32 {
	return atomic.LoadUint32(&bc.config.Chain.ID)
}

func (bc *blockchain) ChainAddress() string {
	return bc.config.Chain.Address
}

// Start starts the blockchain
func (bc *blockchain) Start(ctx context.Context) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// pass registry to be used by state factory's initialization
	ctx, err := bc.context(ctx, false, false)
	if err != nil {
		return err
	}
	if err := bc.lifecycle.OnStart(ctx); err != nil {
		return err
	}
	// get blockchain tip height
	tipHeight := bc.dao.GetTipHeight()
	if tipHeight == 0 {
		return nil
	}
	if bcCtx, ok := protocol.GetBlockchainCtx(ctx); ok {
		for _, p := range bcCtx.Registry.All() {
			if s, ok := p.(lifecycle.Starter); ok {
				if err := s.Start(ctx); err != nil {
					return errors.Wrap(err, "failed to start protocol")
				}
			}
		}
	}

	return bc.startExistingBlockchain(ctx)
}

// Stop stops the blockchain.
func (bc *blockchain) Stop(ctx context.Context) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.lifecycle.OnStop(ctx)
}

func (bc *blockchain) BlockHeaderByHeight(height uint64) (*block.Header, error) {
	return bc.dao.HeaderByHeight(height)
}

func (bc *blockchain) BlockFooterByHeight(height uint64) (*block.Footer, error) {
	return bc.dao.FooterByHeight(height)
}

// TipHash returns tip block's hash
func (bc *blockchain) TipHash() hash.Hash256 {
	tipHeight := bc.dao.GetTipHeight()
	tipHash, err := bc.dao.GetBlockHash(tipHeight)
	if err != nil {
		return hash.ZeroHash256
	}
	return tipHash
}

// TipHeight returns tip block's height
func (bc *blockchain) TipHeight() uint64 {
	return bc.dao.GetTipHeight()
}

// ValidateBlock validates a new block before adding it to the blockchain
func (bc *blockchain) ValidateBlock(blk *block.Block) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	timer := bc.timerFactory.NewTimer("ValidateBlock")
	defer timer.End()
	if blk == nil {
		return ErrInvalidBlock
	}
	tip, err := bc.tipInfo()
	if err != nil {
		return err
	}
	// verify new block has height incremented by 1
	if blk.Height() != 0 && blk.Height() != tip.Height+1 {
		return errors.Wrapf(
			ErrInvalidTipHeight,
			"wrong block height %d, expecting %d",
			blk.Height(),
			tip.Height+1,
		)
	}
	// verify new block has correctly linked to current tip
	if blk.PrevHash() != tip.Hash {
		blk.HeaderLogger(log.L()).Error("Previous block hash doesn't match.",
			log.Hex("expectedBlockHash", tip.Hash[:]))
		return errors.Wrapf(
			ErrInvalidBlock,
			"wrong prev hash %x, expecting %x",
			blk.PrevHash(),
			tip.Hash,
		)
	}
	if err := block.VerifyBlock(blk); err != nil {
		return errors.Wrap(err, "failed to verify block's signature and merkle root")
	}

	producerAddr, err := address.FromBytes(blk.PublicKey().Hash())
	if err != nil {
		return err
	}
	ctx, err := bc.context(context.Background(), true, true)
	if err != nil {
		return err
	}
	ctx = protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight:    blk.Height(),
			BlockTimeStamp: blk.Timestamp(),
			GasLimit:       bc.config.Genesis.BlockGasLimit,
			Producer:       producerAddr,
		},
	)
	if bc.blockValidator == nil {
		return nil
	}

	return bc.blockValidator.Validate(ctx, blk)
}

func (bc *blockchain) Context() (context.Context, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.context(context.Background(), true, true)
}

func (bc *blockchain) contextWithBlock(ctx context.Context, producer address.Address, height uint64, timestamp time.Time) context.Context {
	return protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    height,
			BlockTimeStamp: timestamp,
			Producer:       producer,
			GasLimit:       bc.config.Genesis.BlockGasLimit,
		})
}

func (bc *blockchain) context(ctx context.Context, tipInfoFlag, candidateFlag bool) (context.Context, error) {
	var candidates state.CandidateList
	var tip protocol.TipInfo
	var err error
	if candidateFlag {
		tipHeight := bc.dao.GetTipHeight()
		if candidates, err = bc.candidatesByHeight(tipHeight + 1); err != nil {
			return nil, err
		}
	}
	if tipInfoFlag {
		if tipInfoValue, err := bc.tipInfo(); err == nil {
			tip = *tipInfoValue
		} else {
			return nil, err
		}
	}
	return protocol.WithBlockchainCtx(
		ctx,
		protocol.BlockchainCtx{
			Registry:   bc.registry,
			Genesis:    bc.config.Genesis,
			Tip:        tip,
			Candidates: candidates,
		}), nil
}

func (bc *blockchain) MintNewBlock(
	actionMap map[string][]action.SealedEnvelope,
	timestamp time.Time,
) (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	mintNewBlockTimer := bc.timerFactory.NewTimer("MintNewBlock")
	defer mintNewBlockTimer.End()
	tipHeight := bc.dao.GetTipHeight()
	newblockHeight := tipHeight + 1
	ctx, err := bc.context(context.Background(), true, true)
	if err != nil {
		return nil, err
	}
	ctx = bc.contextWithBlock(ctx, bc.config.ProducerAddress(), newblockHeight, timestamp)
	// run execution and update state trie root hash
	minterPrivateKey := bc.config.ProducerPrivateKey()
	postSystemActions := make([]action.SealedEnvelope, 0)
	for _, p := range bc.registry.All() {
		if psac, ok := p.(protocol.PostSystemActionsCreator); ok {
			elps, err := psac.CreatePostSystemActions(ctx)
			if err != nil {
				return nil, err
			}
			for _, elp := range elps {
				se, err := action.Sign(elp, minterPrivateKey)
				if err != nil {
					return nil, err
				}
				postSystemActions = append(postSystemActions, se)
			}
		}
	}
	blockBuilder, err := bc.bbf.NewBlockBuilder(ctx, actionMap, postSystemActions)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block builder at new block height %d", newblockHeight)
	}
	blk, err := blockBuilder.SignAndBuild(minterPrivateKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block")
	}

	return &blk, nil
}

//  CommitBlock validates and appends a block to the chain
func (bc *blockchain) CommitBlock(blk *block.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	timer := bc.timerFactory.NewTimer("CommitBlock")
	defer timer.End()
	ctx, err := bc.context(context.Background(), true, true)
	if err != nil {
		return err
	}
	return bc.commitBlock(ctx, blk)
}

func (bc *blockchain) AddSubscriber(s BlockCreationSubscriber) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	log.L().Info("Add a subscriber.")
	if s == nil {
		return errors.New("subscriber could not be nil")
	}

	return bc.pubSubManager.AddBlockListener(s)
}

func (bc *blockchain) RemoveSubscriber(s BlockCreationSubscriber) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.pubSubManager.RemoveBlockListener(s)
}

//======================================
// internal functions
//=====================================

func (bc *blockchain) Genesis() genesis.Genesis {
	return bc.config.Genesis
}

//======================================
// private functions
//=====================================

func (bc *blockchain) candidatesByHeight(height uint64) (state.CandidateList, error) {
	if bc.registry == nil {
		return nil, nil
	}
	ctx := protocol.WithBlockchainCtx(
		context.Background(),
		protocol.BlockchainCtx{
			Registry: bc.registry,
			Genesis:  bc.config.Genesis,
		})

	if pp := poll.FindProtocol(bc.registry); pp != nil {
		return pp.CandidatesByHeight(ctx, height)
	}
	return nil, nil
}

func (bc *blockchain) startExistingBlockchain(ctx context.Context) error {
	if bc.sf == nil {
		return errors.New("statefactory cannot be nil")
	}

	stateHeight, err := bc.sf.Height()
	if err != nil {
		return err
	}
	tipHeight := bc.dao.GetTipHeight()
	if stateHeight > tipHeight {
		return errors.New("factory is higher than blockchain")
	}

	for i := stateHeight + 1; i <= tipHeight; i++ {
		blk, err := bc.dao.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		candidates, err := bc.candidatesByHeight(i)
		if err != nil {
			return err
		}
		ctx = protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				Registry: bc.registry,
				Genesis:  bc.config.Genesis,
				Tip: protocol.TipInfo{
					Height: i - 1,
				},
				Candidates: candidates,
			},
		)
		producer, err := address.FromBytes(blk.PublicKey().Hash())
		if err != nil {
			return err
		}
		ctx = bc.contextWithBlock(ctx, producer, blk.Height(), blk.Timestamp())
		if err := bc.sf.Commit(ctx, blk); err != nil {
			return err
		}
	}
	stateHeight, err = bc.sf.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get factory's height")
	}
	log.L().Info("Restarting blockchain.",
		zap.Uint64("chainHeight", tipHeight),
		zap.Uint64("factoryHeight", stateHeight))
	return nil
}

func (bc *blockchain) tipInfo() (*protocol.TipInfo, error) {
	tipHeight := bc.dao.GetTipHeight()
	if tipHeight == 0 {
		return &protocol.TipInfo{
			Height:    0,
			Hash:      bc.config.Genesis.Hash(),
			Timestamp: time.Unix(bc.config.Genesis.Timestamp, 0),
		}, nil
	}
	tipHash, err := bc.dao.GetBlockHash(tipHeight)
	if err != nil {
		return nil, err
	}
	header, err := bc.dao.Header(tipHash)
	if err != nil {
		return nil, err
	}

	return &protocol.TipInfo{
		Height:    tipHeight,
		Hash:      tipHash,
		Timestamp: header.Timestamp(),
	}, nil
}

// commitBlock commits a block to the chain
func (bc *blockchain) commitBlock(ctx context.Context, blk *block.Block) error {
	// early exit if block already exists
	blkHash, err := bc.dao.GetBlockHash(blk.Height())
	if err == nil && blkHash != hash.ZeroHash256 {
		log.L().Debug("Block already exists.", zap.Uint64("height", blk.Height()))
		return nil
	}
	// early exit if it's a db io error
	if err != nil && errors.Cause(err) != db.ErrNotExist && errors.Cause(err) != db.ErrBucketNotExist {
		return err
	}
	// write block into DB
	putTimer := bc.timerFactory.NewTimer("putBlock")
	err = bc.dao.PutBlock(blk)
	putTimer.End()
	if err != nil {
		return err
	}

	// commit state/contract changes
	sfTimer := bc.timerFactory.NewTimer("sf.Commit")
	err = bc.sf.Commit(ctx, blk)
	sfTimer.End()
	// detach working set so it can be freed by GC
	if err != nil {
		log.L().Panic("Error when committing states.", zap.Error(err))
	}
	tipHeight := bc.dao.GetTipHeight()
	tipHash, err := bc.dao.GetBlockHash(tipHeight)
	if err != nil {
		return err
	}
	blk.HeaderLogger(log.L()).Info("Committed a block.", log.Hex("tipHash", tipHash[:]))

	// emit block to all block subscribers
	bc.emitToSubscribers(blk)
	return nil
}

func (bc *blockchain) emitToSubscribers(blk *block.Block) {
	if bc.pubSubManager == nil {
		return
	}
	bc.pubSubManager.SendBlockToSubscribers(blk)
}
