// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"math/big"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/action/protocol/vote"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
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
	errDelegatesNotExist = errors.New("delegates cannot be found")
)

func init() {
	prometheus.MustRegister(blockMtc)
}

// Blockchain represents the blockchain data structure and hosts the APIs to access it
type Blockchain interface {
	lifecycle.StartStopper

	// Balance returns balance of an account
	Balance(addr string) (*big.Int, error)
	// Nonce returns the nonce if the account exists
	Nonce(addr string) (uint64, error)
	// CreateState adds a new account with initial balance to the factory
	CreateState(addr string, init *big.Int) (*state.Account, error)
	// CandidatesByHeight returns the candidate list by a given height
	CandidatesByHeight(height uint64) ([]*state.Candidate, error)
	// ProductivityByEpoch returns the number of produced blocks per delegate in an epoch
	ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error)
	// For exposing blockchain states
	// GetHeightByHash returns Block's height by hash
	GetHeightByHash(h hash.Hash256) (uint64, error)
	// GetHashByHeight returns Block's hash by height
	GetHashByHeight(height uint64) (hash.Hash256, error)
	// GetBlockByHeight returns Block by height
	GetBlockByHeight(height uint64) (*block.Block, error)
	// GetBlockByHash returns Block by hash
	GetBlockByHash(h hash.Hash256) (*block.Block, error)
	// GetTotalActions returns the total number of actions
	GetTotalActions() (uint64, error)
	// GetReceiptByActionHash returns the receipt by action hash
	GetReceiptByActionHash(h hash.Hash256) (*action.Receipt, error)
	// GetActionsFromAddress returns actions from address
	GetActionsFromAddress(address string) ([]hash.Hash256, error)
	// GetActionsToAddress returns actions to address
	GetActionsToAddress(address string) ([]hash.Hash256, error)
	// GetActionCountByAddress returns action count by address
	GetActionCountByAddress(address string) (uint64, error)
	// GetActionByActionHash returns action by action hash
	GetActionByActionHash(h hash.Hash256) (action.SealedEnvelope, error)
	// GetBlockHashByActionHash returns Block hash by action hash
	GetBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error)
	// GetFactory returns the state factory
	GetFactory() factory.Factory
	// GetChainID returns the chain ID
	ChainID() uint32
	// ChainAddress returns chain address on parent chain, the root chain return empty.
	ChainAddress() string
	// TipHash returns tip block's hash
	TipHash() hash.Hash256
	// TipHeight returns tip block's height
	TipHeight() uint64
	// StateByAddr returns account of a given address
	StateByAddr(address string) (*state.Account, error)
	// RecoverChainAndState recovers the chain to target height and refresh state db if necessary
	RecoverChainAndState(targetHeight uint64) error
	// GenesisTimestamp returns the timestamp of genesis
	GenesisTimestamp() int64

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

	// For smart contract operations
	// ExecuteContractRead runs a read-only smart contract operation, this is done off the network since it does not
	// cause any state change
	ExecuteContractRead(caller address.Address, ex *action.Execution) (*action.Receipt, error)

	// AddSubscriber make you listen to every single produced block
	AddSubscriber(BlockCreationSubscriber) error

	// RemoveSubscriber make you listen to every single produced block
	RemoveSubscriber(BlockCreationSubscriber) error
}

// blockchain implements the Blockchain interface
type blockchain struct {
	mu            sync.RWMutex // mutex to protect utk, tipHeight and tipHash
	dao           *blockDAO
	config        config.Config
	tipHeight     uint64
	tipHash       hash.Hash256
	validator     Validator
	lifecycle     lifecycle.Lifecycle
	clk           clock.Clock
	blocklistener []BlockCreationSubscriber
	timerFactory  *prometheustimer.TimerFactory

	// used by account-based model
	sf factory.Factory

	registry *protocol.Registry
}

// Option sets blockchain construction parameter
type Option func(*blockchain, config.Config) error

// DefaultStateFactoryOption sets blockchain's sf from config
func DefaultStateFactoryOption() Option {
	return func(bc *blockchain, cfg config.Config) (err error) {
		if cfg.Chain.EnableTrielessStateDB {
			bc.sf, err = factory.NewStateDB(cfg, factory.DefaultStateDBOption())
		} else {
			bc.sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
		}
		if err != nil {
			return errors.Wrapf(err, "Failed to create state factory")
		}
		return nil
	}
}

// PrecreatedStateFactoryOption sets blockchain's state.Factory to sf
func PrecreatedStateFactoryOption(sf factory.Factory) Option {
	return func(bc *blockchain, conf config.Config) error {
		bc.sf = sf

		return nil
	}
}

