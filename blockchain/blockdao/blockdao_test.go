package blockdao

import (
	"context"
	"hash/fnv"
	"math/big"
	"testing"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/blockchain/filedao"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/compress"
	"github.com/iotexproject/iotex-core/v2/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/v2/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_blockdao"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestNewBlockDAOWithIndexersAndCache(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var (
		blockdao = mock_blockdao.NewMockBlockDAO(ctrl)
		indexers = []BlockIndexer{mock_blockdao.NewMockBlockIndexer(ctrl)}
	)

	t.Run("BlockStoreIsNil", func(t *testing.T) {
		dao := NewBlockDAOWithIndexersAndCache(nil, []BlockIndexer{}, 100)
		r.Nil(dao)
	})
	t.Run("NeedNewCache", func(t *testing.T) {
		t.Run("FailedToNewPrometheusTimer", func(t *testing.T) {
			p := gomonkey.NewPatches()
			defer p.Reset()

			p = p.ApplyFuncReturn(prometheustimer.New, nil, errors.New(t.Name()))

			dao := NewBlockDAOWithIndexersAndCache(blockdao, indexers, 1)
			r.Nil(dao)
		})
	})
	t.Run("Success", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = p.ApplyFuncReturn(prometheustimer.New, nil, nil)

		dao := NewBlockDAOWithIndexersAndCache(blockdao, indexers, 1)
		r.NotNil(dao)
	})
}

func Test_blockDAO_Start(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockblockdao := mock_blockdao.NewMockBlockDAO(ctrl)
	blockdao := &blockDAO{
		lifecycle:  lifecycle.Lifecycle{},
		blockStore: mockblockdao,
	}

	t.Run("FailedToStartLifeCycle", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStart", errors.New(t.Name()))

		err := blockdao.Start(context.Background())
		r.ErrorContains(err, t.Name())
	})

	t.Run("FailedToGetBlockStoreHeight", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStart", nil)
		mockblockdao.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

		err := blockdao.Start(context.Background())
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		expectedHeight := uint64(1)

		p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStart", nil)
		mockblockdao.EXPECT().Height().Return(expectedHeight, nil).Times(1)
		p.ApplyPrivateMethod(&blockDAO{}, "checkIndexers", func(*blockDAO, context.Context) error { return nil })

		err := blockdao.Start(context.Background())
		r.NoError(err)
		r.Equal(blockdao.tipHeight, expectedHeight)
	})
}

func Test_blockDAO_checkIndexers(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockblockdao := mock_blockdao.NewMockBlockDAO(ctrl)
	mockblockindexer := mock_blockdao.NewMockBlockIndexer(ctrl)

	blockdao := &blockDAO{
		lifecycle:  lifecycle.Lifecycle{},
		blockStore: mockblockdao,
		indexers:   []BlockIndexer{mockblockindexer},
	}

	t.Run("FailedToCheckIndexer", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p.ApplyMethodReturn(&BlockIndexerChecker{}, "CheckIndexer", errors.New(t.Name()))

		err := blockdao.checkIndexers(context.Background())
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		var (
			tipHeight = uint64(0)
			daoTip    = uint64(0)
		)

		// // mock required context
		ctx := context.Background()
		ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{})
		ctx = genesis.WithGenesisContext(ctx, genesis.Genesis{})

		// mock tipHeight return
		mockblockindexer.EXPECT().Height().Return(tipHeight, nil).Times(1)
		// mock doaTip return
		mockblockdao.EXPECT().Height().Return(daoTip, nil).Times(1)
		mockblockdao.EXPECT().GetBlockByHeight(gomock.Any()).Return(&block.Block{}, nil).Times(1)

		err := blockdao.checkIndexers(ctx)
		r.NoError(err)
	})
}

func Test_blockDAO_Stop(t *testing.T) {
	r := require.New(t)

	dao := &blockDAO{lifecycle: lifecycle.Lifecycle{}}

	t.Run("FailedToStopLifecycle", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStop", errors.New(t.Name()))

		err := dao.Stop(context.Background())

		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		p := gomonkey.NewPatches()
		defer p.Reset()

		p = p.ApplyMethodReturn(&lifecycle.Lifecycle{}, "OnStop", nil)

		err := dao.Stop(context.Background())

		r.NoError(err)
	})
}

