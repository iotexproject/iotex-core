package blockdao

import (
	"context"
	"hash/fnv"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func getTestBlocks(t *testing.T) []*block.Block {
	amount := uint64(50 << 22)
	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf2, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf3, err := testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf4, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(28), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf5, err := testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(29), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf6, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), 4, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	// create testing executions
	execution1, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution2, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(29), 2, big.NewInt(0), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution3, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 3, big.NewInt(2), 0, big.NewInt(0), nil)
	require.NoError(t, err)

	hash1 := hash.Hash256{}
	fnv.New32().Sum(hash1[:])
	blk1, err := block.NewTestingBuilder().
		SetHeight(1).
		SetPrevBlockHash(hash1).
		SetTimeStamp(testutil.TimestampNow().UTC()).
		AddActions(tsf1, tsf4, execution1).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash2 := hash.Hash256{}
	fnv.New32().Sum(hash2[:])
	blk2, err := block.NewTestingBuilder().
		SetHeight(2).
		SetPrevBlockHash(hash2).
		SetTimeStamp(testutil.TimestampNow().UTC()).
		AddActions(tsf2, tsf5, execution2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash3 := hash.Hash256{}
	fnv.New32().Sum(hash3[:])
	blk3, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash3).
		SetTimeStamp(testutil.TimestampNow().UTC()).
		AddActions(tsf3, tsf6, execution3).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	return []*block.Block{&blk1, &blk2, &blk3}
}

