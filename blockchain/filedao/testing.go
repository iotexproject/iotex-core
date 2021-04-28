// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

type (
	// testInMemFd is an in-memory FileDAO
	testInMemFd struct {
		*fileDAOv2
	}

	// testFailPutBlock will fail PutBlock() at certain height
	testFailPutBlock struct {
		*fileDAO
		failHeight uint64
	}
)

func newTestInMemFd() (*testInMemFd, error) {
	v2, err := newFileDAOv2InMem(1)
	if err != nil {
		return nil, err
	}
	return &testInMemFd{fileDAOv2: v2}, nil
}

// newFileDAOv2InMem creates a in-memory new v2 fileDAO
func newFileDAOv2InMem(bottom uint64) (*fileDAOv2, error) {
	if bottom == 0 {
		return nil, ErrNotSupported
	}

	fd := fileDAOv2{
		header: &FileHeader{
			Version:        FileV2,
			Compressor:     "",
			BlockStoreSize: 16,
			Start:          bottom,
		},
		tip: &FileTip{
			Height: bottom - 1,
		},
		blkCache: cache.NewThreadSafeLruCache(16),
		kvStore:  db.NewMemKVStore(),
		batch:    batch.NewBatch(),
	}
	return &fd, nil
}

func (fd *testInMemFd) GetBlockHash(ctx context.Context, height uint64) (hash.Hash256, error) {
	if height == 0 {
		g, ok := genesis.ExtractGenesisContext(ctx)
		if !ok {
			return hash.ZeroHash256, errors.New("genesis block doesn't exist")
		}
		return g.Hash(), nil
	}
	return fd.fileDAOv2.GetBlockHash(ctx, height)
}

func (fd *testInMemFd) GetBlockHeight(ctx context.Context, h hash.Hash256) (uint64, error) {
	g, ok := genesis.ExtractGenesisContext(ctx)
	if !ok {
		return 0, errors.New("genesis block doesn't exist")
	}
	if g.IsAGenesisHash(h) {
		return 0, nil
	}
	return fd.fileDAOv2.GetBlockHeight(ctx, h)
}

func (fd *testInMemFd) GetBlock(ctx context.Context, h hash.Hash256) (*block.Block, error) {
	g, ok := genesis.ExtractGenesisContext(ctx)
	if !ok {
		return nil, errors.New("genesis block doesn't exist")
	}
	if g.IsAGenesisHash(h) {
		return g.Block(), nil
	}
	return fd.fileDAOv2.GetBlock(ctx, h)
}

func (fd *testInMemFd) GetBlockByHeight(ctx context.Context, height uint64) (*block.Block, error) {
	if height == 0 {
		g, ok := genesis.ExtractGenesisContext(ctx)
		if !ok {
			return nil, errors.New("genesis block doesn't exist")
		}
		return g.Block(), nil
	}
	return fd.fileDAOv2.GetBlockByHeight(ctx, height)
}

func (fd *testInMemFd) Header(ctx context.Context, h hash.Hash256) (*block.Header, error) {
	blk, err := fd.GetBlock(ctx, h)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

func (fd *testInMemFd) HeaderByHeight(ctx context.Context, height uint64) (*block.Header, error) {
	blk, err := fd.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

func (fd *testInMemFd) FooterByHeight(ctx context.Context, height uint64) (*block.Footer, error) {
	blk, err := fd.GetBlockByHeight(ctx, height)
	if err != nil {
		return nil, err
	}
	return &blk.Footer, nil
}

func (fd *testInMemFd) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := fd.GetBlockHeight(ctx, h); err == nil {
		return ErrAlreadyExist
	}
	return fd.fileDAOv2.PutBlock(ctx, blk)
}

func newTestFailPutBlock(fd *fileDAO, height uint64) FileDAO {
	return &testFailPutBlock{fileDAO: fd, failHeight: height}
}

func (tf *testFailPutBlock) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := tf.GetBlockHeight(ctx, h); err == nil {
		return ErrAlreadyExist
	}

	// check if we need to split DB
	if tf.cfg.V2BlocksToSplitDB > 0 {
		if err := tf.prepNextDbFile(blk.Height()); err != nil {
			return err
		}
	}

	if tf.failHeight == blk.Height() {
		return ErrInvalidTipHeight
	}
	return tf.currFd.PutBlock(ctx, blk)
}

