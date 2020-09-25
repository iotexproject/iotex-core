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
	"os"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestChecksumNamespaceAndKeys(t *testing.T) {
	r := require.New(t)

	a := []hash.Hash256{
		// filedao
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
		// filedao_v2
		hash.BytesToHash256([]byte(FileV2)),
		hash.BytesToHash256([]byte{16}),
		hash.BytesToHash256([]byte(compress.Gzip)),
		hash.BytesToHash256([]byte(compress.Snappy)),
		hash.BytesToHash256([]byte(hashDataNS)),
		hash.BytesToHash256([]byte(blockDataNS)),
		hash.BytesToHash256([]byte(headerDataNs)),
		hash.BytesToHash256(fileHeaderKey),
	}

	checksum := crypto.NewMerkleTree(a)
	r.NotNil(checksum)
	h := checksum.HashTree()
	r.Equal("18747e1ac5364ce3f398e03092f159121b55166449657f65ba1f9243e8830391", hex.EncodeToString(h[:]))
}

func TestReadFileHeader(t *testing.T) {
	r := require.New(t)

	cfg := config.Default.DB
	cfg.DbPath = "./filedao_v2.db"

	// test non-existing file
	h, err := readFileHeader(cfg, FileLegacyMaster)
	r.Equal(ErrFileNotExist, err)
	h, err = readFileHeader(cfg, FileAll)
	r.Equal(ErrFileNotExist, err)

	// empty legacy file is invalid
	legacy, err := NewFileDAOLegacy(false, cfg)
	r.NoError(err)
	ctx := context.Background()
	r.NoError(legacy.Start(ctx))
	r.NoError(legacy.Stop(ctx))
	h, err = readFileHeader(cfg, FileLegacyMaster)
	r.Equal(ErrFileInvalid, err)
	h, err = readFileHeader(cfg, FileAll)
	r.Equal(ErrFileInvalid, err)

	// commit 1 block to make it a valid legacy file
	r.NoError(legacy.Start(ctx))
	builder := block.NewTestingBuilder()
	blk := createTestingBlock(builder, 1, hash.ZeroHash256)
	r.NoError(legacy.PutBlock(ctx, blk))
	height, err := legacy.Height()
	r.NoError(err)
	r.EqualValues(1, height)
	r.NoError(legacy.Stop(ctx))

	type testCheckFile struct {
		checkType, version string
		err                error
	}

	// test valid legacy master file
	test1 := []testCheckFile{
		{FileLegacyMaster, FileLegacyMaster, nil},
		{FileLegacyAuxiliary, FileLegacyMaster, nil},
		{FileV2, "", ErrFileInvalid},
		{FileAll, FileLegacyMaster, nil},
	}
	for _, v := range test1 {
		h, err = readFileHeader(cfg, v.checkType)
		r.Equal(v.err, err)
		if err == nil {
			r.Equal(v.version, h.Version)
		}
	}
	os.RemoveAll(cfg.DbPath)

	// test valid v2 master file
	r.NoError(createNewV2File(1, cfg))
	defer os.RemoveAll(cfg.DbPath)

	test2 := []testCheckFile{
		{FileLegacyMaster, "", ErrFileInvalid},
		{FileLegacyAuxiliary, "", ErrFileInvalid},
		{FileV2, FileV2, nil},
		{FileAll, FileV2, nil},
	}
	for _, v := range test2 {
		h, err = readFileHeader(cfg, v.checkType)
		r.Equal(v.err, err)
		if err == nil {
			r.Equal(v.version, h.Version)
		}
	}

	r.Panics(func() { readFileHeader(cfg, "") })
}