func Test_blockDAO_GetBlockHash(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToGetBlock", func(t *testing.T) {
		store.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, errors.New(t.Name())).Times(1)

		h, err := dao.GetBlockHash(100)

		r.Empty(h)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().GetBlockHash(gomock.Any()).Return(hash.Hash256{}, nil).Times(1)

		_, err := dao.GetBlockHash(100)

		r.NoError(err)
	})
}

func Test_blockDAO_GetBlockHeight(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToGetBlock", func(t *testing.T) {
		store.EXPECT().GetBlockHeight(gomock.Any()).Return(uint64(0), errors.New(t.Name())).Times(1)

		_, err := dao.GetBlockHeight(hash.Hash256{})

		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().GetBlockHeight(gomock.Any()).Return(uint64(100), nil).Times(1)

		height, err := dao.GetBlockHeight(hash.Hash256{})

		r.Equal(height, uint64(100))
		r.NoError(err)
	})
}

func Test_blockDAO_GetBlock(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToGetBlock", func(t *testing.T) {
		store.EXPECT().GetBlock(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		blk, err := dao.GetBlock(hash.Hash256{})

		r.Nil(blk)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().GetBlock(gomock.Any()).Return(&block.Block{}, nil).Times(1)

		blk, err := dao.GetBlock(hash.Hash256{})

		r.NotNil(blk)
		r.NoError(err)
	})
}

func Test_blockDAO_GetBlockByHeight(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToGetBlockByHash", func(t *testing.T) {
		store.EXPECT().GetBlockByHeight(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		blk, err := dao.GetBlockByHeight(100)

		r.Nil(blk)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().GetBlockByHeight(gomock.Any()).Return(&block.Block{}, nil).Times(1)

		blk, err := dao.GetBlockByHeight(100)

		r.NotNil(blk)
		r.NoError(err)
	})
}

func Test_blockDAO_HeaderByHeight(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{
		blockStore:  store,
		headerCache: cache.NewThreadSafeLruCache(100),
	}

	t.Run("HitCache", func(t *testing.T) {
		dao.headerCache.Add(uint64(100), &block.Header{})

		blk, err := dao.HeaderByHeight(100)

		r.NotNil(blk)
		r.NoError(err)
	})

	t.Run("FailedToGetHeaderFromBlockStore", func(t *testing.T) {
		store.EXPECT().HeaderByHeight(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		header, err := dao.HeaderByHeight(101)

		r.Nil(header)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().HeaderByHeight(gomock.Any()).Return(&block.Header{}, nil).Times(1)

		header, err := dao.HeaderByHeight(102)

		r.NotNil(header)
		r.NoError(err)
	})

}

func Test_blockDAO_FooterByHeight(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{
		blockStore:  store,
		footerCache: cache.NewThreadSafeLruCache(3),
	}

	t.Run("HitCache", func(t *testing.T) {
		dao.footerCache.Add(uint64(100), &block.Footer{})

		footer, err := dao.FooterByHeight(100)

		r.NotNil(footer)
		r.NoError(err)
	})

	t.Run("FailedToGetFooterFromBlockStore", func(t *testing.T) {
		store.EXPECT().FooterByHeight(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		footer, err := dao.FooterByHeight(101)

		r.Nil(footer)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().FooterByHeight(gomock.Any()).Return(&block.Footer{}, nil).Times(1)

		footer, err := dao.FooterByHeight(102)

		r.NotNil(footer)
		r.NoError(err)
	})
}

func Test_blockDAO_Height(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToGetHeightFromBlockStore", func(t *testing.T) {
		store.EXPECT().Height().Return(uint64(0), errors.New(t.Name())).Times(1)

		height, err := dao.Height()

		r.Zero(height)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().Height().Return(uint64(1000), nil).Times(1)

		height, err := dao.Height()

		r.Equal(height, uint64(1000))
		r.NoError(err)
	})
}

