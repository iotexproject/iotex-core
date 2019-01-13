// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/sql"
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
	addr1 := testaddress.IotxAddrinfo["alfa"]
	addr2 := testaddress.IotxAddrinfo["bravo"]
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
					Transfer: &iproto.TransferPb{Recipient: addr2.RawAddress},
				},
				Sender:       addr1.RawAddress,
				SenderPubKey: addr1.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        101,
			},
			{
				Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{VoteeAddress: addr2.RawAddress},
				},
				Sender:       addr1.RawAddress,
				SenderPubKey: addr1.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        103,
			},
			{
				Action: &iproto.ActionPb_Execution{
					Execution: &iproto.ExecutionPb{Contract: addr2.RawAddress},
				},
				Sender:       addr1.RawAddress,
				SenderPubKey: addr1.PublicKey[:],
				Version:      version.ProtocolVersion,
				Nonce:        104,
			},
		},
	})
	require.NoError(err)

	err = idx.BuildIndex(&blk)
	require.Nil(err)

	db := store.GetDB()

	transfers, votes, executions := action.ClassifyActions(blk.Actions)

	// get transfer
	transferHashes, err := idx.GetTransferHistory(addr1.RawAddress)
	require.Nil(err)
	require.Equal(1, len(transferHashes))
	transfer := transfers[0].Hash()
	require.Equal(transfer, transferHashes[0])

	// get vote
	voteHashes, err := idx.GetVoteHistory(addr1.RawAddress)
	require.Nil(err)
	require.Equal(1, len(voteHashes))
	vote := votes[0].Hash()
	require.Equal(vote, voteHashes[0])

	// get execution
	executionHashes, err := idx.GetExecutionHistory(addr1.RawAddress)
	require.Nil(err)
	require.Equal(1, len(executionHashes))
	execution := executions[0].Hash()
	require.Equal(execution, executionHashes[0])

	// get action
	actionHashes, err := idx.GetActionHistory(addr1.RawAddress)
	require.Nil(err)
	require.Equal(3, len(actionHashes))
	action := blk.Actions[0].Hash()
	require.Equal(action, actionHashes[0])

	// transfer map to block
	blkHash1, err := idx.GetBlockByTransfer(transfers[0].Hash())
	require.Nil(err)
	require.Equal(blkHash1, blk.HashBlock())

	// vote map to block
	blkHash2, err := idx.GetBlockByVote(votes[0].Hash())
	require.Nil(err)
	require.Equal(blkHash2, blk.HashBlock())

	// execution map to block
	blkHash3, err := idx.GetBlockByExecution(executions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash3, blk.HashBlock())

	// action map to block
	blkHash4, err := idx.GetBlockByAction(blk.Actions[0].Hash())
	require.Nil(err)
	require.Equal(blkHash4, blk.HashBlock())

	// delete transfers
	stmt, err := db.Prepare("DELETE FROM transfer_history WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)
	stmt, err = db.Prepare("DELETE FROM transfer_to_block WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)

	// delete votes
	stmt, err = db.Prepare("DELETE FROM vote_history WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)
	stmt, err = db.Prepare("DELETE FROM vote_to_block WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)

	// delete executions
	stmt, err = db.Prepare("DELETE FROM execution_history WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)
	stmt, err = db.Prepare("DELETE FROM execution_to_block WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)

	// delete actions
	stmt, err = db.Prepare("DELETE FROM action_history WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)
	stmt, err = db.Prepare("DELETE FROM action_to_block WHERE node_address=?")
	require.Nil(err)
	_, err = stmt.Exec(nodeAddr)
	require.Nil(err)
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