func TestNewFileDAO(t *testing.T) {
	r := require.New(t)

	cfg := config.Default.DB
	cfg.DbPath = "./filedao_v2.db"
	defer os.RemoveAll(cfg.DbPath)

	// test non-existing file
	h, v2files, err := checkChainDBFiles(cfg)
	r.Equal(ErrFileNotExist, err)
	r.Nil(v2files)

	// test empty db file, this will create new v2 file
	fd, err := NewFileDAO(false, cfg)
	r.NoError(err)
	r.NotNil(fd)
	h, err = readFileHeader(cfg, FileAll)
	r.NoError(err)
	r.Equal(FileV2, h.Version)
	ctx := context.Background()
	r.NoError(fd.Start(ctx))
	testCommitBlocks(t, fd, 1, 10, hash.ZeroHash256)
	testVerifyChainDB(t, fd, 1, 10)
	r.NoError(fd.Stop(ctx))

	// remove the v2 file, create legacy db file
	os.RemoveAll(cfg.DbPath)
	cfg.SplitDBHeight = 5
	cfg.SplitDBSizeMB = 200
	legacy, err := NewFileDAOLegacy(false, cfg)
	r.NoError(err)
	r.NoError(legacy.Start(ctx))
	testCommitBlocks(t, legacy, 1, 10, hash.ZeroHash256)
	testVerifyChainDB(t, legacy, 1, 10)
	r.NoError(legacy.Stop(ctx))
	// block 1~5 in default file.db, block 6~10 in file-000000001.db
	cfg.DbPath = kthAuxFileName("./filedao_v2.db", 1)
	defer os.RemoveAll(cfg.DbPath)
	v, err := readFileHeader(cfg, FileLegacyAuxiliary)
	r.NoError(err)
	r.Equal(FileLegacyAuxiliary, v.Version)

	// add a v2 file
	v2file := kthAuxFileName("./filedao_v2.db", 2)
	cfg.DbPath = v2file
	defer os.RemoveAll(v2file)
	v2, err := newFileDAOv2(11, cfg)
	r.NoError(err)
	r.NoError(v2.Start(ctx))
	testCommitBlocks(t, v2, 11, 32, hash.ZeroHash256)
	testVerifyChainDB(t, v2, 11, 32)
	v2.Stop(ctx)

	// now we have:
	// block 1~5 in legacy file.db
	// block 6~10 in legacy file-000000001.db
	// block 11~32 in v2 file-000000002.db
	cfg.DbPath = "./filedao_v2.db"
	fd, err = NewFileDAO(false, cfg)
	r.NoError(err)
	r.NotNil(fd)
	h, err = readFileHeader(cfg, FileAll)
	r.NoError(err)
	r.Equal(FileLegacyMaster, h.Version)
	r.NoError(fd.Start(ctx))
	defer fd.Stop(ctx)
	testVerifyChainDB(t, fd, 1, 32)
}

func TestCheckFiles(t *testing.T) {
	r := require.New(t)

	auxTests := []struct {
		file, base string
		index      int
		ok         bool
	}{
		{"/tmp/chain-00000003.db", "/tmp/chain", 0, false},
		{"/tmp/chain-00000003", "/tmp/chain.db", 0, false},
		{"/tmp/chain-00000003.dat", "/tmp/chain.db", 0, false},
		{"/tmp/chair-00000003.dat", "/tmp/chain.db", 0, false},
		{"/tmp/chain=00000003.db", "/tmp/chain.db", 0, false},
		{"/tmp/chain-0000003.db", "/tmp/chain.db", 0, false},
		{"/tmp/chain-00000003.db", "/tmp/chain.db", 3, true},
	}

	for _, v := range auxTests {
		index, ok := isAuxFile(v.file, v.base)
		r.Equal(v.index, index)
		r.Equal(v.ok, ok)
	}

	cfg := config.Default.DB
	cfg.DbPath = "./filedao_v2.db"
	files := possibleDBFiles(cfg.DbPath)
	r.Nil(files)

	// create 3 v2 files
	for i := 1; i <= 3; i++ {
		cfg.DbPath = kthAuxFileName("./filedao_v2.db", i)
		r.NoError(createNewV2File(1, cfg))
	}
	defer func() {
		for i := 1; i <= 3; i++ {
			os.RemoveAll(kthAuxFileName("./filedao_v2.db", i))
		}
	}()
	cfg.DbPath = "./filedao_v2.db"
	files = checkAuxFiles(cfg, FileV2)
	r.Equal(3, len(files))
	for i := 1; i <= 3; i++ {
		r.Equal(files[i-1], kthAuxFileName("./filedao_v2.db", i))
	}
}

func testCommitBlocks(t *testing.T, fd FileDAO, start, end uint64, h hash.Hash256) hash.Hash256 {
	r := require.New(t)

	ctx := context.Background()
	builder := block.NewTestingBuilder()
	for i := start; i <= end; i++ {
		blk := createTestingBlock(builder, i, h)
		r.NoError(fd.PutBlock(ctx, blk))
		h = blk.HashBlock()
	}
	return h
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
