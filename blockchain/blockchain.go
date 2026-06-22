// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"math/big"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/facebookgo/clock"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
)

// const
const (
	SigP256k1  = "secp256k1"
	SigP256sm2 = "p256sm2"
)

var (
	_blockMtc = prometheus.NewGaugeVec(
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
	// ErrPaused indicates the error of blockchain is paused
	ErrPaused = errors.New("blockchain is paused")
)

func init() {
	prometheus.MustRegister(_blockMtc)
}

type (
	// MintOptions is the options to mint a new block
	MintOptions struct {
		ProducerPrivateKey crypto.PrivateKey
	}
	// MintOption sets the mint options
	MintOption func(*MintOptions)
	// Blockchain represents the blockchain data structure and hosts the APIs to access it
	Blockchain interface {
		lifecycle.StartStopper

		// For exposing blockchain states
		// BlockHeaderByHeight return block header by height
		BlockHeaderByHeight(height uint64) (*block.Header, error)
		// BlockHeader return block header by hash
		BlockHeader(hash hash.Hash256) (*block.Header, error)
		// BlockFooterByHeight return block footer by height
		BlockFooterByHeight(height uint64) (*block.Footer, error)
		// ChainID returns the chain ID
		ChainID() uint32
		// EvmNetworkID returns the evm network ID
		EvmNetworkID() uint32
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
		// ContextAtHeight returns context at given height
		ContextAtHeight(context.Context, uint64) (context.Context, error)

		// For block operations
		// MintNewBlock creates a new block with given actions
		// Note: the coinbase transfer will be added to the given transfers when minting a new block
		MintNewBlock(time.Time, ...MintOption) (*block.Block, error)
		// CommitBlock validates and appends a block to the chain
		CommitBlock(blk *block.Block) error
		// ValidateBlock validates a new block before adding it to the blockchain
		ValidateBlock(*block.Block, ...BlockValidationOption) error

		// AddSubscriber make you listen to every single produced block
		AddSubscriber(BlockCreationSubscriber) error

		// RemoveSubscriber make you listen to every single produced block
		RemoveSubscriber(BlockCreationSubscriber) error
		//  Pause pauses the blockchain
		Pause(bool)
	}

	// BlockMinter is the block minter interface
	BlockMinter interface {
		// Mint creates a new block
		Mint(context.Context, crypto.PrivateKey) (*block.Block, error)
	}

	// BLSProducerResolver maps a BLS producer pubkey carried on a
	// post-fork block header to the candidate's iotex addresses. It is
	// injected at construction time so the blockchain package can stay
	// free of any staking-protocol import.
	//
	// operator is what BlockCtx.Producer holds post-fork (consumers
	// that still need a stable iotex-shaped identity — slasher
	// productivity, poll/util.go, log lines — read this). reward is
	// what BlockCtx.FeeRecipient holds (EVM Coinbase, rewarding
	// attribution). Implementations must return a non-nil error if
	// the BLS pubkey has not been registered by any candidate.
	BLSProducerResolver interface {
		ResolveBLSProducer(ctx context.Context, blsPubKey []byte) (operator, reward address.Address, err error)
	}

	// blockchain implements the Blockchain interface
	blockchain struct {
		mu             sync.RWMutex // mutex to protect utk, tipHeight and tipHash
		dao            blockdao.BlockDAO
		config         Config
		genesis        genesis.Genesis
		blockValidator block.Validator
		lifecycle      lifecycle.Lifecycle
		clk            clock.Clock
		pubSubManager  PubSubManager
		timerFactory   *prometheustimer.TimerFactory
		// blsResolver is consulted to populate BlockCtx.Producer +
		// FeeRecipient for post-fork BLS-signed headers. May be nil
		// in tests that never feed a BLS-signed block in; in that
		// case the post-fork path is unreachable and the pre-fork
		// path (blk.PublicKey().Address()) handles every header.
		blsResolver BLSProducerResolver

		// used by account-based model
		bbf   BlockMinter
		pause bool
	}
)

// WithProducerPrivateKey sets the producer private key
func WithProducerPrivateKey(pk crypto.PrivateKey) MintOption {
	return func(options *MintOptions) {
		options.ProducerPrivateKey = pk
	}
}

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

// Option sets blockchain construction parameter
type Option func(*blockchain) error

// BLSProducerResolverOption supplies the resolver that maps a BLS
// producer pubkey on a post-fork header to (operator, reward) iotex
// addresses for BlockCtx assembly. Required once the
// UseBLSProducerIdentity hardfork activates; before that the
// pre-fork branch ignores it.
func BLSProducerResolverOption(r BLSProducerResolver) Option {
	return func(bc *blockchain) error {
		bc.blsResolver = r
		return nil
	}
}

// BlockValidatorOption sets block validator
func BlockValidatorOption(blockValidator block.Validator) Option {
	return func(bc *blockchain) error {
		bc.blockValidator = blockValidator
		return nil
	}
}

// ClockOption overrides the default clock
func ClockOption(clk clock.Clock) Option {
	return func(bc *blockchain) error {
		bc.clk = clk
		return nil
	}
}

type (
	BlockValidationCfg struct {
		skipSidecarValidation bool
	}

	BlockValidationOption func(*BlockValidationCfg)
)

func SkipSidecarValidationOption() BlockValidationOption {
	return func(opts *BlockValidationCfg) {
		opts.skipSidecarValidation = true
	}
}

// NewBlockchain creates a new blockchain and DB instance
func NewBlockchain(cfg Config, g genesis.Genesis, dao blockdao.BlockDAO, bbf BlockMinter, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchain{
		config:        cfg,
		genesis:       g,
		dao:           dao,
		bbf:           bbf,
		clk:           clock.New(),
		pubSubManager: NewPubSub(cfg.StreamingBlockBufferSize),
	}
	for _, opt := range opts {
		if err := opt(chain); err != nil {
			log.S().Panicf("Failed to execute blockchain creation option %p: %v", opt, err)
		}
	}
	timerFactory, err := prometheustimer.New(
		"iotex_blockchain_perf",
		"Performance of blockchain module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(cfg.ID), 10)},
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
	return atomic.LoadUint32(&bc.config.ID)
}

func (bc *blockchain) EvmNetworkID() uint32 {
	return atomic.LoadUint32(&bc.config.EVMNetworkID)
}

func (bc *blockchain) ChainAddress() string {
	return bc.config.Address
}

// Start starts the blockchain
func (bc *blockchain) Start(ctx context.Context) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	// pass registry to be used by state factory's initialization
	ctx = protocol.WithFeatureWithHeightCtx(genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				ChainID:      bc.ChainID(),
				EvmNetworkID: bc.EvmNetworkID(),
				GetBlockHash: bc.dao.GetBlockHash,
				GetBlockTime: bc.getBlockTime,
			},
		), bc.genesis))
	return bc.lifecycle.OnStart(ctx)
}

