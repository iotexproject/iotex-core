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

	"github.com/facebookgo/clock"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

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
	// For exposing blockchain states
	// GetHeightByHash returns Block's height by hash
	GetHeightByHash(h hash.Hash256) (uint64, error)
	// GetHashByHeight returns Block's hash by height
	GetHashByHeight(height uint64) (hash.Hash256, error)
	// GetBlockByHeight returns Block by height
	GetBlockByHeight(height uint64) (*block.Block, error)
	// GetBlockByHash returns Block by hash
	GetBlockByHash(h hash.Hash256) (*block.Block, error)
	// GetTotalTransfers returns the total number of transfers
	GetTotalTransfers() (uint64, error)
	// GetTotalVotes returns the total number of votes
	GetTotalVotes() (uint64, error)
	// GetTotalExecutions returns the total number of executions
	GetTotalExecutions() (uint64, error)
	// GetTotalActions returns the total number of actions
	GetTotalActions() (uint64, error)
	// GetTransfersFromAddress returns transaction from address
	GetTransfersFromAddress(address string) ([]hash.Hash256, error)
	// GetTransfersToAddress returns transaction to address
	GetTransfersToAddress(address string) ([]hash.Hash256, error)
	// GetTransfersByTransferHash returns transfer by transfer hash
	GetTransferByTransferHash(h hash.Hash256) (*action.Transfer, error)
	// GetBlockHashByTransferHash returns Block hash by transfer hash
	GetBlockHashByTransferHash(h hash.Hash256) (hash.Hash256, error)
	// GetVoteFromAddress returns vote from address
	GetVotesFromAddress(address string) ([]hash.Hash256, error)
	// GetVoteToAddress returns vote to address
	GetVotesToAddress(address string) ([]hash.Hash256, error)
	// GetVotesByVoteHash returns vote by vote hash
	GetVoteByVoteHash(h hash.Hash256) (*action.Vote, error)
	// GetBlockHashByVoteHash returns Block hash by vote hash
	GetBlockHashByVoteHash(h hash.Hash256) (hash.Hash256, error)
	// GetExecutionsFromAddress returns executions from address
	GetExecutionsFromAddress(address string) ([]hash.Hash256, error)
	// GetExecutionsToAddress returns executions to address
	GetExecutionsToAddress(address string) ([]hash.Hash256, error)
	// GetExecutionByExecutionHash returns execution by execution hash
	GetExecutionByExecutionHash(h hash.Hash256) (*action.Execution, error)
	// GetBlockHashByExecutionHash returns Block hash by execution hash
	GetBlockHashByExecutionHash(h hash.Hash256) (hash.Hash256, error)
	// GetReceiptByActionHash returns the receipt by action hash
	GetReceiptByActionHash(h hash.Hash256) (*action.Receipt, error)
	// GetActionsFromAddress returns actions from address
	GetActionsFromAddress(address string) ([]hash.Hash256, error)
	// GetActionsToAddress returns actions to address
	GetActionsToAddress(address string) ([]hash.Hash256, error)
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

	// For block operations
	// MintNewBlock creates a new block with given actions
	// Note: the coinbase transfer will be added to the given transfers when minting a new block
	MintNewBlock(
		actionMap map[string][]action.SealedEnvelope,
		producerPubKey keypair.PublicKey,
		producerPriKey keypair.PrivateKey,
		producerAddr string,
		timestamp int64,
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
	genesis       *Genesis
	tipHeight     uint64
	tipHash       hash.Hash256
	validator     Validator
	lifecycle     lifecycle.Lifecycle
	clk           clock.Clock
	blocklistener []BlockCreationSubscriber
	timerFactory  *prometheustimer.TimerFactory

	// used by account-based model
	sf factory.Factory

	genesisConfig genesis.Genesis
	registry      *protocol.Registry
}

// Option sets blockchain construction parameter
type Option func(*blockchain, config.Config) error

// key specifies the type of recovery height key used by context
type key string

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
		bc.dao = newBlockDAO(db.NewOnDiskDB(cfg.DB), cfg.Chain.EnableIndex && !cfg.Chain.EnableAsyncIndexWrite)
		return nil
	}
}

