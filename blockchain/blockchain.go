// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"encoding/hex"
	"math"
	"math/big"
	"sort"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/action"
	"github.com/iotexproject/iotex-core/blockchain/trx"
	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/logger"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/trie"
	"github.com/iotexproject/iotex-core/txvm"
)

// Blockchain represents the blockchain data structure and hosts the APIs to access it
type Blockchain interface {
	service.Service
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
	// GetTransactionByAddress returns transaction from address
	GetTransfersFromAddress(address string) ([]common.Hash32B, error)
	// GetTransactionByAddress returns transaction to address
	GetTransfersToAddress(address string) ([]common.Hash32B, error)
	// GetTransfersByTxHash returns transaction by transfer hash
	GetTransferByTransferHash(hash common.Hash32B) (*action.Transfer, error)
	// GetBlockHashByTxHash returns Block hash by transfer hash
	GetBlockHashByTransferHash(hash common.Hash32B) (common.Hash32B, error)
	// TipHash returns tip block's hash
	TipHash() (common.Hash32B, error)
	// TipHeight returns tip block's height
	TipHeight() (uint64, error)
	// MintNewBlock creates a new block with given transactions
	// Note: the coinbase transaction will be added to the given transactions
	// when minting a new block.
	MintNewBlock([]*trx.Tx, []*action.Transfer, []*action.Vote, *iotxaddress.Address, string) (*Block, error)
	// MintNewDummyBlock creates a new dummy block with no transactions
	MintNewDummyBlock() (*Block, error)
	// AddBlockCommit adds a new block into blockchain
	AddBlockCommit(blk *Block) error
	// AddBlockSync adds a past block into blockchain
	// used by block syncer when the chain in out-of-sync
	AddBlockSync(blk *Block) error
	// BalanceOf returns the balance of an address
	BalanceOf(address string) *big.Int
	// StateByAddr returns state of a given address
	StateByAddr(string) (*state.State, error)
	// CreateTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
	CreateTransaction(from *iotxaddress.Address, amount uint64, to []*Payee) *trx.Tx
	// CreateRawTransaction creates an unsigned transaction paying 'amount' from 'from' to 'to'
	CreateRawTransaction(from *iotxaddress.Address, amount uint64, to []*Payee) *trx.Tx
	// CreateTransfer creates a signed transfer paying 'amount' from 'from' to 'to'
	CreateTransfer(nonce uint64, from *iotxaddress.Address, amount *big.Int, to *iotxaddress.Address) (*action.Transfer, error)
	// CreateRawTransfer creates an unsigned transfer paying 'amount' from 'from' to 'to'
	CreateRawTransfer(nonce uint64, from *iotxaddress.Address, amount *big.Int, to *iotxaddress.Address) *action.Transfer
	// CreateVote creates a signed vote
	CreateVote(nonce uint64, selfPubKey []byte, votePubKey []byte) (*action.Vote, error)
	// CreateRawVote creates an unsigned vote
	CreateRawVote(nonce uint64, selfPubKey []byte, votePubKey []byte) *action.Vote
	// ValidateBlock validates a new block before adding it to the blockchain
	ValidateBlock(blk *Block) error

	// The following methods are used only for utxo-based model
	// Reset resets UTXO
	ResetUTXO()
	// UtxoPool returns the UTXO pool of current blockchain
	UtxoPool() map[common.Hash32B][]*trx.TxOutput

	// Validator returns the current validator object
	Validator() Validator
	// SetValidator sets the current validator object
	SetValidator(Validator)
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

	// used by utxo-based model
	utk *UtxoTracker // tracks the current UTXO pool

	// used by account-based model
	sf state.Factory
}

