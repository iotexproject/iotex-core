package blockindex

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestIndexBuilder(t *testing.T) {
	require := require.New(t)

	blks := getTestBlocks(t)

	t1Hash, _ := blks[0].Actions[0].Hash()
	t4Hash, _ := blks[0].Actions[1].Hash()
	e1Hash, _ := blks[0].Actions[2].Hash()
	t2Hash, _ := blks[1].Actions[0].Hash()
	t5Hash, _ := blks[1].Actions[1].Hash()
	e2Hash, _ := blks[1].Actions[2].Hash()
	t3Hash, _ := blks[2].Actions[0].Hash()
	t6Hash, _ := blks[2].Actions[1].Hash()
	e3Hash, _ := blks[2].Actions[2].Hash()

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

	testIndexer := func(dao blockdao.BlockDAO, indexer Indexer, t *testing.T) {
		ctx := protocol.WithBlockchainCtx(
			genesis.WithGenesisContext(context.Background(), genesis.Default),
			protocol.BlockchainCtx{
				ChainID: config.Default.Chain.ID,
			})
		require.NoError(dao.Start(ctx))
		require.NoError(indexer.Start(ctx))
		ib := &IndexBuilder{
			dao:     dao,
			indexer: indexer,
			genesis: genesis.Default,
		}
		defer func() {
			require.NoError(ib.Stop(ctx))
			require.NoError(dao.Stop(ctx))
		}()

		// put 2 blocks first
		require.NoError(dao.PutBlock(ctx, blks[0]))
		require.NoError(dao.PutBlock(ctx, blks[1]))
		startHeight, err := ib.indexer.Height()
		require.NoError(err)
		require.EqualValues(0, startHeight)
		tipHeight, err := dao.Height()
		require.NoError(err)
		require.EqualValues(2, tipHeight)

		// init() should build index for first 2 blocks
		require.NoError(ib.init(ctx))
		height, err := ib.indexer.Height()
		require.NoError(err)
		require.EqualValues(2, height)

		// test handle 1 new block
		require.NoError(dao.PutBlock(ctx, blks[2]))
		ib.ReceiveBlock(blks[2])
		time.Sleep(500 * time.Millisecond)

		height, err = ib.indexer.Height()
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

	path := "test-indexer"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)
	indexPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
		testutil.CleanupPath(indexPath)
	}()
	cfg := db.DefaultConfig
	cfg.DbPath = testPath

	for _, v := range []struct {
		dao   blockdao.BlockDAO
		inMem bool
	}{
		{
			blockdao.NewBlockDAOInMemForTest(nil), true,
		},
		{
			blockdao.NewBlockDAO(nil, cfg), false,
		},
	} {
		t.Run("test indexbuilder", func(t *testing.T) {
			var (
				indexer Indexer
				err     error
			)
			if v.inMem {
				indexer, err = NewIndexer(db.NewMemKVStore(), hash.ZeroHash256)
			} else {
				cfg.DbPath = indexPath
				indexer, err = NewIndexer(db.NewBoltDB(cfg), hash.ZeroHash256)
			}
			require.NoError(err)
			testIndexer(v.dao, indexer, t)
		})
	}
}
