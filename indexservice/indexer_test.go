package indexservice

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

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
					Transfer: &iproto.TransferPb{Sender: userAddr1, Recipient: userAddr2},
				},
					Version: version.ProtocolVersion,
					Nonce:   101,
				},
				{Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{VoterAddress: userAddr1, VoteeAddress: userAddr2},
				},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				{Action: &iproto.ActionPb_Execution{
					Execution: &iproto.ExecutionPb{Executor: userAddr1, Contract: userAddr2},
				},
					Version: version.ProtocolVersion,
					Nonce:   104,
				},
			},
		})

		err = idx.BuildIndex(&blk)
		require.Nil(err)

		db := rdsStore.GetDB()

		// get transfer
		transferHashes, err := idx.GetTransferHistory(userAddr1)
		require.Nil(err)
		require.Equal(1, len(transferHashes))
		transfer := blk.Transfers[0].Hash()
		require.Equal(transfer, transferHashes[0])

		// get vote
		voteHashes, err := idx.GetVoteHistory(userAddr1)
		require.Nil(err)
		require.Equal(1, len(voteHashes))
		vote := blk.Votes[0].Hash()
		require.Equal(vote, voteHashes[0])

		// get execution
		executionHashes, err := idx.GetExecutionHistory(userAddr1)
		require.Nil(err)
		require.Equal(1, len(executionHashes))
		execution := blk.Executions[0].Hash()
		require.Equal(execution, executionHashes[0])

		// transfer map to block
		blkHash1, err := idx.GetBlockByTransfer(blk.Transfers[0].Hash())
		require.Nil(err)
		require.Equal(blkHash1, blk.HashBlock())

		// vote map to block
		blkHash2, err := idx.GetBlockByVote(blk.Votes[0].Hash())
		require.Nil(err)
		require.Equal(blkHash2, blk.HashBlock())

		// execution map to block
		blkHash3, err := idx.GetBlockByExecution(blk.Executions[0].Hash())
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
