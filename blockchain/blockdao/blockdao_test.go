package blockdao

import (
	"context"
	"hash/fnv"
	"math/big"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/filedao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func getTestBlocks(t *testing.T) []*block.Block {
	amount := uint64(50 << 22)
	tsf1, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(28), 1, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf2, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(29), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf3, err := action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf4, err := action.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(28), 2, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf5, err := action.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(29), 3, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	tsf6, err := action.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(30), 4, big.NewInt(int64(amount)), nil, testutil.TestGasLimit, big.NewInt(0))
	require.NoError(t, err)

	// create testing executions
	execution1, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 1, big.NewInt(1), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution2, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(29), 2, big.NewInt(0), 0, big.NewInt(0), nil)
	require.NoError(t, err)
	execution3, err := action.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 3, big.NewInt(2), 0, big.NewInt(0), nil)
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

	// receipts for the 3 blocks
	receipts := [][]*action.Receipt{
		{
			{Status: 1, BlockHeight: 1, ActionHash: t1Hash, GasConsumed: 15, ContractAddress: "1"},
			{Status: 0, BlockHeight: 1, ActionHash: t4Hash, GasConsumed: 216, ContractAddress: "2"},
			{Status: 2, BlockHeight: 1, ActionHash: e1Hash, GasConsumed: 6, ContractAddress: "3"},
		},
		{
			{Status: 3, BlockHeight: 2, ActionHash: t2Hash, GasConsumed: 1500, ContractAddress: "1"},
			{Status: 5, BlockHeight: 2, ActionHash: t5Hash, GasConsumed: 34, ContractAddress: "2"},
			{Status: 9, BlockHeight: 2, ActionHash: e2Hash, GasConsumed: 655, ContractAddress: "3"},
		},
		{
			{Status: 7, BlockHeight: 3, ActionHash: t3Hash, GasConsumed: 488, ContractAddress: "1"},
			{Status: 6, BlockHeight: 3, ActionHash: t6Hash, GasConsumed: 2, ContractAddress: "2"},
			{Status: 2, BlockHeight: 3, ActionHash: e3Hash, GasConsumed: 1099, ContractAddress: "3"},
		},
	}

	testBlockDao := func(dao BlockDAO, t *testing.T) {
		ctx := protocol.WithBlockchainCtx(
			genesis.WithGenesisContext(context.Background(), genesis.Default),
			protocol.BlockchainCtx{
				ChainID: config.Default.Chain.ID,
			})
		require.NoError(dao.Start(ctx))
		defer func() {
			require.NoError(dao.Stop(ctx))
		}()
		require.True(dao.ContainsTransactionLog())

		for i := 0; i < 3; i++ {
			// test putBlock/Receipt
			blks[i].Receipts = receipts[i]
			require.NoError(dao.PutBlock(ctx, blks[i]))
			blks[i].Receipts = nil
			tipBlk := blks[i]

			// test FileDAO's API
			hash, err := dao.GetBlockHash(tipBlk.Height())
			require.NoError(err)
			require.Equal(tipBlk.HashBlock(), hash)
			height, err := dao.GetBlockHeight(hash)
			require.NoError(err)
			require.Equal(tipBlk.Height(), height)
			blk, err := dao.GetBlock(hash)
			require.NoError(err)
			require.Equal(len(blk.Actions), len(tipBlk.Actions))
			for i := 0; i < len(blk.Actions); i++ {
				hashVal1, hashErr1 := blk.Actions[i].Hash()
				require.NoError(hashErr1)
				hashVal2, hashErr2 := tipBlk.Actions[i].Hash()
				require.NoError(hashErr2)
				require.Equal(hashVal1, hashVal2)
			}
			blk, err = dao.GetBlockByHeight(height)
			require.NoError(err)
			require.Equal(len(blk.Actions), len(tipBlk.Actions))
			for i := 0; i < len(blk.Actions); i++ {
				hashVal1, hashErr1 := blk.Actions[i].Hash()
				require.NoError(hashErr1)
				hashVal2, hashErr2 := tipBlk.Actions[i].Hash()
				require.NoError(hashErr2)
				require.Equal(hashVal1, hashVal2)
			}
			r, err := dao.GetReceipts(height)
			require.NoError(err)
			require.Equal(len(receipts[i]), len(r))
			for j := range receipts[i] {
				b1, err := r[j].Serialize()
				require.NoError(err)
				b2, err := receipts[i][j].Serialize()
				require.NoError(err)
				require.Equal(b1, b2)
			}

			// test BlockDAO's API, 2nd loop to test LRU cache
			for i := 0; i < 2; i++ {
				header, err := dao.Header(hash)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				header, err = dao.HeaderByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				footer, err := dao.FooterByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Footer, footer)
			}
		}

		height, err := dao.Height()
		require.NoError(err)
		require.EqualValues(len(blks), height)

		// commit an existing block
		require.Equal(filedao.ErrAlreadyExist, dao.PutBlock(ctx, blks[2]))

		// check non-exist block
		h, err := dao.GetBlockHash(5)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Equal(hash.ZeroHash256, h)
		height, err = dao.GetBlockHeight(hash.ZeroHash256)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.EqualValues(0, height)
		blk, err := dao.GetBlock(hash.ZeroHash256)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Nil(blk)
		blk, err = dao.GetBlockByHeight(5)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Nil(blk)
		r, err := dao.GetReceipts(5)
		require.Equal(db.ErrNotExist, errors.Cause(err))
		require.Nil(r)

		// Test GetReceipt/ActionByActionHash
		for i, v := range daoTests[0].hashTotal {
			blk := blks[i/3]
			h := hash.BytesToHash256(v)
			receipt, err := dao.GetReceiptByActionHash(h, blk.Height())
			require.NoError(err)
			b1, err := receipt.Serialize()
			require.NoError(err)
			b2, err := receipts[i/3][i%3].Serialize()
			require.NoError(err)
			require.Equal(b1, b2)
			action, actIndex, err := dao.GetActionByActionHash(h, blk.Height())
			require.NoError(err)
			require.Equal(int(actIndex), i%3)
			require.Equal(blk.Actions[i%3], action)
		}
	}

	testDeleteDao := func(dao BlockDAO, t *testing.T) {
		ctx := protocol.WithBlockchainCtx(
			genesis.WithGenesisContext(context.Background(), genesis.Default),
			protocol.BlockchainCtx{
				ChainID: config.Default.Chain.ID,
			})
		require.NoError(dao.Start(ctx))
		defer func() {
			require.NoError(dao.Stop(ctx))
		}()

		// put blocks
		for i := 0; i < 3; i++ {
			blks[i].Receipts = receipts[i]
			require.NoError(dao.PutBlock(ctx, blks[i]))
			blks[i].Receipts = nil
		}

		// delete tip block one by one, verify address/action after each deletion
		for i, action := range daoTests {
			if i == 0 {
				// tests[0] is the whole address/action data at block height 3
				continue
			}
			prevTipHeight, err := dao.Height()
			require.NoError(err)
			prevTipHash, err := dao.GetBlockHash(prevTipHeight)
			require.NoError(err)
			require.NoError(dao.DeleteBlockToTarget(prevTipHeight - 1))
			tipHeight, err := dao.Height()
			require.NoError(err)
			require.EqualValues(prevTipHeight-1, tipHeight)
			_, err = dao.GetBlockHash(prevTipHeight)
			require.Error(err)
			_, err = dao.GetBlockHeight(prevTipHash)
			require.Error(err)

			if tipHeight == 0 {
				h, err := dao.GetBlockHash(0)
				require.NoError(err)
				require.Equal(block.GenesisHash(), h)
				continue
			}
			tipBlk := blks[tipHeight-1]
			require.Equal(tipBlk.Height(), tipHeight)

			// test FileDAO's API
			h, err := dao.GetBlockHash(tipHeight)
			require.NoError(err)
			require.Equal(tipBlk.HashBlock(), h)
			height, err := dao.GetBlockHeight(h)
			require.NoError(err)
			require.Equal(tipHeight, height)
			blk, err := dao.GetBlock(h)
			require.NoError(err)
			require.Equal(len(blk.Actions), len(tipBlk.Actions))
			for i := 0; i < len(blk.Actions); i++ {
				hashVal1, hashErr1 := blk.Actions[i].Hash()
				require.NoError(hashErr1)
				hashVal2, hashErr2 := tipBlk.Actions[i].Hash()
				require.NoError(hashErr2)
				require.Equal(hashVal1, hashVal2)
			}
			blk, err = dao.GetBlockByHeight(height)
			require.NoError(err)
			require.Equal(len(blk.Actions), len(tipBlk.Actions))
			for i := 0; i < len(blk.Actions); i++ {
				hashVal1, hashErr1 := blk.Actions[i].Hash()
				require.NoError(hashErr1)
				hashVal2, hashErr2 := tipBlk.Actions[i].Hash()
				require.NoError(hashErr2)
				require.Equal(hashVal1, hashVal2)
			}

			// test BlockDAO's API, 2nd loop to test LRU cache
			for i := 0; i < 2; i++ {
				header, err := dao.Header(h)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				header, err = dao.HeaderByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				footer, err := dao.FooterByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Footer, footer)
			}

			// Test GetReceipt/ActionByActionHash
			for i, v := range action.hashTotal {
				blk := blks[i/3]
				h := hash.BytesToHash256(v)
				receipt, err := dao.GetReceiptByActionHash(h, blk.Height())
				require.NoError(err)
				b1, err := receipt.Serialize()
				require.NoError(err)
				b2, err := receipts[i/3][i%3].Serialize()
				require.NoError(err)
				require.Equal(b1, b2)
				action, actIndex, err := dao.GetActionByActionHash(h, blk.Height())
				require.NoError(err)
				require.Equal(int(actIndex), i%3)
				require.Equal(blk.Actions[i%3], action)
			}
		}
	}

	testPath, err := testutil.PathOfTempFile("test-kv-store")
	require.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	daoList := []struct {
		inMemory, legacy bool
		compressBlock    string
	}{
		{true, false, ""},
		{false, true, ""},
		{false, true, compress.Gzip},
		{false, false, ""},
		{false, false, compress.Gzip},
		{false, false, compress.Snappy},
	}

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	genesis.SetGenesisTimestamp(genesis.Default.Timestamp)
	block.LoadGenesisHash(&genesis.Default)
	for _, v := range daoList {
		testutil.CleanupPath(testPath)
		dao, err := createTestBlockDAO(v.inMemory, v.legacy, v.compressBlock, cfg)
		require.NoError(err)
		require.NotNil(dao)
		t.Run("test store blocks", func(t *testing.T) {
			testBlockDao(dao, t)
		})
	}

	for _, v := range daoList {
		testutil.CleanupPath(testPath)
		dao, err := createTestBlockDAO(v.inMemory, v.legacy, v.compressBlock, cfg)
		require.NoError(err)
		require.NotNil(dao)
		t.Run("test delete blocks", func(t *testing.T) {
			testDeleteDao(dao, t)
		})
	}
}