func TestBlockDAO(t *testing.T) {

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

	addr28 := hash.BytesToHash160(identityset.Address(28).Bytes())
	addr29 := hash.BytesToHash160(identityset.Address(29).Bytes())
	addr30 := hash.BytesToHash160(identityset.Address(30).Bytes())
	addr31 := hash.BytesToHash160(identityset.Address(31).Bytes())

	type index struct {
		addr   hash.Hash160
		hashes [][]byte
	}

	daoTests := []struct {
		total     uint64
		hashTotal [][]byte
		actions   [4]index
	}{
		{
			9,
			[][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t2Hash[:], t5Hash[:], e2Hash[:], t3Hash[:], t6Hash[:], e3Hash[:]},
			[4]index{
				{addr28, [][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t6Hash[:]}},
				{addr29, [][]byte{t4Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]}},
				{addr30, [][]byte{t5Hash[:], t3Hash[:], t6Hash[:], e3Hash[:]}},
				{addr31, [][]byte{e1Hash[:], e2Hash[:], e3Hash[:]}},
			},
		},
		{
			6,
			[][]byte{t1Hash[:], t4Hash[:], e1Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]},
			[4]index{
				{addr28, [][]byte{t1Hash[:], t4Hash[:], e1Hash[:]}},
				{addr29, [][]byte{t4Hash[:], t2Hash[:], t5Hash[:], e2Hash[:]}},
				{addr30, [][]byte{t5Hash[:]}},
				{addr31, [][]byte{e1Hash[:], e2Hash[:]}},
			},
		},
		{
			3,
			[][]byte{t1Hash[:], t4Hash[:], e1Hash[:]},
			[4]index{
				{addr28, [][]byte{t1Hash[:], t4Hash[:], e1Hash[:]}},
				{addr29, [][]byte{t4Hash[:]}},
				{addr30, nil},
				{addr31, [][]byte{e1Hash[:]}},
			},
		},
		{
			0,
			nil,
			[4]index{
				{addr28, nil},
				{addr29, nil},
				{addr30, nil},
				{addr31, nil},
			},
		},
	}

	testBlockDao := func(kvstore db.KVStore, indexer blockindex.Indexer, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		dao := NewBlockDAO(kvstore, indexer, false, config.Default.DB)
		require.NoError(dao.Start(ctx))
		defer func() {
			require.NoError(dao.Stop(ctx))
		}()

		// receipts for the 3 blocks
		receipts := [][]*action.Receipt{
			{
				{1, 1, t1Hash, 15, "1", []*action.Log{}},
				{0, 1, t4Hash, 216, "2", []*action.Log{}},
				{2, 1, e1Hash, 6, "3", []*action.Log{}},
			},
			{
				{3, 2, t2Hash, 1500, "1", []*action.Log{}},
				{5, 2, t5Hash, 34, "2", []*action.Log{}},
				{9, 2, e2Hash, 655, "3", []*action.Log{}},
			},
			{
				{7, 3, t3Hash, 488, "1", []*action.Log{}},
				{6, 3, t6Hash, 2, "2", []*action.Log{}},
				{2, 3, e3Hash, 1099, "3", []*action.Log{}},
			},
		}

		height, err := indexer.GetBlockchainHeight()
		require.NoError(err)
		require.EqualValues(0, height)

		for i := 0; i < 3; i++ {
			// test putBlock/Receipt
			blks[i].Receipts = receipts[i]
			require.NoError(dao.PutBlock(blks[i]))
			require.NoError(dao.Commit())
			blks[i].Receipts = nil

			// test getBlockchainHeight
			height, err := indexer.GetBlockchainHeight()
			require.NoError(err)
			require.Equal(blks[i].Height(), height)

			// test getTipHash
			hash, err := dao.GetTipHash()
			require.NoError(err)
			require.Equal(blks[i].HashBlock(), hash)

			// test getBlock()
			blk, err := dao.GetBlock(blks[i].HashBlock())
			require.NoError(err)
			require.Equal(blks[i], blk)
		}

		// Test getReceiptByActionHash
		for j := range daoTests[0].hashTotal {
			h := hash.BytesToHash256(daoTests[0].hashTotal[j])
			receipt, err := dao.GetReceiptByActionHash(h, uint64(j/3)+1)
			require.NoError(err)
			require.Equal(receipts[j/3][j%3], receipt)
			action, err := dao.GetActionByActionHash(h, uint64(j/3)+1)
			require.NoError(err)
			require.Equal(h, action.Hash())
		}
	}

	testDeleteDao := func(kvstore db.KVStore, indexer blockindex.Indexer, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		dao := NewBlockDAO(kvstore, indexer, false, config.Default.DB)
		require.NoError(dao.Start(ctx))
		defer func() {
			require.NoError(dao.Stop(ctx))
		}()

		// put blocks
		for i := 0; i < 3; i++ {
			require.NoError(dao.PutBlock(blks[i]))
		}
		require.NoError(dao.Commit())
		height, err := indexer.GetBlockchainHeight()
		require.NoError(err)
		require.EqualValues(3, height)

		// delete tip block one by one, verify address/action after each deletion
		for i := range daoTests {
			if i == 0 {
				// tests[0] is the whole address/action data at block height 3
				continue
			}
			prevTipHeight, err := dao.GetTipHeight()
			require.NoError(err)
			prevTipHash, err := dao.GetBlockHash(prevTipHeight)
			require.NoError(err)
			require.NoError(dao.DeleteTipBlock())
			tipHeight, err := indexer.GetBlockchainHeight()
			require.NoError(err)
			require.EqualValues(prevTipHeight-1, tipHeight)
			tipHeight, err = dao.GetTipHeight()
			require.NoError(err)
			require.EqualValues(prevTipHeight-1, tipHeight)
			h, err := indexer.GetBlockHash(tipHeight)
			require.NoError(err)
			h1, err := dao.GetTipHash()
			require.NoError(err)
			require.Equal(h, h1)
			_, err = dao.GetBlockHash(prevTipHeight)
			require.Error(err)
			_, err = dao.GetBlockHeight(prevTipHash)
			require.Error(err)
			if i <= 2 {
				require.Equal(blks[2-i].HashBlock(), h)
			} else {
				require.Equal(hash.ZeroHash256, h)
			}
			total, err := indexer.GetTotalActions()
			require.NoError(err)
			require.EqualValues(daoTests[i].total, total)
			if total > 0 {
				_, err = indexer.GetActionHashFromIndex(1, total)
				require.Equal(db.ErrInvalid, errors.Cause(err))
				actions, err := indexer.GetActionHashFromIndex(0, total)
				require.NoError(err)
				require.Equal(actions, daoTests[i].hashTotal)
			}
			for j := range daoTests[i].actions {
				actionCount, err := indexer.GetActionCountByAddress(daoTests[i].actions[j].addr)
				require.NoError(err)
				require.EqualValues(len(daoTests[i].actions[j].hashes), actionCount)
				if actionCount > 0 {
					actions, err := indexer.GetActionsByAddress(daoTests[i].actions[j].addr, 0, actionCount)
					require.NoError(err)
					require.Equal(actions, daoTests[i].actions[j].hashes)
				}
			}
		}
		// cannot delete genesis block
		require.Error(dao.DeleteTipBlock())
	}

	t.Run("In-memory KV Store for blocks", func(t *testing.T) {
		indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), hash.ZeroHash256)
		require.NoError(t, err)
		testBlockDao(db.NewMemKVStore(), indexer, t)
	})
	path := "test-kv-store"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	indexFile, _ := ioutil.TempFile(os.TempDir(), path)
	indexPath := indexFile.Name()
	cfg := config.Default.DB
	t.Run("Bolt DB for blocks", func(t *testing.T) {
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
		testBlockDao(db.NewBoltDB(cfg), indexer, t)
	})

	t.Run("In-memory KV Store deletions", func(t *testing.T) {
		indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), hash.ZeroHash256)
		require.NoError(t, err)
		testDeleteDao(db.NewMemKVStore(), indexer, t)
	})
	t.Run("Bolt DB deletions", func(t *testing.T) {
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
		testDeleteDao(db.NewBoltDB(cfg), indexer, t)
	})
}