// InMemStateFactoryOption sets blockchain's factory.Factory as in memory sf
func InMemStateFactoryOption() Option {
	return func(bc *blockchain, cfg config.Config) error {
		sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
		if err != nil {
			return errors.Wrapf(err, "Failed to create state factory")
		}
		bc.sf = sf

		return nil
	}
}

// PrecreatedDaoOption sets blockchain's dao
func PrecreatedDaoOption(dao *blockDAO) Option {
	return func(bc *blockchain, conf config.Config) error {
		bc.dao = dao

		return nil
	}
}

// BoltDBDaoOption sets blockchain's dao with BoltDB from config.Chain.ChainDBPath
func BoltDBDaoOption() Option {
	return func(bc *blockchain, cfg config.Config) error {
		cfg.DB.DbPath = cfg.Chain.ChainDBPath // TODO: remove this after moving TrieDBPath from cfg.Chain to cfg.DB
		_, gateway := cfg.Plugins[config.GatewayPlugin]
		bc.dao = newBlockDAO(
			db.NewOnDiskDB(cfg.DB),
			gateway && !cfg.Chain.EnableAsyncIndexWrite,
			cfg.Chain.CompressBlock,
			cfg.Chain.MaxCacheSize,
		)
		return nil
	}
}

// InMemDaoOption sets blockchain's dao with MemKVStore
func InMemDaoOption() Option {
	return func(bc *blockchain, cfg config.Config) error {
		_, gateway := cfg.Plugins[config.GatewayPlugin]
		bc.dao = newBlockDAO(
			db.NewMemKVStore(),
			gateway && !cfg.Chain.EnableAsyncIndexWrite,
			cfg.Chain.CompressBlock,
			cfg.Chain.MaxCacheSize,
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
func NewBlockchain(cfg config.Config, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchain{
		config: cfg,
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
	chain.validator = &validator{sf: chain.sf, validatorAddr: cfg.ProducerAddress().String()}

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
func (bc *blockchain) Start(ctx context.Context) (err error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if err = bc.lifecycle.OnStart(ctx); err != nil {
		return err
	}
	// get blockchain tip height
	if bc.tipHeight, err = bc.dao.getBlockchainHeight(); err != nil {
		return err
	}
	if bc.tipHeight == 0 {
		return bc.startEmptyBlockchain()
	}
	// get blockchain tip hash
	if bc.tipHash, err = bc.dao.getBlockHash(bc.tipHeight); err != nil {
		return err
	}
	return bc.startExistingBlockchain()
}

// Stop stops the blockchain.
func (bc *blockchain) Stop(ctx context.Context) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	return bc.lifecycle.OnStop(ctx)
}

// Balance returns balance of address
func (bc *blockchain) Balance(addr string) (*big.Int, error) {
	return bc.sf.Balance(addr)
}

// Nonce returns the nonce if the account exists
func (bc *blockchain) Nonce(addr string) (uint64, error) {
	return bc.sf.Nonce(addr)
}

// CandidatesByHeight returns the candidate list by a given height
func (bc *blockchain) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	return bc.candidatesByHeight(height)
}

// ProductivityByEpoch returns the map of the number of blocks produced per delegate in an epoch
func (bc *blockchain) ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error) {
	p, ok := bc.registry.Find(rolldpos.ProtocolID)
	if !ok {
		return 0, nil, errors.New("rolldpos protocol is not registered")
	}
	rp, ok := p.(*rolldpos.Protocol)
	if !ok {
		return 0, nil, errors.New("fail to cast rolldpos protocol")
	}

	var isCurrentEpoch bool
	currentEpochNum := rp.GetEpochNum(bc.tipHeight)
	if epochNum > currentEpochNum {
		return 0, nil, errors.New("epoch number is larger than current epoch number")
	}
	if epochNum == currentEpochNum {
		isCurrentEpoch = true
	}

	epochStartHeight := rp.GetEpochHeight(epochNum)
	var epochEndHeight uint64
	if isCurrentEpoch {
		epochEndHeight = bc.tipHeight
	} else {
		epochEndHeight = rp.GetEpochLastBlockHeight(epochNum)
	}
	numBlks := epochEndHeight - epochStartHeight + 1

	p, ok = bc.registry.Find(poll.ProtocolID)
	if !ok {
		return 0, nil, errors.New("poll protocol is not registered")
	}
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		BlockHeight: bc.tipHeight,
		Registry:    bc.registry,
	})
	ws, err := bc.sf.NewWorkingSet()
	if err != nil {
		return 0, nil, err
	}
	s, err := p.ReadState(ctx, ws, []byte("ActiveBlockProducersByHeight"),
		byteutil.Uint64ToBytes(epochStartHeight))
	if err != nil {
		return 0, nil, status.Error(codes.NotFound, err.Error())
	}
	var activeConsensusBlockProducers state.CandidateList
	if err := activeConsensusBlockProducers.Deserialize(s); err != nil {
		return 0, nil, err
	}

	produce := make(map[string]uint64)
	for _, bp := range activeConsensusBlockProducers {
		produce[bp.Address] = 0
	}
	for i := uint64(0); i < numBlks; i++ {
		blk, err := bc.getBlockByHeight(epochStartHeight + i)
		if err != nil {
			return 0, nil, err
		}
		produce[blk.ProducerAddress()]++
	}
	return numBlks, produce, nil
}

