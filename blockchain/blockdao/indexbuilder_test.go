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
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestIndexer(t *testing.T) {

	blks := getTestBlocks(t)
	require.Equal(t, 3, len(blks))
	blkHash1 := blks[0].HashBlock()
	blkHash2 := blks[1].HashBlock()
	blkHash3 := blks[2].HashBlock()

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

	testIndexer := func(kvstore db.KVStore, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()
		blockDao := NewBlockDAO(kvstore, false, false, 0, config.Default.DB)
		dao, ok := blockDao.(*blockDAO)
		require.True(ok)
		require.NoError(dao.Start(ctx))
		defer func() {
			require.NoError(dao.Stop(ctx))
		}()

		zero := make([]byte, 8)
		require.NoError(dao.kvstore.Put(blockNS, totalActionsKey, zero))
		v, _ := dao.kvstore.Get(blockNS, totalActionsKey)
		require.Equal(zero, v)
		require.NoError(dao.putBlock(blks[0]))
		require.NoError(dao.putBlock(blks[1]))

		ib := &IndexBuilder{
			pendingBlks: make(chan *block.Block, 64),
			cancelChan:  make(chan interface{}),
			dao:         dao,
			reindex:     false,
			dirtyAddr:   make(map[hash.Hash160]db.CountingIndex),
		}
		require.NoError(ib.Start(context.Background()))
		defer ib.Stop(context.Background())
		require.True(ib.reindex)
		height, err := ib.getNextHeight()
		require.NoError(err)
		require.EqualValues(3, height)

		// test handle new block
		require.NoError(dao.putBlock(blks[2]))
		ib.pendingBlks <- blks[2]
		time.Sleep(500 * time.Millisecond)

		height, err = ib.getNextHeight()
		require.NoError(err)
		require.EqualValues(4, height)

		// Test getBlockHashByActionHash
		blkHash, err := dao.getBlockHashByActionHash(t1Hash)
		require.NoError(err)
		require.Equal(blkHash1, blkHash)
		blkHash, err = dao.getBlockHashByActionHash(t2Hash)
		require.NoError(err)
		require.Equal(blkHash2, blkHash)
		blkHash, err = dao.getBlockHashByActionHash(t3Hash)
		require.NoError(err)
		require.Equal(blkHash3, blkHash)

		// Test get actions
		total, err := dao.getTotalActions()
		require.NoError(err)
		require.EqualValues(indexTests.total, total)
		_, err = dao.getActionHashFromIndex(1, total)
		require.Equal(db.ErrInvalid, errors.Cause(err))
		actions, err := dao.getActionHashFromIndex(0, total)
		require.NoError(err)
		require.Equal(actions, indexTests.hashTotal)
		for j := range indexTests.actions {
			actionCount, err := dao.getActionCountByAddress(indexTests.actions[j].addr)
			require.NoError(err)
			require.EqualValues(len(indexTests.actions[j].hashes), actionCount)
			if actionCount > 0 {
				actions, err := dao.getActionsByAddress(indexTests.actions[j].addr, 0, actionCount)
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

			// test getNumActions
			numActions, err := dao.getNumActions(blks[i].Height())
			require.NoError(err)
			require.EqualValues(len(blks[i].Actions), numActions)

			// test getTranferAmount
			transferAmount, err := dao.getTranferAmount(blks[i].Height())
			require.NoError(err)
			require.Equal(amount, transferAmount)
		}
	}

	t.Run("In-memory KV indexer", func(t *testing.T) {
		testIndexer(db.NewMemKVStore(), t)
	})
	path := "test-indexer"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg := config.Default.DB
	cfg.DbPath = testPath

	t.Run("Bolt DB indexer", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer testutil.CleanupPath(t, testPath)
		testIndexer(db.NewBoltDB(cfg), t)
	})
}