// Stop stops the blockchain.
func (bc *blockchain) Stop(ctx context.Context) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.lifecycle.OnStop(ctx)
}

func (bc *blockchain) Pause(pause bool) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.pause = pause
}

func (bc *blockchain) BlockHeaderByHeight(height uint64) (*block.Header, error) {
	return bc.dao.HeaderByHeight(height)
}

func (bc *blockchain) BlockHeader(hash hash.Hash256) (*block.Header, error) {
	return bc.dao.Header(hash)
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
func (bc *blockchain) ValidateBlock(blk *block.Block, opts ...BlockValidationOption) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	timer := bc.timerFactory.NewTimer("ValidateBlock")
	defer timer.End()
	if blk == nil {
		return ErrInvalidBlock
	}
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return err
	}
	tip, err := bc.tipInfo(tipHeight)
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
	// verify EIP1559 header (baseFee adjustment)
	if blk.Header.BaseFee() != nil {
		if err = protocol.VerifyEIP1559Header(bc.genesis.Blockchain, tip, &blk.Header); err != nil {
			return errors.Wrap(err, "failed to verify EIP1559 header (baseFee adjustment)")
		}
	}
	if !blk.Header.VerifySignature() {
		return errors.Errorf("failed to verify block's signature with public key: %x", blk.PublicKey())
	}
	if err := blk.VerifyTxRoot(); err != nil {
		return err
	}

	ctx, err := bc.context(context.Background(), tipHeight)
	if err != nil {
		return err
	}
	producerAddr, feeRecipient, err := bc.resolveBlockProducer(ctx, blk)
	if err != nil {
		return err
	}
	cfg := BlockValidationCfg{}
	for _, opt := range opts {
		opt(&cfg)
	}
	ctx = protocol.WithBlockCtx(ctx,
		protocol.BlockCtx{
			BlockHeight:           blk.Height(),
			BlockTimeStamp:        blk.Timestamp(),
			GasLimit:              bc.genesis.BlockGasLimitByHeight(blk.Height()),
			Producer:              producerAddr,
			FeeRecipient:          feeRecipient,
			ProducerPubKey:        blk.Header.ProducerPubKey(),
			BaseFee:               blk.BaseFee(),
			ExcessBlobGas:         blk.ExcessBlobGas(),
			SkipSidecarValidation: cfg.skipSidecarValidation,
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
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return nil, err
	}
	return bc.context(ctx, tipHeight)
}

func (bc *blockchain) ContextAtHeight(ctx context.Context, height uint64) (context.Context, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.context(ctx, height)
}