// GetHeightByHash returns block's height by hash
func (bc *blockchain) GetHeightByHash(h hash.Hash256) (uint64, error) {
	return bc.dao.getBlockHeight(h)
}

// GetHashByHeight returns block's hash by height
func (bc *blockchain) GetHashByHeight(height uint64) (hash.Hash256, error) {
	return bc.dao.getBlockHash(height)
}

// GetBlockByHeight returns block from the blockchain hash by height
func (bc *blockchain) GetBlockByHeight(height uint64) (*block.Block, error) {
	blk, err := bc.getBlockByHeight(height)
	if blk == nil || err != nil {
		return blk, err
	}
	blk.HeaderLogger(log.L()).Debug("Get block.")
	return blk, err
}

// GetBlockByHash returns block from the blockchain hash by hash
func (bc *blockchain) GetBlockByHash(h hash.Hash256) (*block.Block, error) {
	return bc.dao.getBlock(h)
}

// GetTotalActions returns the total number of actions
func (bc *blockchain) GetTotalActions() (uint64, error) {
	return bc.dao.getTotalActions()
}

// GetReceiptByActionHash returns the receipt by action hash
func (bc *blockchain) GetReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	return bc.dao.getReceiptByActionHash(h)
}

// GetActionsFromAddress returns actions from address
func (bc *blockchain) GetActionsFromAddress(addrStr string) ([]hash.Hash256, error) {
	addr, err := address.FromString(addrStr)
	if err != nil {
		return nil, err
	}
	return getActionsBySenderAddress(bc.dao.kvstore, hash.BytesToHash160(addr.Bytes()))
}

// GetActionToAddress returns action to address
func (bc *blockchain) GetActionsToAddress(addrStr string) ([]hash.Hash256, error) {
	addr, err := address.FromString(addrStr)
	if err != nil {
		return nil, err
	}
	return getActionsByRecipientAddress(bc.dao.kvstore, hash.BytesToHash160(addr.Bytes()))
}

// GetActionCountByAddress returns action count by address
func (bc *blockchain) GetActionCountByAddress(addrStr string) (uint64, error) {
	addr, err := address.FromString(addrStr)
	if err != nil {
		return 0, err
	}
	fromCount, err := getActionCountBySenderAddress(bc.dao.kvstore, hash.BytesToHash160(addr.Bytes()))
	if err != nil {
		return 0, err
	}
	toCount, err := getActionCountByRecipientAddress(bc.dao.kvstore, hash.BytesToHash160(addr.Bytes()))
	if err != nil {
		return 0, err
	}
	return fromCount + toCount, nil
}

func (bc *blockchain) getActionByActionHashHelper(h hash.Hash256) (hash.Hash256, error) {
	return getBlockHashByActionHash(bc.dao.kvstore, h)
}

// GetActionByActionHash returns action by action hash
func (bc *blockchain) GetActionByActionHash(h hash.Hash256) (action.SealedEnvelope, error) {
	blkHash, err := bc.getActionByActionHashHelper(h)
	if err != nil {
		return action.SealedEnvelope{}, err
	}

	blk, err := bc.dao.getBlock(blkHash)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	for _, act := range blk.Actions {
		if act.Hash() == h {
			return act, nil
		}
	}
	return action.SealedEnvelope{}, errors.Errorf("block %x does not have transfer %x", blkHash, h)
}

// GetBlockHashByActionHash returns Block hash by action hash
func (bc *blockchain) GetBlockHashByActionHash(h hash.Hash256) (hash.Hash256, error) {
	return getBlockHashByActionHash(bc.dao.kvstore, h)
}

