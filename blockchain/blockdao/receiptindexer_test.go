// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestReceiptIndexer(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()
	testPath, err := testutil.PathOfTempFile("test-receipt-indexer")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()
	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	kvs := db.NewBoltDB(cfg)
	ri := NewReceiptIndexer(kvs, 3)
	r.NoError(ri.Start(ctx))
	for i := uint64(1); i <= 3; i++ {
		blk1 := createTestBlock(r, i, int(i))
		r.NoError(ri.PutBlock(ctx, blk1))
		height, err := ri.Height()
		r.NoError(err)
		r.EqualValues(i, height)
		receipts, err := ri.Receipts(i)
		r.NoError(err)
		r.Equal(int(i), len(receipts))
		for j, rcpt := range receipts {
			r.Equal(len(blk1.Receipts[j].Logs()), len(rcpt.Logs()))
			for k, lg := range rcpt.Logs() {
				r.NotEqual(blk1.Receipts[j].Logs()[k].NotFixTopicCopyBug, lg.NotFixTopicCopyBug)
				for m, n := range blk1.Receipts[j].Logs()[k].Topics {
					r.Equal(n, lg.Topics[m])
				}
				r.Equal(blk1.Receipts[j].Logs()[k].Data, lg.Data)
			}
		}
	}
	r.NoError(ri.PutBlock(ctx, createTestBlock(r, 4, 4)))
	r.NoError(ri.Stop(ctx))
	r.NoError(ri.Start(ctx))
	r.EqualValues(3, ri.height)
	r.NoError(ri.Stop(ctx))
}

func createTestBlock(require *require.Assertions, height uint64, numOfReceipts int) *block.Block {
	pb := &iotextypes.BlockHeader{
		Core: &iotextypes.BlockHeaderCore{
			Height:    height,
			Timestamp: timestamppb.Now(),
		},
		ProducerPubkey: identityset.PrivateKey(1).PublicKey().Bytes(),
	}
	blk := &block.Block{}
	require.NoError(blk.LoadFromBlockHeaderProto(pb))
	receipts := make([]*action.Receipt, numOfReceipts)
	for i := 0; i < numOfReceipts; i++ {
		actionHash := hash.BytesToHash256([]byte{byte(i)})
		receipts[i] = &action.Receipt{
			Status:            uint64(iotextypes.ReceiptStatus_Success),
			BlockHeight:       height,
			ActionHash:        actionHash,
			GasConsumed:       uint64(10000 + i),
			BlobGasUsed:       0,
			BlobGasPrice:      big.NewInt(0),
			ContractAddress:   "io",
			TxIndex:           uint32(i),
			EffectiveGasPrice: big.NewInt(int64(i)),
		}
		topics := make([]hash.Hash256, i+1)
		for j := 0; j <= i; j++ {
			topics[j] = hash.BytesToHash256([]byte{byte(10*i + j)})
		}
		receipts[i].AddLogs(&action.Log{
			Address:            "io",
			Topics:             topics,
			Data:               []byte{byte(i)},
			BlockHeight:        height,
			ActionHash:         actionHash,
			Index:              uint32(i),
			NotFixTopicCopyBug: true,
		})
	}
	blk.Receipts = receipts
	return blk
}
