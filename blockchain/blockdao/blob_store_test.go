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

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db"
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
		testPath, err := testutil.PathOfTempFile("test-blob-store")
		r.NoError(err)
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
		time.Sleep(time.Second * 3 / 2)
		// slot 0 - 13 has expired
		for i, height := range []uint64{0, 3, 5, 10, 13, 18, 29, 37} {
			hashes := createTestHash(i, height)
			raw, err := bs.kvStore.Get(_heightIndexNS, keyForBlock(height))
			if i <= 4 {
				r.ErrorIs(err, db.ErrNotExist)
			} else {
				index, err := deserializeBlobIndex(raw)
				r.NoError(err)
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
		r.NoError(bs.Stop(ctx))
		r.NoError(bs.Start(ctx))
		r.EqualValues(37, bs.currWriteBlock)
		r.EqualValues(13, bs.currExpireBlock)
		r.NoError(bs.Stop(ctx))
	})
	t.Run("PutBlock", func(t *testing.T) {
		ctx := context.Background()
		testPath, err := testutil.PathOfTempFile("test-blob-store")
		r.NoError(err)
		cfg := db.DefaultConfig
		cfg.DbPath = testPath
		kvs := db.NewBoltDB(cfg)
		bs := NewBlobStore(kvs, 24, time.Second)
		r.NoError(bs.Start(ctx))
		defer func() {
			r.NoError(bs.Stop(ctx))
			testutil.CleanupPath(testPath)
		}()
		// cannot store blocks less than tip height
	})
}

func createTestHash(i int, height uint64) [][]byte {
	h1 := hash.BytesToHash256([]byte{byte(i + 128)})
	h2 := hash.BytesToHash256([]byte{byte(height)})
	return [][]byte{h1[:], h2[:]}
}

func createTestBlobTxData(n uint64) *types.BlobTxSidecar {
	testBlob := kzg4844.Blob{byte(n)}
	testBlobCommit := MustNoErrorV(kzg4844.BlobToCommitment(testBlob))
	testBlobProof := MustNoErrorV(kzg4844.ComputeBlobProof(testBlob, testBlobCommit))
	return &types.BlobTxSidecar{
		Blobs:       []kzg4844.Blob{testBlob},
		Commitments: []kzg4844.Commitment{testBlobCommit},
		Proofs:      []kzg4844.Proof{testBlobProof},
	}
}
