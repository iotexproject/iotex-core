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
	"github.com/iotexproject/go-pkgs/bloom"
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
	"github.com/iotexproject/iotex-core/crypto"
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
)

func init() {
	prometheus.MustRegister(blockMtc)
}

// Blockchain represents the blockchain data structure and hosts the APIs to access it
type Blockchain interface {
	lifecycle.StartStopper

	// For exposing blockchain states

	// BlockHeaderByHeight return block header by height
	BlockHeaderByHeight(height uint64) (*block.Header, error)
	// BlockHeaderByHash return block header by hash
	BlockHeaderByHash(h hash.Hash256) (*block.Header, error)
	// BlockFooterByHeight return block footer by height
	BlockFooterByHeight(height uint64) (*block.Footer, error)
	// BlockFooterByHash return block footer by hash
	BlockFooterByHash(h hash.Hash256) (*block.Footer, error)
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

	// For action operations
	// Validator returns the current validator object
	Validator() Validator
	// SetValidator sets the current validator object
	SetValidator(val Validator)

	// AddSubscriber make you listen to every single produced block
	AddSubscriber(BlockCreationSubscriber) error

	// RemoveSubscriber make you listen to every single produced block
	RemoveSubscriber(BlockCreationSubscriber) error
}

// ProductivityByEpoch returns the map of the number of blocks produced per delegate in an epoch
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
	mu            sync.RWMutex // mutex to protect utk, tipHeight and tipHash
	dao           blockdao.BlockDAO
	config        config.Config
	tipHeight     uint64
	tipHash       hash.Hash256
	validator     Validator
	lifecycle     lifecycle.Lifecycle
	clk           clock.Clock
	blocklistener []BlockCreationSubscriber
	timerFactory  *prometheustimer.TimerFactory

	// used by account-based model
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
func NewBlockchain(cfg config.Config, dao blockdao.BlockDAO, sf factory.Factory, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchain{
		config: cfg,
		dao:    dao,
		sf:     sf,
		clk:    clock.New(),
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
	senderBlackList := make(map[string]bool)
	for _, bannedSender := range cfg.ActPool.BlackList {
		senderBlackList[bannedSender] = true
	}
	chain.validator = &validator{
		sf:              chain.sf,
		validatorAddr:   cfg.ProducerAddress().String(),
		senderBlackList: senderBlackList,
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
	ctx = bc.contextWithBlock(ctx, bc.config.ProducerAddress(), 0, time.Unix(bc.config.Genesis.Timestamp, 0))
	if err := bc.lifecycle.OnStart(ctx); err != nil {
		return err
	}
	// get blockchain tip height
	if bc.tipHeight, err = bc.dao.GetTipHeight(); err != nil {
		return err
	}
	if bc.tipHeight == 0 {
		return nil
	}
	// get blockchain tip hash
	if bc.tipHash, err = bc.dao.GetTipHash(); err != nil {
		return err
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
	return bc.blockHeaderByHeight(height)
}

func (bc *blockchain) BlockHeaderByHash(h hash.Hash256) (*block.Header, error) {
	return bc.dao.Header(h)
}

func (bc *blockchain) BlockFooterByHeight(height uint64) (*block.Footer, error) {
	return bc.blockFooterByHeight(height)
}

func (bc *blockchain) BlockFooterByHash(h hash.Hash256) (*block.Footer, error) {
	return bc.dao.Footer(h)
}

// TipHash returns tip block's hash
func (bc *blockchain) TipHash() hash.Hash256 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.tipHash
}

// TipHeight returns tip block's height
func (bc *blockchain) TipHeight() uint64 {
	return atomic.LoadUint64(&bc.tipHeight)
}

// ValidateBlock validates a new block before adding it to the blockchain
func (bc *blockchain) ValidateBlock(blk *block.Block) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	timer := bc.timerFactory.NewTimer("ValidateBlock")
	defer timer.End()
	ctx, err := bc.context(context.Background(), true, true)
	if err != nil {
		return err
	}
	return bc.validator.Validate(ctx, blk)
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
		if candidates, err = bc.candidatesByHeight(bc.tipHeight + 1); err != nil {
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

	newblockHeight := bc.tipHeight + 1
	// run execution and update state trie root hash
	ctx, err := bc.context(context.Background(), true, true)
	if err != nil {
		return nil, err
	}
	ctx = bc.contextWithBlock(ctx, bc.config.ProducerAddress(), newblockHeight, timestamp)
	sk := bc.config.ProducerPrivateKey()
	postSystemActions := make([]action.SealedEnvelope, 0)
	for _, p := range bc.registry.All() {
		if psac, ok := p.(protocol.PostSystemActionsCreator); ok {
			elps, err := psac.CreatePostSystemActions(ctx)
			if err != nil {
				return nil, err
			}
			for _, elp := range elps {
				se, err := action.Sign(elp, sk)
				if err != nil {
					return nil, err
				}
				postSystemActions = append(postSystemActions, se)
			}
		}
	}
	rc, actions, ws, err := bc.sf.PickAndRunActions(ctx, actionMap, postSystemActions)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to update state changes in new block %d", newblockHeight)
	}

	blockMtc.WithLabelValues("numActions").Set(float64(len(actions)))

	ra := block.NewRunnableActionsBuilder().
		AddActions(actions...).
		Build()

	prevBlkHash := bc.tipHash
	// The first block's previous block hash is pointing to the digest of genesis config. This is to guarantee all nodes
	// could verify that they start from the same genesis
	if newblockHeight == 1 {
		prevBlkHash = bc.config.Genesis.Hash()
	}
	digest, err := ws.Digest()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get digest")
	}
	blk, err := block.NewBuilder(ra).
		SetHeight(newblockHeight).
		SetTimestamp(timestamp).
		SetPrevBlockHash(prevBlkHash).
		SetDeltaStateDigest(digest).
		SetReceipts(rc).
		SetReceiptRoot(calculateReceiptRoot(rc)).
		SetLogsBloom(calculateLogsBloom(bc.config, newblockHeight, rc)).
		SignAndBuild(sk)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create block")
	}
	blk.WorkingSet = ws

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

