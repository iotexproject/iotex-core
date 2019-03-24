// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/protobuf/ptypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/sql"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"github.com/iotexproject/iotex-core/testutil"
)

func testSQLite3StorePutGet(store sql.Store, t *testing.T) {
	require := require.New(t)
	ctx := context.Background()

	err := store.Start(ctx)
	require.Nil(err)
	defer func() {
		err = store.Stop(ctx)
		require.Nil(err)
	}()

	nodeAddr := "aaa"
	addr1 := testaddress.Addrinfo["alfa"].String()
	pubKey1 := testaddress.Keyinfo["alfa"].PubKey
	addr2 := testaddress.Addrinfo["bravo"].String()
	cfg := config.Default
	idx := Indexer{
		cfg:                cfg.Indexer,
		store:              store,
		hexEncodedNodeAddr: nodeAddr,
	}
	err = idx.CreateTablesIfNotExist()
	require.Nil(err)

	blk := block.Block{}

	err = blk.ConvertFromBlockPb(&iotextypes.Block{
		Header: &iotextypes.BlockHeader{
			Core: &iotextypes.BlockHeaderCore{
				Version:   version.ProtocolVersion,
				Height:    123456789,
				Timestamp: ptypes.TimestampNow(),
			},
			ProducerPubkey: pubKey1.Bytes(),
		},
		Actions: []*iotextypes.Action{
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Transfer{
						Transfer: &iotextypes.Transfer{Recipient: addr2},
					},
					Version: version.ProtocolVersion,
					Nonce:   101,
				},
				SenderPubKey: pubKey1.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Vote{
						Vote: &iotextypes.Vote{VoteeAddress: addr2},
					},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				SenderPubKey: pubKey1.Bytes(),
			},
			{
				Core: &iotextypes.ActionCore{
					Action: &iotextypes.ActionCore_Execution{
						Execution: &iotextypes.Execution{Contract: addr2},
					},
					Version: version.ProtocolVersion,
					Nonce:   104,
				},
				SenderPubKey: pubKey1.Bytes(),
			},
		},
	})
	require.NoError(err)
	receipts := []*action.Receipt{
		{
			ActHash:         hash.Hash256b([]byte("1")),
			ReturnValue:     []byte("1"),
			Status:          1,
			GasConsumed:     1,
			ContractAddress: "1",
			Logs:            []*action.Log{},
		},
		{
			ActHash:         hash.Hash256b([]byte("2")),
			ReturnValue:     []byte("2"),
			Status:          2,
			GasConsumed:     2,
			ContractAddress: "2",
			Logs:            []*action.Log{},
		},
	}
	blk.Receipts = make([]*action.Receipt, 0)
	/*for _, receipt := range receipts {
		blk.Receipts = append(blk.Receipts, receipt)
	}*/
	blk.Receipts = append(blk.Receipts, receipts...)

	err = idx.BuildIndex(&blk)
	require.Nil(err)

	db := store.GetDB()

	transfers, votes, executions := action.ClassifyActions(blk.Actions)

	// get receipt
	blkHash, err := idx.GetBlockByIndex(config.IndexReceipt, receipts[0].Hash())
	require.Nil(err)
	require.Equal(blkHash, blk.HashBlock())

	blkHash, err = idx.GetBlockByIndex(config.IndexReceipt, receipts[1].Hash())
	require.Nil(err)
	require.Equal(blkHash, blk.HashBlock())

	// get transfer
	transferHashes, err := idx.GetIndexHistory(config.IndexTransfer, addr1)
	require.Nil(err)
	require.Equal(1, len(transferHashes))
	transfer := transfers[0].Hash()
	require.Equal(transfer, transferHashes[0])

	// get vote
	voteHashes, err := idx.GetIndexHistory(config.IndexVote, addr1)
	require.Nil(err)
	require.Equal(1, len(voteHashes))
	vote := votes[0].Hash()
	require.Equal(vote, voteHashes[0])

	// get execution
	executionHashes, err := idx.GetIndexHistory(config.IndexExecution, addr1)
	require.Nil(err)
	require.Equal(1, len(executionHashes))
	execution := executions[0].Hash()
	require.Equal(execution, executionHashes[0])

	// get action
	actionHashes, err := idx.GetIndexHistory(config.IndexAction, addr1)
	require.Nil(err)
	require.Equal(3, len(actionHashes))
	action := blk.Actions[0].Hash()
	require.Equal(action, actionHashes[0])

	// transfer map to block
	blkHash1, err := idx.GetBlockByIndex(config.IndexTransfer, transfers[0].Hash())
	require.Nil(err)
	require.Equal(blkHash1, blk.HashBlock())

	// vote map to block
	blkHash2, err := idx.GetBlockByIndex(config.IndexVote, votes[0].Hash())
	require.Nil(err)
	require.Equal(blkHash2, blk.HashBlock())

	// execution map to block
	blkHash3, err := idx.GetBlockByIndex(config.IndexExecution, executions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash3, blk.HashBlock())

	// action map to block
	blkHash4, err := idx.GetBlockByIndex(config.IndexAction, blk.Actions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash4, blk.HashBlock())

	// create block by index tables
	for _, indexIdentifier := range idx.cfg.BlockByIndexList {
		stmt, err := db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE node_address=?",
			idx.getBlockByIndexTableName(indexIdentifier)))
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
	}

	// create index history tables
	for _, indexIdentifier := range idx.cfg.IndexHistoryList {
		stmt, err := db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE node_address=?",
			idx.getIndexHistoryTableName(indexIdentifier)))
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
	}
}

func TestIndexServiceOnSqlite3(t *testing.T) {
	t.Run("Indexer", func(t *testing.T) {
		testutil.CleanupPath(t, config.Default.DB.SQLITE3.SQLite3File)
		defer testutil.CleanupPath(t, config.Default.DB.SQLITE3.SQLite3File)
		testSQLite3StorePutGet(sql.NewSQLite3(config.Default.DB.SQLITE3), t)
	})
}

func TestIndexServiceOnAwsRDS(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	t.Run("Indexer", func(t *testing.T) {
		testSQLite3StorePutGet(sql.NewAwsRDS(config.Default.DB.RDS), t)
	})
}
