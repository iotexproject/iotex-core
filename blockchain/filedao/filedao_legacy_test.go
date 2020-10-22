// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"crypto/sha256"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestFileDAOLegacy_PutBlock(t *testing.T) {
	r := require.New(t)

	// PutBlock with out compression
	normalConfig := config.Default.DB
	normalConfig.DbPath = "./filedao_legacy_normal.db"
	normalConfig.CompressLegacy = false

	normalFileDAO, err := newFileDAOLegacy(normalConfig)
	r.NoError(err)
	normalLegacy := normalFileDAO.(*fileDAOLegacy)

	normalContext := context.Background()
	r.NoError(normalLegacy.Start(normalContext))
	normalBuilder := block.NewTestingBuilder()
	normalBlock := createTestingBlock(normalBuilder, 1, hash.ZeroHash256)
	r.NoError(normalLegacy.PutBlock(normalContext, normalBlock))
	normalKV, _, err := normalLegacy.getTopDB(normalBlock.Height())
	r.NoError(err)
	normalHash := normalBlock.HashBlock()
	normalHeader, err := normalKV.Get(blockHeaderNS, normalHash[:])
	r.NoError(err)

	// PutBlock with compression
	compressConfig := config.Default.DB
	compressConfig.DbPath = "./filedao_legacy_compress.db"
	compressConfig.CompressLegacy = true // enable compress

	compressFileDAO, err := newFileDAOLegacy(compressConfig)
	r.NoError(err)
	compressLegacy := compressFileDAO.(*fileDAOLegacy)

	compressContext := context.Background()
	r.NoError(compressLegacy.Start(compressContext))
	compressBuilder := block.NewTestingBuilder()
	compressBlock := createTestingBlock(compressBuilder, 1, hash.ZeroHash256)
	r.NoError(compressLegacy.PutBlock(compressContext, compressBlock))
	compressKV, _, err := compressLegacy.getTopDB(compressBlock.Height())
	r.NoError(err)
	compressHash := compressBlock.HashBlock()
	compressHeader, err := compressKV.Get(blockHeaderNS, compressHash[:])
	r.NoError(err)

	// compare header length
	r.True(len(normalHeader) > len(compressHeader))

	r.NoError(compressLegacy.Stop(compressContext))
	r.NoError(normalLegacy.Stop(normalContext))
	r.NoError(os.RemoveAll(compressConfig.DbPath))
	r.NoError(os.RemoveAll(normalConfig.DbPath))
}

func TestFileDAOLegacy_DeleteTipBlock(t *testing.T) {
	r := require.New(t)

	cfg := config.Default.DB
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

	cfg := config.Default.DB
	cfg.DbPath = "./filedao_legacy.db"

	fd, err := newFileDAOLegacy(cfg)
	r.NoError(err)
	legacy := fd.(*fileDAOLegacy)

	ctx := context.Background()
	r.NoError(legacy.Start(ctx))

	// getBlockValue when block is not exist
	_, err = legacy.getBlockValue(blockHeaderNS, hash.ZeroHash256)
	r.Equal(db.ErrNotExist, errors.Cause(err))

	// getBlockValue when block is exist
	builder := block.NewTestingBuilder()
	blk := createTestingBlock(builder, 1, sha256.Sum256([]byte("1")))
	r.NoError(legacy.PutBlock(ctx, blk))

	value, err := legacy.getBlockValue(blockHeaderNS, blk.HashBlock())
	r.NoError(err)
	header, err := blk.Header.Serialize()
	r.NoError(err)
	r.Equal(value, header)

	// getBlockValue when NS is not exist
	value, err = legacy.getBlockValue(blockHeaderNS+"_error_case", blk.HashBlock())
	r.Error(db.ErrNotExist, errors.Cause(err))

	r.NoError(legacy.Stop(ctx))
	os.RemoveAll(cfg.DbPath)
}