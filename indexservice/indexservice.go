package indexservice

import (
	"database/sql"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/rds"
	"github.com/pkg/errors"
)

// IndexService handle the index build for blocks
type IndexService struct {
	cfg      config.IndexService
	rds      rds.Store
	nodeAddr string
}

// BuildIndex build the index for a block
func (idx *IndexService) BuildIndex(blk *blockchain.Block) error {
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
func (idx *IndexService) UpdateTransferHistory(blk *blockchain.Block, tx *sql.Tx) error {
	insertQuery := "INSERT transfer_history SET node_address=?,user_address=?,transfer_hash=?"
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()

		// put new transfer for sender
		senderAddr := transfer.Sender()
		if _, err := tx.Exec(insertQuery, idx.nodeAddr, senderAddr, transferHash[:]); err != nil {
			return err
		}

		// put new transfer for recipient
		receiverAddr := transfer.Recipient()
		if _, err := tx.Exec(insertQuery, idx.nodeAddr, receiverAddr, transferHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateTransferToBlock map transfer hash to block hash
func (idx *IndexService) UpdateTransferToBlock(blk *blockchain.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT transfer_to_block SET transfer_hash=?,block_hash=?"
	for _, transfer := range blk.Transfers {
		transferHash := transfer.Hash()
		if _, err := tx.Exec(insertQuery, transferHash[:], blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateVoteHistory stores vote information into vote history table
func (idx *IndexService) UpdateVoteHistory(blk *blockchain.Block, tx *sql.Tx) error {
	insertQuery := "INSERT vote_history SET node_address=?,user_address=?,vote_hash=?"
	for _, vote := range blk.Votes {
		voteHash := vote.Hash()

		// put new vote for sender
		senderAddr := vote.Voter()
		if _, err := tx.Exec(insertQuery, idx.nodeAddr, senderAddr, voteHash[:]); err != nil {
			return err
		}

		// put new vote for recipient
		recipientAddr := vote.Votee()
		if _, err := tx.Exec(insertQuery, idx.nodeAddr, recipientAddr, voteHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateVoteToBlock map vote hash to block hash
func (idx *IndexService) UpdateVoteToBlock(blk *blockchain.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT vote_to_block SET vote_hash=?,block_hash=?"
	for _, vote := range blk.Votes {
		voteHash := vote.Hash()
		if _, err := tx.Exec(insertQuery, voteHash[:], blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateExecutionHistory stores execution information into execution history table
func (idx *IndexService) UpdateExecutionHistory(blk *blockchain.Block, tx *sql.Tx) error {
	insertQuery := "INSERT execution_history SET node_address=?,user_address=?,execution_hash=?"
	for _, execution := range blk.Executions {
		executionHash := execution.Hash()

		// put new execution for executor
		executorAddr := execution.Executor()
		if _, err := tx.Exec(insertQuery, idx.nodeAddr, executorAddr, executionHash[:]); err != nil {
			return err
		}

		// put new execution for contract
		contractAddr := execution.Contract()
		if _, err := tx.Exec(insertQuery, idx.nodeAddr, contractAddr, executionHash[:]); err != nil {
			return err
		}
	}
	return nil
}

// UpdateExecutionToBlock map execution hash to block hash
func (idx *IndexService) UpdateExecutionToBlock(blk *blockchain.Block, tx *sql.Tx) error {
	blockHash := blk.HashBlock()
	insertQuery := "INSERT execution_to_block SET execution_hash=?,block_hash=?"
	for _, execution := range blk.Executions {
		executionHash := execution.Hash()
		if _, err := tx.Exec(insertQuery, executionHash[:], blockHash[:]); err != nil {
			return err
		}
	}
	return nil
}