// InMemDaoOption sets blockchain's dao with MemKVStore
func InMemDaoOption() Option {
	return func(bc *blockchain, cfg config.Config) error {
		bc.dao = newBlockDAO(db.NewMemKVStore(), cfg.Chain.EnableIndex && !cfg.Chain.EnableAsyncIndexWrite)

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

// GenesisOption sets the blockchain with the genesis configs
func GenesisOption(genesisConfig genesis.Genesis) Option {
	return func(bc *blockchain, conf config.Config) error {
		bc.genesisConfig = genesisConfig
		return nil
	}
}

// NewBlockchain creates a new blockchain and DB instance
func NewBlockchain(cfg config.Config, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchain{
		config:  cfg,
		genesis: Gen,
		clk:     clock.New(),
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
	chain.validator = &validator{sf: chain.sf, validatorAddr: producerAddress(cfg).String()}

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
		_, err = bc.getBlockByHeight(0)
		if errors.Cause(err) == db.ErrNotExist {
			return bc.startEmptyBlockchain()
		}
		if err != nil {
			return err
		}
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
	return bc.sf.CandidatesByHeight(height)
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

// TODO: To be deprecated
// GetTotalTransfers returns the total number of transfers
func (bc *blockchain) GetTotalTransfers() (uint64, error) {
	if !bc.config.Chain.EnableIndex {
		return 0, errors.New("index not enabled")
	}
	return bc.dao.getTotalTransfers()
}

// TODO: To be deprecated
// GetTotalVotes returns the total number of votes
func (bc *blockchain) GetTotalVotes() (uint64, error) {
	if !bc.config.Chain.EnableIndex {
		return 0, errors.New("index not enabled")
	}
	return bc.dao.getTotalVotes()
}

// TODO: To be deprecated
// GetTotalExecutions returns the total number of executions
func (bc *blockchain) GetTotalExecutions() (uint64, error) {
	if !bc.config.Chain.EnableIndex {
		return 0, errors.New("index not enabled")
	}
	return bc.dao.getTotalExecutions()
}

// GetTotalActions returns the total number of actions
func (bc *blockchain) GetTotalActions() (uint64, error) {
	if !bc.config.Chain.EnableIndex {
		return 0, errors.New("index not enabled")
	}
	return bc.dao.getTotalActions()
}

// TODO: To be deprecated
// GetTransfersFromAddress returns transfers from address
func (bc *blockchain) GetTransfersFromAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getTransfersBySenderAddress(bc.dao.kvstore, address)
}

// TODO: To be deprecated
// GetTransfersToAddress returns transfers to address
func (bc *blockchain) GetTransfersToAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getTransfersByRecipientAddress(bc.dao.kvstore, address)
}

// TODO: To be deprecated
// GetTransferByTransferHash returns transfer by transfer hash
func (bc *blockchain) GetTransferByTransferHash(h hash.Hash256) (*action.Transfer, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	blkHash, err := getBlockHashByTransferHash(bc.dao.kvstore, h)
	if err != nil {
		return nil, err
	}
	blk, err := bc.dao.getBlock(blkHash)
	if err != nil {
		return nil, err
	}
	transfers, _, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range transfers {
		if transfer.Hash() == h {
			return transfer, nil
		}
	}
	return nil, errors.Errorf("block %x does not have transfer %x", blkHash, h)
}

// TODO: To be deprecated
// GetBlockHashByTxHash returns Block hash by transfer hash
func (bc *blockchain) GetBlockHashByTransferHash(h hash.Hash256) (hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return hash.ZeroHash256, errors.New("index not enabled")
	}
	return getBlockHashByTransferHash(bc.dao.kvstore, h)
}

// TODO: To be deprecated
// GetVoteFromAddress returns votes from address
func (bc *blockchain) GetVotesFromAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getVotesBySenderAddress(bc.dao.kvstore, address)
}

// TODO: To be deprecated
// GetVoteToAddress returns votes to address
func (bc *blockchain) GetVotesToAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getVotesByRecipientAddress(bc.dao.kvstore, address)
}

// TODO: To be deprecated
// GetVotesByVoteHash returns vote by vote hash
func (bc *blockchain) GetVoteByVoteHash(h hash.Hash256) (*action.Vote, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	blkHash, err := getBlockHashByVoteHash(bc.dao.kvstore, h)
	if err != nil {
		return nil, err
	}
	blk, err := bc.dao.getBlock(blkHash)
	if err != nil {
		return nil, err
	}

	for _, selp := range blk.Actions {
		if v, ok := selp.Action().(*action.Vote); ok {
			if selp.Hash() == h {
				return v, nil
			}
		}
	}
	return nil, errors.Errorf("block %x does not have vote %x", blkHash, h)
}