// SetValidator sets the current validator object
func (bc *blockchain) SetValidator(val Validator) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.validator = val
}

// Validator gets the current validator object
func (bc *blockchain) Validator() Validator {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.validator
}

func (bc *blockchain) AddSubscriber(s BlockCreationSubscriber) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	log.L().Info("Add a subscriber.")
	if s == nil {
		return errors.New("subscriber could not be nil")
	}
	bc.blocklistener = append(bc.blocklistener, s)

	return nil
}

func (bc *blockchain) RemoveSubscriber(s BlockCreationSubscriber) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	for i, sub := range bc.blocklistener {
		if sub == s {
			bc.blocklistener = append(bc.blocklistener[:i], bc.blocklistener[i+1:]...)
			log.L().Info("Successfully unsubscribe block creation.")
			return nil
		}
	}
	return errors.New("cannot find subscription")
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

	if pp := poll.FindProtocol(bc.registry); pp != nil {
		return pp.CandidatesByHeight(height)
	}
	return nil, nil
}

func (bc *blockchain) blockHeaderByHeight(height uint64) (*block.Header, error) {
	hash, err := bc.dao.GetBlockHash(height)
	if err != nil {
		return nil, err
	}
	return bc.dao.Header(hash)
}

func (bc *blockchain) blockFooterByHeight(height uint64) (*block.Footer, error) {
	hash, err := bc.dao.GetBlockHash(height)
	if err != nil {
		return nil, err
	}
	return bc.dao.Footer(hash)
}

