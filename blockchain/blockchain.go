// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"math"
	"os"
	"sort"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockdb"
	cm "github.com/iotexproject/iotex-core/common"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
	"github.com/iotexproject/iotex-core/proto"
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

type Blockchain interface {
	// Init initializes the blockchain
	Init() error
	// Close closes the Db connection
	Close() error
	// GetHeightByHash returns block's height by hash
	GetHeightByHash(hash cp.Hash32B) (uint32, error)
	// GetHashByHeight returns block's hash by height
	GetHashByHeight(height uint32) (cp.Hash32B, error)
	// GetBlockByHeight returns block from the blockchain hash by height
	GetBlockByHeight(height uint32) (*Block, error)
	// GetBlockByHash returns block from the blockchain hash by hash
	GetBlockByHash(hash cp.Hash32B) (*Block, error)
	// TipHash returns tip block's hash
	TipHash() cp.Hash32B
	// TipHeight returns tip block's height
	TipHeight() uint32
	// Reset reset for next block
	Reset()
	// ValidateBlock validates a new block before adding it to the blockchain
	ValidateBlock(blk *Block) error
	// MintNewBlock creates a new block with given transactions.
	// Note: the coinbase transaction will be added to the given transactions
	// when minting a new block.
	MintNewBlock([]*Tx, iotxaddress.Address, string) *Block
	// AddBlockCommit adds a new block into blockchain
	AddBlockCommit(blk *Block) error
	// AddBlockSync adds a past block into blockchain
	// used by block syncer when the chain in out-of-sync
	AddBlockSync(blk *Block) error
	// BalanceOf returns the balance of a given address
	BalanceOf(string) uint64
	// UtxoPool returns the UTXO pool of current blockchain
	UtxoPool() map[cp.Hash32B][]*TxOutput
	// CreateTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
	CreateTransaction(from iotxaddress.Address, amount uint64, to []*Payee) *Tx
	// CreateRawTransaction creates a signed transaction paying 'amount' from 'from' to 'to'
	CreateRawTransaction(from iotxaddress.Address, amount uint64, to []*Payee) *Tx
}

// blockchain implements the Blockchain interface
type blockchain struct {
	blockDb *blockdb.BlockDB
	config  *config.Config
	genesis *Genesis
	chainID uint32
	height  uint32
	tip     cp.Hash32B
	Utk     *UtxoTracker // tracks the current UTXO pool
}

// NewBlockchain creates a new blockchain instance
func NewBlockchain(db *blockdb.BlockDB, cfg *config.Config, gen *Genesis) *blockchain {
	chain := &blockchain{
		blockDb: db,
		config:  cfg,
		genesis: gen,
		Utk:     NewUtxoTracker()}
	return chain
}

// Init initializes the blockchain
func (bc *blockchain) Init() error {
	tip, height, err := bc.blockDb.Init()
	if err != nil {
		return err
	}

	copy(bc.tip[:], tip)
	bc.height = height

	// build UTXO pool
	// Genesis block has height 0
	for i := uint32(0); i <= bc.height; i++ {
		blk, err := bc.GetBlockByHeight(i)
		if err != nil {
			return err
		}
		bc.Utk.UpdateUtxoPool(blk)
	}
	return nil
}

// Close closes the Db connection
func (bc *blockchain) Close() error {
	return bc.blockDb.Close()
}

// commitBlock commits Block to Db
func (bc *blockchain) commitBlock(blk *Block) (err error) {
	// post-commit actions
	defer func() {
		// update tip hash and height
		if r := recover(); r != nil {
			return
		}

		// update tip hash/height
		bc.tip = blk.HashBlock()
		bc.height = blk.Header.height

		// update UTXO pool
		bc.Utk.UpdateUtxoPool(blk)
	}()

	// serialize the block
	serialized, err := blk.Serialize()
	if err != nil {
		panic(err)
	}

	hash := blk.HashBlock()
	if err = bc.blockDb.CheckInBlock(serialized, hash[:], blk.Header.height); err != nil {
		panic(err)
	}
	return
}

// GetHeightByHash returns block's height by hash
func (bc *blockchain) GetHeightByHash(hash cp.Hash32B) (uint32, error) {
	return bc.blockDb.GetBlockHeight(hash[:])
}

// GetHashByHeight returns block's hash by height
func (bc *blockchain) GetHashByHeight(height uint32) (cp.Hash32B, error) {
	hash := cp.ZeroHash32B
	dbHash, err := bc.blockDb.GetBlockHash(height)
	copy(hash[:], dbHash)
	return hash, err
}

// GetBlockByHeight returns block from the blockchain hash by height
func (bc *blockchain) GetBlockByHeight(height uint32) (*Block, error) {
	hash, err := bc.GetHashByHeight(height)
	if err != nil {
		return nil, err
	}
	return bc.GetBlockByHash(hash)
}