func (bc *blockchain) contextWithBlock(ctx context.Context, producer address.Address, producerPubKey []byte, height uint64, timestamp time.Time, baseFee *big.Int, blobgas uint64) context.Context {
	return bc.contextWithBlockAndRecipient(ctx, producer, producer, producerPubKey, height, timestamp, baseFee, blobgas)
}

// contextWithBlockAndRecipient is the full-arity variant that lets
// the caller distinguish Producer (iotex-shaped identity used by
// slasher / display) from FeeRecipient (EVM Coinbase / reward
// attribution). Pre-fork they are the same; post-fork they split
// per the Eth2 fee_recipient model.
func (bc *blockchain) contextWithBlockAndRecipient(ctx context.Context, producer, feeRecipient address.Address, producerPubKey []byte, height uint64, timestamp time.Time, baseFee *big.Int, blobgas uint64) context.Context {
	return protocol.WithBlockCtx(
		ctx,
		protocol.BlockCtx{
			BlockHeight:    height,
			BlockTimeStamp: timestamp,
			Producer:       producer,
			FeeRecipient:   feeRecipient,
			ProducerPubKey: producerPubKey,
			GasLimit:       bc.genesis.BlockGasLimitByHeight(height),
			BaseFee:        baseFee,
			ExcessBlobGas:  blobgas,
		})
}

// resolveBlockProducer returns (producer, feeRecipient, producerPubKey)
// for a header-bearing block. Pre-fork the two iotex addresses are
// equal (== blk.PublicKey().Address()); post-fork they are looked up
// from candidate state via the injected BLSProducerResolver — the
// BLS pubkey on the header alone cannot derive an iotex address.
//
// Returns an error rather than panicking if a post-fork block carries
// a BLS pubkey the resolver cannot match to a candidate, or if no
// resolver was wired up at all. blockchain callers should treat that
// as a block-validation failure (invalid producer identity) instead
// of installing a half-populated BlockCtx.
func (bc *blockchain) resolveBlockProducer(ctx context.Context, blk *block.Block) (producer, feeRecipient address.Address, err error) {
	pubKey := blk.Header.ProducerPubKey()
	if g := genesis.MustExtractGenesisContext(ctx); g.IsToBeEnabled(blk.Height()) && len(pubKey) == crypto.BLSPubkeyLength {
		if bc.blsResolver == nil {
			return nil, nil, errors.New("BLS producer resolver not configured but block uses BLS-signed header")
		}
		op, rew, err := bc.blsResolver.ResolveBLSProducer(ctx, pubKey)
		if err != nil {
			return nil, nil, errors.Wrap(err, "failed to resolve BLS producer identity")
		}
		return op, rew, nil
	}
	addr := blk.PublicKey().Address()
	if addr == nil {
		return nil, nil, errors.New("failed to derive producer address from header public key")
	}
	return addr, addr, nil
}

func (bc *blockchain) context(ctx context.Context, height uint64) (context.Context, error) {
	tip, err := bc.tipInfo(height)
	if err != nil {
		return nil, err
	}
	ctx = genesis.WithGenesisContext(
		protocol.WithBlockchainCtx(
			ctx,
			protocol.BlockchainCtx{
				Tip:          *tip,
				ChainID:      bc.ChainID(),
				EvmNetworkID: bc.EvmNetworkID(),
				GetBlockHash: bc.dao.GetBlockHash,
				GetBlockTime: bc.getBlockTime,
			},
		),
		bc.genesis,
	)
	return protocol.WithFeatureWithHeightCtx(ctx), nil
}