// GetFactory returns the state factory
func (bc *blockchain) GetFactory() factory.Factory {
	return bc.sf
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
	return bc.validateBlock(blk)
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
	ws, err := bc.sf.NewWorkingSet()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to obtain working set from state factory")
	}

	gasLimitForContext := bc.config.Genesis.BlockGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			BlockHeight:    newblockHeight,
			BlockTimeStamp: timestamp,
			Producer:       bc.config.ProducerAddress(),
			GasLimit:       gasLimitForContext,
			ActionGasLimit: bc.config.Genesis.ActionGasLimit,
			Registry:       bc.registry,
		})
	_, rc, actions, err := bc.pickAndRunActions(ctx, actionMap, ws)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to update state changes in new block %d", newblockHeight)
	}

	blockMtc.WithLabelValues("numActions").Set(float64(len(actions)))

	sk := bc.config.ProducerPrivateKey()
	ra := block.NewRunnableActionsBuilder().
		SetHeight(newblockHeight).
		SetTimeStamp(timestamp).
		AddActions(actions...).
		Build(sk.PublicKey())

	prevBlkHash := bc.tipHash
	// The first block's previous block hash is pointing to the digest of genesis config. This is to guarantee all nodes
	// could verify that they start from the same genesis
	if newblockHeight == 1 {
		prevBlkHash = bc.config.Genesis.Hash()
	}
	blk, err := block.NewBuilder(ra).
		SetPrevBlockHash(prevBlkHash).
		SetDeltaStateDigest(ws.Digest()).
		SetReceipts(rc).
		SetReceiptRoot(calculateReceiptRoot(rc)).
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

// StateByAddr returns the account of an address
func (bc *blockchain) StateByAddr(address string) (*state.Account, error) {
	if bc.sf != nil {
		s, err := bc.sf.AccountState(address)
		if err != nil {
			log.L().Warn("Failed to get account.", zap.String("address", address), zap.Error(err))
			return nil, errors.New("account does not exist")
		}
		return s, nil
	}
	return nil, errors.New("state factory is nil")
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

// ExecuteContractRead runs a read-only smart contract operation, this is done off the network since it does not
// cause any state change
func (bc *blockchain) ExecuteContractRead(caller address.Address, ex *action.Execution) (*action.Receipt, error) {
	// use latest block as carrier to run the offline execution
	// the block itself is not used
	h := bc.TipHeight()
	blk, err := bc.GetBlockByHeight(h)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get block in ExecuteContractRead")
	}
	ws, err := bc.sf.NewWorkingSet()
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain working set from state factory")
	}
	producer, err := address.FromString(blk.ProducerAddress())
	if err != nil {
		return nil, err
	}
	gasLimit := bc.config.Genesis.BlockGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		BlockHeight:    blk.Height(),
		BlockTimeStamp: blk.Timestamp(),
		Producer:       producer,
		Caller:         caller,
		GasLimit:       gasLimit,
		ActionGasLimit: bc.config.Genesis.ActionGasLimit,
		GasPrice:       big.NewInt(0),
		IntrinsicGas:   0,
	})
	return evm.ExecuteContract(
		ctx,
		ws,
		ex,
		bc,
	)
}

// CreateState adds a new account with initial balance to the factory
func (bc *blockchain) CreateState(addr string, init *big.Int) (*state.Account, error) {
	if bc.sf == nil {
		return nil, errors.New("empty state factory")
	}
	ws, err := bc.sf.NewWorkingSet()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create clean working set")
	}
	account, err := accountutil.LoadOrCreateAccount(ws, addr, init)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new account %s", addr)
	}
	gasLimit := bc.config.Genesis.BlockGasLimit
	callerAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			GasLimit:       gasLimit,
			ActionGasLimit: bc.config.Genesis.ActionGasLimit,
			Caller:         callerAddr,
			ActionHash:     hash.ZeroHash256,
			Nonce:          0,
			Registry:       bc.registry,
		})
	if _, err = ws.RunActions(ctx, 0, nil); err != nil {
		return nil, errors.Wrap(err, "failed to run the account creation")
	}
	if err = bc.sf.Commit(ws); err != nil {
		return nil, errors.Wrap(err, "failed to commit the account creation")
	}
	return account, nil
}

