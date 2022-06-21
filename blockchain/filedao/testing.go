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

	cfg, _ := CreateModuleConfig(config.Default.DB)
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
		cfg:      cfg,
	}
	return &fd, nil
}

func (fd *testInMemFd) GetBlockHash(height uint64) (hash.Hash256, error) {
	if height == 0 {
		return block.GenesisHash(), nil
	}
	return fd.fileDAOv2.GetBlockHash(height)
}

func (fd *testInMemFd) GetBlockHeight(h hash.Hash256) (uint64, error) {
	if h == block.GenesisHash() {
		return 0, nil
	}
	return fd.fileDAOv2.GetBlockHeight(h)
}

func (fd *testInMemFd) GetBlock(h hash.Hash256) (*block.Block, error) {
	if h == block.GenesisHash() {
		return block.GenesisBlock(), nil
	}
	return fd.fileDAOv2.GetBlock(h)
}

func (fd *testInMemFd) GetBlockByHeight(height uint64) (*block.Block, error) {
	if height == 0 {
		return block.GenesisBlock(), nil
	}
	return fd.fileDAOv2.GetBlockByHeight(height)
}

func (fd *testInMemFd) Header(h hash.Hash256) (*block.Header, error) {
	blk, err := fd.GetBlock(h)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

func (fd *testInMemFd) HeaderByHeight(height uint64) (*block.Header, error) {
	blk, err := fd.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return &blk.Header, nil
}

func (fd *testInMemFd) FooterByHeight(height uint64) (*block.Footer, error) {
	blk, err := fd.GetBlockByHeight(height)
	if err != nil {
		return nil, err
	}
	return &blk.Footer, nil
}

func (fd *testInMemFd) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := fd.GetBlockHeight(h); err == nil {
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
	if _, err := tf.GetBlockHeight(h); err == nil {
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

	height, err := fd.Height()
	r.NoError(err)
	r.Equal(end, height)
	for i := end; i >= start; i-- {
		h, err := fd.GetBlockHash(i)
		r.NoError(err)
		height, err = fd.GetBlockHeight(h)
		r.NoError(err)
		r.Equal(height, i)
		blk, err := fd.GetBlockByHeight(i)
		r.NoError(err)
		r.Equal(h, blk.HashBlock())
		receipt, err := fd.GetReceipts(i)
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
			r.NoError(fd.DeleteTipBlock())
			_, err = fd.GetBlockHash(i)
			r.Equal(db.ErrNotExist, err)
			_, err = fd.GetBlockHeight(h)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetBlock(h)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetBlockByHeight(i)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.GetReceipts(i)
			r.Equal(db.ErrNotExist, errors.Cause(err))
			_, err = fd.TransactionLogs(i)
			r.Equal(ErrNotSupported, err)
		}
	}
}

func createTestingBlock(builder *block.TestingBuilder, height uint64, h hash.Hash256) *block.Block {
	block.LoadGenesisHash(&config.Default.Genesis)
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