func Test_blockDAO_Header(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{
		blockStore:  store,
		headerCache: cache.NewThreadSafeLruCache(3),
	}

	t.Run("HitCache", func(t *testing.T) {
		h := hash.Hash256{0x76, 0x6d, 0x8b, 0x5b, 0x98, 0xa5, 0xb2, 0xdb, 0x8d, 0x99, 0x0, 0xd2, 0x9c, 0xd1, 0x31, 0xf1, 0x59, 0xb6, 0x2f, 0x7e, 0x74, 0x6b, 0x92, 0x1b, 0x42, 0x68, 0x97, 0x4a, 0x47, 0x3e, 0x8d, 0xc5}
		dao.headerCache.Add(h, &block.Header{})

		header, err := dao.Header(h)

		r.NotNil(header)
		r.NoError(err)
	})

	t.Run("FailedToGetHeaderFromBlockStore", func(t *testing.T) {
		store.EXPECT().Header(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		header, err := dao.Header(hash.Hash256{})

		r.Nil(header)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().Header(gomock.Any()).Return(&block.Header{}, nil).Times(1)

		header, err := dao.Header(hash.Hash256{})

		r.NotNil(header)
		r.NoError(err)
	})
}

func Test_blockDAO_GetReceipts(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToGetReceipts", func(t *testing.T) {
		store.EXPECT().GetReceipts(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		receipts, err := dao.GetReceipts(100)

		r.Nil(receipts)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().GetReceipts(gomock.Any()).Return([]*action.Receipt{{}, {}}, nil).Times(1)

		receipts, err := dao.GetReceipts(100)

		r.Len(receipts, 2)
		r.NoError(err)
	})
}

func Test_blockDAO_ContainsTransactionLog(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	store.EXPECT().ContainsTransactionLog().Return(true).Times(1)
	r.True(dao.ContainsTransactionLog())
	store.EXPECT().ContainsTransactionLog().Return(false).Times(1)
	r.False(dao.ContainsTransactionLog())
}

func Test_blockDAO_TransactionLogs(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{blockStore: store}

	t.Run("FailedToTransactionLogs", func(t *testing.T) {
		store.EXPECT().TransactionLogs(gomock.Any()).Return(nil, errors.New(t.Name())).Times(1)

		txlogs, err := dao.TransactionLogs(0)

		r.Nil(txlogs)
		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().TransactionLogs(gomock.Any()).Return(&iotextypes.TransactionLogs{}, nil).Times(1)

		txlogs, err := dao.TransactionLogs(0)

		r.NotNil(txlogs)
		r.NoError(err)
	})
}

func Test_blockDAO_PutBlock(t *testing.T) {
	r := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	indexer := mock_blockdao.NewMockBlockIndexer(ctrl)
	store := mock_blockdao.NewMockBlockDAO(ctrl)

	dao := &blockDAO{
		indexers:   []BlockIndexer{indexer},
		blockStore: store,
	}
	ctx := genesis.WithGenesisContext(context.Background(), genesis.TestDefault())
	blk, err := block.NewTestingBuilder().SignAndBuild(identityset.PrivateKey(1))
	r.NoError(err)
	t.Run("FailedToPutBlockToBlockStore", func(t *testing.T) {
		store.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(errors.New(t.Name())).Times(1)

		err := dao.PutBlock(context.Background(), &block.Block{})

		r.ErrorContains(err, t.Name())
	})

	t.Run("FailedToPutBlockToIndexer", func(t *testing.T) {
		store.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		indexer.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(errors.New(t.Name())).Times(1)
		err = dao.PutBlock(ctx, &blk)

		r.ErrorContains(err, t.Name())
	})

	t.Run("Success", func(t *testing.T) {
		store.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(nil).Times(1)
		indexer.EXPECT().PutBlock(gomock.Any(), gomock.Any()).Return(nil).Times(1)

		err := dao.PutBlock(ctx, &blk)

		r.NoError(err)
	})
}

func Test_lruCache(t *testing.T) {
	r := require.New(t)

	_cache := cache.NewThreadSafeLruCache(5)

	t.Run("EmptyCache", func(t *testing.T) {
		lruCachePut(nil, "any", "any")
		v, ok := lruCacheGet(nil, "any")
		r.Nil(v)
		r.False(ok)
	})

	t.Run("ShouldEqualAndFalse", func(t *testing.T) {
		v1, ok1 := lruCacheGet(_cache, "any")
		v2, ok2 := _cache.Get("any")
		r.Equal(v1, v2)
		r.Equal(ok1, ok2)
		r.False(ok1)
	})

	t.Run("AfterPutShouldEqualAndTrue", func(t *testing.T) {
		lruCachePut(_cache, "any", "any")

		v1, ok1 := lruCacheGet(_cache, "any")
		v2, ok2 := _cache.Get("any")
		r.Equal(v1, v2)
		r.Equal(ok1, ok2)
		r.True(ok1)
	})
}

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

