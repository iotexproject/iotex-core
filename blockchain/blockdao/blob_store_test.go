// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	. "github.com/iotexproject/iotex-core/pkg/util/assertions"
	"github.com/iotexproject/iotex-core/testutil"
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
		testPath := MustNoErrorV(testutil.PathOfTempFile("test-blob-store"))
		defer func() {
			testutil.CleanupPath(testPath)
		}()
		cfg := db.DefaultConfig
		cfg.DbPath = testPath
		kvs := db.NewBoltDB(cfg)
		bs := NewBlobStore(kvs, 24, time.Second)
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
			hashes := createTestHash(i, height)
			r.NoError(bs.putBlob(_value[i%7], height, hashes))
			r.Equal(height, bs.currWriteBlock)
			raw := MustNoErrorV(bs.kvStore.Get(_heightIndexNS, keyForBlock(height)))
			index := MustNoErrorV(deserializeBlobIndex(raw))
			r.Equal(index.hashes, hashes)
			for i := range hashes {
				r.Equal(height, MustNoErrorV(bs.getHeightByHash(hashes[i])))
			}
			r.Equal(_value[i%7], MustNoErrorV(bs.kvStore.Get(_blobDataNS, keyForBlock(height))))
		}
		time.Sleep(time.Second * 3 / 2)
		// slot 0 - 13 has expired
		for i, height := range []uint64{0, 3, 5, 10, 13, 18, 29, 37} {
			hashes := createTestHash(i, height)
			raw, err := bs.kvStore.Get(_heightIndexNS, keyForBlock(height))
			if i <= 4 {
				r.ErrorIs(err, db.ErrNotExist)
			} else {
				index := MustNoErrorV(deserializeBlobIndex(raw))
				r.Equal(index.hashes, hashes)
			}
			for j := range hashes {
				h, err := bs.getHeightByHash(hashes[j])
				if i <= 4 {
					r.ErrorIs(err, db.ErrNotExist)
				} else {
					r.NoError(err)
					r.Equal(height, h)
				}
			}
			v, err := bs.kvStore.Get(_blobDataNS, keyForBlock(height))
			if i <= 4 {
				r.ErrorIs(err, db.ErrNotExist)
			} else {
				r.NoError(err)
				r.Equal(_value[i%7], v)
			}
		}
		// verify write and expire block
		r.NoError(bs.expireBlob(41))
		r.NoError(bs.Stop(ctx))
		r.NoError(bs.Start(ctx))
		r.EqualValues(37, bs.currWriteBlock)
		r.EqualValues(17, bs.currExpireBlock)
		r.NoError(bs.Stop(ctx))
	})
	t.Run("PutBlock", func(t *testing.T) {
		ctx := context.Background()
		testPath := MustNoErrorV(testutil.PathOfTempFile("test-blob-store"))
		defer func() {
			testutil.CleanupPath(testPath)
		}()
		cfg := db.DefaultConfig
		cfg.DbPath = testPath
		kvs := db.NewBoltDB(cfg)
		bs := NewBlobStore(kvs, 24, time.Second)
		testPath1 := MustNoErrorV(testutil.PathOfTempFile("test-blob-store"))
		cfg.DbPath = testPath1
		fd := MustNoErrorV(createFileDAO(false, false, compress.Snappy, cfg))
		r.NotNil(fd)
		dao := NewBlockDAOWithIndexersAndCache(fd, nil, 10, WithBlobStore(bs))
		r.NoError(dao.Start(ctx))
		defer func() {
			r.NoError(dao.Stop(ctx))
			testutil.CleanupPath(testPath1)
		}()

		blks := MustNoErrorV(block.CreateTestBlockWithBlob(1, cfg.BlockStoreBatchSize+7))
		for _, blk := range blks {
			r.Nil(blk.Actions[0].BlobTxSidecar())
			r.Nil(blk.Actions[2].BlobTxSidecar())
			r.NotNil(blk.Actions[1].BlobTxSidecar())
			r.NotNil(blk.Actions[3].BlobTxSidecar())
			r.NoError(dao.PutBlock(ctx, blk))
		}
		// cannot store blocks less than tip height
		r.ErrorContains(bs.PutBlock(blks[len(blks)-1]), "block height 23 is less than current tip height")
		for i := 0; i < cfg.BlockStoreBatchSize+7; i++ {
			blk := MustNoErrorV(dao.GetBlockByHeight(1 + uint64(i)))
			if i < cfg.BlockStoreBatchSize {
				// blocks written to disk has sidecar removed
				r.False(blk.HasBlob())
				r.Equal(4, len(blk.Actions))
				// verify sidecar
				sc, hashes, err := dao.GetBlobsByHeight(1 + uint64(i))
				r.NoError(err)
				for j := 0; j < 2; j++ {
					// sc[0] <--> Actions[1].sidecar
					// sc[1] <--> Actions[3].sidecar
					r.Equal(blks[i].Actions[2*j+1].BlobTxSidecar(), sc[j])
					h := MustNoErrorV(blk.Actions[2*j+1].Hash())
					r.Equal(hex.EncodeToString(h[:]), hashes[j][2:])
					sc1, h1, err := dao.GetBlob(h)
					r.NoError(err)
					r.Equal(hashes[j], h1)
					r.Equal(sc[j], sc1)
					r.NotEqual(blks[i].Actions[2*j+1], blk.Actions[2*j+1])
				}
			} else {
				// blocks in the staging buffer still has sidecar attached
				r.True(blk.HasBlob())
				r.Equal(blks[i], blk)
			}
			h := blk.HashBlock()
			height := MustNoErrorV(dao.GetBlockHeight(h))
			r.Equal(1+uint64(i), height)
			hash := MustNoErrorV(dao.GetBlockHash(height))
			r.Equal(h, hash)
		}
	})
}

func createTestHash(i int, height uint64) [][]byte {
	h1 := hash.BytesToHash256([]byte{byte(i + 128)})
	h2 := hash.BytesToHash256([]byte{byte(height)})
	return [][]byte{h1[:], h2[:]}
}
