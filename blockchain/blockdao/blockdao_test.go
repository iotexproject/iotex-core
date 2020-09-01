package blockdao

import (
	"context"
	"encoding/hex"
	"hash/fnv"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestChecksumNamespaceAndKeys(t *testing.T) {
	require := require.New(t)

	a := []hash.Hash256{
		// blockdao
		hash.BytesToHash256([]byte(blockHashHeightMappingNS)),
		hash.BytesToHash256([]byte(systemLogNS)),
		hash.BytesToHash256(topHeightKey),
		hash.BytesToHash256(topHashKey),
		hash.BytesToHash256(hashPrefix),
		// filedao_legacy
		hash.BytesToHash256([]byte(blockNS)),
		hash.BytesToHash256([]byte(blockHeaderNS)),
		hash.BytesToHash256([]byte(blockBodyNS)),
		hash.BytesToHash256([]byte(blockFooterNS)),
		hash.BytesToHash256([]byte(receiptsNS)),
		hash.BytesToHash256(heightPrefix),
		hash.BytesToHash256(heightToFileBucket),
	}

	checksum := crypto.NewMerkleTree(a)
	require.NotNil(checksum)
	h := checksum.HashTree()
	require.Equal("3ed359035cea947b14288bc0f581391c63d087e161d82b452e0353375cde2f0d", hex.EncodeToString(h[:]))
}

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
	require := require.New(t)

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

	testBlockDao := func(kvStore db.KVStore, t *testing.T) {
		dao := NewBlockDAO(kvStore, []BlockIndexer{}, false, config.Default.DB)
		ctx := protocol.WithBlockchainCtx(
			context.Background(),
			protocol.BlockchainCtx{
				Genesis: config.Default.Genesis,
			},
		)
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
			require.Equal(tipBlk, blk)
			blk, err = dao.GetBlockByHeight(height)
			require.NoError(err)
			require.Equal(tipBlk, blk)
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
				body, err := dao.Body(hash)
				require.NoError(err)
				require.Equal(&tipBlk.Body, body)
				footer, err := dao.Footer(hash)
				require.NoError(err)
				require.Equal(&tipBlk.Footer, footer)
				header, err = dao.HeaderByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				footer, err = dao.FooterByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Footer, footer)
			}
		}

		height, err := dao.Height()
		require.NoError(err)
		require.EqualValues(len(blks), height)

		// commit an existing block
		require.Equal(ErrAlreadyExist, dao.PutBlock(ctx, blks[2]))

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
			action, err := dao.GetActionByActionHash(h, blk.Height())
			require.NoError(err)
			require.Equal(blk.Actions[i%3], action)
		}
	}

	testDeleteDao := func(kvStore db.KVStore, t *testing.T) {
		dao := NewBlockDAO(kvStore, []BlockIndexer{}, false, config.Default.DB)
		ctx := protocol.WithBlockchainCtx(
			context.Background(),
			protocol.BlockchainCtx{
				Genesis: config.Default.Genesis,
			},
		)
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
				require.Equal(hash.ZeroHash256, h)
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
			require.Equal(tipBlk, blk)
			blk, err = dao.GetBlockByHeight(height)
			require.NoError(err)
			require.Equal(tipBlk, blk)

			// test BlockDAO's API, 2nd loop to test LRU cache
			for i := 0; i < 2; i++ {
				header, err := dao.Header(h)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				body, err := dao.Body(h)
				require.NoError(err)
				require.Equal(&tipBlk.Body, body)
				footer, err := dao.Footer(h)
				require.NoError(err)
				require.Equal(&tipBlk.Footer, footer)
				header, err = dao.HeaderByHeight(height)
				require.NoError(err)
				require.Equal(&tipBlk.Header, header)
				footer, err = dao.FooterByHeight(height)
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
				action, err := dao.GetActionByActionHash(h, blk.Height())
				require.NoError(err)
				require.Equal(blk.Actions[i%3], action)
			}
		}
	}

	t.Run("In-memory KV Store for blocks", func(t *testing.T) {
		testBlockDao(db.NewMemKVStore(), t)
	})
	path := "test-kv-store"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)

	cfg := config.Default.DB
	t.Run("Bolt DB for blocks", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer func() {
			testutil.CleanupPath(t, testPath)
		}()
		cfg.DbPath = testPath
		testBlockDao(db.NewBoltDB(cfg), t)
	})

	t.Run("In-memory KV Store deletions", func(t *testing.T) {
		testDeleteDao(db.NewMemKVStore(), t)
	})
	t.Run("Bolt DB deletions", func(t *testing.T) {
		testutil.CleanupPath(t, testPath)
		defer func() {
			testutil.CleanupPath(t, testPath)
		}()
		cfg.DbPath = testPath
		testDeleteDao(db.NewBoltDB(cfg), t)
	})
}

func BenchmarkBlockCache(b *testing.B) {
	test := func(cacheSize int, b *testing.B) {
		b.StopTimer()
		path := "test-kv-store"
		testPath, err := testutil.PathOfTempFile(path)
		require.NoError(b, err)
		indexPath, err := testutil.PathOfTempFile(path)
		require.NoError(b, err)
		cfg := config.DB{
			NumRetries: 1,
		}
		defer func() {
			require.NoError(b, os.RemoveAll(testPath))
			require.NoError(b, os.RemoveAll(indexPath))
		}()
		cfg.DbPath = indexPath
		cfg.DbPath = testPath
		store := db.NewBoltDB(cfg)

		db := config.Default.DB
		db.MaxCacheSize = cacheSize
		blkDao := NewBlockDAO(store, []BlockIndexer{}, false, db)
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