// TODO: To be deprecated
// GetBlockHashByVoteHash returns Block hash by vote hash
func (bc *blockchain) GetBlockHashByVoteHash(h hash.Hash256) (hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return hash.ZeroHash256, errors.New("index not enabled")
	}
	return getBlockHashByVoteHash(bc.dao.kvstore, h)
}

// TODO: To be deprecated
// GetExecutionsFromAddress returns executions from address
func (bc *blockchain) GetExecutionsFromAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getExecutionsByExecutorAddress(bc.dao.kvstore, address)
}

// TODO: To be deprecated
// GetExecutionsToAddress returns executions to address
func (bc *blockchain) GetExecutionsToAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getExecutionsByContractAddress(bc.dao.kvstore, address)
}

// TODO: To be deprecated
// GetExecutionByExecutionHash returns execution by execution hash
func (bc *blockchain) GetExecutionByExecutionHash(h hash.Hash256) (*action.Execution, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	blkHash, err := getBlockHashByExecutionHash(bc.dao.kvstore, h)
	if err != nil {
		return nil, err
	}
	blk, err := bc.dao.getBlock(blkHash)
	if err != nil {
		return nil, err
	}
	_, _, executions := action.ClassifyActions(blk.Actions)
	for _, execution := range executions {
		if execution.Hash() == h {
			return execution, nil
		}
	}
	return nil, errors.Errorf("block %x does not have execution %x", blkHash, h)
}

// TODO: To be deprecated
// GetBlockHashByExecutionHash returns Block hash by execution hash
func (bc *blockchain) GetBlockHashByExecutionHash(h hash.Hash256) (hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return hash.ZeroHash256, errors.New("index not enabled")
	}
	return getBlockHashByExecutionHash(bc.dao.kvstore, h)
}

// GetReceiptByActionHash returns the receipt by action hash
func (bc *blockchain) GetReceiptByActionHash(h hash.Hash256) (*action.Receipt, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return bc.dao.getReceiptByActionHash(h)
}

// GetActionsFromAddress returns actions from address
func (bc *blockchain) GetActionsFromAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getActionsBySenderAddress(bc.dao.kvstore, address)
}

// GetActionToAddress returns action to address
func (bc *blockchain) GetActionsToAddress(address string) ([]hash.Hash256, error) {
	if !bc.config.Chain.EnableIndex {
		return nil, errors.New("index not enabled")
	}
	return getActionsByRecipientAddress(bc.dao.kvstore, address)
}

func (bc *blockchain) getActionByActionHashHelper(h hash.Hash256) (hash.Hash256, error) {
	blkHash, err := getBlockHashByTransferHash(bc.dao.kvstore, h)
	if err == nil {
		return blkHash, nil
	}
	blkHash, err = getBlockHashByVoteHash(bc.dao.kvstore, h)
	if err == nil {
		return blkHash, nil
	}
	blkHash, err = getBlockHashByExecutionHash(bc.dao.kvstore, h)
	if err == nil {
		return blkHash, nil
	}
	return getBlockHashByActionHash(bc.dao.kvstore, h)
}

