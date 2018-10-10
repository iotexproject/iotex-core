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

type TransferHistory struct {
	NodeAddress string
	UserAddress string
	TrasferHash string
}
type TransferToBlock struct {
	TrasferHash string
	BlockHash   string
}

type VoteHistory struct {
	NodeAddress string
	UserAddress string
	VoteHash    string
}
type VoteToBlock struct {
	VoteHash  string
	BlockHash string
}

type ExecutionHistory struct {
	NodeAddress   string
	UserAddress   string
	ExecutionHash string
}
type ExecutionToBlock struct {
	ExecutionHash string
	BlockHash     string
}

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

		nodeAddr := "aa"
		cfg := config.Default
		idx := Indexer{
			cfg:      cfg.Indexer,
			rds:      rdsStore,
			nodeAddr: nodeAddr,
		}

		blk := blockchain.Block{}
		blk.ConvertFromBlockPb(&iproto.BlockPb{
			Header: &iproto.BlockHeaderPb{
				Version: version.ProtocolVersion,
				Height:  123456789,
			},
			Actions: []*iproto.ActionPb{
				{Action: &iproto.ActionPb_Transfer{
					Transfer: &iproto.TransferPb{},
				},
					Version: version.ProtocolVersion,
					Nonce:   101,
				},
				{Action: &iproto.ActionPb_Vote{
					Vote: &iproto.VotePb{},
				},
					Version: version.ProtocolVersion,
					Nonce:   103,
				},
				{Action: &iproto.ActionPb_Execution{
					Execution: &iproto.ExecutionPb{},
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
		stmt, err := db.Prepare("SELECT * FROM transfer_history WHERE node_address=?")
		require.Nil(err)
		rows, err := stmt.Query(nodeAddr)
		require.Nil(err)
		var transferHistory TransferHistory
		parsedRows, err := rds.ParseRows(rows, &transferHistory)
		require.Nil(err)
		require.Equal(2, len(parsedRows))
		require.Equal(nodeAddr, parsedRows[0].(*TransferHistory).NodeAddress)
		transfer := blk.Transfers[0].Hash()
		require.Equal(string(transfer[:]), parsedRows[0].(*TransferHistory).TrasferHash)

		// get vote
		stmt, err = db.Prepare("SELECT * FROM vote_history WHERE node_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddr)
		require.Nil(err)
		var voteHistory VoteHistory
		parsedRows, err = rds.ParseRows(rows, &voteHistory)
		require.Nil(err)
		require.Equal(2, len(parsedRows))
		require.Equal(nodeAddr, parsedRows[0].(*VoteHistory).NodeAddress)
		vote := blk.Votes[0].Hash()
		require.Equal(string(vote[:]), parsedRows[0].(*VoteHistory).VoteHash)

		// get execution
		stmt, err = db.Prepare("SELECT * FROM execution_history WHERE node_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddr)
		require.Nil(err)
		var executionHistory ExecutionHistory
		parsedRows, err = rds.ParseRows(rows, &executionHistory)
		require.Nil(err)
		require.Equal(2, len(parsedRows))
		require.Equal(nodeAddr, parsedRows[0].(*ExecutionHistory).NodeAddress)
		execution := blk.Executions[0].Hash()
		require.Equal(string(execution[:]), parsedRows[0].(*ExecutionHistory).ExecutionHash)

		// transfer map to block
		stmt, err = db.Prepare("SELECT * FROM transfer_to_block WHERE transfer_hash=?")
		require.Nil(err)
		transferHash := blk.Transfers[0].Hash()
		rows, err = stmt.Query(transferHash[:])
		require.Nil(err)
		var transferToBlock TransferToBlock
		parsedRows, err = rds.ParseRows(rows, &transferToBlock)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		blkHash := blk.HashBlock()
		require.Equal(string(blkHash[:]), parsedRows[0].(*TransferToBlock).BlockHash)

		// vote map to block
		stmt, err = db.Prepare("SELECT * FROM vote_to_block WHERE vote_hash=?")
		require.Nil(err)
		voteHash := blk.Votes[0].Hash()
		rows, err = stmt.Query(voteHash[:])
		require.Nil(err)
		var voteToBlock VoteToBlock
		parsedRows, err = rds.ParseRows(rows, &voteToBlock)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		blkHash = blk.HashBlock()
		require.Equal(string(blkHash[:]), parsedRows[0].(*VoteToBlock).BlockHash)

		// execution map to block
		stmt, err = db.Prepare("SELECT * FROM execution_to_block WHERE execution_hash=?")
		require.Nil(err)
		executionHash := blk.Executions[0].Hash()
		rows, err = stmt.Query(executionHash[:])
		require.Nil(err)
		var executionToBlock ExecutionToBlock
		parsedRows, err = rds.ParseRows(rows, &executionToBlock)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		blkHash = blk.HashBlock()
		require.Equal(string(blkHash[:]), parsedRows[0].(*ExecutionToBlock).BlockHash)

		// delete transfers
		stmt, err = db.Prepare("DELETE FROM transfer_history WHERE node_address=?")
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
		stmt, err = db.Prepare("DELETE FROM transfer_to_block WHERE transfer_hash=?")
		require.Nil(err)
		transferHash = blk.Transfers[0].Hash()
		_, err = stmt.Exec(transferHash[:])
		require.Nil(err)

		// delete votes
		stmt, err = db.Prepare("DELETE FROM vote_history WHERE node_address=?")
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
		stmt, err = db.Prepare("DELETE FROM vote_to_block WHERE vote_hash=?")
		require.Nil(err)
		voteHash = blk.Votes[0].Hash()
		_, err = stmt.Exec(voteHash[:])
		require.Nil(err)

		// delete executions
		stmt, err = db.Prepare("DELETE FROM execution_history WHERE node_address=?")
		require.Nil(err)
		_, err = stmt.Exec(nodeAddr)
		require.Nil(err)
		stmt, err = db.Prepare("DELETE FROM execution_to_block WHERE execution_hash=?")
		require.Nil(err)
		executionHash = blk.Executions[0].Hash()
		_, err = stmt.Exec(executionHash[:])
		require.Nil(err)
	}

	path := "/tmp/test-indexservice-" + strconv.Itoa(rand.Int())
	t.Run("RDS Store", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testRDSStorePutGet(rds.NewAwsRDS(cfg), t)
	})
}
