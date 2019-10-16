package blockdao

import (
	"context"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/pkg/util/fileutil"
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
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf1, tsf4, execution1).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash2 := hash.Hash256{}
	fnv.New32().Sum(hash2[:])
	blk2, err := block.NewTestingBuilder().
		SetHeight(2).
		SetPrevBlockHash(hash2).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf2, tsf5, execution2).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)

	hash3 := hash.Hash256{}
	fnv.New32().Sum(hash3[:])
	blk3, err := block.NewTestingBuilder().
		SetHeight(3).
		SetPrevBlockHash(hash3).
		SetTimeStamp(testutil.TimestampNow()).
		AddActions(tsf3, tsf6, execution3).
		SignAndBuild(identityset.PrivateKey(27))
	require.NoError(t, err)
	return []*block.Block{&blk1, &blk2, &blk3}
}

func TestBlockDAO(t *testing.T) {

	blks := getTestBlocks(t)
	assert.Equal(t, 3, len(blks))
	blkHash := []hash.Hash256{
		blks[0].HashBlock(), blks[1].HashBlock(), blks[2].HashBlock(),
	}

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

	testBlockDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		blkDAO := NewBlockDAO(kvstore, false, false, 0, config.Default.DB)
		dao, ok := blkDAO.(*blockDAO)
		require.True(t, ok)
		err := dao.Start(ctx)
		assert.Nil(t, err)
		defer func() {
			require.NoError(t, dao.Stop(ctx))
		}()

		height, err := dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), height)

		// block put order is 0 2 1
		err = dao.putBlock(blks[0])
		assert.Nil(t, err)
		blk, err := dao.getBlock(blkHash[0])
		assert.Nil(t, err)
		require.NotNil(t, blk)
		assert.Equal(t, blks[0].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.GetBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(1), height)

		err = dao.PutBlock(blks[2])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blkHash[2])
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[2].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		err = dao.putBlock(blks[1])
		assert.Nil(t, err)
		blk, err = dao.getBlock(blkHash[1])
		assert.Nil(t, err)
		assert.NotNil(t, blk)
		assert.Equal(t, blks[1].Actions[0].Hash(), blk.Actions[0].Hash())
		height, err = dao.getBlockchainHeight()
		assert.Nil(t, err)
		assert.Equal(t, uint64(3), height)

		// test hash <--> height mapping
		for i := uint64(0); i < 3; i++ {
			hash, err := dao.getBlockHash(blks[i].Height())
			assert.NoError(t, err)
			assert.Equal(t, blkHash[i], hash)
			height, err = dao.getBlockHeight(hash)
			assert.Nil(t, err)
			assert.Equal(t, blks[i].Height(), height)
		}

		// test getTipHash
		hash, err := dao.getTipHash()
		assert.Nil(t, err)
		assert.Equal(t, blks[2].HashBlock(), hash)
	}

	testActionsDao := func(kvstore db.KVStore, t *testing.T) {
		ctx := context.Background()
		blkDAO := NewBlockDAO(kvstore, true, false, 0, config.Default.DB)
		dao, ok := blkDAO.(*blockDAO)
		require.True(t, ok)
		err := dao.Start(ctx)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, dao.Stop(ctx))
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

		for i := uint64(0); i < 3; i++ {
			require.NoError(t, dao.PutBlock(blks[i]))
			require.NoError(t, dao.PutReceipts(i+1, receipts[i]))
		}

		// Test getBlockHashByActionHash
		h, err := dao.getBlockHashByActionHash(t1Hash)
		require.NoError(t, err)
		require.Equal(t, blkHash[0], h)
		h, err = dao.getBlockHashByActionHash(t2Hash)
		require.NoError(t, err)
		require.Equal(t, blkHash[1], h)
		h, err = dao.getBlockHashByActionHash(t3Hash)
		require.NoError(t, err)
		require.Equal(t, blkHash[2], h)

		// Test getReceiptByActionHash
		for j := range daoTests[0].hashTotal {
			h := hash.BytesToHash256(daoTests[0].hashTotal[j])
			receipt, err := dao.getReceiptByActionHash(h)
			require.NoError(t, err)
			require.Equal(t, receipts[j/3][j%3], receipt)
		}

		// Test get actions
		total, err := dao.getTotalActions()
		require.NoError(t, err)
		require.EqualValues(t, daoTests[0].total, total)
		_, err = dao.getActionHashFromIndex(1, total)
		require.Equal(t, db.ErrInvalid, errors.Cause(err))
		actions, err := dao.getActionHashFromIndex(0, total)
		require.NoError(t, err)
		require.Equal(t, actions, daoTests[0].hashTotal)
		for j := range daoTests[0].actions {
			actionCount, err := dao.getActionCountByAddress(daoTests[0].actions[j].addr)
			require.NoError(t, err)
			require.EqualValues(t, len(daoTests[0].actions[j].hashes), actionCount)
			if actionCount > 0 {
				actions, err := dao.getActionsByAddress(daoTests[0].actions[j].addr, 0, actionCount)
				require.NoError(t, err)
				require.Equal(t, actions, daoTests[0].actions[j].hashes)
			}
		}

		// non-existing address has 0 actions
		actionCount, err := dao.getActionCountByAddress(hash.BytesToHash160(identityset.Address(13).Bytes()))
		require.NoError(t, err)
		require.EqualValues(t, 0, actionCount)

		for i := 0; i < 3; i++ {
			amount := big.NewInt(0)
			tsfs, _ := action.ClassifyActions(blks[i].Actions)
			for _, tsf := range tsfs {
				amount.Add(amount, tsf.Amount())
			}

			// test getNumActions
			numActions, err := dao.getNumActions(blks[i].Height())
			require.NoError(t, err)
			require.EqualValues(t, len(blks[i].Actions), numActions)

			// test getTranferAmount
			transferAmount, err := dao.getTranferAmount(blks[i].Height())
			require.NoError(t, err)
			require.Equal(t, amount, transferAmount)
		}
	}

	testDeleteDao := func(kvstore db.KVStore, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		blkDAO := NewBlockDAO(kvstore, true, false, 0, config.Default.DB)
		dao, ok := blkDAO.(*blockDAO)
		require.True(ok)
		err := dao.Start(ctx)
		require.NoError(err)
		defer func() {
			require.NoError(dao.Stop(ctx))
		}()

		// Put blocks first
		require.NoError(dao.putBlock(blks[0]))
		require.NoError(dao.putBlock(blks[1]))
		require.NoError(dao.putBlock(blks[2]))

		// delete tip block one by one, verify address/action after each deletion
		for i := range daoTests {
			if i == 0 {
				// tests[0] is the whole address/action data at block height 3
				continue
			}
			err = dao.deleteTipBlock()
			require.NoError(err)
			tipHeight, err := dao.getBlockchainHeight()
			require.NoError(err)
			require.EqualValues(uint64(3-i), tipHeight)
			h, err := dao.getTipHash()
			require.NoError(err)
			if i <= 2 {
				require.Equal(blks[2-i].HashBlock(), h)
			} else {
				require.Equal(hash.ZeroHash256, h)
			}
			total, err := dao.getTotalActions()
			require.NoError(err)
			require.EqualValues(daoTests[i].total, total)
			if total > 0 {
				_, err = dao.getActionHashFromIndex(1, total)
				require.Equal(db.ErrInvalid, errors.Cause(err))
				actions, err := dao.getActionHashFromIndex(0, total)
				require.NoError(err)
				require.Equal(actions, daoTests[i].hashTotal)
			}
			for j := range daoTests[i].actions {
				actionCount, err := dao.getActionCountByAddress(daoTests[i].actions[j].addr)
				require.NoError(err)
				require.EqualValues(len(daoTests[i].actions[j].hashes), actionCount)
				if actionCount > 0 {
					actions, err := dao.getActionsByAddress(daoTests[i].actions[j].addr, 0, actionCount)
					require.NoError(err)
					require.Equal(actions, daoTests[i].actions[j].hashes)
				}
			}
		}
		// cannot delete genesis block
		require.Equal("cannot delete genesis block", dao.deleteTipBlock().Error())
	}

	t.Run("In-memory KV Store for blocks", func(t *testing.T) {
		testBlockDao(db.NewMemKVStore(), t)
	})
	path := "test-kv-store"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg := config.Default.DB
	cfg.DbPath = testPath
	t.Run("Bolt DB for blocks", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer testutil.CleanupPath(t, testPath)
		testBlockDao(db.NewBoltDB(cfg), t)
	})

	t.Run("In-memory KV Store for actions", func(t *testing.T) {
		testActionsDao(db.NewMemKVStore(), t)
	})
	t.Run("Bolt DB for actions", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer testutil.CleanupPath(t, testPath)
		testActionsDao(db.NewBoltDB(cfg), t)
	})

	t.Run("In-memory KV Store deletions", func(t *testing.T) {
		testDeleteDao(db.NewMemKVStore(), t)
	})
	t.Run("Bolt DB deletions", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer testutil.CleanupPath(t, testPath)
		testDeleteDao(db.NewBoltDB(cfg), t)
	})
}

func BenchmarkBlockCache(b *testing.B) {
	test := func(cacheSize int, b *testing.B) {
		b.StopTimer()
		path := filepath.Join(os.TempDir(), fmt.Sprintf("test-%d.db", rand.Int()))
		cfg := config.DB{
			DbPath:     path,
			NumRetries: 1,
		}
		defer func() {
			if !fileutil.FileExists(path) {
				return
			}
			require.NoError(b, os.RemoveAll(path))
		}()
		store := db.NewBoltDB(cfg)

		blkDao := NewBlockDAO(store, false, false, cacheSize, config.Default.DB)
		dao, ok := blkDao.(*blockDAO)
		require.True(b, ok)
		require.NoError(b, blkDao.Start(context.Background()))
		defer func() {
			require.NoError(b, blkDao.Stop(context.Background()))
		}()
		prevHash := hash.ZeroHash256
		var err error
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
			err = dao.putBlock(&blk)
			require.NoError(b, err)
			prevHash = blk.HashBlock()
		}
		b.ResetTimer()
		b.StartTimer()
		for n := 0; n < b.N; n++ {
			hash, _ := dao.getBlockHash(uint64(rand.Intn(numBlks) + 1))
			_, _ = dao.getBlock(hash)
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
