// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package indexservice

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/rds"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/proto"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/stretchr/testify/require"
)

var (
	cfg = &config.Default.DB.RDS
)

func TestIndexService(t *testing.T) {
	testRDSStorePutGet := func(rdsStore rds.Store, t *testing.T) {
		t.Skip("Skipping when RDS credentail not provided.")

		require := require.New(t)
		ctx := context.Background()

		err := rdsStore.Start(ctx)
		require.Nil(err)
		defer func() {
			err = rdsStore.Stop(ctx)
			require.Nil(err)
		}()

		nodeAddr := "aaa"
		userAddr1 := "bb"
		userAddr2 := "cc"
		cfg := config.Default
		idx := Indexer{
			cfg:                cfg.Indexer,
			rds:                rdsStore,
			hexEncodedNodeAddr: nodeAddr,
		}

		blk := blockchain.Block{}
		blk.ConvertFromBlockPb(&iproto.BlockPb{
			Header: &iproto.BlockHeaderPb{
				Version: version.ProtocolVersion,
				Height:  123456789,
			},
			Actions: []*iproto.ActionPb{
				{Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{Recipient: userAddr2},
				},
					Version: version.ProtocolVersion,
					Sender:  userAddr1,
					Nonce:   101,
				},
				{Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{VoteeAddress: userAddr2},
				},
					Version: version.ProtocolVersion,
					Sender:  userAddr1,
					Nonce:   103,
				},
				{Action: &iproto.ActionPb_Execution{
					Execution: &iproto.ExecutionPb{Contract: userAddr2},
				},
					Version: version.ProtocolVersion,
					Sender:  userAddr1,
					Nonce:   104,
				},
			},
		})

		err = idx.BuildIndex(&blk)
		require.Nil(err)

		db := rdsStore.GetDB()

		transfers, votes, executions := action.ClassifyActions(blk.Actions)

		// get transfer
		transferHashes, err := idx.GetTransferHistory(userAddr1)
		require.Nil(err)
		require.Equal(1, len(transferHashes))
		transfer := transfers[0].Hash()
		require.Equal(transfer, transferHashes[0])

		// get vote
		voteHashes, err := idx.GetVoteHistory(userAddr1)
		require.Nil(err)
		require.Equal(1, len(voteHashes))
		vote := votes[0].Hash()
		require.Equal(vote, voteHashes[0])

		// get execution
		executionHashes, err := idx.GetExecutionHistory(userAddr1)
		require.Nil(err)
		require.Equal(1, len(executionHashes))
		execution := executions[0].Hash()
		require.Equal(execution, executionHashes[0])

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
	}

	path := "/tmp/test-indexer-" + strconv.Itoa(rand.Int())
	t.Run("Indexer", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testRDSStorePutGet(rds.NewAwsRDS(cfg), t)
	})
}