func createTestBlockDAO(inMemory, legacy bool, compressBlock string, cfg db.Config) (BlockDAO, error) {
	if inMemory {
		return NewBlockDAOInMemForTest(nil), nil
	}
	deser := block.NewDeserializer(config.Default.Chain.EVMNetworkID)
	if legacy {
		fileDAO, err := filedao.CreateFileDAO(true, cfg, deser)
		if err != nil {
			return nil, err
		}
		return createBlockDAO(fileDAO, nil, cfg), nil
	}

	cfg.Compressor = compressBlock
	return NewBlockDAO(nil, cfg, deser), nil
}

func BenchmarkBlockCache(b *testing.B) {
	test := func(cacheSize int, b *testing.B) {
		b.StopTimer()
		path := "test-kv-store"
		testPath, err := testutil.PathOfTempFile(path)
		require.NoError(b, err)
		indexPath, err := testutil.PathOfTempFile(path)
		require.NoError(b, err)
		cfg := db.Config{
			NumRetries: 1,
		}
		defer func() {
			testutil.CleanupPath(testPath)
			testutil.CleanupPath(indexPath)
		}()
		cfg.DbPath = indexPath
		cfg.DbPath = testPath
		cfg.MaxCacheSize = cacheSize
		deser := block.NewDeserializer(config.Default.Chain.EVMNetworkID)
		blkDao := NewBlockDAO([]BlockIndexer{}, cfg, deser)
		require.NoError(b, blkDao.Start(context.Background()))
		defer func() {
			require.NoError(b, blkDao.Stop(context.Background()))
		}()
		prevHash := hash.ZeroHash256
		numBlks := 8640
		for i := 1; i <= numBlks; i++ {
			actions := make([]action.SealedEnvelope, 10)
			for j := 0; j < 10; j++ {
				actions[j], err = action.SignedTransfer(
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
			err = blkDao.PutBlock(context.Background(), &blk)
			require.NoError(b, err)
			prevHash = blk.HashBlock()
		}
		b.ResetTimer()
	}
	b.Run("cache", func(b *testing.B) {
		test(8640, b)
	})
	b.Run("no-cache", func(b *testing.B) {
		test(0, b)
	})
}