func testCommitBlocks(t *testing.T, fd BaseFileDAO, start, end uint64, h hash.Hash256) error {
	ctx := context.Background()
	builder := block.NewTestingBuilder()
	for i := start; i <= end; i++ {
		blk := createTestingBlock(builder, i, h)
		if err := fd.PutBlock(ctx, blk); err != nil {
			return err
		}
		h = blk.HashBlock()
	}
	return nil
}

func testVerifyChainDB(t *testing.T, fd FileDAO, start, end uint64) {
	r := require.New(t)
	ctx := genesis.WithGenesisContext(
		action.WithEVMNetworkContext(
			context.Background(),
			action.EVMNetworkContext{
				ChainID: config.Default.Chain.EVMNetworkID,
			},
		),
		config.Default.Genesis,
	)

	height, err := fd.Height()
	r.NoError(err)
	r.Equal(end, height)
	for i := end; i >= start; i-- {
		h, err := fd.GetBlockHash(ctx, i)
		r.NoError(err)
		height, err = fd.GetBlockHeight(ctx, h)
		r.NoError(err)
		r.Equal(height, i)
		blk, err := fd.GetBlockByHeight(ctx, i)
		r.NoError(err)
		r.Equal(h, blk.HashBlock())
		receipt, err := fd.GetReceipts(ctx, i)
		r.NoError(err)
		r.EqualValues(1, receipt[0].Status)
		r.Equal(height, receipt[0].BlockHeight)
		r.Equal(blk.Header.PrevHash(), receipt[0].ActionHash)
		log, err := fd.TransactionLogs(i)
		r.NoError(err)
		r.NotNil(log)
		l := log.Logs[0]
		r.Equal(receipt[0].ActionHash[:], l.ActionHash)
		r.EqualValues(1, l.NumTransactions)
		tx := l.Transactions[0]
		r.Equal(big.NewInt(100).String(), tx.Amount)
		r.Equal(hex.EncodeToString(l.ActionHash[:]), tx.Sender)
		r.Equal(hex.EncodeToString(l.ActionHash[:]), tx.Recipient)
		r.Equal(iotextypes.TransactionLogType_NATIVE_TRANSFER, tx.Type)

		if false {
			// test DeleteTipBlock()
			r.NoError(fd.DeleteTipBlock(ctx))
			_, err = fd.GetBlockHash(ctx, i)
			r.Equal(db.ErrNotExist, err)
			_, err = fd.GetBlockHeight(ctx, h)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetBlock(ctx, h)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetBlockByHeight(ctx, i)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetReceipts(ctx, i)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			log, err = fd.TransactionLogs(i)
			r.Equal(ErrNotSupported, err)
		}
	}
}

func createTestingBlock(builder *block.TestingBuilder, height uint64, h hash.Hash256) *block.Block {
	r := &action.Receipt{
		Status:      1,
		BlockHeight: height,
		ActionHash:  h,
	}
	blk, _ := builder.
		SetHeight(height).
		SetPrevBlockHash(h).
		SetReceipts([]*action.Receipt{
			r.AddTransactionLogs(&action.TransactionLog{
				Type:      iotextypes.TransactionLogType_NATIVE_TRANSFER,
				Amount:    big.NewInt(100),
				Sender:    hex.EncodeToString(h[:]),
				Recipient: hex.EncodeToString(h[:]),
			}),
		}).
		SetTimeStamp(testutil.TimestampNow().UTC()).
		SignAndBuild(identityset.PrivateKey(27))
	return &blk
}
