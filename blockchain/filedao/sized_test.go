package filedao

import (
	"context"
	"crypto/tls"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
)

func TestBlockSize(t *testing.T) {
	t.Skip("used for estimating block size")
	r := require.New(t)
	conn, err := grpc.NewClient("api.mainnet.iotex.one:443", grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS12})))
	r.NoError(err)
	defer conn.Close()

	cli := iotexapi.NewAPIServiceClient(conn)
	for _, h := range []uint64{29276276} {
		resp, err := cli.GetRawBlocks(context.Background(), &iotexapi.GetRawBlocksRequest{
			StartHeight:         h,
			Count:               1,
			WithReceipts:        true,
			WithTransactionLogs: true,
		})
		r.NoError(err)
		r.Len(resp.Blocks, 1)
		deserializer := block.NewDeserializer(4689)
		blk, err := deserializer.FromBlockProto(resp.Blocks[0].Block)
		r.NoError(err)
		receipts := make([]*action.Receipt, 0, len(resp.Blocks[0].Receipts))
		for _, receiptpb := range resp.Blocks[0].Receipts {
			receipt := &action.Receipt{}
			receipt.ConvertFromReceiptPb(receiptpb)
			receipts = append(receipts, receipt)
		}
		blk.Receipts = receipts
		r.NoError(fillTransactionLog(receipts, resp.Blocks[0].TransactionLogs.Logs))
		data, err := convertToBlockStore(blk)
		r.NoError(err)
		t.Logf("block %d size= %d", h, len(data))
	}
}

func TestSizedDaoIntegrity(t *testing.T) {
	r := require.New(t)
	desr := block.NewDeserializer(4689)
	ctx := context.Background()
	blocks := make([]*block.Block, 0, 20)
	builder := block.NewTestingBuilder()
	h := hash.ZeroHash256
	for i := 1; i <= cap(blocks); i++ {
		blk := createTestingBlock(builder, uint64(i), h)
		blocks = append(blocks, blk)
		h = blk.HashBlock()
		t.Logf("block height %d hash: %x", i, h)
	}
	// case: putblocks 1 - 10
	datadir := t.TempDir()
	dao, err := NewSizedFileDao(10, datadir, desr)
	r.NoError(err)
	r.NoError(dao.Start(ctx))
	for i := 0; i < 10; i++ {
		r.NoError(dao.PutBlock(ctx, blocks[i]))
	}
	// verify
	testVerifyChainDB(t, dao, 1, 10)
	// case: put block not expected
	r.ErrorIs(dao.PutBlock(ctx, blocks[8]), ErrInvalidTipHeight)
	r.ErrorIs(dao.PutBlock(ctx, blocks[9]), ErrInvalidTipHeight)
	r.ErrorIs(dao.PutBlock(ctx, blocks[11]), ErrInvalidTipHeight)
	// case: put blocks 11 - 20
	for i := 10; i < 20; i++ {
		r.NoError(dao.PutBlock(ctx, blocks[i]))
		blk, err := dao.GetBlockByHeight(11)
		r.NoError(err)
		r.Equal(blocks[10].HashBlock(), blk.HashBlock(), "height %d", i)
	}
	// verify new blocks added and old blocks removed
	testVerifyChainDB(t, dao, 11, 20)
	testVerifyChainDBNotExists(t, dao, blocks[:10])
	// case: reload
	r.NoError(dao.Stop(ctx))
	dao, err = NewSizedFileDao(10, datadir, desr)
	r.NoError(err)
	r.NoError(dao.Start(ctx))
	// verify blocks reloaded
	testVerifyChainDB(t, dao, 11, 20)
	testVerifyChainDBNotExists(t, dao, blocks[:10])
	// case: damaged db
	inner := dao.(*sizedDao)
	_, err = inner.store.Put([]byte("damaged"))
	r.NoError(err)
	r.NoError(dao.Stop(ctx))
	dao, err = NewSizedFileDao(10, datadir, desr)
	r.NoError(err)
	// verify invalid data is ignored
	r.NoError(dao.Start(ctx))
	testVerifyChainDB(t, dao, 11, 20)
	testVerifyChainDBNotExists(t, dao, blocks[:10])
	// case: remove non-continous blocks
	inner = dao.(*sizedDao)
	inner.store.Delete(inner.heightToID[15])
	r.NoError(dao.Stop(ctx))
	dao, err = NewSizedFileDao(10, datadir, desr)
	r.NoError(err)
	// verify blocks 11 - 15 are removed
	r.NoError(dao.Start(ctx))
	testVerifyChainDB(t, dao, 16, 20)
	testVerifyChainDBNotExists(t, dao, blocks[:15])
	r.NoError(dao.Stop(ctx))
}

func TestSizedDaoStorage(t *testing.T) {
	r := require.New(t)
	desr := block.NewDeserializer(4689)
	ctx := context.Background()

	datadir := t.TempDir()
	dao, err := NewSizedFileDao(10, datadir, desr)
	r.NoError(err)
	r.NoError(dao.Start(ctx))

	// put empty block
	height := uint64(1)
	builder := block.NewTestingBuilder()
	h := hash.ZeroHash256
	blk := createTestingBlock(builder, height, h)
	height++
	h = blk.HashBlock()
	r.NoError(dao.PutBlock(ctx, blk))
	// put block with 5 actions
	blk = createTestingBlock(builder, height, h, withActionNum(5))
	height++
	h = blk.HashBlock()
	r.NoError(dao.PutBlock(ctx, blk))
	// put block with 100 actions
	blk = createTestingBlock(builder, height, h, withActionNum(100))
	height++
	h = blk.HashBlock()
	r.NoError(dao.PutBlock(ctx, blk))
	// put block with 1000 actions
	blk = createTestingBlock(builder, height, h, withActionNum(1000))
	height++
	h = blk.HashBlock()
	r.NoError(dao.PutBlock(ctx, blk))
	// put block with 5000 actions
	blk = createTestingBlock(builder, height, h, withActionNum(5000))
	height++
	h = blk.HashBlock()
	r.NoError(dao.PutBlock(ctx, blk))
	// put block with 10000 actions
	blk = createTestingBlock(builder, height, h, withActionNum(10000))
	height++
	h = blk.HashBlock()
	r.NoError(dao.PutBlock(ctx, blk))
	r.NoError(dao.Stop(ctx))
}

func testVerifyChainDBNotExists(t *testing.T, fd FileDAO, blocks []*block.Block) {
	r := require.New(t)
	for _, blk := range blocks {
		_, err := fd.GetBlockHash(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.GetBlockHeight(blk.HashBlock())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.GetBlockByHeight(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.GetBlock(blk.HashBlock())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.GetReceipts(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
		_, err = fd.TransactionLogs(blk.Height())
		r.ErrorIs(err, db.ErrNotExist)
	}
}
