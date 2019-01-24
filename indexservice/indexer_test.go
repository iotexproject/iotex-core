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

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/sql"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/test/testaddress"
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
	addr1 := testaddress.Addrinfo["alfa"].Bech32()
	pubKey1 := testaddress.Keyinfo["alfa"].PubKey
	addr2 := testaddress.Addrinfo["bravo"].Bech32()
	cfg := config.Default
	idx := Indexer{
		cfg:                cfg.Indexer,
		store:              store,
		hexEncodedNodeAddr: nodeAddr,
	}
	err = idx.CreateTablesIfNotExist()
	require.Nil(err)

	blk := block.Block{}

	err = blk.ConvertFromBlockPb(&iproto.BlockPb{
		Header: &iproto.BlockHeaderPb{
			Version: version.ProtocolVersion,
			Height:  123456789,
		},
		Actions: []*iproto.ActionPb{
			{
				Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{Recipient: addr2},
				},
				Sender:       addr1,
				SenderPubKey: pubKey1[:],
				Version:      version.ProtocolVersion,
				Nonce:        101,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{VoteeAddress: addr2},
				},
				Sender:       addr1,
				SenderPubKey: pubKey1[:],
				Version:      version.ProtocolVersion,
				Nonce:        103,
			},
			{
				Action: &iproto.ActionPb_Execution{
					Execution: &iproto.ExecutionPb{Contract: addr2},
				},
				Sender:       addr1,
				SenderPubKey: pubKey1[:],
				Version:      version.ProtocolVersion,
				Nonce:        104,
			},
		},
	})
	require.NoError(err)
	receipts := []*action.Receipt{
		{
			Hash:            byteutil.BytesTo32B(hash.Hash256b([]byte("1"))),
			ReturnValue:     []byte("1"),
			Status:          1,
			GasConsumed:     1,
			ContractAddress: "1",
			Logs:            []*action.Log{},
		},
		{
			Hash:            byteutil.BytesTo32B(hash.Hash256b([]byte("2"))),
			ReturnValue:     []byte("2"),
			Status:          2,
			GasConsumed:     2,
			ContractAddress: "2",
			Logs:            []*action.Log{},
		},
	}
	blk.Receipts = make(map[hash.Hash32B]*action.Receipt)
	for _, receipt := range receipts {
		blk.Receipts[receipt.Hash] = receipt
	}

	err = idx.BuildIndex(&blk)
	require.Nil(err)

	db := store.GetDB()

	transfers, votes, executions := action.ClassifyActions(blk.Actions)

	// get receipt
	blkHash, err := idx.GetBlockByIndex(IndexReceipt, receipts[0].Hash)
	require.Nil(err)
	require.Equal(blkHash, blk.HashBlock())

	blkHash, err = idx.GetBlockByIndex(IndexReceipt, receipts[1].Hash)
	require.Nil(err)
	require.Equal(blkHash, blk.HashBlock())

	// get transfer
	transferHashes, err := idx.GetIndexHistory(IndexTransfer, addr1)
	require.Nil(err)
	require.Equal(1, len(transferHashes))
	transfer := transfers[0].Hash()
	require.Equal(transfer, transferHashes[0])

	// get vote
	voteHashes, err := idx.GetIndexHistory(IndexVote, addr1)
	require.Nil(err)
	require.Equal(1, len(voteHashes))
	vote := votes[0].Hash()
	require.Equal(vote, voteHashes[0])

	// get execution
	executionHashes, err := idx.GetIndexHistory(IndexExecution, addr1)
	require.Nil(err)
	require.Equal(1, len(executionHashes))
	execution := executions[0].Hash()
	require.Equal(execution, executionHashes[0])

	// get action
	actionHashes, err := idx.GetIndexHistory(IndexAction, addr1)
	require.Nil(err)
	require.Equal(3, len(actionHashes))
	action := blk.Actions[0].Hash()
	require.Equal(action, actionHashes[0])

	// transfer map to block
	blkHash1, err := idx.GetBlockByIndex(IndexTransfer, transfers[0].Hash())
	require.Nil(err)
	require.Equal(blkHash1, blk.HashBlock())

	// vote map to block
	blkHash2, err := idx.GetBlockByIndex(IndexVote, votes[0].Hash())
	require.Nil(err)
	require.Equal(blkHash2, blk.HashBlock())

	// execution map to block
	blkHash3, err := idx.GetBlockByIndex(IndexExecution, executions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash3, blk.HashBlock())

	// action map to block
	blkHash4, err := idx.GetBlockByIndex(IndexAction, blk.Actions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash4, blk.HashBlock())

	// create block by index tables
	for _, indexIdentifier := range BlockByIndexList {
		stmt, err := db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE node_address=?", idx.getBlockByIndexTableName(indexIdentifier)))
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
	}

	// create index history tables
	for _, indexIdentifier := range IndexHistoryList {
		stmt, err := db.Prepare(fmt.Sprintf("DELETE FROM %s WHERE node_address=?", idx.getIndexHistoryTableName(indexIdentifier)))
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
	}
}

func TestIndexServiceOnSqlite3(t *testing.T) {
	t.Run("Indexer", func(t *testing.T) {
		testSQLite3StorePutGet(sql.NewSQLite3(config.Default.DB.SQLITE3), t)
	})
}

func TestIndexServiceOnAwsRDS(t *testing.T) {
	t.Skip("Skipping when RDS credentail not provided.")
	t.Run("Indexer", func(t *testing.T) {
		testSQLite3StorePutGet(sql.NewAwsRDS(config.Default.DB.RDS), t)
	})
}
