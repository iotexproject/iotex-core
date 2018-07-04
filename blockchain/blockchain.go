// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math/big"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/state"
)

// Blockchain represents the blockchain data structure and hosts the APIs to access it
type Blockchain interface {
	service.Service

	// Balance returns balance of an account
	Balance(addr string) (*big.Int, error)
	// Nonce returns the nonce if the account exists
	Nonce(addr string) (uint64, error)
	// For exposing blockchain states
	// GetHeightByHash returns Block's height by hash
	GetHeightByHash(hash common.Hash32B) (uint64, error)
	// GetHashByHeight returns Block's hash by height
	GetHashByHeight(height uint64) (common.Hash32B, error)
	// GetBlockByHeight returns Block by height
	GetBlockByHeight(height uint64) (*Block, error)
	// GetBlockByHash returns Block by hash
	GetBlockByHash(hash common.Hash32B) (*Block, error)
	// GetTotalTransfers returns the total number of transfers
	GetTotalTransfers() (uint64, error)
	// GetTotalVotes returns the total number of votes
	GetTotalVotes() (uint64, error)
	// GetTransfersFromAddress returns transaction from address
	GetTransfersFromAddress(address string) ([]common.Hash32B, error)
	// GetTransfersToAddress returns transaction to address
	GetTransfersToAddress(address string) ([]common.Hash32B, error)
	// GetTransfersByTransferHash returns transfer by transfer hash
	GetTransferByTransferHash(hash common.Hash32B) (*action.Transfer, error)
	// GetBlockHashByTransferHash returns Block hash by transfer hash
	GetBlockHashByTransferHash(hash common.Hash32B) (common.Hash32B, error)
	// GetVoteFromAddress returns vote from address
	GetVotesFromAddress(address string) ([]common.Hash32B, error)
	// GetVoteToAddress returns vote to address
	GetVotesToAddress(address string) ([]common.Hash32B, error)
	// GetVotesByVoteHash returns vote by vote hash
	GetVoteByVoteHash(hash common.Hash32B) (*action.Vote, error)
	// GetBlockHashByVoteHash returns Block hash by vote hash
	GetBlockHashByVoteHash(hash common.Hash32B) (common.Hash32B, error)
	// TipHash returns tip block's hash
	TipHash() (common.Hash32B, error)
	// TipHeight returns tip block's height
	TipHeight() (uint64, error)
	// StateByAddr returns state of a given address
	StateByAddr(address string) (*state.State, error)

	// For block operations
	// MintNewBlock creates a new block with given actions
	// Note: the coinbase transfer will be added to the given transfers when minting a new block
	MintNewBlock(tsf []*action.Transfer, vote []*action.Vote, address *iotxaddress.Address, data string) (*Block, error)
	// MintNewDummyBlock creates a new dummy block with no transactions
	MintNewDummyBlock() (*Block, error)
	// CommitBlock validates and appends a block to the chain
	CommitBlock(blk *Block) error
	// ValidateBlock validates a new block before adding it to the blockchain
	ValidateBlock(blk *Block) error

	// For action operations
	// CreateRawTransfer creates an unsigned transfer paying 'amount' from 'from' to 'to'
	CreateRawTransfer(nonce uint64, from *iotxaddress.Address, amount *big.Int, to *iotxaddress.Address) *action.Transfer
	// CreateRawVote creates an unsigned vote
	CreateRawVote(nonce uint64, from *iotxaddress.Address, to *iotxaddress.Address) *action.Vote

	// Validator returns the current validator object
	Validator() Validator
	// SetValidator sets the current validator object
	SetValidator(val Validator)
}

// blockchain implements the Blockchain interface
type blockchain struct {
	service.CompositeService
	mu        sync.RWMutex // mutex to protect utk, tipHeight and tipHash
	dao       *blockDAO
	config    *config.Config
	genesis   *Genesis
	chainID   uint32
	tipHeight uint64
	tipHash   common.Hash32B
	validator Validator

	// used by account-based model
	sf state.Factory
}

// Option sets blockchain construction parameter
type Option func(*blockchain, *config.Config) error

