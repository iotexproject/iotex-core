// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	s "github.com/iotexproject/iotex-core/db/sql"
	"github.com/iotexproject/iotex-core/pkg/hash"
)

// If you want to add a new index table, please:
// 1. add a index unique identifier in indexconfig
// 2. add this index unique identifier to one of table list in index config

type (
	// BlockByIndex defines the base schema of "index to block" table
	BlockByIndex struct {
		NodeAddress string
		IndexHash   []byte
		BlockHash   []byte
	}
	// IndexHistory defines the schema of "index history" table
	IndexHistory struct {
		NodeAddress string
		UserAddress string
		IndexHash   string
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
	if err := idx.store.Transact(func(tx *sql.Tx) error {
		transfers, votes, executions := action.ClassifyActions(blk.Actions)
		// log transfer index
		for _, transfer := range transfers {
			callerAddr, err := address.FromBytes(transfer.SrcPubkey().Hash())
			if err != nil {
				return err
			}
			// put new transfer for sender
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexTransfer, callerAddr.String(), transfer.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update transfer to transfer history table")
			}
			// put new transfer for recipient
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexTransfer, transfer.Recipient(), transfer.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update transfer to transfer history table")
			}
			// map transfer to block
			if err := idx.UpdateBlockByIndex(blk, tx, config.IndexTransfer, transfer.Hash(), blk.HashBlock()); err != nil {
				return errors.Wrapf(err, "failed to update transfer to block")
			}
		}

		// log vote index
		for _, vote := range votes {
			callerAddr, err := address.FromBytes(vote.SrcPubkey().Hash())
			if err != nil {
				return err
			}
			// put new vote for sender
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexVote, callerAddr.String(), vote.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update vote to vote history table")
			}
			// put new vote for recipient
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexVote, vote.Votee(), vote.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update vote to vote history table")
			}
			// map vote to block
			if err := idx.UpdateBlockByIndex(blk, tx, config.IndexVote, vote.Hash(), blk.HashBlock()); err != nil {
				return errors.Wrapf(err, "failed to update transfer to block")
			}
		}

		// log execution index
		for _, execution := range executions {
			callerAddr, err := address.FromBytes(execution.SrcPubkey().Hash())
			if err != nil {
				return err
			}
			// put new execution for executor
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexExecution, callerAddr.String(), execution.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update execution to execution history table")
			}
			// put new execution for contract
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexExecution, execution.Contract(), execution.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update execution to execution history table")
			}
			// map execution to block
			if err := idx.UpdateBlockByIndex(blk, tx, config.IndexExecution, execution.Hash(), blk.HashBlock()); err != nil {
				return errors.Wrapf(err, "failed to update transfer to block")
			}
		}

		// log action index
		for _, selp := range blk.Actions {
			callerAddr, err := address.FromBytes(selp.SrcPubkey().Hash())
			if err != nil {
				return err
			}
			// put new action for sender
			if err := idx.UpdateIndexHistory(blk, tx, config.IndexAction, callerAddr.String(), selp.Hash()); err != nil {
				return errors.Wrapf(err, "failed to update action to action history table")
			}
			// put new transfer for recipient
			dst, ok := selp.Destination()
			if ok {
				if err := idx.UpdateIndexHistory(blk, tx, config.IndexAction, dst, selp.Hash()); err != nil {
					return errors.Wrapf(err, "failed to update action to action history table")
				}
			}
			// map action to block
			if err := idx.UpdateBlockByIndex(blk, tx, config.IndexAction, selp.Hash(), blk.HashBlock()); err != nil {
				return errors.Wrapf(err, "failed to update transfer to block")
			}
		}

		// log receipt index
		for _, receipt := range blk.Receipts {
			// map receipt to block
			if err := idx.UpdateBlockByIndex(blk, tx, config.IndexReceipt, receipt.Hash(), blk.HashBlock()); err != nil {
				return errors.Wrapf(err, "failed to update receipt to block")
			}
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

// UpdateBlockByIndex maps index hash to block hash
func (idx *Indexer) UpdateBlockByIndex(blk *block.Block, tx *sql.Tx, indexIdentifier string, indexHash hash.Hash256,
	blockHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (node_address,index_hash,block_hash) VALUES (?, ?, ?)",
		idx.getBlockByIndexTableName(indexIdentifier))
	if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, hex.EncodeToString(indexHash[:]), blockHash[:]); err != nil {
		return err
	}
	return nil
}

