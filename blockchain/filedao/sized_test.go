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
)

func TestBlockSize(t *testing.T) {
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
		data, err := convertToBlockStore(blk)
		r.NoError(err)
		t.Logf("block %d size= %d", h, len(data))
	}
}

func TestSizedDao(t *testing.T) {
	r := require.New(t)
	dao, err := NewSizedFileDao(10, t.TempDir(), block.NewDeserializer(4089))
	r.NoError(err)

	ctx := context.Background()
	r.NoError(dao.Start(ctx))
	r.NoError(testCommitBlocks(t, dao, 1, 100, hash.ZeroHash256))
	r.NoError(dao.Stop(ctx))
}