// DefaultStateFactoryOption sets blockchain's sf from config
func DefaultStateFactoryOption() Option {
	return func(bc *blockchain, cfg *config.Config) error {
		sf, err := state.NewFactory(cfg, state.DefaultTrieOption())
		if err != nil {
			return errors.Wrapf(err, "Failed to create state factory")
		}
		bc.sf = sf

		return nil
	}
}

// PrecreatedStateFactoryOption sets blockchain's state.Factory to sf
func PrecreatedStateFactoryOption(sf state.Factory) Option {
	return func(bc *blockchain, conf *config.Config) error {
		bc.sf = sf

		return nil
	}
}

// InMemStateFactoryOption sets blockchain's state.Factory as in memory sf
func InMemStateFactoryOption() Option {
	return func(bc *blockchain, cfg *config.Config) error {
		sf, err := state.NewFactory(cfg, state.InMemTrieOption())
		if err != nil {
			return errors.Wrapf(err, "Failed to create state factory")
		}
		bc.sf = sf

		return nil
	}
}

// PrecreatedDaoOption sets blockchain's dao
func PrecreatedDaoOption(dao *blockDAO) Option {
	return func(bc *blockchain, conf *config.Config) error {
		bc.dao = dao

		return nil
	}
}

// BoltDBDaoOption sets blockchain's dao with BoltDB from config.Chain.ChainDBPath
func BoltDBDaoOption() Option {
	return func(bc *blockchain, cfg *config.Config) error {
		bc.dao = newBlockDAO(db.NewBoltDB(cfg.Chain.ChainDBPath, nil))

		return nil
	}
}

// InMemDaoOption sets blockchain's dao with MemKVStore
func InMemDaoOption() Option {
	return func(bc *blockchain, cfg *config.Config) error {
		bc.dao = newBlockDAO(db.NewMemKVStore())

		return nil
	}
}

// NewBlockchain creates a new blockchain and DB instance
func NewBlockchain(cfg *config.Config, opts ...Option) Blockchain {
	// create the Blockchain
	chain := &blockchain{
		config:  cfg,
		genesis: Gen,
	}
	for _, opt := range opts {
		if err := opt(chain, cfg); err != nil {
			logger.Error().Err(err).Msgf("Failed to create blockchain option %s", opt)
			return nil
		}
	}
	chain.initValidator()
	chain.addDaoService()
	if err := chain.initStateFactory(); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize state.Factory")
		return nil
	}
	if err := chain.Init(); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize blockchain")
		return nil
	}
	if err := chain.Start(); err != nil {
		logger.Error().Err(err).Msg("Failed to start blockchain")
		return nil
	}

	height, err := chain.TipHeight()
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get blockchain height")
		return nil
	}
	if height > 0 {
		return chain
	}
	genesis := NewGenesisBlock(cfg)
	if genesis == nil {
		logger.Error().Msg("Cannot create genesis block.")
		return nil
	}
	// Genesis block has height 0
	if genesis.Header.height != 0 {
		logger.Error().
			Uint64("Genesis block has height", genesis.Height()).
			Msg("Expecting 0")
		return nil
	}
	// add Genesis block as very first block
	if err := chain.CommitBlock(genesis); err != nil {
		logger.Error().Err(err).Msg("Failed to commit Genesis block")
		return nil
	}
	return chain
}

func (bc *blockchain) addDaoService() {
	bc.AddService(bc.dao)
}

func (bc *blockchain) initValidator() {
	bc.validator = &validator{sf: bc.sf}
}

func (bc *blockchain) initStateFactory() error {
	sf := bc.sf
	if sf != nil {
		// add producer into Trie
		if _, err := sf.CreateState(Gen.CreatorAddr, Gen.TotalSupply); err != nil {
			logger.Error().Err(err).Msg("Failed to add Creator into StateFactory")
			return err
		}
	}

	return nil
}