// RecoverChainAndState recovers the chain to target height and refresh state db if necessary
func (bc *blockchain) RecoverChainAndState(targetHeight uint64) error {
	var buildStateFromScratch bool
	stateHeight, err := bc.sf.Height()
	if err != nil {
		buildStateFromScratch = true
	}
	if targetHeight > 0 {
		if err := bc.recoverToHeight(targetHeight); err != nil {
			return errors.Wrapf(err, "failed to recover blockchain to target height %d", targetHeight)
		}
		if stateHeight > bc.tipHeight {
			buildStateFromScratch = true
		}
	}

	if buildStateFromScratch {
		return bc.refreshStateDB()
	}
	return nil
}

func (bc *blockchain) GenesisTimestamp() int64 {
	return bc.config.Genesis.Timestamp
}

//======================================
// private functions
//=====================================

func (bc *blockchain) protocol(id string) (protocol.Protocol, bool) {
	if bc.registry == nil {
		return nil, false
	}
	return bc.registry.Find(id)
}

func (bc *blockchain) mustGetRollDPoSProtocol() *rolldpos.Protocol {
	p, ok := bc.protocol(rolldpos.ProtocolID)
	if !ok {
		log.L().Panic("protocol rolldpos has not been registered")
	}
	rp, ok := p.(*rolldpos.Protocol)
	if !ok {
		log.L().Panic("failed to cast to rolldpos protocol")
	}

	return rp
}

func (bc *blockchain) candidatesByHeight(height uint64) (state.CandidateList, error) {
	if bc.config.Genesis.EnableGravityChainVoting {
		rp := bc.mustGetRollDPoSProtocol()
		return bc.sf.CandidatesByHeight(rp.GetEpochHeight(rp.GetEpochNum(height)))
	}
	for {
		candidates, err := bc.sf.CandidatesByHeight(height)
		if err == nil {
			return candidates, nil
		}
		if height == 0 {
			return nil, err
		}
		height--
	}
}

func (bc *blockchain) getBlockByHeight(height uint64) (*block.Block, error) {
	hash, err := bc.dao.getBlockHash(height)
	if err != nil {
		return nil, err
	}
	return bc.dao.getBlock(hash)
}

func (bc *blockchain) startEmptyBlockchain() error {
	var ws factory.WorkingSet
	var err error
	if ws, err = bc.sf.NewWorkingSet(); err != nil {
		return errors.Wrap(err, "failed to obtain working set from state factory")
	}
	if !bc.config.Chain.EmptyGenesis {
		// Initialize the states before any actions happen on the blockchain
		if err := bc.createGenesisStates(ws); err != nil {
			return err
		}
		_ = ws.UpdateBlockLevelInfo(0)
	}
	// add Genesis states
	if err := bc.sf.Commit(ws); err != nil {
		return errors.Wrap(err, "failed to commit Genesis states")
	}
	return nil
}