// UpdateIndexHistory stores index information into index history table
func (idx *Indexer) UpdateIndexHistory(blk *block.Block, tx *sql.Tx, indexIdentifier string, userAddr string,
	indexHash hash.Hash256) error {
	insertQuery := fmt.Sprintf("INSERT INTO %s (node_address,user_address,index_hash) VALUES (?, ?, ?)",
		idx.getIndexHistoryTableName(indexIdentifier))
	if _, err := tx.Exec(insertQuery, idx.hexEncodedNodeAddr, userAddr, indexHash[:]); err != nil {
		return err
	}
	return nil
}

// GetIndexHistory gets index history
func (idx *Indexer) GetIndexHistory(indexIdentifier string, userAddr string) ([]hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE node_address=? AND user_address=?",
		idx.getIndexHistoryTableName(indexIdentifier))
	indexHashes, err := idx.getIndexHistory(getQuery, userAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get index history")
	}
	return indexHashes, nil
}

// GetBlockByIndex returns block hash by index hash
func (idx *Indexer) GetBlockByIndex(indexIdentifier string, indexHash hash.Hash256) (hash.Hash256, error) {
	getQuery := fmt.Sprintf("SELECT * FROM %s WHERE node_address=? AND index_hash=?",
		idx.getBlockByIndexTableName(indexIdentifier))
	blkHash, err := idx.blockByIndex(getQuery, indexHash)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to get block hash by index hash")
	}
	return blkHash, nil
}

// blockByIndex returns block by receipt hash
func (idx *Indexer) blockByIndex(getQuery string, actionHash hash.Hash256) (hash.Hash256, error) {
	db := idx.store.GetDB()

	stmt, err := db.Prepare(getQuery)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to prepare get query")
	}

	rows, err := stmt.Query(idx.hexEncodedNodeAddr, hex.EncodeToString(actionHash[:]))
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to execute get query")
	}

	var blockByIndex BlockByIndex
	parsedRows, err := s.ParseSQLRows(rows, &blockByIndex)
	if err != nil {
		return hash.ZeroHash256, errors.Wrapf(err, "failed to parse results")
	}

	if len(parsedRows) == 0 {
		return hash.ZeroHash256, ErrNotExist
	}

	var hash hash.Hash256
	copy(hash[:], parsedRows[0].(*BlockByIndex).BlockHash)
	return hash, nil
}

// getIndexHistory gets index history
func (idx *Indexer) getIndexHistory(getQuery string, userAddr string) ([]hash.Hash256, error) {
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

	indexHashes := make([]hash.Hash256, 0, len(parsedRows))
	for _, parsedRow := range parsedRows {
		var hash hash.Hash256
		copy(hash[:], parsedRow.(*IndexHistory).IndexHash)
		indexHashes = append(indexHashes, hash)
	}
	return indexHashes, nil
}

func (idx *Indexer) getBlockByIndexTableName(indexIndentifier string) string {
	return fmt.Sprintf("block_by_index_%s", indexIndentifier)
}

func (idx *Indexer) getIndexHistoryTableName(indexIndentifier string) string {
	return fmt.Sprintf("index_history_%s", indexIndentifier)
}

// CreateTablesIfNotExist creates tables in local database
func (idx *Indexer) CreateTablesIfNotExist() error {
	// create block by index tables
	for _, indexIdentifier := range idx.cfg.BlockByIndexList {
		if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ([node_address] TEXT NOT NULL, "+
			"[index_hash] BLOB(32) NOT NULL, [block_hash] BLOB(32) NOT NULL)", idx.getBlockByIndexTableName(indexIdentifier))); err != nil {
			return err
		}
	}

	// create index history tables
	for _, indexIdentifier := range idx.cfg.IndexHistoryList {
		if _, err := idx.store.GetDB().Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s ([node_address] TEXT NOT NULL, "+
			"[user_address] TEXT NOT NULL, [index_hash] BLOB(32) NOT NULL)", idx.getIndexHistoryTableName(indexIdentifier))); err != nil {
			return err
		}
	}

	return nil
}