func (bc *blockchain) MintNewBlock(timestamp time.Time, opts ...MintOption) (*block.Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	mintNewBlockTimer := bc.timerFactory.NewTimer("MintNewBlock")
	defer mintNewBlockTimer.End()
	var options MintOptions
	for _, opt := range opts {
		opt(&options)
	}
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return nil, err
	}
	newblockHeight := tipHeight + 1
	ctx, err := bc.context(context.Background(), tipHeight)
	if err != nil {
		return nil, err
	}
	tip := protocol.MustGetBlockchainCtx(ctx).Tip
	producerPrivateKey := options.ProducerPrivateKey
	if producerPrivateKey == nil {
		privateKeys := bc.config.ProducerPrivateKeys()
		if len(privateKeys) == 0 {
			return nil, errors.New("no producer private key available")
		}
		producerPrivateKey = privateKeys[0]
	}
	minterAddress := producerPrivateKey.PublicKey().Address()
	log.L().Info("Minting a new block.", zap.Uint64("height", newblockHeight), zap.String("minter", minterAddress.String()))
	// TODO(Y5/Y6 — node identity wiring): once the node's own BLS
	// pubkey is sourced from config, populate BlockCtx.ProducerPubKey
	// with that BLS pubkey post-fork (rather than the secp256k1 bytes
	// below) and resolve FeeRecipient from the candidate's Reward
	// address. Today the mint path is consistent with the pre-fork
	// behaviour: this is correct pre-fork; post-fork it produces a
	// BlockCtx that disagrees with the validator's view, so any node
	// that mints under UseBLSProducerIdentity will compute a divergent
	// state root. Mint-time wiring is intentionally deferred to Y5/Y6.
	ctx = bc.contextWithBlock(ctx, minterAddress, producerPrivateKey.PublicKey().Bytes(), newblockHeight, timestamp, protocol.CalcBaseFee(genesis.MustExtractGenesisContext(ctx).Blockchain, &tip), protocol.CalcExcessBlobGas(tip.ExcessBlobGas, tip.BlobGasUsed))
	ctx = protocol.WithFeatureCtx(ctx)
	// run execution and update state trie root hash
	blk, err := bc.bbf.Mint(ctx, producerPrivateKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block")
	}
	_blockMtc.WithLabelValues("MintGas").Set(float64(blk.GasUsed()))
	_blockMtc.WithLabelValues("MintActions").Set(float64(len(blk.Actions)))
	return blk, nil
}

// CommitBlock validates and appends a block to the chain
func (bc *blockchain) CommitBlock(blk *block.Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.pause {
		return errors.Wrapf(ErrPaused, "blockchain is paused, cannot commit block %d, %x", blk.Height(), blk.HashBlock())
	}
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
	return bc.genesis
}

//======================================
// private functions
//=====================================

func (bc *blockchain) tipInfo(tipHeight uint64) (*protocol.TipInfo, error) {
	if tipHeight == 0 {
		return &protocol.TipInfo{
			Height:    0,
			Hash:      bc.genesis.Hash(),
			Timestamp: time.Unix(bc.genesis.Timestamp, 0),
		}, nil
	}
	header, err := bc.dao.HeaderByHeight(tipHeight)
	if err != nil {
		return nil, err
	}

	return &protocol.TipInfo{
		Height:        tipHeight,
		GasUsed:       header.GasUsed(),
		Hash:          header.HashBlock(),
		Timestamp:     header.Timestamp(),
		BaseFee:       header.BaseFee(),
		BlobGasUsed:   header.BlobGasUsed(),
		ExcessBlobGas: header.ExcessBlobGas(),
	}, nil
}

// commitBlock commits a block to the chain
func (bc *blockchain) commitBlock(blk *block.Block) error {
	tipHeight, err := bc.dao.Height()
	if err != nil {
		return err
	}
	ctx, err := bc.context(context.Background(), tipHeight)
	if err != nil {
		return err
	}
	producerAddr, feeRecipient, err := bc.resolveBlockProducer(ctx, blk)
	if err != nil {
		return err
	}
	ctx = bc.contextWithBlockAndRecipient(ctx, producerAddr, feeRecipient, blk.Header.ProducerPubKey(), blk.Height(), blk.Timestamp(), blk.BaseFee(), blk.ExcessBlobGas())
	ctx = protocol.WithFeatureCtx(ctx)
	// write block into DB
	putTimer := bc.timerFactory.NewTimer("putBlock")
	err = bc.dao.PutBlock(ctx, blk)
	putTimer.End()
	switch errors.Cause(err) {
	case filedao.ErrAlreadyExist, blockdao.ErrAlreadyExist:
		return nil
	case nil:
		// do nothing
	default:
		return err
	}
	blkHash := blk.HashBlock()
	if blk.Height()%100 == 0 {
		blk.HeaderLogger(log.L()).Info("Committed a block.", log.Hex("tipHash", blkHash[:]))
	}
	_blockMtc.WithLabelValues("numActions").Set(float64(len(blk.Actions)))
	if blk.BaseFee() != nil {
		basefeeQev := new(big.Int).Div(blk.BaseFee(), big.NewInt(unit.Qev))
		_blockMtc.WithLabelValues("baseFee").Set(float64(basefeeQev.Int64()))
	}
	_blockMtc.WithLabelValues("excessBlobGas").Set(float64(blk.ExcessBlobGas()))
	_blockMtc.WithLabelValues("blobGasUsed").Set(float64(blk.BlobGasUsed()))
	_blockMtc.WithLabelValues("gasUsed").Set(float64(blk.GasUsed()))
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

func (bc *blockchain) getBlockTime(height uint64) (time.Time, error) {
	if height == 0 {
		return time.Unix(bc.genesis.Timestamp, 0), nil
	}
	header, err := bc.dao.HeaderByHeight(height)
	if err != nil {
		return time.Time{}, err
	}
	return header.Timestamp(), nil
}
