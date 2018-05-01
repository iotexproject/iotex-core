// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/iotxaddress"
)

// IBlockchain defines the interface of blockchain
type IBlockchain interface {
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
	MintNewBlock([]*Tx, string, string, []byte) *Block
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