// NewBlockchain creates a new blockchain instance
func NewBlockchain(dao *blockDAO, cfg *config.Config, sf state.Factory) Blockchain {
	if sf != nil {
		// add Genesis block miner into Trie
		if _, err := sf.CreateState(cfg.Chain.MinerAddr.RawAddress, Gen.TotalSupply); err != nil {
			logger.Error().Err(err).Msg("Failed to add miner into StateFactory")
			return nil
		}
		// add initial delegates into Trie
		for _, pk := range Gen.InitDelegatesPubKey {
			pubk, err := hex.DecodeString(pk)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to denoce public key")
				return nil
			}
			address, err := iotxaddress.GetAddress(pubk, iotxaddress.IsTestnet, iotxaddress.ChainID)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to get address from public key")
				return nil
			}
			if _, err := sf.CreateState(address.RawAddress, uint64(0)); err != nil {
				logger.Error().Err(err).Msg("Failed to add initial delegates state into StateFactory")
				return nil
			}
		}
	}

	utk := NewUtxoTracker()
	chain := &blockchain{
		dao:       dao,
		config:    cfg,
		genesis:   Gen,
		utk:       utk,
		sf:        sf,
		validator: &validator{sf: sf, utk: utk},
	}
	chain.AddService(dao)
	return chain
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

	// populate UTXO or state factory
	for i := uint64(0); i <= bc.tipHeight; i++ {
		blk, err := bc.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		if blk != nil {
			bc.utk.UpdateUtxoPool(blk)
			if bc.sf != nil && blk.Transfers != nil {
				if err := bc.sf.CommitStateChanges(bc.tipHeight, blk.Transfers, blk.Votes); err != nil {
					return err
				}
			}
		}
	}
	return nil
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

// ResetUTXO resets UTXO
func (bc *blockchain) ResetUTXO() {
	bc.utk.Reset()
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

// MintNewBlock creates a new block with given transactions.
// Note: the coinbase transaction will be added to the given transactions
// when minting a new block.
func (bc *blockchain) MintNewBlock(txs []*trx.Tx, tsf []*action.Transfer, vote []*action.Vote,
	producer *iotxaddress.Address, data string) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	tsf = append(tsf, action.NewCoinBaseTransfer(big.NewInt(int64(bc.genesis.BlockReward)), producer.RawAddress))

	blk := NewBlock(bc.chainID, bc.tipHeight+1, bc.tipHash, txs, tsf, vote)
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

// AddBlockCommit appends a new block into blockchain
func (bc *blockchain) AddBlockCommit(blk *Block) error {
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	return bc.commitBlock(blk)
}

// AddBlockSync adds a past block into blockchain
// used by block syncer when the chain in out-of-sync
func (bc *blockchain) AddBlockSync(blk *Block) error {
	// directly commit block into blockchain DB
	return bc.commitBlock(blk)
}

// CreateBlockchain creates a new blockchain and DB instance
func CreateBlockchain(cfg *config.Config, sf state.Factory) Blockchain {
	var kvStore db.KVStore
	if cfg.Chain.InMemTest {
		kvStore = db.NewMemKVStore()
		// If TrieDBPath is empty, we disable account-based testing
		if len(cfg.Chain.TrieDBPath) == 0 {
			sf = nil
		} else {
			trie, err := trie.NewTrie("", true)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to initialize in-memory trie")
				return nil
			}
			sf = state.NewFactory(trie)
		}
	} else {
		kvStore = db.NewBoltDB(cfg.Chain.ChainDBPath, nil)
	}
	return createAndInitBlockchain(kvStore, sf, cfg)
}

// TODO: Please deprecate this method when Utxo is deprecated
// BalanceOf returns the balance of an address
func (bc *blockchain) BalanceOf(address string) *big.Int {
	if bc.sf != nil {
		s, err := bc.StateByAddr(address)
		if err != nil {
			return big.NewInt(0)
		}
		return s.Balance
	}

	_, balance := bc.utk.UtxoEntries(address, math.MaxUint64)
	return balance
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

// UtxoPool returns the UTXO pool of current blockchain
func (bc *blockchain) UtxoPool() map[common.Hash32B][]*trx.TxOutput {
	return bc.utk.utxoPool
}

// CreateTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateTransaction(from *iotxaddress.Address, amount uint64, to []*Payee) *trx.Tx {
	return bc.createTx(from, amount, to, false)
}

// CreateRawTransaction creates a unsigned transaction paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateRawTransaction(from *iotxaddress.Address, amount uint64, to []*Payee) *trx.Tx {
	return bc.createTx(from, amount, to, true)
}

// SetValidator sets the current validator object
func (bc *blockchain) SetValidator(val Validator) {
	bc.validator = val
}

// Validator gets the current validator object
func (bc *blockchain) Validator() Validator {
	return bc.validator
}