func (bc *blockchain) startExistingBlockchain() error {
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
		blk, err := bc.getBlockByHeight(i)
		if err != nil {
			return err
		}
		ws, err := bc.sf.NewWorkingSet()
		if err != nil {
			return errors.Wrap(err, "failed to obtain working set from state factory")
		}
		if _, err := bc.runActions(blk.RunnableActions(), ws); err != nil {
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

func (bc *blockchain) validateBlock(blk *block.Block) error {
	validateTimer := bc.timerFactory.NewTimer("validate")
	prevBlkHash := bc.tipHash
	if blk.Height() == 1 {
		prevBlkHash = bc.config.Genesis.Hash()
	}
	err := bc.validator.Validate(blk, bc.tipHeight, prevBlkHash)
	validateTimer.End()
	if err != nil {
		return errors.Wrapf(err, "error when validating block %d", blk.Height())
	}
	// run actions and update state factory
	ws, err := bc.sf.NewWorkingSet()
	if err != nil {
		return errors.Wrap(err, "Failed to obtain working set from state factory")
	}
	runTimer := bc.timerFactory.NewTimer("runActions")
	receipts, err := bc.runActions(blk.RunnableActions(), ws)
	runTimer.End()
	if err != nil {
		log.L().Panic("Failed to update state.", zap.Uint64("tipHeight", bc.tipHeight), zap.Error(err))
	}

	if err = blk.VerifyDeltaStateDigest(ws.Digest()); err != nil {
		return err
	}

	if err = blk.VerifyReceiptRoot(calculateReceiptRoot(receipts)); err != nil {
		return errors.Wrap(err, "Failed to verify receipt root")
	}

	blk.Receipts = receipts

	// attach working set to be committed to state factory
	blk.WorkingSet = ws
	return nil
}

// commitBlock commits a block to the chain
func (bc *blockchain) commitBlock(blk *block.Block) error {
	// Check if it is already exists, and return earlier
	blkHash, err := bc.dao.getBlockHash(blk.Height())
	if blkHash != hash.ZeroHash256 {
		log.L().Debug("Block already exists.", zap.Uint64("height", blk.Height()))
		return nil
	}
	// If it's a ready db io error, return earlier with the error
	if errors.Cause(err) != db.ErrNotExist {
		return err
	}
	// write block into DB
	putTimer := bc.timerFactory.NewTimer("putBlock")
	err = bc.dao.putBlock(blk)
	putTimer.End()
	if err != nil {
		return err
	}

	// update tip hash and height
	atomic.StoreUint64(&bc.tipHeight, blk.Height())
	bc.tipHash = blk.HashBlock()

	if bc.sf != nil {
		sfTimer := bc.timerFactory.NewTimer("sf.Commit")
		err := bc.sf.Commit(blk.WorkingSet)
		sfTimer.End()
		// detach working set so it can be freed by GC
		blk.WorkingSet = nil
		if err != nil {
			log.L().Panic("Error when committing states.", zap.Error(err))
		}

		// write smart contract receipt into DB
		receiptTimer := bc.timerFactory.NewTimer("putReceipt")
		err = bc.dao.putReceipts(blk.Height(), blk.Receipts)
		receiptTimer.End()
		if err != nil {
			return errors.Wrapf(err, "failed to put smart contract receipts into DB on height %d", blk.Height())
		}
	}
	blk.HeaderLogger(log.L()).Info("Committed a block.", log.Hex("tipHash", bc.tipHash[:]))

	// emit block to all block subscribers
	bc.emitToSubscribers(blk)
	return nil
}

func (bc *blockchain) runActions(
	acts block.RunnableActions,
	ws factory.WorkingSet,
) ([]*action.Receipt, error) {
	if bc.sf == nil {
		return nil, errors.New("statefactory cannot be nil")
	}
	gasLimit := bc.config.Genesis.BlockGasLimit
	// update state factory
	producer, err := address.FromBytes(acts.BlockProducerPubKey().Hash())
	if err != nil {
		return nil, err
	}

	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			BlockHeight:    acts.BlockHeight(),
			BlockTimeStamp: acts.BlockTimeStamp(),
			Producer:       producer,
			GasLimit:       gasLimit,
			ActionGasLimit: bc.config.Genesis.ActionGasLimit,
			Registry:       bc.registry,
		})

	return ws.RunActions(ctx, acts.BlockHeight(), acts.Actions())
}

func (bc *blockchain) pickAndRunActions(ctx context.Context, actionMap map[string][]action.SealedEnvelope,
	ws factory.WorkingSet) (hash.Hash256, []*action.Receipt, []action.SealedEnvelope, error) {
	if bc.sf == nil {
		return hash.ZeroHash256, nil, nil, errors.New("statefactory cannot be nil")
	}
	receipts := make([]*action.Receipt, 0)
	executedActions := make([]action.SealedEnvelope, 0)

	raCtx := protocol.MustGetRunActionsCtx(ctx)
	// initial action iterator
	actionIterator := actioniterator.NewActionIterator(actionMap)
	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}

		receipt, err := ws.RunAction(raCtx, nextAction)
		if err != nil {
			if errors.Cause(err) == action.ErrHitGasLimit {
				// hit block gas limit, we should not process actions belong to this user anymore since we
				// need monotonically increasing nounce. But we can continue processing other actions
				// that belong other users
				actionIterator.PopAccount()
				continue
			}
			return hash.ZeroHash256, nil, nil, errors.Wrapf(err, "Failed to update state changes for selp %x", nextAction.Hash())
		}
		if receipt != nil {
			raCtx.GasLimit -= receipt.GasConsumed
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, nextAction)

		// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
		// than certain threshold
		if raCtx.GasLimit < bc.config.Chain.AllowedBlockGasResidue {
			break
		}
	}
	rp := bc.mustGetRollDPoSProtocol()
	epochNum := rp.GetEpochNum(raCtx.BlockHeight)
	lastBlkHeight := rp.GetEpochLastBlockHeight(epochNum)
	// generate delegates for next round
	skip, putPollResult, err := bc.createPutPollResultAction(raCtx.BlockHeight)
	switch errors.Cause(err) {
	case nil:
		if !skip {
			receipt, err := ws.RunAction(raCtx, putPollResult)
			if err != nil {
				return hash.ZeroHash256, nil, nil, err
			}
			if receipt != nil {
				receipts = append(receipts, receipt)
			}
			executedActions = append(executedActions, putPollResult)
		}
	case errDelegatesNotExist:
		if raCtx.BlockHeight == lastBlkHeight {
			// TODO (zhi): if some bp by pass this condition, we need to reject block in validation step
			return hash.ZeroHash256, nil, nil, errors.Wrapf(
				err,
				"failed to prepare delegates for next epoch %d",
				epochNum+1,
			)
		}
	default:
		return hash.ZeroHash256, nil, nil, err
	}
	// Process grant block reward action
	grant, err := bc.createGrantRewardAction(action.BlockReward)
	if err != nil {
		return hash.ZeroHash256, nil, nil, err
	}
	receipt, err := ws.RunAction(raCtx, grant)
	if err != nil {
		return hash.ZeroHash256, nil, nil, err
	}
	if receipt != nil {
		receipts = append(receipts, receipt)
	}
	executedActions = append(executedActions, grant)

	// Process grant epoch reward action if the block is the last one in an epoch
	if raCtx.BlockHeight == lastBlkHeight {
		grant, err = bc.createGrantRewardAction(action.EpochReward)
		if err != nil {
			return hash.ZeroHash256, nil, nil, err
		}
		receipt, err = ws.RunAction(raCtx, grant)
		if err != nil {
			return hash.ZeroHash256, nil, nil, err
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, grant)
	}

	blockMtc.WithLabelValues("gasConsumed").Set(float64(bc.config.Genesis.BlockGasLimit - raCtx.GasLimit))

	return ws.UpdateBlockLevelInfo(raCtx.BlockHeight), receipts, executedActions, nil
}

