package indexservice

import (
	"database/sql"

	"github.com/pkg/errors"

	"encoding/hex"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/rds"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type TransferHistory struct {
	NodeAddress string
	UserAddress string
	TrasferHash string
}

type TransferToBlock struct {
	NodeAddress string
	TrasferHash string
	BlockHash   string
}

type VoteHistory struct {
	NodeAddress string
	UserAddress string
	VoteHash    string
}

type VoteToBlock struct {
	NodeAddress string
	VoteHash    string
	BlockHash   string
}

type ExecutionHistory struct {
	NodeAddress   string
	UserAddress   string
	ExecutionHash string
}

type ExecutionToBlock struct {
	NodeAddress   string
	ExecutionHash string
	BlockHash     string
}

// Indexer handle the index build for blocks
type Indexer struct {
	cfg                config.Indexer
	rds                rds.Store
	hexEncodedNodeAddr string
}

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
)

// BuildIndex build the index for a block
func (idx *Indexer) BuildIndex(blk *blockchain.Block) error {
	idx.rds.Transact(func(tx *sql.Tx) error {
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

		return nil
	})
	return nil
}

// UpdateTransferHistory stores transfer information into transfer history table
func (idx *Indexer) UpdateTransferHistory(blk *blockchain.Block, tx *sql.Tx) error {
	insertQuery := "INSERT transfer_history SET node_address=?,user_address=?,transfer_hash=?"
	for _, transfer := range blk.Transfers {
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

// GetTransferHistory get transfer history
func (idx *Indexer) GetTransferHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM transfer_history WHERE node_address=? AND user_address=?"
	db := idx.rds.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var transferHistory TransferHistory
	parsedRows, err := rds.ParseRows(rows, &transferHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	var transferHashes []hash.Hash32B
	for _, parsedRow := range parsedRows {
		var hash hash.Hash32B
		copy(hash[:], parsedRow.(*TransferHistory).TrasferHash)
		transferHashes = append(transferHashes, hash)
	}
	return transferHashes, nil
}

// UpdateTransferToBlock map transfer hash to block hash
func (idx *Indexer) UpdateTransferToBlock(blk *blockchain.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT transfer_to_block SET node_address=?,transfer_hash=?,block_hash=?"
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(transferHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByTransfer return block hash by transfer hash
func (idx *Indexer) GetBlockByTransfer(transferHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM transfer_to_block WHERE node_address=? AND transfer_hash=?"
	db := idx.rds.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, hex.EncodeToString(transferHash[:]))
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to execute get query")
	}

	var transferToBlock TransferToBlock
	parsedRows, err := rds.ParseRows(rows, &transferToBlock)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash32B, ErrNotExist
	}

	var hash hash.Hash32B
	copy(hash[:], parsedRows[0].(*TransferToBlock).BlockHash)
	return hash, nil
}

// UpdateVoteHistory stores vote information into vote history table
func (idx *Indexer) UpdateVoteHistory(blk *blockchain.Block, tx *sql.Tx) error {
	insertQuery := "INSERT vote_history SET node_address=?,user_address=?,vote_hash=?"
	for _, vote := range blk.Votes {
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

// GetVoteHistory get vote history
func (idx *Indexer) GetVoteHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM vote_history WHERE node_address=? AND user_address=?"
	db := idx.rds.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var voteHistory VoteHistory
	parsedRows, err := rds.ParseRows(rows, &voteHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	var voteHashes []hash.Hash32B
	for _, parsedRow := range parsedRows {
		var hash hash.Hash32B
		copy(hash[:], parsedRow.(*VoteHistory).VoteHash)
		voteHashes = append(voteHashes, hash)
	}
	return voteHashes, nil
}

// UpdateVoteToBlock map vote hash to block hash
func (idx *Indexer) UpdateVoteToBlock(blk *blockchain.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT vote_to_block SET node_address=?,vote_hash=?,block_hash=?"
	for _, vote := range blk.Votes {
		voteHash := vote.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(voteHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByVote return block hash by vote hash
func (idx *Indexer) GetBlockByVote(voteHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM vote_to_block WHERE node_address=? AND vote_hash=?"
	db := idx.rds.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, hex.EncodeToString(voteHash[:]))
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to execute get query")
	}

	var voteToBlock VoteToBlock
	parsedRows, err := rds.ParseRows(rows, &voteToBlock)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash32B, ErrNotExist
	}

	var hash hash.Hash32B
	copy(hash[:], parsedRows[0].(*VoteToBlock).BlockHash)
	return hash, nil
}

// UpdateExecutionHistory stores execution information into execution history table
func (idx *Indexer) UpdateExecutionHistory(blk *blockchain.Block, tx *sql.Tx) error {
	insertQuery := "INSERT execution_history SET node_address=?,user_address=?,execution_hash=?"
	for _, execution := range blk.Executions {
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

// GetExecutionHistory get execution history
func (idx *Indexer) GetExecutionHistory(userAddr string) ([]hash.Hash32B, error) {
	getQuery := "SELECT * FROM execution_history WHERE node_address=? AND user_address=?"
	db := idx.rds.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute get query")
	}

	var executionHistory ExecutionHistory
	parsedRows, err := rds.ParseRows(rows, &executionHistory)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse results")
	}

	var executionHashes []hash.Hash32B
	for _, parsedRow := range parsedRows {
		var hash hash.Hash32B
		copy(hash[:], parsedRow.(*ExecutionHistory).ExecutionHash)
		executionHashes = append(executionHashes, hash)
	}
	return executionHashes, nil
}

// UpdateExecutionToBlock map execution hash to block hash
func (idx *Indexer) UpdateExecutionToBlock(blk *blockchain.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT execution_to_block SET node_address=?,execution_hash=?,block_hash=?"
	for _, execution := range blk.Executions {
		executionHash := execution.Hash()
		if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(executionHash[:]), blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// GetBlockByExecution return block hash by execution hash
func (idx *Indexer) GetBlockByExecution(executionHash hash.Hash32B) (hash.Hash32B, error) {
	getQuery := "SELECT * FROM execution_to_block WHERE node_address=? AND execution_hash=?"
	db := idx.rds.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, hex.EncodeToString(executionHash[:]))
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to execute get query")
	}

	var executionToBlock ExecutionToBlock
	parsedRows, err := rds.ParseRows(rows, &executionToBlock)
	if err != nil {
		return hash.ZeroHash32B, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash32B, ErrNotExist
	}

	var hash hash.Hash32B
	copy(hash[:], parsedRows[0].(*ExecutionToBlock).BlockHash)
	return hash, nil
}