// CreateTransfer creates a signed transfer paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateTransfer(nonce uint64, from *iotxaddress.Address, amount *big.Int, to *iotxaddress.Address) (*action.Transfer, error) {
	tsf := action.NewTransfer(nonce, amount, from.RawAddress, to.RawAddress)
	stsf, err := tsf.Sign(from)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to sign transfer")
		return nil, err
	}
	return stsf, nil
}

// CreateRawTransfer creates an unsigned transfer paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateRawTransfer(nonce uint64, from *iotxaddress.Address, amount *big.Int, to *iotxaddress.Address) *action.Transfer {
	return action.NewTransfer(nonce, amount, from.RawAddress, to.RawAddress)
}

// CreateVote creates a signed vote
func (bc *blockchain) CreateVote(nonce uint64, selfPubKey []byte, votePubKey []byte) (*action.Vote, error) {
	vote := action.NewVote(nonce, selfPubKey, votePubKey)
	from, err := iotxaddress.GetAddress(selfPubKey, iotxaddress.IsTestnet, iotxaddress.ChainID)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to get voter's address")
		return nil, errors.Wrapf(err, "invalid address")
	}
	sVote, err := vote.Sign(from)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to sign vote")
		return nil, err
	}
	return sVote, nil
}

// CreateRawVote creates an unsigned vote
func (bc *blockchain) CreateRawVote(nonce uint64, selfPubKey []byte, votePubKey []byte) *action.Vote {
	return action.NewVote(nonce, selfPubKey, votePubKey)
}

//======================================
// private functions
//=====================================
// commitBlock commits Block to DB
func (bc *blockchain) commitBlock(blk *Block) error {
	if err := bc.dao.putBlock(blk); err != nil {
		return err
	}
	// update tip hash and height
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.tipHeight = blk.Header.height
	bc.tipHash = blk.HashBlock()

	// update UTXO or state factory
	bc.utk.UpdateUtxoPool(blk)
	if bc.sf == nil || (blk.Transfers == nil && blk.Votes == nil) {
		return nil
	}
	return bc.sf.CommitStateChanges(bc.tipHeight, blk.Transfers, blk.Votes)
}

func createAndInitBlockchain(kvstore db.KVStore, sf state.Factory, cfg *config.Config) Blockchain {
	dao := newBlockDAO(kvstore)
	// create the Blockchain
	chain := NewBlockchain(dao, cfg, sf)
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
	genesis := NewGenesisBlock()
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
	if err := chain.AddBlockCommit(genesis); err != nil {
		logger.Error().Err(err).Msg("Failed to commit Genesis block")
		return nil
	}
	return chain
}

// createTx creates a transaction paying 'amount' from 'from' to 'to'
func (bc *blockchain) createTx(from *iotxaddress.Address, amount uint64, to []*Payee, isRaw bool) *trx.Tx {
	utxo, change := bc.utk.UtxoEntries(from.RawAddress, amount)
	if utxo == nil {
		logger.Error().Str("addr", from.RawAddress).Msg("Failed to get UTXO")
		return nil
	}

	in := []*trx.TxInput{}
	for _, out := range utxo {
		unlock := []byte(out.TxOutputPb.String())
		if !isRaw {
			var err error
			unlock, err = txvm.SignatureScript([]byte(out.TxOutputPb.String()), from.PublicKey, from.PrivateKey)
			if err != nil {
				return nil
			}
		}

		in = append(in, bc.utk.CreateTxInputUtxo(out.txHash, out.outIndex, unlock))
	}

	out := []*trx.TxOutput{}
	for _, payee := range to {
		out = append(out, bc.utk.CreateTxOutputUtxo(payee.Address, payee.Amount))
	}
	if change.Sign() == 1 {
		out = append(out, bc.utk.CreateTxOutputUtxo(from.RawAddress, change.Uint64()))
	}

	// Sort TxInput in lexicographical order based on TxHash + OutIndex
	sort.Sort(trx.TxInSorter(in))

	// Sort TxOutput in lexicographical order based on Value + LockScript and reset OutIndex
	sort.Sort(trx.TxOutSorter(out))
	resetOutIndex(out)

	return trx.NewTx(in, out, 0)
}

//======================================
// util functions
//=====================================
func resetOutIndex(out []*trx.TxOutput) {
	for i := 0; i < len(out); i++ {
		out[i].OutIndex = int32(i)
	}
}