func (bc *blockchain) createPutPollResultAction(height uint64) (skip bool, se action.SealedEnvelope, err error) {
	skip = true
	if !bc.config.Genesis.EnableGravityChainVoting {
		return
	}
	pl, ok := bc.protocol(poll.ProtocolID)
	if !ok {
		log.L().Panic("protocol poll has not been registered")
	}
	pp, ok := pl.(poll.Protocol)
	if !ok {
		log.L().Panic("Failed to cast to poll.Protocol")
	}
	rp := bc.mustGetRollDPoSProtocol()
	epochNum := rp.GetEpochNum(height)
	epochHeight := rp.GetEpochHeight(epochNum)
	nextEpochHeight := rp.GetEpochHeight(epochNum + 1)
	if height < epochHeight+(nextEpochHeight-epochHeight)/2 {
		return
	}
	log.L().Debug(
		"createPutPollResultAction",
		zap.Uint64("height", height),
		zap.Uint64("epochNum", epochNum),
		zap.Uint64("epochHeight", epochHeight),
		zap.Uint64("nextEpochHeight", nextEpochHeight),
	)
	_, err = bc.candidatesByHeight(nextEpochHeight)
	switch errors.Cause(err) {
	case nil:
		return
	case state.ErrStateNotExist:
		skip = false
	default:
		return
	}
	l, err := pp.DelegatesByHeight(epochHeight)
	switch errors.Cause(err) {
	case nil:
		if len(l) == 0 {
			err = errors.Wrapf(
				errDelegatesNotExist,
				"failed to fetch delegates by epoch height %d, empty list",
				epochHeight,
			)
			return
		}
	case db.ErrNotExist:
		err = errors.Wrapf(
			errDelegatesNotExist,
			"failed to fetch delegates by epoch height %d, original error %v",
			epochHeight,
			err,
		)
		return
	default:
		return
	}
	sk := bc.config.ProducerPrivateKey()
	nonce := uint64(0)
	pollAction := action.NewPutPollResult(nonce, nextEpochHeight, l)
	builder := action.EnvelopeBuilder{}
	se, err = action.Sign(builder.SetNonce(nonce).SetAction(pollAction).Build(), sk)
	return skip, se, err
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

// RecoverToHeight recovers the blockchain to target height
func (bc *blockchain) recoverToHeight(targetHeight uint64) error {
	for bc.tipHeight > targetHeight {
		if err := bc.dao.deleteTipBlock(); err != nil {
			return err
		}
		bc.tipHeight--
	}
	return nil
}

// RefreshStateDB deletes the existing state DB and creates a new one with state changes from genesis block
func (bc *blockchain) refreshStateDB() error {
	// Delete existing state DB and reinitialize it
	if fileutil.FileExists(bc.config.Chain.TrieDBPath) && os.Remove(bc.config.Chain.TrieDBPath) != nil {
		return errors.New("failed to delete existing state DB")
	}
	if err := DefaultStateFactoryOption()(bc, bc.config); err != nil {
		return errors.Wrap(err, "failed to reinitialize state DB")
	}

	for _, p := range bc.registry.All() {
		bc.sf.AddActionHandlers(p)
	}

	if err := bc.sf.Start(context.Background()); err != nil {
		return errors.Wrap(err, "failed to start state factory")
	}
	if err := bc.startEmptyBlockchain(); err != nil {
		return err
	}
	if err := bc.sf.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop state factory")
	}
	return nil
}

