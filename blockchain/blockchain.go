// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math"
	"math/big"
	"sort"
	"sync"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/common/service"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/txvm"
)

const (
	// GenesisCoinbaseData is the text in genesis block
	GenesisCoinbaseData = "The Times 03/Jan/2009 Chancellor on brink of second bailout for banks"
)

var (
	// ErrInvalidBlock is the error returned when the block is not valid
	ErrInvalidBlock = errors.New("failed to validate the block")
)

// Blockchain represents the blockchain data structure and hosts the APIs to access it
type Blockchain interface {
	service.Service
	// GetHeightByHash returns block's height by hash
	GetHeightByHash(hash common.Hash32B) (uint64, error)
	// GetHashByHeight returns block's hash by height
	GetHashByHeight(height uint64) (common.Hash32B, error)
	// GetBlockByHeight returns block from the blockchain hash by height
	GetBlockByHeight(height uint64) (*Block, error)
	// GetBlockByHash returns block from the blockchain hash by hash
	GetBlockByHash(hash common.Hash32B) (*Block, error)
	// TipHash returns tip block's hash
	TipHash() (common.Hash32B, error)
	// TipHeight returns tip block's height
	TipHeight() (uint64, error)
	// Reset reset for next block
	Reset()
	// ValidateBlock validates a new block before adding it to the blockchain
	ValidateBlock(blk *Block) error
	// MintNewBlock creates a new block with given transactions.
	// Note: the coinbase transaction will be added to the given transactions
	// when minting a new block.
	MintNewBlock([]*Tx, iotxaddress.Address, string) (*Block, error)
	// AddBlockCommit adds a new block into blockchain
	AddBlockCommit(blk *Block) error
	// AddBlockSync adds a past block into blockchain
	// used by block syncer when the chain in out-of-sync
	AddBlockSync(blk *Block) error
	// BalanceOf returns the balance of a given address
	BalanceOf(string) *big.Int
	// UtxoPool returns the UTXO pool of current blockchain
	UtxoPool() map[common.Hash32B][]*TxOutput
	// CreateTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
	CreateTransaction(from iotxaddress.Address, amount uint64, to []*Payee) *Tx
	// CreateRawTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
	CreateRawTransaction(from iotxaddress.Address, amount uint64, to []*Payee) *Tx
}

// blockchain implements the Blockchain interface
type blockchain struct {
	service.CompositeService
	mu        sync.RWMutex
	dao       *blockDAO
	config    *config.Config
	genesis   *Genesis
	Utk       *UtxoTracker // tracks the current UTXO pool
	chainID   uint32
	tipHeight uint64
	tipHash   common.Hash32B
}

// NewBlockchain creates a new blockchain instance
func NewBlockchain(dao *blockDAO, cfg *config.Config, gen *Genesis) Blockchain {
	chain := &blockchain{
		dao:     dao,
		config:  cfg,
		genesis: gen,
		Utk:     NewUtxoTracker()}
	chain.AddService(dao)
	return chain
}

// Start starts the blockchain
func (bc *blockchain) Start() error {
	err := bc.CompositeService.Start()
	if err != nil {
		return err
	}
	// get blockchain tip height
	bc.mu.Lock()
	if bc.tipHeight, err = bc.dao.getBlockchainHeight(); err != nil {
		bc.mu.Unlock()
		return err
	}
	// get blockchain tip hash
	if bc.tipHash, err = bc.dao.getBlockHash(bc.tipHeight); err != nil {
		bc.mu.Unlock()
		return err
	}
	bc.mu.Unlock()
	// build UTXO pool
	// Genesis block has height 0
	for i := uint64(0); i <= bc.tipHeight; i++ {
		blk, err := bc.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		if blk != nil {
			bc.Utk.UpdateUtxoPool(blk)
		}
	}
	return nil
}

// commitBlock commits Block to Db
func (bc *blockchain) commitBlock(blk *Block) error {
	if err := bc.dao.putBlock(blk); err != nil {
		return err
	}
	// update tip hash and height
	bc.mu.Lock()
	bc.tipHeight = blk.Header.height
	bc.tipHash = blk.HashBlock()
	bc.mu.Unlock()
	// update UTXO pool
	bc.Utk.UpdateUtxoPool(blk)
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

// Reset reset for next block
func (bc *blockchain) Reset() {
	bc.Utk.Reset()
}

// ValidateBlock validates a new block before adding it to the blockchain
func (bc *blockchain) ValidateBlock(blk *Block) error {
	if blk == nil {
		return errors.Wrap(ErrInvalidBlock, "Block is nil")
	}

	bc.mu.RLock()
	// verify new block has height incremented by 1
	if blk.Header.height != 0 && blk.Header.height != bc.tipHeight+1 {
		bc.mu.RUnlock()
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong block height %d, expecting %d",
			blk.Header.height,
			bc.tipHeight+1)
	}

	// verify new block has correctly linked to current tip
	if blk.Header.prevBlockHash != bc.tipHash {
		bc.mu.RUnlock()
		return errors.Wrapf(
			ErrInvalidBlock,
			"Wrong prev hash %x, expecting %x",
			blk.Header.prevBlockHash,
			bc.tipHash)
	}
	bc.mu.RUnlock()
	// validate all Tx conforms to blockchain protocol

	// validate UXTO contained in this Tx
	return bc.Utk.ValidateUtxo(blk)
}