func BenchmarkBlockCache(b *testing.B) {
	test := func(cacheSize int, b *testing.B) {
		b.StopTimer()
		path := "test-kv-store"
		testFile, _ := ioutil.TempFile(os.TempDir(), path)
		testPath := testFile.Name()
		indexFile, _ := ioutil.TempFile(os.TempDir(), path)
		indexPath := indexFile.Name()
		cfg := config.DB{
			NumRetries: 1,
		}
		defer func() {
			require.NoError(b, os.RemoveAll(testPath))
			require.NoError(b, os.RemoveAll(indexPath))
		}()
		cfg.DbPath = indexPath
		indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg), hash.ZeroHash256)
		require.NoError(b, err)
		cfg.DbPath = testPath
		store := db.NewBoltDB(cfg)

		db := config.Default.DB
		db.MaxCacheSize = cacheSize
		blkDao := NewBlockDAO(store, indexer, false, db)
		require.NoError(b, blkDao.Start(context.Background()))
		defer func() {
			require.NoError(b, blkDao.Stop(context.Background()))
		}()
		prevHash := hash.ZeroHash256
		numBlks := 8640
		for i := 1; i <= numBlks; i++ {
			actions := make([]action.SealedEnvelope, 10)
			for j := 0; j < 10; j++ {
				actions[j], err = testutil.SignedTransfer(
					identityset.Address(j).String(),
					identityset.PrivateKey(j+1),
					1,
					unit.ConvertIotxToRau(1),
					nil,
					testutil.TestGasLimit,
					testutil.TestGasPrice,
				)
				require.NoError(b, err)
			}
			tb := block.TestingBuilder{}
			blk, err := tb.SetPrevBlockHash(prevHash).
				SetVersion(1).
				SetTimeStamp(time.Now()).
				SetHeight(uint64(i)).
				AddActions(actions...).
				SignAndBuild(identityset.PrivateKey(0))
			require.NoError(b, err)
			err = blkDao.PutBlock(&blk)
			require.NoError(b, err)
			prevHash = blk.HashBlock()
		}
		b.ResetTimer()
		b.StartTimer()
		for n := 0; n < b.N; n++ {
			hash, _ := indexer.GetBlockHash(uint64(rand.Intn(numBlks) + 1))
			_, _ = blkDao.GetBlock(hash)
		}
		b.StopTimer()
	}
	b.Run("cache", func(b *testing.B) {
		test(8640, b)
	})
	b.Run("no-cache", func(b *testing.B) {
		test(0, b)
	})
}
