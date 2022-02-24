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

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
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
		Context(context.Context) (context.Context, error)

		// For block operations
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(timestamp time.Time) (*block.Block, error)
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
		NewBlockBuilder(context.Context, func(action.Envelope) (action.SealedEnvelope, error)) (*block.Builder, error)
	}
)

// Productivity returns the map of the number of blocks produced per delegate in given epoch
func Productivity(bc Blockchain, startHeight uint64, endHeight uint64) (map[string]uint64, error) {
	stats := make(map[string]uint64)
	for i := startHeight; i <= endHeight; i++ {
		header, err := bc.BlockHeaderByHeight(i)
		if err != nil {
			return nil, err
		}
		producer := header.ProducerAddress()
		stats[producer]++
	}

	return stats, nil
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
	bbf BlockBuilderFactory
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
func BoltDBDaoOption(indexers ...blockdao.BlockIndexer) Option {
	return func(bc *blockchain, cfg config.Config) error {
		if bc.dao != nil {
			return nil
		}
		cfg.DB.DbPath = cfg.Chain.ChainDBPath // TODO: remove this after moving TrieDBPath from cfg.Chain to cfg.DB
		cfg.DB.CompressLegacy = cfg.Chain.CompressBlock
		bc.dao = blockdao.NewBlockDAO(indexers, cfg.DB)
		return nil
	}
}

// InMemDaoOption sets blockchain's dao with MemKVStore
func InMemDaoOption(indexers ...blockdao.BlockIndexer) Option {
	return func(bc *blockchain, cfg config.Config) error {
		if bc.dao != nil {
			return nil
		}
		bc.dao = blockdao.NewBlockDAOInMemForTest(indexers)
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

// NewBlockchain creates a new blockchain and DB instance
func NewBlockchain(cfg config.Config, dao blockdao.BlockDAO, bbf BlockBuilderFactory, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchain{
		config:        cfg,
		dao:           dao,
		bbf:           bbf,
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
	if chain.dao == nil {
		log.L().Panic("blockdao is nil")
	}
	chain.lifecycle.Add(chain.dao)
	chain.lifecycle.Add(chain.pubSubManager)

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
	ctx, err := bc.context(ctx, false)
	if err != nil {
		return err
	}
	return bc.lifecycle.OnStart(ctx)
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
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return hash.ZeroHash256
	}
	tipHash, err := bc.dao.GetBlockHash(tipHeight)
	if err != nil {
		return hash.ZeroHash256
	}
	return tipHash
}

// TipHeight returns tip block's height
func (bc *blockchain) TipHeight() uint64 {
	tipHeight, err := bc.dao.Height()
	if err != nil {
		log.L().Panic("failed to get tip height", zap.Error(err))
	}
	return tipHeight
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

	if !blk.Header.VerifySignature() {
		return errors.Errorf("failed to verify block's signature with public key: %x", blk.PublicKey())
	}
	if err := blk.VerifyTxRoot(); err != nil {
		return err
	}

	producerAddr := blk.PublicKey().Address()
	if producerAddr == nil {
		return errors.New("failed to get address")
	}
	ctx, err := bc.context(context.Background(), true)
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
	ctx = protocol.WithFeatureCtx(ctx)
	if bc.blockValidator == nil {
		return nil
	}

	return bc.blockValidator.Validate(ctx, blk)
}

func (bc *blockchain) Context(ctx context.Context) (context.Context, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	return bc.context(ctx, true)
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

func (bc *blockchain) context(ctx context.Context, tipInfoFlag bool) (context.Context, error) {
	var tip protocol.TipInfo
	if tipInfoFlag {
		if tipInfoValue, err := bc.tipInfo(); err == nil {
			tip = *tipInfoValue
		} else {
			return nil, err
		}
	}

	ctx = genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				Tip:     tip,
				ChainID: bc.ChainID(),
			},
		),
		bc.config.Genesis,
	)
	return protocol.WithFeatureWithHeightCtx(ctx), nil
}

func (bc *blockchain) MintNewBlock(timestamp time.Time) (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	mintNewBlockTimer := bc.timerFactory.NewTimer("MintNewBlock")
	defer mintNewBlockTimer.End()
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return nil, err
	}
	newblockHeight := tipHeight + 1
	ctx, err := bc.context(context.Background(), true)
	if err != nil {
		return nil, err
	}
	ctx = bc.contextWithBlock(ctx, bc.config.ProducerAddress(), newblockHeight, timestamp)
	ctx = protocol.WithFeatureCtx(ctx)
	// run execution and update state trie root hash
	minterPrivateKey := bc.config.ProducerPrivateKey()
	blockBuilder, err := bc.bbf.NewBlockBuilder(
		ctx,
		func(elp action.Envelope) (action.SealedEnvelope, error) {
			return action.Sign(elp, minterPrivateKey)
		},
	)
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
	return bc.commitBlock(blk)
}

func (bc *blockchain) AddSubscriber(s BlockCreationSubscriber) error {
	log.L().Info("Add a subscriber.")
	if s == nil {
		return errors.New("subscriber could not be nil")
	}

	return bc.pubSubManager.AddBlockListener(s)
}

func (bc *blockchain) RemoveSubscriber(s BlockCreationSubscriber) error {
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

func (bc *blockchain) tipInfo() (*protocol.TipInfo, error) {
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return nil, err
	}
	if tipHeight == 0 {
		return &protocol.TipInfo{
			Height:    0,
			Hash:      bc.config.Genesis.Hash(),
			Timestamp: time.Unix(bc.config.Genesis.Timestamp, 0),
		}, nil
	}
	header, err := bc.dao.HeaderByHeight(tipHeight)
	if err != nil {
		return nil, err
	}

	return &protocol.TipInfo{
		Height:    tipHeight,
		Hash:      header.HashBlock(),
		Timestamp: header.Timestamp(),
	}, nil
}

// commitBlock commits a block to the chain
func (bc *blockchain) commitBlock(blk *block.Block) error {
	ctx, err := bc.context(context.Background(), true)
	if err != nil {
		return err
	}

	// write block into DB
	putTimer := bc.timerFactory.NewTimer("putBlock")
	err = bc.dao.PutBlock(ctx, blk)
	putTimer.End()
	switch {
	case errors.Cause(err) == filedao.ErrAlreadyExist:
		return nil
	case err != nil:
		return err
	}
	blkHash := blk.HashBlock()
	if blk.Height()%100 == 0 {
		blk.HeaderLogger(log.L()).Info("Committed a block.", log.Hex("tipHash", blkHash[:]))
	}
	blockMtc.WithLabelValues("numActions").Set(float64(len(blk.Actions)))
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
