// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func makeTestBlock(t *testing.T, height uint64, prevHash [32]byte) *block.Block {
	t.Helper()
	blk, err := block.NewTestingBuilder().
		SetHeight(height).
		SetPrevBlockHash(prevHash).
		SetTimeStamp(testutil.TimestampNow().UTC()).
		SetVersion(1).
		SignAndBuild(identityset.PrivateKey(1))
	require.NoError(t, err)
	return &blk
}

func TestWindowBlockStore(t *testing.T) {
	r := require.New(t)
	ctx := context.Background()

	const window = uint64(3)
	cfg := db.DefaultConfig
	path := t.TempDir() + "/window.db"
	deser := block.NewDeserializer(4689)

	store, err := NewWindowBlockStore(cfg, path, window, deser)
	r.NoError(err)
	r.NoError(store.Start(ctx))

	// empty store
	h, err := store.Height()
	r.NoError(err)
	r.Equal(uint64(0), h)

	// store blocks 1..5 with window=3 → only 3,4,5 should survive
	var prevHash [32]byte
	blks := make([]*block.Block, 6)
	for i := uint64(1); i <= 5; i++ {
		blks[i] = makeTestBlock(t, i, prevHash)
		prevHash = blks[i].HashBlock()
		r.NoError(store.PutBlock(ctx, blks[i]))
	}

	h, err = store.Height()
	r.NoError(err)
	r.Equal(uint64(5), h)

	t.Run("window blocks are readable", func(t *testing.T) {
		for _, height := range []uint64{3, 4, 5} {
			blk, err := store.GetBlockByHeight(height)
			r.NoError(err)
			r.Equal(height, blk.Height())

			hdr, err := store.HeaderByHeight(height)
			r.NoError(err)
			r.Equal(height, hdr.Height())

			hash, err := store.GetBlockHash(height)
			r.NoError(err)
			r.Equal(blks[height].HashBlock(), hash)

			gotHeight, err := store.GetBlockHeight(hash)
			r.NoError(err)
			r.Equal(height, gotHeight)

			blkByHash, err := store.GetBlock(hash)
			r.NoError(err)
			r.Equal(height, blkByHash.Height())
		}
	})

	t.Run("evicted blocks return ErrNotExist", func(t *testing.T) {
		for _, height := range []uint64{1, 2} {
			_, err := store.GetBlockByHeight(height)
			r.True(errors.Cause(err) == db.ErrNotExist, "expected ErrNotExist at height %d, got %v", height, err)
		}
	})

	t.Run("genesis block always readable", func(t *testing.T) {
		blk, err := store.GetBlockByHeight(0)
		r.NoError(err)
		r.Equal(uint64(0), blk.Height())
	})

	t.Run("transaction logs", func(t *testing.T) {
		r.True(store.ContainsTransactionLog())
		_, err := store.TransactionLogs(5)
		r.NoError(err)
	})

	t.Run("footer", func(t *testing.T) {
		ftr, err := store.FooterByHeight(5)
		r.NoError(err)
		r.NotNil(ftr)
	})

	t.Run("header by hash", func(t *testing.T) {
		hash5, _ := store.GetBlockHash(5)
		hdr, err := store.Header(hash5)
		r.NoError(err)
		r.Equal(uint64(5), hdr.Height())
	})

	r.NoError(store.Stop(ctx))

	t.Run("persistence: data survives restart", func(t *testing.T) {
		store2, err := NewWindowBlockStore(cfg, path, window, deser)
		r.NoError(err)
		r.NoError(store2.Start(ctx))
		defer store2.Stop(ctx)

		h, err := store2.Height()
		r.NoError(err)
		r.Equal(uint64(5), h)

		blk, err := store2.GetBlockByHeight(5)
		r.NoError(err)
		r.Equal(uint64(5), blk.Height())

		_, err = store2.GetBlockByHeight(1)
		r.True(errors.Cause(err) == db.ErrNotExist)
	})
}

func TestWindowBlockStore_ZeroWindowSizeError(t *testing.T) {
	_, err := NewWindowBlockStore(db.DefaultConfig, t.TempDir()+"/w.db", 0, block.NewDeserializer(1))
	require.Error(t, err)
}

func TestWindowBlockStore_GetBlockHash_Genesis(t *testing.T) {
	r := require.New(t)
	store, err := NewWindowBlockStore(db.DefaultConfig, t.TempDir()+"/w.db", 10, block.NewDeserializer(1))
	r.NoError(err)
	r.NoError(store.Start(context.Background()))
	defer store.Stop(context.Background())

	h, err := store.GetBlockHash(0)
	r.NoError(err)
	r.Equal(block.GenesisHash(), h)
}

// Ensure windowBlockStore implements BlockStore at compile time (also checked by var _ at EOF).
var _ BlockStore = (*windowBlockStore)(nil)