// MintNewBlock creates a new block with given transactions.
// Note: the coinbase transaction will be added to the given transactions
// when minting a new block.
func (bc *blockchain) MintNewBlock(txs []*Tx, producer iotxaddress.Address, data string) (*Block, error) {
	cbTx := NewCoinbaseTx(producer.RawAddress, bc.genesis.BlockReward, data)
	if cbTx == nil {
		errMsg := "Cannot create coinbase transaction"
		glog.Error(errMsg)
		return nil, errors.Errorf(errMsg)
	}

	txs = append(txs, cbTx)
	bc.mu.RLock()
	blk := NewBlock(bc.chainID, bc.tipHeight+1, bc.tipHash, txs)
	bc.mu.RUnlock()
	if producer.PrivateKey == nil {
		glog.Warning("Unsigned block...")
		return blk, nil
	}

	blkHash := blk.HashBlock()
	blk.Header.blockSig = cp.Sign(producer.PrivateKey, blkHash[:])
	return blk, nil
}

// AddBlockCommit adds a new block into blockchain
func (bc *blockchain) AddBlockCommit(blk *Block) error {
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}

	// commit block into blockchain DB
	return bc.commitBlock(blk)
}

// AddBlockSync adds a past block into blockchain
// used by block syncer when the chain in out-of-sync
func (bc *blockchain) AddBlockSync(blk *Block) error {
	// directly commit block into blockchain DB
	return bc.commitBlock(blk)
}

// CreateBlockchain creates a new blockchain and DB instance
func CreateBlockchain(address string, cfg *config.Config, gen *Genesis) Blockchain {
	dao := newBlockDAO(db.NewBoltDB(cfg.Chain.ChainDBPath, nil))

	chain := NewBlockchain(dao, cfg, gen)
	if err := chain.Init(); err != nil {
		glog.Errorf("Failed to initialize blockchain, error = %v", err)
		return nil
	}
	if err := chain.Start(); err != nil {
		glog.Errorf("Failed to start blockchain, error = %v", err)
		return nil
	}

	height, err := chain.TipHeight()
	if err != nil {
		glog.Errorf("Failed to get blockchain height, error = %v", err)
	}
	if height == 0 {
		if gen == nil {
			glog.Error("Genesis should not be nil.")
			return nil
		}
		genesis := NewGenesisBlock(gen)
		if genesis == nil {
			glog.Error("Cannot create genesis block.")
			return nil
		}

		// Genesis block has height 0
		if genesis.Header.height != 0 {
			glog.Errorf("Genesis block has height = %d, expecting 0", genesis.Height())
			return nil
		}

		// add Genesis block as very first block
		if err := chain.AddBlockCommit(genesis); err != nil {
			glog.Error(err)
			return nil
		}
	}
	return chain
}

// BalanceOf returns the balance of an address
func (bc *blockchain) BalanceOf(address string) *big.Int {
	_, balance := bc.Utk.UtxoEntries(address, math.MaxUint64)
	return balance
}

// UtxoPool returns the UTXO pool of current blockchain
func (bc *blockchain) UtxoPool() map[common.Hash32B][]*TxOutput {
	return bc.Utk.utxoPool
}

// createTx creates a transaction paying 'amount' from 'from' to 'to'
func (bc *blockchain) createTx(from iotxaddress.Address, amount uint64, to []*Payee, isRaw bool) *Tx {
	utxo, change := bc.Utk.UtxoEntries(from.RawAddress, amount)
	if utxo == nil {
		glog.Errorf("Fail to get UTXO for %v", from.RawAddress)
		return nil
	}

	in := []*TxInput{}
	for _, out := range utxo {
		unlock := []byte(out.TxOutputPb.String())
		if !isRaw {
			var err error
			unlock, err = txvm.SignatureScript([]byte(out.TxOutputPb.String()), from.PublicKey, from.PrivateKey)
			if err != nil {
				return nil
			}
		}

		in = append(in, bc.Utk.CreateTxInputUtxo(out.txHash, out.outIndex, unlock))
	}

	out := []*TxOutput{}
	for _, payee := range to {
		out = append(out, bc.Utk.CreateTxOutputUtxo(payee.Address, payee.Amount))
	}
	if change.Sign() == 1 {
		out = append(out, bc.Utk.CreateTxOutputUtxo(from.RawAddress, change.Uint64()))
	}

	// Sort TxInput in lexicographical order based on TxHash + OutIndex
	sort.Sort(txInSorter(in))

	// Sort TxOutput in lexicographical order based on Value + LockScript and reset OutIndex
	sort.Sort(txOutSorter(out))
	resetOutIndex(out)

	return NewTx(1, in, out, 0)
}

// CreateTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateTransaction(from iotxaddress.Address, amount uint64, to []*Payee) *Tx {
	return bc.createTx(from, amount, to, false)
}

// CreateRawTransaction creates a unsigned transaction paying 'amount' from 'from' to 'to'
func (bc *blockchain) CreateRawTransaction(from iotxaddress.Address, amount uint64, to []*Payee) *Tx {
	return bc.createTx(from, amount, to, true)
}

func resetOutIndex(out []*TxOutput) {
	for i := 0; i < len(out); i++ {
		out[i].outIndex = int32(i)
	}
}
