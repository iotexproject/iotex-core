// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"database/sql"
	"encoding/hex"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	s "github.com/iotexproject/iotex-core/db/sql"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type (
	// BlockByIndex defines the base schema of "index to block" table
	BlockByIndex struct {
		NodeAddress string
		Index       []byte
		BlockHash   []byte
	}
	// IndexHistory defines the schema of "index history" table
	IndexHistory struct {
		NodeAddress string
		UserAddress string
		Index       string
	}
)

// Indexer handles the index build for blocks
type Indexer struct {
	cfg                config.Indexer
	store              s.Store
	hexEncodedNodeAddr string
}

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
)

// HandleBlock is an implementation of interface BlockCreationSubscriber
func (idx *Indexer) HandleBlock(blk *block.Block) error {
	return idx.BuildIndex(blk)
}

// BuildIndex builds the index for a block
func (idx *Indexer) BuildIndex(blk *block.Block) error {
	idx.store.Transact(func(tx *sql.Tx) error {
		// log transfer to transfer history table
		if err := idx.UpdateTransferHistory(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update transfer to transfer history table")
		}
		// map transfer to block
		if err := idx.UpdateTransferToBlock(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update transfer to block")
		}

		// log vote to vote history table
		if err := idx.UpdateVoteHistory(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update vote to vote history table")
		}
		// map vote to block
		if err := idx.UpdateVoteToBlock(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update vote to block")
		}

		// log execution to execution history table
		if err := idx.UpdateExecutionHistory(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update execution to execution history table")
		}
		// map execution to block
		if err := idx.UpdateExecutionToBlock(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update execution to block")
		}

		// log action to action history table
		if err := idx.UpdateActionHistory(blk, tx); err != nil {
			return errors.Wrapf(err, "failed to update action to action history table")
		}
		// map action to block
		if err := idx.UpdateActionToBlock(blk, tx); err != nil {
			return errors.Wrap(err, "failed to update action to block")
		}

		// map hash to receipt
		if err := idx.UpdateReceiptToBlock(blk, tx); err != nil {
			return errors.Wrap(err, "failed to update hash to receipt")
		}

		return nil
	})
	return nil
}

// UpdateTransferHistory stores transfer information into transfer history table
func (idx *Indexer) UpdateTransferHistory(blk *block.Block, tx *sql.Tx) error {
	insertQuery := "INSERT INTO transfer_history (node_address,user_address,transfer_hash) VALUES (?, ?, ?)"
	transfers, _, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range transfers {
		transferHash := transfer.Hash()

		// put new transfer for sender
		senderAddr := transfer.Sender()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, senderAddr, transferHash[:]); err != nil {
			return err
		}

		// put new transfer for recipient
		receiverAddr := transfer.Recipient()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, receiverAddr, transferHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetTransferHistory gets transfer history
func (idx *Indexer) GetTransferHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM transfer_history WHERE node_address=? AND user_address=?"
	transferHashes, err := idx.getIndexHistory(getQuery, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get transfer history")
	}
	return transferHashes, nil
}

