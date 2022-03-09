// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestFileProto(t *testing.T) {
	r := require.New(t)

	h := &FileHeader{
		Version:        FileV2,
		Compressor:     "test",
		BlockStoreSize: 32,
		Start:          3,
	}
	ser, err := h.Serialize()
	r.NoError(err)
	h1, err := DeserializeFileHeader(ser)
	r.NoError(err)
	r.Equal(h, h1)

	c := &FileTip{
		Height: 1003,
		Hash:   hash.Hash256{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	}
	ser, err = c.Serialize()
	r.NoError(err)
	c1, err := DeserializeFileTip(ser)
	r.NoError(err)
	r.Equal(c, c1)
}

func TestFileReadWrite(t *testing.T) {
	testHeader := func(kv db.KVStore, t *testing.T) {
		r := require.New(t)
		r.NotNil(kv)

		ctx := context.Background()
		r.NoError(kv.Start(ctx))
		defer kv.Stop(ctx)

		h, err := ReadHeaderV2(kv)
		r.Equal(db.ErrNotExist, errors.Cause(err))
		r.Nil(h)

		h = &FileHeader{
			Version:        FileV2,
			Compressor:     "test",
			BlockStoreSize: 32,
			Start:          3,
		}
		r.NoError(WriteHeaderV2(kv, h))

		h1, err := ReadHeaderV2(kv)
		r.NoError(err)
		r.Equal(h, h1)

		c, err := ReadTip(kv, headerDataNs, topHeightKey)
		r.Equal(db.ErrNotExist, errors.Cause(err))
		r.Nil(c)

		c = &FileTip{
			Height: 1003,
			Hash:   hash.Hash256{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		}
		r.NoError(WriteTip(kv, headerDataNs, topHeightKey, c))

		c1, err := ReadTip(kv, headerDataNs, topHeightKey)
		r.NoError(err)
		r.Equal(c, c1)
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-header")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(testPath)
	}()

	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	t.Run("test file header", func(t *testing.T) {
		testHeader(db.NewBoltDB(cfg), t)
	})
}