func (bc *blockchain) createGrantRewardAction(rewardType int) (action.SealedEnvelope, error) {
	gb := action.GrantRewardBuilder{}
	grant := gb.SetRewardType(rewardType).Build()
	eb := action.EnvelopeBuilder{}
	envelope := eb.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(grant.GasLimit()).
		SetAction(&grant).
		Build()
	sk := bc.config.ProducerPrivateKey()
	return action.Sign(envelope, sk)
}

func (bc *blockchain) createGenesisStates(ws factory.WorkingSet) error {
	if bc.registry == nil {
		// TODO: return nil to avoid test cases to blame on missing rewarding protocol
		return nil
	}
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		BlockHeight:    0,
		BlockTimeStamp: time.Unix(bc.config.Genesis.Timestamp, 0),
		GasLimit:       0,
		ActionGasLimit: bc.config.Genesis.ActionGasLimit,
		Producer:       nil,
		Caller:         nil,
		ActionHash:     hash.ZeroHash256,
		Nonce:          0,
		Registry:       bc.registry,
	})
	if err := bc.createAccountGenesisStates(ctx, ws); err != nil {
		return err
	}
	if err := bc.createPollGenesisStates(ctx, ws); err != nil {
		return err
	}
	return bc.createRewardingGenesisStates(ctx, ws)
}

func (bc *blockchain) createAccountGenesisStates(ctx context.Context, ws factory.WorkingSet) error {
	p, ok := bc.registry.Find(account.ProtocolID)
	if !ok {
		return nil
	}
	ap, ok := p.(*account.Protocol)
	if !ok {
		return errors.Errorf("error when casting protocol")
	}
	addrs, balances := bc.config.Genesis.InitBalances()
	return ap.Initialize(ctx, ws, addrs, balances)
}

func (bc *blockchain) createRewardingGenesisStates(ctx context.Context, ws factory.WorkingSet) error {
	p, ok := bc.registry.Find(rewarding.ProtocolID)
	if !ok {
		return nil
	}
	rp, ok := p.(*rewarding.Protocol)
	if !ok {
		return errors.Errorf("error when casting protocol")
	}
	return rp.Initialize(
		ctx,
		ws,
		bc.config.Genesis.InitBalance(),
		bc.config.Genesis.BlockReward(),
		bc.config.Genesis.EpochReward(),
		bc.config.Genesis.NumDelegatesForEpochReward,
		bc.config.Genesis.ExemptAddrsFromEpochReward(),
		bc.config.Genesis.FoundationBonus(),
		bc.config.Genesis.NumDelegatesForFoundationBonus,
		bc.config.Genesis.FoundationBonusLastEpoch,
		bc.config.Genesis.ProductivityThreshold,
	)
}

func (bc *blockchain) createPollGenesisStates(ctx context.Context, ws factory.WorkingSet) error {
	if bc.config.Genesis.EnableGravityChainVoting {
		p, ok := bc.protocol(poll.ProtocolID)
		if !ok {
			return errors.Errorf("protocol %s is not found", poll.ProtocolID)
		}
		pp, ok := p.(poll.Protocol)
		if !ok {
			return errors.Errorf("error when casting poll protocol")
		}
		return pp.Initialize(
			ctx,
			ws,
		)
	}
	p, ok := bc.protocol(vote.ProtocolID)
	if !ok {
		return errors.Errorf("protocol %s is not found", vote.ProtocolID)
	}
	vp, ok := p.(*vote.Protocol)
	if !ok {
		return errors.Errorf("error when casting vote protocol")
	}
	addrs := make([]address.Address, 0)
	for _, d := range bc.config.Genesis.Delegates {
		addrs = append(addrs, d.OperatorAddr())
	}
	return vp.Initialize(ctx, ws, addrs)
}

func calculateReceiptRoot(receipts []*action.Receipt) hash.Hash256 {
	h := make([]hash.Hash256, 0, len(receipts))
	for _, receipt := range receipts {
		h = append(h, receipt.Hash())
	}
	if len(h) == 0 {
		return hash.ZeroHash256
	}
	res := crypto.NewMerkleTree(h).HashTree()
	return res
}
