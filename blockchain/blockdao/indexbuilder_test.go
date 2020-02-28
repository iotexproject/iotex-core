package blockdao

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestIndexer(t *testing.T) {

	blks := getTestBlocks(t)

	t1Hash := blks[0].Actions[0].Hash()
	t4Hash := blks[0].Actions[1].Hash()
	e1Hash := blks[0].Actions[2].Hash()
	t2Hash := blks[1].Actions[0].Hash()
	t5Hash := blks[1].Actions[1].Hash()
	e2Hash := blks[1].Actions[2].Hash()
	t3Hash := blks[2].Actions[0].Hash()
	t6Hash := blks[2].Actions[1].Hash()
	e3Hash := blks[2].Actions[2].Hash()

	type index struct {
		addr   hash.Hash160
		hashes [][]byte
	}

	indexTests := struct {
		total     uint64
		hashTotal [][]byte
		actions   [4]index
	}{
		9,
		[][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t2Hash[:], t5Hash[:], e2Hash[:], t3Hash[:], t6Hash[:], e3Hash[:]},
		[4]index{
			{hash.BytesToHash160(identityset.Address(28).Bytes()),
				[][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t6Hash[:]}},
			{hash.BytesToHash160(identityset.Address(29).Bytes()),
				[][]byte{t4Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]}},
			{hash.BytesToHash160(identityset.Address(30).Bytes()),
				[][]byte{t5Hash[:], t3Hash[:], t6Hash[:], e3Hash[:]}},
			{hash.BytesToHash160(identityset.Address(31).Bytes()),
				[][]byte{e1Hash[:], e2Hash[:], e3Hash[:]}},
		},
	}

	testIndexer := func(kvStore db.KVStore, indexer blockindex.Indexer, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()
		dao := NewBlockDAO(kvStore, nil, false, config.Default.DB)
		require.NoError(dao.Start(ctx))
		require.NoError(indexer.Start(ctx))
		defer func() {
			require.NoError(indexer.Stop(ctx))
			require.NoError(dao.Stop(ctx))
		}()

		ib := &IndexBuilder{
			dao:     dao,
			indexer: indexer,
		}
		defer ib.Stop(context.Background())

		// put 2 blocks first
		require.NoError(dao.PutBlock(blks[0]))
		require.NoError(dao.PutBlock(blks[1]))
		require.NoError(dao.Commit())
		startHeight, err := ib.indexer.GetBlockchainHeight()
		require.NoError(err)
		require.EqualValues(0, startHeight)
		tipHeight := dao.GetTipHeight()
		require.EqualValues(2, tipHeight)

		// init() should build index for first 2 blocks
		require.NoError(ib.init())
		height, err := ib.indexer.GetBlockchainHeight()
		require.NoError(err)
		require.EqualValues(2, height)

		// test handle 1 new block
		require.NoError(dao.PutBlock(blks[2]))
		require.NoError(dao.Commit())
		ib.ReceiveBlock(blks[2])
		time.Sleep(500 * time.Millisecond)

		height, err = ib.indexer.GetBlockchainHeight()
		require.NoError(err)
		require.EqualValues(3, height)

		// Test GetActionIndex
		actIndex, err := indexer.GetActionIndex(t1Hash[:])
		require.NoError(err)
		require.Equal(blks[0].Height(), actIndex.BlockHeight())
		actIndex, err = indexer.GetActionIndex(t2Hash[:])
		require.NoError(err)
		require.Equal(blks[1].Height(), actIndex.BlockHeight())
		actIndex, err = indexer.GetActionIndex(t3Hash[:])
		require.NoError(err)
		require.Equal(blks[2].Height(), actIndex.BlockHeight())

		// Test get actions
		total, err := indexer.GetTotalActions()
		require.NoError(err)
		require.EqualValues(indexTests.total, total)
		_, err = indexer.GetActionHashFromIndex(1, total)
		require.Equal(db.ErrInvalid, errors.Cause(err))
		actions, err := indexer.GetActionHashFromIndex(0, total)
		require.NoError(err)
		require.Equal(actions, indexTests.hashTotal)
		for j := range indexTests.actions {
			actionCount, err := indexer.GetActionCountByAddress(indexTests.actions[j].addr)
			require.NoError(err)
			require.EqualValues(len(indexTests.actions[j].hashes), actionCount)
			if actionCount > 0 {
				actions, err := indexer.GetActionsByAddress(indexTests.actions[j].addr, 0, actionCount)
				require.NoError(err)
				require.Equal(actions, indexTests.actions[j].hashes)
			}
		}

		for i := 0; i < 3; i++ {
			amount := big.NewInt(0)
			tsfs, _ := action.ClassifyActions(blks[i].Actions)
			for _, tsf := range tsfs {
				amount.Add(amount, tsf.Amount())
			}

			// test getNumActions/getTranferAmount
			index, err := indexer.GetBlockIndex(blks[i].Height())
			require.NoError(err)
			require.Equal(blks[i].HashBlock(), hash.BytesToHash256(index.Hash()))
			require.EqualValues(len(blks[i].Actions), index.NumAction())
			require.Equal(amount, index.TsfAmount())
		}
	}

	t.Run("In-memory KV indexer", func(t *testing.T) {
		indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), hash.ZeroHash256)
		require.NoError(t, err)
		testIndexer(db.NewMemKVStore(), indexer, t)
	})
	path := "test-indexer"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	require.NoError(t, testFile.Close())
	indexFile, _ := ioutil.TempFile(os.TempDir(), path)
	indexPath := indexFile.Name()
	require.NoError(t, indexFile.Close())
	cfg := config.Default.DB
	t.Run("Bolt DB indexer", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		testutil.CleanupPath(t, indexPath)
		defer func() {
			testutil.CleanupPath(t, testPath)
			testutil.CleanupPath(t, indexPath)
		}()
		cfg.DbPath = indexPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg), hash.ZeroHash256)
		require.NoError(t, err)
		cfg.DbPath = testPath
		testIndexer(db.NewBoltDB(cfg), indexer, t)
	})
}
