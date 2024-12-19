// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/compress"
	. "github.com/iotexproject/iotex-core/v2/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestChecksumNamespaceAndKeys(t *testing.T) {
	r := require.New(t)
	a := []hash.Hash256{
		hash.BytesToHash256([]byte(_blobDataNS)),
		hash.BytesToHash256([]byte(_heightIndexNS)),
		hash.BytesToHash256([]byte(_hashHeightNS)),
		hash.BytesToHash256(_writeHeight),
		hash.BytesToHash256(_expireHeight),
	}
	h := crypto.NewMerkleTree(a).HashTree()
	r.Equal("6e00ae20664b1e2161a9a1d2e7ebb479194a21686f588c6ff80c462df46c5d9d", hex.EncodeToString(h[:]))
}

func TestBlobStore(t *testing.T) {
	r := require.New(t)
	t.Run("putBlob", func(t *testing.T) {
		ctx := context.Background()
		testPath, err := testutil.PathOfTempFile("test-blob-store")
		r.NoError(err)
		defer func() {
			testutil.CleanupPath(testPath)
		}()
		cfg := db.DefaultConfig
		cfg.DbPath = testPath
		kvs := db.NewBoltDB(cfg)
		bs := NewBlobStore(kvs, 24)
		r.NoError(bs.Start(ctx))
		var (
			_value [7][]byte = [7][]byte{
				{0, 1, 2, 3},
				{1, 1, 2, 3},
				{2, 1, 2, 3},
				{3, 1, 2, 3},
				{4, 1, 2, 3},
				{5, 1, 2, 3},
				{6, 1, 2, 3},
			}
		)
		for i, height := range []uint64{0, 3, 5, 10, 13, 18, 29, 37} {
			b := batch.NewBatch()
			hashes := createTestHash(i, height)
			bs.putBlob(_value[i%7], height, hashes, b)
			r.NoError(bs.kvStore.WriteBatch(b))
			raw, err := bs.kvStore.Get(_heightIndexNS, keyForBlock(height))
			r.NoError(err)
			index, err := deserializeBlobIndex(raw)
			r.NoError(err)
			r.Equal(index.hashes, hashes)
			for i := range hashes {
				h, err := bs.getHeightByHash(hashes[i])
				r.NoError(err)
				r.Equal(height, h)
			}
			v, err := bs.kvStore.Get(_blobDataNS, keyForBlock(height))
			r.NoError(err)
			r.Equal(_value[i%7], v)
		}
		// verify write and expire block
		r.NoError(bs.Stop(ctx))
		r.NoError(bs.Start(ctx))
		r.EqualValues(37, bs.currWriteBlock)
		r.NoError(bs.Stop(ctx))
	})
	t.Run("PutBlock", func(t *testing.T) {
		ctx := context.Background()
		testPath, err := testutil.PathOfTempFile("test-blob-store")
		r.NoError(err)
		defer func() {
			testutil.CleanupPath(testPath)
		}()
		cfg := db.DefaultConfig
		cfg.DbPath = testPath
		kvs := db.NewBoltDB(cfg)
		bs := NewBlobStore(kvs, 24)
		testPath1, err := testutil.PathOfTempFile("test-blob-store")
		r.NoError(err)
		cfg.DbPath = testPath1
		fd, err := createFileDAO(false, false, compress.Snappy, cfg)
		r.NoError(err)
		r.NotNil(fd)
		dao := NewBlockDAOWithIndexersAndCache(fd, nil, 10, WithBlobStore(bs))
		r.NoError(err)
		r.NoError(dao.Start(ctx))
		defer func() {
			r.NoError(dao.Stop(ctx))
			testutil.CleanupPath(testPath1)
		}()

		blks, err := block.CreateTestBlockWithBlob(1, cfg.BlockStoreBatchSize+7)
		r.NoError(err)
		for _, blk := range blks {
			r.True(blk.HasBlob())
			r.NoError(dao.PutBlock(ctx, blk))
		}
		// cannot store blocks less than tip height
		err = bs.PutBlock(blks[len(blks)-1])
		r.ErrorContains(err, "block height 23 is less than current tip height")
		for i := 0; i < cfg.BlockStoreBatchSize+7; i++ {
			blk, err := dao.GetBlockByHeight(1 + uint64(i))
			r.NoError(err)
			if i < 7 {
				// blocks written to disk has sidecar removed
				r.False(blk.HasBlob())
				r.Equal(4, len(blk.Actions))
				// verify sidecar
				sc, hashes, err := dao.GetBlobsByHeight(1 + uint64(i))
				r.NoError(err)
				r.Equal(blks[i].Actions[1].BlobTxSidecar(), sc[0])
				h := MustNoErrorV(blks[i].Actions[1].Hash())
				r.Equal(hex.EncodeToString(h[:]), hashes[0][2:])
				sc1, h1, err := dao.GetBlob(h)
				r.NoError(err)
				r.Equal(hashes[0], h1)
				r.Equal(sc[0], sc1)
				r.Equal(blks[i].Actions[3].BlobTxSidecar(), sc[1])
				h = MustNoErrorV(blks[i].Actions[3].Hash())
				r.Equal(hex.EncodeToString(h[:]), hashes[1][2:])
				sc3, h3, err := dao.GetBlob(h)
				r.NoError(err)
				r.Equal(hashes[1], h3)
				r.Equal(sc[1], sc3)

			} else {
				// blocks in the staging buffer still has sidecar attached
				r.True(blk.HasBlob())
				r.Equal(blks[i], blk)
			}
			h := blk.HashBlock()
			height, err := dao.GetBlockHeight(h)
			r.NoError(err)
			r.Equal(1+uint64(i), height)
			hash, err := dao.GetBlockHash(height)
			r.NoError(err)
			r.Equal(h, hash)
		}
	})
}

func createTestHash(i int, height uint64) [][]byte {
	h1 := hash.BytesToHash256([]byte{byte(i + 128)})
	h2 := hash.BytesToHash256([]byte{byte(height)})
	return [][]byte{h1[:], h2[:]}
}