func (bc *blockchain) startExistingBlockchain(ctx context.Context) error {
	if bc.sf == nil {
		return errors.New("statefactory cannot be nil")
	}

	stateHeight, err := bc.sf.Height()
	if err != nil {
		return err
	}
	if stateHeight > bc.tipHeight {
		return errors.New("factory is higher than blockchain")
	}

	for i := stateHeight + 1; i <= bc.tipHeight; i++ {
		blk, err := bc.dao.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		producer, err := address.FromBytes(blk.PublicKey().Hash())
		if err != nil {
			return err
		}
		ctx = bc.contextWithBlock(ctx, producer, blk.Height(), blk.Timestamp())
		_, ws, err := bc.sf.RunActions(ctx, blk.RunnableActions().Actions())
		if err != nil {
			return err
		}
		if err := bc.sf.Commit(ws); err != nil {
			return err
		}
	}
	stateHeight, err = bc.sf.Height()
	if err != nil {
		return errors.Wrap(err, "failed to get factory's height")
	}
	log.L().Info("Restarting blockchain.",
		zap.Uint64("chainHeight",
			bc.tipHeight),
		zap.Uint64("factoryHeight", stateHeight))
	return nil
}

func (bc *blockchain) tipInfo() (*protocol.TipInfo, error) {
	if bc.tipHeight == 0 {
		return &protocol.TipInfo{
			Height:    0,
			Hash:      bc.config.Genesis.Hash(),
			Timestamp: time.Unix(bc.config.Genesis.Timestamp, 0),
		}, nil
	}
	header, err := bc.dao.Header(bc.tipHash)
	if err != nil {
		return nil, err
	}

	return &protocol.TipInfo{
		Height:    bc.tipHeight,
		Hash:      bc.tipHash,
		Timestamp: header.Timestamp(),
	}, nil
}

// commitBlock commits a block to the chain
func (bc *blockchain) commitBlock(blk *block.Block) error {
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
	if err = bc.dao.PutBlock(blk); err == nil {
		err = bc.dao.Commit()
	}
	putTimer.End()
	if err != nil {
		return err
	}

	// update tip hash and height
	atomic.StoreUint64(&bc.tipHeight, blk.Height())
	bc.tipHash = blk.HashBlock()

	// commit state/contract changes
	sfTimer := bc.timerFactory.NewTimer("sf.Commit")
	err = bc.sf.Commit(blk.WorkingSet)
	sfTimer.End()
	// detach working set so it can be freed by GC
	blk.WorkingSet = nil
	if err != nil {
		log.L().Panic("Error when committing states.", zap.Error(err))
	}
	blk.HeaderLogger(log.L()).Info("Committed a block.", log.Hex("tipHash", bc.tipHash[:]))

	// emit block to all block subscribers
	bc.emitToSubscribers(blk)
	return nil
}

func (bc *blockchain) emitToSubscribers(blk *block.Block) {
	if bc.blocklistener == nil {
		return
	}
	for _, s := range bc.blocklistener {
		go func(bcs BlockCreationSubscriber, b *block.Block) {
			if err := bcs.HandleBlock(b); err != nil {
				log.L().Error("Failed to handle new block.", zap.Error(err))
			}
		}(s, blk)
	}
}

func calculateReceiptRoot(receipts []*action.Receipt) hash.Hash256 {
	if len(receipts) == 0 {
		return hash.ZeroHash256
	}
	h := make([]hash.Hash256, 0, len(receipts))
	for _, receipt := range receipts {
		h = append(h, receipt.Hash())
	}
	res := crypto.NewMerkleTree(h).HashTree()
	return res
}

func calculateLogsBloom(cfg config.Config, height uint64, receipts []*action.Receipt) bloom.BloomFilter {
	if height < cfg.Genesis.AleutianBlockHeight {
		return nil
	}
	bloom, _ := bloom.NewBloomFilter(2048, 3)
	for _, receipt := range receipts {
		for _, log := range receipt.Logs {
			for _, topic := range log.Topics {
				bloom.Add(topic[:])
			}
		}
	}
	return bloom
}