// TODO: Move the test to filedao. The test is not for BlockDAO, but filedao.
// The current blockDAO's implementation is more about indexers and cache, which
// are not covered in the unit tests.
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

	testBlockDao := func(dao BlockStore, t *testing.T) {
		ctx := protocol.WithBlockchainCtx(
			genesis.WithGenesisContext(context.Background(), genesis.TestDefault()),
			protocol.BlockchainCtx{
				ChainID: 1,
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
			receipt, err := receiptByActionHash(receipts[i/3], h)
			require.NoError(err)
			b1, err := receipt.Serialize()
			require.NoError(err)
			b2, err := receipts[i/3][i%3].Serialize()
			require.NoError(err)
			require.Equal(b1, b2)
			action, actIndex, err := blk.ActionByHash(h)
			require.NoError(err)
			require.Equal(int(actIndex), i%3)
			require.Equal(blk.Actions[i%3], action)
		}
	}

	testDeleteDao := func(dao filedao.FileDAO, t *testing.T) {
		ctx := protocol.WithBlockchainCtx(
			genesis.WithGenesisContext(context.Background(), genesis.TestDefault()),
			protocol.BlockchainCtx{
				ChainID: 1,
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
			for {
				height, err := dao.Height()
				require.NoError(err)
				if height <= prevTipHeight-1 {
					break
				}
				require.NoError(dao.DeleteTipBlock())
			}
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
				receipt, err := receiptByActionHash(receipts[i/3], h)
				require.NoError(err)
				b1, err := receipt.Serialize()
				require.NoError(err)
				b2, err := receipts[i/3][i%3].Serialize()
				require.NoError(err)
				require.Equal(b1, b2)
				action, actIndex, err := blk.ActionByHash(h)
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
	g := genesis.TestDefault()
	genesis.SetGenesisTimestamp(g.Timestamp)
	block.LoadGenesisHash(&g)
	for _, v := range daoList {
		testutil.CleanupPath(testPath)
		dao, err := createFileDAO(v.inMemory, v.legacy, v.compressBlock, cfg)
		require.NoError(err)
		require.NotNil(dao)
		t.Run("test store blocks", func(t *testing.T) {
			testBlockDao(dao, t)
		})
	}

	for _, v := range daoList {
		testutil.CleanupPath(testPath)
		dao, err := createFileDAO(v.inMemory, v.legacy, v.compressBlock, cfg)
		require.NoError(err)
		require.NotNil(dao)
		t.Run("test delete blocks", func(t *testing.T) {
			testDeleteDao(dao, t)
		})
	}
}

func createFileDAO(inMemory, legacy bool, compressBlock string, cfg db.Config) (filedao.FileDAO, error) {
	if inMemory {
		return filedao.NewFileDAOInMemForTest()
	}
	deser := block.NewDeserializer(4689)
	if legacy {
		return filedao.CreateFileDAO(true, cfg, deser)
	}

	cfg.Compressor = compressBlock
	return filedao.NewFileDAO(cfg, deser)
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
		deser := block.NewDeserializer(4689)
		fileDAO, err := filedao.NewFileDAO(cfg, deser)
		require.NoError(b, err)
		blkDao := NewBlockDAOWithIndexersAndCache(fileDAO, []BlockIndexer{}, cfg.MaxCacheSize)
		require.NoError(b, blkDao.Start(context.Background()))
		defer func() {
			require.NoError(b, blkDao.Stop(context.Background()))
		}()
		prevHash := hash.ZeroHash256
		numBlks := 8640
		for i := 1; i <= numBlks; i++ {
			actions := make([]*action.SealedEnvelope, 10)
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

func receiptByActionHash(receipts []*action.Receipt, h hash.Hash256) (*action.Receipt, error) {
	for _, r := range receipts {
		if r.ActionHash == h {
			return r, nil
		}
	}
	return nil, errors.Errorf("receipt of action %x isn't found", h)
}