// Start starts the blockchain
func (bc *blockchain) Start() (err error) {
	if err = bc.CompositeService.Start(); err != nil {
		return err
	}

	// get blockchain tip height
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.tipHeight, err = bc.dao.getBlockchainHeight(); err != nil {
		return err
	}
	if bc.tipHeight == 0 {
		return nil
	}
	// get blockchain tip hash
	if bc.tipHash, err = bc.dao.getBlockHash(bc.tipHeight); err != nil {
		return err
	}

	// populate state factory
	for i := uint64(0); i <= bc.tipHeight; i++ {
		blk, err := bc.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		if blk != nil {
			if bc.sf != nil && blk.Transfers != nil {
				if err := bc.sf.CommitStateChanges(bc.tipHeight, blk.Transfers, blk.Votes); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Balance returns balance of address
func (bc *blockchain) Balance(addr string) (*big.Int, error) {
	return bc.sf.Balance(addr)
}

// Nonce returns the nonce if the account exists
func (bc *blockchain) Nonce(addr string) (uint64, error) {
	return bc.sf.Nonce(addr)
}

// GetHeightByHash returns block's height by hash
func (bc *blockchain) GetHeightByHash(hash common.Hash32B) (uint64, error) {
	return bc.dao.getBlockHeight(hash)
}

// GetHashByHeight returns block's hash by height
func (bc *blockchain) GetHashByHeight(height uint64) (common.Hash32B, error) {
	return bc.dao.getBlockHash(height)
}

// GetBlockByHeight returns block from the blockchain hash by height
func (bc *blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	hash, err := bc.GetHashByHeight(height)
	if err != nil {
		return nil, err
	}
	return bc.GetBlockByHash(hash)
}

// GetBlockByHash returns block from the blockchain hash by hash
func (bc *blockchain) GetBlockByHash(hash common.Hash32B) (*Block, error) {
	return bc.dao.getBlock(hash)
}

func (bc *blockchain) GetTotalTransfers() (uint64, error) {
	totalTransfers, err := bc.dao.getTotalTransfers()
	if err != nil {
		return uint64(0), err
	}

	return totalTransfers, nil
}

func (bc *blockchain) GetTotalVotes() (uint64, error) {
	totalVotes, err := bc.dao.getTotalVotes()
	if err != nil {
		return uint64(0), err
	}

	return totalVotes, nil
}

func (bc *blockchain) GetTransfersFromAddress(address string) ([]common.Hash32B, error) {
	transfersFromAddress, err := bc.dao.getTransfersBySenderAddress(address)
	if err != nil {
		return nil, err
	}

	return transfersFromAddress, nil
}

func (bc *blockchain) GetTransfersToAddress(address string) ([]common.Hash32B, error) {
	transfersToAddress, err := bc.dao.getTransfersByRecipientAddress(address)
	if err != nil {
		return nil, err
	}

	return transfersToAddress, nil
}

// GetTransferByTransferHash returns transfer by Transfer hash
func (bc *blockchain) GetTransferByTransferHash(hash common.Hash32B) (*action.Transfer, error) {
	blkHash, err := bc.dao.getBlockHashByTransferHash(hash)
	if err != nil {
		return nil, err
	}
	blk, err := bc.dao.getBlock(blkHash)
	if err != nil {
		return nil, err
	}
	for _, transfer := range blk.Transfers {
		if transfer.Hash() == hash {
			return transfer, nil
		}
	}
	return nil, errors.Errorf("block %x does not have transfer %x", blkHash, hash)
}

// GetBlockHashByTxHash returns Block hash by transfer hash
func (bc *blockchain) GetBlockHashByTransferHash(hash common.Hash32B) (common.Hash32B, error) {
	return bc.dao.getBlockHashByTransferHash(hash)
}

// GetVoteFromAddress returns vote from address
func (bc *blockchain) GetVotesFromAddress(address string) ([]common.Hash32B, error) {
	return bc.dao.getVotesBySenderAddress(address)
}

// GetVoteToAddress returns vote to address
func (bc *blockchain) GetVotesToAddress(address string) ([]common.Hash32B, error) {
	return bc.dao.getVotesByRecipientAddress(address)
}

// GetVotesByVoteHash returns vote by vote hash
func (bc *blockchain) GetVoteByVoteHash(hash common.Hash32B) (*action.Vote, error) {
	blkHash, err := bc.dao.getBlockHashByVoteHash(hash)
	if err != nil {
		return nil, err
	}
	blk, err := bc.dao.getBlock(blkHash)
	if err != nil {
		return nil, err
	}
	for _, vote := range blk.Votes {
		if vote.Hash() == hash {
			return vote, nil
		}
	}
	return nil, errors.Errorf("block %x does not have vote %x", blkHash, hash)
}

// GetBlockHashByVoteHash returns Block hash by vote hash
func (bc *blockchain) GetBlockHashByVoteHash(hash common.Hash32B) (common.Hash32B, error) {
	return bc.dao.getBlockHashByVoteHash(hash)
}

// TipHash returns tip block's hash
func (bc *blockchain) TipHash() (common.Hash32B, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.tipHash, nil
}

// TipHeight returns tip block's height
func (bc *blockchain) TipHeight() (uint64, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	return bc.tipHeight, nil
}

// ValidateBlock validates a new block before adding it to the blockchain
func (bc *blockchain) ValidateBlock(blk *Block) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if bc.validator == nil {
		panic("no block validator")
	}

	return bc.validator.Validate(blk, bc.tipHeight, bc.tipHash)
}

// MintNewBlock creates a new block with given actions
// Note: the coinbase transfer will be added to the given transfers
// when minting a new block
func (bc *blockchain) MintNewBlock(tsf []*action.Transfer, vote []*action.Vote,
	producer *iotxaddress.Address, data string) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	tsf = append(tsf, action.NewCoinBaseTransfer(big.NewInt(int64(bc.genesis.BlockReward)), producer.RawAddress))

	blk := NewBlock(bc.chainID, bc.tipHeight+1, bc.tipHash, tsf, vote)
	if producer.PrivateKey == nil {
		logger.Warn().Msg("Unsigned block...")
		return blk, nil
	}

	blk.Header.Pubkey = producer.PublicKey

	blkHash := blk.HashBlock()
	blk.Header.blockSig = cp.Sign(producer.PrivateKey, blkHash[:])
	return blk, nil
}

// MintNewDummyBlock creates a new dummy block without any tx/action.
func (bc *blockchain) MintNewDummyBlock() (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	prevBlk, err := bc.GetBlockByHeight(bc.tipHeight)
	if err != nil {
		return nil, err
	}
	timestamp := prevBlk.Header.timestamp + 1

	blk := &Block{
		Header: &BlockHeader{
			version:       common.ProtocolVersion,
			chainID:       bc.chainID,
			height:        bc.tipHeight + 1,
			timestamp:     timestamp,
			prevBlockHash: bc.tipHash,
			txRoot:        common.ZeroHash32B,
			stateRoot:     common.ZeroHash32B,
			blockSig:      []byte{}},
	}

	return blk, nil
}

//  CommitBlock validates and appends a block to the chain
func (bc *blockchain) CommitBlock(blk *Block) error {
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	return bc.commitBlock(blk)
}

// StateByAddr returns the state of an address
func (bc *blockchain) StateByAddr(address string) (*state.State, error) {
	if bc.sf != nil {
		s, err := bc.sf.State(address)
		if err != nil {
			logger.Warn().Err(err).Str("Address", address)
			return nil, errors.New("account does not exist")
		}

		return s, nil
	}

	return nil, errors.New("state factory is nil")
}

// SetValidator sets the current validator object
func (bc *blockchain) SetValidator(val Validator) {
	bc.validator = val
}

// Validator gets the current validator object
func (bc *blockchain) Validator() Validator {
	return bc.validator
}

// CreateRawTransfer creates an unsigned transfer paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateRawTransfer(nonce uint64, from *iotxaddress.Address, amount *big.Int, to *iotxaddress.Address) *action.Transfer {
	return action.NewTransfer(nonce, amount, from.RawAddress, to.RawAddress)
}

// CreateRawVote creates an unsigned vote
func (bc *blockchain) CreateRawVote(nonce uint64, from *iotxaddress.Address, to *iotxaddress.Address) *action.Vote {
	return action.NewVote(nonce, from.PublicKey, to.PublicKey)
}

//======================================
// private functions
//=====================================
// commitBlock commits a block to the chain
func (bc *blockchain) commitBlock(blk *Block) error {
	if err := bc.dao.putBlock(blk); err != nil {
		return err
	}
	// update tip hash and height
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.tipHeight = blk.Header.height
	bc.tipHash = blk.HashBlock()

	// update state factory
	if bc.sf == nil || (blk.Transfers == nil && blk.Votes == nil) {
		return nil
	}
	return bc.sf.CommitStateChanges(bc.tipHeight, blk.Transfers, blk.Votes)
}