// GetActionByActionHash returns action by action hash
func (bc *blockchain) GetActionByActionHash(h hash.Hash256) (action.SealedEnvelope, error) {
	if !bc.config.Chain.EnableIndex {
		return action.SealedEnvelope{}, errors.New("index not enabled")
	}

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
	if !bc.config.Chain.EnableIndex {
		return hash.ZeroHash256, errors.New("index not enabled")
	}
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
	producerPubKey keypair.PublicKey,
	producerPriKey keypair.PrivateKey,
	producerAddr string,
	timestamp int64,
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
	producer, err := address.FromString(producerAddr)
	if err != nil {
		return nil, err
	}

	gasLimitForContext := bc.genesisConfig.BlockGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EpochNumber: getEpochNum(
				newblockHeight,
				bc.genesisConfig.NumDelegates,
				bc.genesisConfig.NumSubEpochs,
			),
			BlockHeight: newblockHeight,
			// this field should be removed
			BlockHash:      hash.ZeroHash256,
			BlockTimeStamp: bc.now(),
			Producer:       producer,
			GasLimit:       &gasLimitForContext,
			ActionGasLimit: bc.genesisConfig.ActionGasLimit,
			Registry:       bc.registry,
		})
	root, rc, actions, err := bc.pickAndRunActions(ctx, actionMap, ws)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to update state changes in new block %d", newblockHeight)
	}

	ra := block.NewRunnableActionsBuilder().
		SetHeight(newblockHeight).
		SetTimeStamp(timestamp).
		AddActions(actions...).
		Build(producerAddr, producerPubKey)

	validateActionsOnlyTimer := bc.timerFactory.NewTimer("ValidateActionsOnly")
	if err := bc.validator.ValidateActionsOnly(
		actions,
		producerPubKey,
		bc.ChainID(),
		newblockHeight,
	); err != nil {
		validateActionsOnlyTimer.End()
		return nil, err
	}
	validateActionsOnlyTimer.End()

	blk, err := block.NewBuilder(ra).
		SetChainID(bc.config.Chain.ID).
		SetPrevBlockHash(bc.tipHash).
		SetStateRoot(root).
		SetDeltaStateDigest(ws.Digest()).
		SetReceipts(rc).
		SetReceiptRoot(calculateReceiptRoot(rc)).
		SignAndBuild(producerPubKey, producerPriKey)
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
	gasLimit := bc.genesisConfig.BlockGasLimit
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		BlockHeight:    blk.Height(),
		BlockHash:      blk.HashBlock(),
		BlockTimeStamp: blk.Timestamp(),
		Producer:       producer,
		Caller:         caller,
		GasLimit:       &gasLimit,
		ActionGasLimit: bc.genesisConfig.ActionGasLimit,
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
	account, err := util.LoadOrCreateAccount(ws, addr, init)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create new account %s", addr)
	}
	genesisBlk, err := bc.GetBlockByHeight(0)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get genesis block")
	}
	gasLimit := bc.genesisConfig.BlockGasLimit
	callerAddr, err := address.FromString(addr)
	if err != nil {
		return nil, err
	}
	producer, err := address.FromString(genesisBlk.ProducerAddress())
	if err != nil {
		return nil, err
	}
	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EpochNumber:    0,
			Producer:       producer,
			GasLimit:       &gasLimit,
			ActionGasLimit: bc.genesisConfig.ActionGasLimit,
			Caller:         callerAddr,
			ActionHash:     hash.ZeroHash256,
			Nonce:          0,
			Registry:       bc.registry,
		})
	if _, _, err = ws.RunActions(ctx, 0, nil); err != nil {
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

//======================================
// private functions
//=====================================

func (bc *blockchain) getBlockByHeight(height uint64) (*block.Block, error) {
	hash, err := bc.dao.getBlockHash(height)
	if err != nil {
		return nil, err
	}
	return bc.dao.getBlock(hash)
}

func (bc *blockchain) startEmptyBlockchain() error {
	var (
		genesis block.Block
		ws      factory.WorkingSet
		err     error
	)
	pk, sk, addr, err := bc.genesisProducer()
	if err != nil {
		return errors.Wrap(err, "failed to get the key and address of producer")
	}

	if bc.sf == nil {
		return errors.New("statefactory cannot be nil")
	}
	if ws, err = bc.sf.NewWorkingSet(); err != nil {
		return errors.Wrap(err, "failed to obtain working set from state factory")
	}
	if bc.config.Chain.GenesisActionsPath != "" || !bc.config.Chain.EmptyGenesis {
		acts := NewGenesisActions(bc.config.Chain, ws)
		racts := block.NewRunnableActionsBuilder().
			SetHeight(0).
			SetTimeStamp(Gen.Timestamp).
			AddActions(acts...).
			Build(addr, pk)
		// run execution and update state trie root hash
		root, receipts, err := bc.runActions(racts, ws)
		if err != nil {
			return errors.Wrap(err, "failed to update state changes in Genesis block")
		}

		// Initialize the states before any actions happen on the blockchain
		if err := bc.createGenesisStates(ws); err != nil {
			return err
		}

		genesis, err = block.NewBuilder(racts).
			SetChainID(bc.ChainID()).
			SetPrevBlockHash(Gen.ParentHash).
			SetStateRoot(root).
			SetDeltaStateDigest(ws.Digest()).
			SetReceipts(receipts).
			SetReceiptRoot(calculateReceiptRoot(receipts)).
			SignAndBuild(pk, sk)
		if err != nil {
			return errors.Wrapf(err, "Failed to create block")
		}
	} else {
		racts := block.NewRunnableActionsBuilder().
			SetHeight(0).
			SetTimeStamp(Gen.Timestamp).
			Build(addr, pk)
		genesis, err = block.NewBuilder(racts).
			SetChainID(bc.ChainID()).
			SetPrevBlockHash(hash.ZeroHash256).
			SignAndBuild(pk, sk)
		if err != nil {
			return errors.Wrapf(err, "Failed to create block")
		}
	}
	genesis.WorkingSet = ws
	// add Genesis block as very first block
	if err := bc.commitBlock(&genesis); err != nil {
		return errors.Wrap(err, "failed to commit Genesis block")
	}
	return nil
}

func (bc *blockchain) startExistingBlockchain() error {
	if bc.sf == nil {
		return errors.New("statefactory cannot be nil")
	}

	stateHeight, err := bc.sf.Height()
	if err != nil {
		return errors.New("invalid state DB")
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
		if _, _, err := bc.runActions(blk.RunnableActions(), ws); err != nil {
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
	err := bc.validator.Validate(blk, bc.tipHeight, bc.tipHash)
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
	root, receipts, err := bc.runActions(blk.RunnableActions(), ws)
	runTimer.End()
	if err != nil {
		log.L().Panic("Failed to update state.", zap.Uint64("tipHeight", bc.tipHeight), zap.Error(err))
	}

	if err = blk.VerifyStateRoot(root); err != nil {
		return err
	}

	if err = blk.VerifyDeltaStateDigest(ws.Digest()); err != nil {
		return err
	}

	if err = blk.VerifyReceiptRoot(calculateReceiptRoot(receipts)); err != nil {
		return errors.Wrap(err, "Failed to verify receipt root")
	}

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
) (hash.Hash256, []*action.Receipt, error) {
	if bc.sf == nil {
		return hash.ZeroHash256, nil, errors.New("statefactory cannot be nil")
	}
	gasLimit := bc.genesisConfig.BlockGasLimit
	// update state factory
	producer, err := address.FromString(acts.BlockProducerAddr())
	if err != nil {
		return hash.ZeroHash256, nil, err
	}

	ctx := protocol.WithRunActionsCtx(context.Background(),
		protocol.RunActionsCtx{
			EpochNumber: getEpochNum(
				acts.BlockHeight(),
				bc.genesisConfig.NumDelegates,
				bc.genesisConfig.NumSubEpochs,
			),
			BlockHeight: acts.BlockHeight(),
			// this field should be removed
			BlockHash:      hash.ZeroHash256,
			BlockTimeStamp: int64(acts.BlockTimeStamp()),
			Producer:       producer,
			GasLimit:       &gasLimit,
			ActionGasLimit: bc.genesisConfig.ActionGasLimit,
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

	raCtx, ok := protocol.GetRunActionsCtx(ctx)
	if !ok {
		return hash.ZeroHash256, nil, nil, errors.New("failed to get action context")
	}
	// initial action iterator
	actionIterator := actioniterator.NewActionIterator(actionMap)
	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}

		receipt, err := ws.RunAction(ctx, nextAction)
		if err != nil {
			if errors.Cause(err) == action.ErrHitGasLimit {
				// hit block gas limit, we should not process actions belong to this user anymore since we
				// need monotonically increasing nounce. But we can continue processing other actions
				// that belong other users
				actionIterator.PopAccount()
				continue
			}
			return hash.ZeroHash256, nil, nil, errors.Wrapf(err, "Failed to update state changes for selp %s", nextAction.Hash())
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		executedActions = append(executedActions, nextAction)

		// To prevent loop all actions in act_pool, we stop processing action when remaining gas is below
		// than certain threshold
		if *raCtx.GasLimit < bc.config.Chain.AllowedBlockGasResidue {
			break
		}
	}

	// Process grant block reward action
	grant, err := bc.createGrantBlockRewardAction()
	if err != nil {
		return hash.ZeroHash256, nil, nil, err
	}
	receipt, err := ws.RunAction(ctx, grant)
	if receipt != nil {
		receipts = append(receipts, receipt)
	}
	executedActions = append(executedActions, grant)

	return ws.UpdateBlockLevelInfo(raCtx.BlockHeight), receipts, executedActions, nil
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

func (bc *blockchain) now() int64 { return bc.clk.Now().Unix() }

func (bc *blockchain) genesisProducer() (keypair.PublicKey, keypair.PrivateKey, string, error) {
	pk, err := keypair.DecodePublicKey(GenesisProducerPublicKey)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "failed to decode public key")
	}
	sk, err := keypair.DecodePrivateKey(GenesisProducerPrivateKey)
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "failed to decode private key")
	}
	pkHash := keypair.HashPubKey(pk)
	addr, err := address.FromBytes(pkHash[:])
	if err != nil {
		return nil, nil, "", errors.Wrap(err, "failed to create address")
	}
	return pk, sk, addr.String(), nil
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
	if err := bc.buildStateInGenesis(); err != nil {
		return errors.Wrap(err, "failed to build state for genesis block")
	}
	if err := bc.sf.Stop(context.Background()); err != nil {
		return errors.Wrap(err, "failed to stop state factory")
	}
	return nil
}

func (bc *blockchain) buildStateInGenesis() error {
	pk, _, addr, err := bc.genesisProducer()
	if err != nil {
		return errors.Wrap(err, "failed to get the key and address of producer")
	}
	ws, err := bc.sf.NewWorkingSet()
	if err != nil {
		return errors.Wrap(err, "failed to obtain working set from state factory")
	}
	acts := NewGenesisActions(bc.config.Chain, ws)
	racts := block.NewRunnableActionsBuilder().
		SetHeight(0).
		SetTimeStamp(Gen.Timestamp).
		AddActions(acts...).
		Build(addr, pk)
	// run execution and update state trie root hash
	if _, _, err := bc.runActions(racts, ws); err != nil {
		return errors.Wrap(err, "failed to update state changes in Genesis block")
	}
	// Initialize the states before any actions happen on the blockchain
	if err := bc.createGenesisStates(ws); err != nil {
		return err
	}
	if err := bc.sf.Commit(ws); err != nil {
		return errors.Wrap(err, "failed to commit state changes in Genesis block")
	}
	return nil
}

func (bc *blockchain) createGrantBlockRewardAction() (action.SealedEnvelope, error) {
	gb := action.GrantRewardBuilder{}
	grant := gb.SetRewardType(action.BlockReward).Build()
	eb := action.EnvelopeBuilder{}
	envelope := eb.SetNonce(0).
		SetGasPrice(big.NewInt(0)).
		SetGasLimit(grant.GasLimit()).
		SetAction(&grant).
		Build()
	_, sk, err := bc.config.KeyPair()
	if err != nil {
		log.L().Panic("Failed to get block producer private key.", zap.Error(err))
	}
	return action.Sign(envelope, sk)
}

func (bc *blockchain) createGenesisStates(ws factory.WorkingSet) error {
	if bc.registry == nil {
		// TODO: return nil to avoid test cases to blame on missing rewarding protocol
		return nil
	}
	ctx := protocol.WithRunActionsCtx(context.Background(), protocol.RunActionsCtx{
		EpochNumber:    0,
		BlockHeight:    0,
		BlockHash:      hash.ZeroHash256,
		BlockTimeStamp: bc.genesisConfig.Timestamp,
		GasLimit:       nil,
		ActionGasLimit: bc.genesisConfig.ActionGasLimit,
		Producer:       nil,
		Caller:         nil,
		ActionHash:     hash.ZeroHash256,
		Nonce:          0,
		Registry:       bc.registry,
	})
	p, ok := bc.registry.Find(rewarding.ProtocolID)
	if !ok {
		return errors.Errorf("protocol %s isn't found", rewarding.ProtocolID)
	}
	rp, ok := p.(*rewarding.Protocol)
	if !ok {
		return errors.Errorf("error when casting protocol")
	}
	return rp.Initialize(
		ctx,
		ws,
		bc.genesisConfig.Rewarding.InitAdminAddr(),
		bc.genesisConfig.InitBalance(),
		bc.genesisConfig.BlockReward(),
		bc.genesisConfig.EpochReward(),
	)
}

func calculateReceiptRoot(receipts []*action.Receipt) hash.Hash256 {
	var h []hash.Hash256
	for _, receipt := range receipts {
		h = append(h, receipt.Hash())
	}
	if len(h) == 0 {
		return hash.ZeroHash256
	}
	res := crypto.NewMerkleTree(h).HashTree()
	return res
}

func producerAddress(cfg config.Config) address.Address {
	pubKey, _, err := cfg.KeyPair()
	if err != nil {
		log.L().Panic("Failed to get block producer public key.", zap.Error(err))
	}
	pkHash := keypair.HashPubKey(pubKey)
	address, err := address.FromBytes(pkHash[:])
	if err != nil {
		log.L().Panic("Failed to get block producer address.", zap.Error(err))
	}
	return address
}

// TODO: consolidate with the same method in consensus module
func getEpochNum(
	height uint64,
	numDelegates uint64,
	numSubEpochs uint64,
) uint64 {
	return (height-1)/uint64(numDelegates)/uint64(numSubEpochs) + 1
}
