// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"crypto/sha256"
	"os"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestFileDAOLegacy_PutBlock(t *testing.T) {
	var (
		normalHeaderSize, compressHeaderSize int
	)
	testFdInterface := func(cfg db.Config, t *testing.T) {
		r := require.New(t)

		testutil.CleanupPath(cfg.DbPath)
		fdLegacy, err := newFileDAOLegacy(cfg)
		r.NoError(err)
		fd, ok := fdLegacy.(*fileDAOLegacy)
		r.True(ok)
		ctx := context.Background()
		r.NoError(fd.Start(ctx))
		defer func() {
			r.NoError(fd.Stop(ctx))
		}()

		blk := createTestingBlock(block.NewTestingBuilder(), 1, hash.ZeroHash256)
		r.NoError(fd.PutBlock(ctx, blk))
		kv, _, err := fd.getTopDB(blk.Height())
		r.NoError(err)
		h := blk.HashBlock()
		header, err := kv.Get(_blockHeaderNS, h[:])
		r.NoError(err)
		if cfg.CompressLegacy {
			compressHeaderSize = len(header)
		} else {
			normalHeaderSize = len(header)
		}

		// verify API for genesis block
		h, err = fd.GetBlockHash(0)
		r.NoError(err)
		r.Equal(block.GenesisHash(), h)
		height, err := fd.GetBlockHeight(h)
		r.NoError(err)
		r.Zero(height)
		blk, err = fd.GetBlockByHeight(0)
		r.NoError(err)
		r.Equal(block.GenesisBlock(), blk)
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("filedao-legacy")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	genesis.SetGenesisTimestamp(config.Default.Genesis.Timestamp)
	block.LoadGenesisHash(&config.Default.Genesis)
	for _, compress := range []bool{false, true} {
		cfg.CompressLegacy = compress
		t.Run("test fileDAOLegacy interface", func(t *testing.T) {
			testFdInterface(cfg, t)
		})
	}

	// compare header length
	r.True(normalHeaderSize > compressHeaderSize)
}

func TestFileDAOLegacy_DeleteTipBlock(t *testing.T) {
	r := require.New(t)

	cfg := db.DefaultConfig
	cfg.DbPath = "./filedao_legacy.db"
	cfg.CompressLegacy = true // enable compress

	fd, err := newFileDAOLegacy(cfg)
	r.NoError(err)
	legacy := fd.(*fileDAOLegacy)

	ctx := context.Background()
	r.NoError(fd.Start(ctx))

	err = legacy.DeleteTipBlock()
	r.Equal("cannot delete genesis block", err.Error())

	height, err := legacy.Height()
	r.NoError(err)

	builder := block.NewTestingBuilder()
	h := hash.ZeroHash256
	blk := createTestingBlock(builder, height+1, h)
	r.NoError(fd.PutBlock(ctx, blk))

	r.NoError(legacy.DeleteTipBlock())
	_, err = fd.GetBlock(h)
	r.Equal(db.ErrNotExist, errors.Cause(err))
	_, err = legacy.GetBlockByHeight(height + 1)
	r.Equal(db.ErrNotExist, errors.Cause(err))
	_, err = fd.GetReceipts(1)
	r.Equal(db.ErrNotExist, errors.Cause(err))

	r.NoError(fd.Stop(ctx))
	os.RemoveAll(cfg.DbPath)
}

func TestFileDAOLegacy_getBlockValue(t *testing.T) {
	r := require.New(t)

	cfg := db.DefaultConfig
	cfg.DbPath = "./filedao_legacy.db"

	fd, err := newFileDAOLegacy(cfg)
	r.NoError(err)
	legacy := fd.(*fileDAOLegacy)

	ctx := context.Background()
	r.NoError(legacy.Start(ctx))

	// getBlockValue when block is not exist
	_, err = legacy.getBlockValue(_blockHeaderNS, hash.ZeroHash256)
	r.Equal(db.ErrNotExist, errors.Cause(err))

	// getBlockValue when block is exist
	builder := block.NewTestingBuilder()
	blk := createTestingBlock(builder, 1, sha256.Sum256([]byte("1")))
	r.NoError(legacy.PutBlock(ctx, blk))

	value, err := legacy.getBlockValue(_blockHeaderNS, blk.HashBlock())
	r.NoError(err)
	header, err := blk.Header.Serialize()
	r.NoError(err)
	r.Equal(value, header)

	// getBlockValue when NS is not exist
	_, err = legacy.getBlockValue(_blockHeaderNS+"_error_case", blk.HashBlock())
	r.Error(db.ErrNotExist, errors.Cause(err))

	r.NoError(legacy.Stop(ctx))
	os.RemoveAll(cfg.DbPath)
}
