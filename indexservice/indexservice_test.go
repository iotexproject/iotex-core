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
	cfg = &config.Default.RDS
)

func TestIndexService(t *testing.T) {
	testRDSStorePutGet := func(rdsStore rds.RDSStore, t *testing.T) {
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
		idx := IndexService{
			cfg:      cfg.IndexService,
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

		idx.BuildIndex(&blk)

		db := rdsStore.GetDB()

		// get transfer
		stmt, err := db.Prepare("SELECT * FROM transfer_history WHERE node_address=?")
		require.Nil(err)
		rows, err := stmt.Query(nodeAddr)
		require.Nil(err)
		parsedRows, err := rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(2, len(parsedRows))
		require.Equal(nodeAddr, string(parsedRows[0][0]))
		transfer := blk.Transfers[0].Hash()
		require.Equal(transfer[:], []byte(parsedRows[0][2]))

		// get vote
		stmt, err = db.Prepare("SELECT * FROM vote_history WHERE node_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddr)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(2, len(parsedRows))
		require.Equal(nodeAddr, string(parsedRows[0][0]))
		vote := blk.Votes[0].Hash()
		require.Equal(vote[:], []byte(parsedRows[0][2]))

		// get execution
		stmt, err = db.Prepare("SELECT * FROM execution_history WHERE node_address=?")
		require.Nil(err)
		rows, err = stmt.Query(nodeAddr)
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(2, len(parsedRows))
		require.Equal(nodeAddr, string(parsedRows[0][0]))
		execution := blk.Executions[0].Hash()
		require.Equal(execution[:], []byte(parsedRows[0][2]))

		// transfer map to block
		stmt, err = db.Prepare("SELECT * FROM transfer_to_block WHERE transfer_hash=?")
		require.Nil(err)
		transferHash := blk.Transfers[0].Hash()
		rows, err = stmt.Query(transferHash[:])
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		blkHash := blk.HashBlock()
		require.Equal(string(blkHash[:]), string(parsedRows[0][1]))

		// vote map to block
		stmt, err = db.Prepare("SELECT * FROM vote_to_block WHERE vote_hash=?")
		require.Nil(err)
		voteHash := blk.Votes[0].Hash()
		rows, err = stmt.Query(voteHash[:])
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		blkHash = blk.HashBlock()
		require.Equal(string(blkHash[:]), string(parsedRows[0][1]))

		// execution map to block
		stmt, err = db.Prepare("SELECT * FROM execution_to_block WHERE execution_hash=?")
		require.Nil(err)
		executionHash := blk.Executions[0].Hash()
		rows, err = stmt.Query(executionHash[:])
		require.Nil(err)
		parsedRows, err = rdsStore.ParseRows(rows)
		require.Nil(err)
		require.Equal(1, len(parsedRows))
		blkHash = blk.HashBlock()
		require.Equal(string(blkHash[:]), string(parsedRows[0][1]))

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