// GetBlockByHash returns block from the blockchain hash by hash
func (bc *blockchain) GetBlockByHash(hash cp.Hash32B) (*Block, error) {
	serialized, err := bc.blockDb.CheckOutBlock(hash[:])
	if err != nil {
		return nil, err
	}

	// deserialize the block
	blk := Block{}
	if err := blk.Deserialize(serialized); err != nil {
		return nil, err
	}
	return &blk, nil
}

// TipHash returns tip block's hash
func (bc *blockchain) TipHash() cp.Hash32B {
	return bc.tip
}

// TipHeight returns tip block's height
func (bc *blockchain) TipHeight() uint32 {
	return bc.height
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
	// verify new block has correctly linked to current tip
	if blk.Header.prevBlockHash != bc.tip {
		return errors.Wrapf(ErrInvalidBlock, "Wrong prev hash %x, expecting %x", blk.Header.prevBlockHash, bc.tip)
	}

	// verify new block has height incremented by 1
	if blk.Header.height != 0 && blk.Header.height != bc.height+1 {
		return errors.Wrapf(ErrInvalidBlock, "Wrong block height %d, expecting %d", blk.Header.height, bc.height+1)
	}

	// validate all Tx conforms to blockchain protocol

	// validate UXTO contained in this Tx
	return bc.Utk.ValidateUtxo(blk)
}

// MintNewBlock creates a new block with given transactions.
// Note: the coinbase transaction will be added to the given transactions
// when minting a new block.
func (bc *blockchain) MintNewBlock(txs []*Tx, toaddr iotxaddress.Address, data string) *Block {
	cbTx := NewCoinbaseTx(toaddr.RawAddress, bc.genesis.BlockReward, data)
	if cbTx == nil {
		glog.Error("Cannot create coinbase transaction")
		return nil
	}

	txs = append(txs, cbTx)
	blk := NewBlock(bc.chainID, bc.height+1, bc.tip, txs)
	if producer.PrivateKey == nil {
		glog.Warning("Unsigned block...")
		return blk
	}

	blkHash := blk.HashBlock()
	blk.Header.blockSig = cp.Sign(producer.PrivateKey, blkHash[:])
	return blk
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

// StoreBlock persists the blocks in the range to file on disk
func (bc *blockchain) StoreBlock(start, end uint32) error {
	return bc.blockDb.StoreBlockToFile(start, end)
}

// ReadBlock read the block from file on disk
func (bc *blockchain) ReadBlock(height uint32) *Block {
	file, err := os.Open(blockdb.BlockData)
	defer file.Close()
	if err != nil {
		glog.Error(err)
		return nil
	}

	// read block index
	indexSize := make([]byte, 4)
	file.Read(indexSize)
	size := cm.MachineEndian.Uint32(indexSize)
	indexBytes := make([]byte, size)
	if n, err := file.Read(indexBytes); err != nil || n != int(size) {
		glog.Error(err)
		return nil
	}
	blkIndex := iproto.BlockIndex{}
	if proto.Unmarshal(indexBytes, &blkIndex) != nil {
		glog.Error(err)
		return nil
	}

	// read the specific block
	index := height - blkIndex.Start
	file.Seek(int64(4+size+blkIndex.Offset[index]), 0)
	size = blkIndex.Offset[index+1] - blkIndex.Offset[index]
	blkBytes := make([]byte, size)
	if n, err := file.Read(blkBytes); err != nil || n != int(size) {
		glog.Error(err)
		return nil
	}
	blk := Block{}
	if blk.Deserialize(blkBytes) != nil {
		glog.Error(err)
		return nil
	}
	return &blk
}

// CreateBlockchain creates a new blockchain and DB instance
func CreateBlockchain(address string, cfg *config.Config, gen *Genesis) *blockchain {
	db, dbFileExist := blockdb.NewBlockDB(cfg)
	if db == nil {
		glog.Error("cannot find db")
		return nil
	}

	chain := NewBlockchain(db, cfg, gen)
	if dbFileExist {
		glog.Info("Blockchain already exists.")

		if err := chain.Init(); err != nil {
			glog.Errorf("Failed to create Blockchain, error = %v", err)
			return nil
		}
		return chain
	}

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
	return chain
}

// BalanceOf returns the balance of an address
func (bc *blockchain) BalanceOf(address string) uint64 {
	_, balance := bc.Utk.UtxoEntries(address, math.MaxUint64)
	return balance
}

// UtxoPool returns the UTXO pool of current blockchain
func (bc *blockchain) UtxoPool() map[cp.Hash32B][]*TxOutput {
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
	if change > 0 {
		out = append(out, bc.Utk.CreateTxOutputUtxo(from.RawAddress, change))
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