// UpdateTransferToBlock maps transfer hash to block hash
func (idx *Indexer) UpdateTransferToBlock(blk *block.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT INTO transfer_to_block (node_address,transfer_hash,block_hash) VALUES (?, ?, ?)"
	transfers, _, _ := action.ClassifyActions(blk.Actions)
	for _, transfer := range transfers {
		transferHash := transfer.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(transferHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByTransfer returns block hash by transfer hash
func (idx *Indexer) GetBlockByTransfer(transferHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM transfer_to_block WHERE node_address=? AND transfer_hash=?"
	blkHash, err := idx.blockByIndex(getQuery, transferHash)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to get block hash by transfer hash")
	}
	return blkHash, nil
}

// UpdateVoteHistory stores vote information into vote history table
func (idx *Indexer) UpdateVoteHistory(blk *block.Block, tx *sql.Tx) error {
	insertQuery := "INSERT INTO vote_history (node_address,user_address,vote_hash) VALUES (?, ?, ?)"
	_, votes, _ := action.ClassifyActions(blk.Actions)
	for _, vote := range votes {
		voteHash := vote.Hash()

		// put new vote for sender
		senderAddr := vote.Voter()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, senderAddr, voteHash[:]); err != nil {
			return err
		}

		// put new vote for recipient
		recipientAddr := vote.Votee()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, recipientAddr, voteHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetVoteHistory gets vote history
func (idx *Indexer) GetVoteHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM vote_history WHERE node_address=? AND user_address=?"
	voteHashes, err := idx.getIndexHistory(getQuery, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get vote history")
	}
	return voteHashes, nil
}

// UpdateVoteToBlock maps vote hash to block hash
func (idx *Indexer) UpdateVoteToBlock(blk *block.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT INTO vote_to_block (node_address,vote_hash,block_hash) VALUES (?, ?, ?)"
	_, votes, _ := action.ClassifyActions(blk.Actions)
	for _, vote := range votes {
		voteHash := vote.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(voteHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByVote returns block hash by vote hash
func (idx *Indexer) GetBlockByVote(voteHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM vote_to_block WHERE node_address=? AND vote_hash=?"
	blkHash, err := idx.blockByIndex(getQuery, voteHash)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to get block hash by vote hash")
	}
	return blkHash, nil
}

// UpdateExecutionHistory stores execution information into execution history table
func (idx *Indexer) UpdateExecutionHistory(blk *block.Block, tx *sql.Tx) error {
	insertQuery := "INSERT INTO execution_history (node_address,user_address,execution_hash) VALUES (?, ?, ?)"
	_, _, executions := action.ClassifyActions(blk.Actions)
	for _, execution := range executions {
		executionHash := execution.Hash()

		// put new execution for executor
		executorAddr := execution.Executor()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, executorAddr, executionHash[:]); err != nil {
			return err
		}

		// put new execution for contract
		contractAddr := execution.Contract()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, contractAddr, executionHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetExecutionHistory gets execution history
func (idx *Indexer) GetExecutionHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM execution_history WHERE node_address=? AND user_address=?"
	executionHashes, err := idx.getIndexHistory(getQuery, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get execution history")
	}
	return executionHashes, nil
}

// UpdateExecutionToBlock maps execution hash to block hash
func (idx *Indexer) UpdateExecutionToBlock(blk *block.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT INTO execution_to_block (node_address,execution_hash,block_hash) VALUES (?, ?, ?)"
	_, _, executions := action.ClassifyActions(blk.Actions)
	for _, execution := range executions {
		executionHash := execution.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(executionHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByExecution returns block hash by execution hash
func (idx *Indexer) GetBlockByExecution(executionHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM execution_to_block WHERE node_address=? AND execution_hash=?"
	blkHash, err := idx.blockByIndex(getQuery, executionHash)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to get block hash by execution hash")
	}
	return blkHash, nil
}

// UpdateActionHistory stores action information into action history table
func (idx *Indexer) UpdateActionHistory(blk *block.Block, tx *sql.Tx) error {
	insertQuery := "INSERT INTO action_history (node_address,user_address,action_hash) VALUES (?, ?, ?)"
	for _, selp := range blk.Actions {
		actionHash := selp.Hash()

		// put new action for sender
		senderAddr := selp.SrcAddr()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, senderAddr, actionHash[:]); err != nil {
			return err
		}

		// put new transfer for recipient
		receiverAddr := selp.DstAddr()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, receiverAddr, actionHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetActionHistory gets action history
func (idx *Indexer) GetActionHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM action_history WHERE node_address=? AND user_address=?"
	actionHashes, err := idx.getIndexHistory(getQuery, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get action history")
	}
	return actionHashes, nil
}

// UpdateActionToBlock maps action hash to block hash
func (idx *Indexer) UpdateActionToBlock(blk *block.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT INTO action_to_block (node_address,action_hash,block_hash) VALUES (?, ?, ?)"
	for _, selp := range blk.Actions {
		actionHash := selp.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(actionHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByAction returns block hash by action hash
func (idx *Indexer) GetBlockByAction(actionHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM action_to_block WHERE node_address=? AND action_hash=?"
	blkHash, err := idx.blockByIndex(getQuery, actionHash)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to get block hash by action hash")
	}
	return blkHash, nil
}

// UpdateReceiptToBlock maps receipt hash to block
func (idx *Indexer) UpdateReceiptToBlock(blk *block.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT INTO receipt_to_block (node_address,receipt_hash,block_hash) VALUES (?, ?, ?)"
	for hash := range blk.Receipts {
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(hash[:]),
			blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByReceipt returns block by receipt hash
func (idx *Indexer) GetBlockByReceipt(receiptHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM receipt_to_block WHERE node_address=? AND receipt_hash=?"
	blkHash, err := idx.blockByIndex(getQuery, receiptHash)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to get block hash by receipt hash")
	}
	return blkHash, nil
}

// blockByIndex returns block by receipt hash
func (idx *Indexer) blockByIndex(getQuery string, actionHash hash.Hash32B) (hash.Hash32B, error) {
	db := idx.store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, hex.EncodeToString(actionHash[:]))
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to execute get query")
	}

	var blockByIndex BlockByIndex
	parsedRows, err := s.ParseSQLRows(rows, &blockByIndex)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash32B, ErrNotExist
	}

	var hash hash.Hash32B
	copy(hash[:], parsedRows[0].(*BlockByIndex).BlockHash)
	return hash, nil
}

// getIndexHistory gets index history
func (idx *Indexer) getIndexHistory(getQuery string, userAddr string) ([]hash.Hash32B, error) {
	db := idx.store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var indexHistory IndexHistory
	parsedRows, err := s.ParseSQLRows(rows, &indexHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	var indexHashes []hash.Hash32B
	for _, parsedRow := range parsedRows {
		var hash hash.Hash32B
		copy(hash[:], parsedRow.(*IndexHistory).Index)
		indexHashes = append(indexHashes, hash)
	}
	return indexHashes, nil
}

// CreateTablesIfNotExist creates tables in local database
func (idx *Indexer) CreateTablesIfNotExist() error {
	// create action tables
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS action_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [action_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS action_to_block ([node_address] TEXT NOT NULL, [action_hash] " +
		"BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}

	// create transfer tables
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS transfer_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [transfer_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS transfer_to_block ([node_address] TEXT NOT NULL, [transfer_hash] " +
		"BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}

	// create vote tables
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS vote_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [vote_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS vote_to_block ([node_address] TEXT NOT NULL, [vote_hash] " +
		"BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}

	// create execution tables
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS execution_history ([node_address] TEXT NOT NULL, [user_address] " +
		"TEXT NOT NULL, [execution_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS execution_to_block ([node_address] TEXT NOT NULL, [execution_hash] " +
		"BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}

	// create receipt index
	if _, err := idx.store.GetDB().Exec("CREATE TABLE IF NOT EXISTS receipt_to_block ([node_address] TEXT NOT NULL, [receipt_hash] " +
		"BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)"); err != nil {
		return err
	}

	return nil
}
