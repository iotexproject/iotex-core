package filedao

import (
	"bytes"
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestHeaderProto(t *testing.T) {
	r := require.New(t)

	sh := hash.Hash256{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	h := &FileHeader{
		Version:        FileV2,
		Compressor:     "test",
		BlockStoreSize: 32,
		Start:          3,
		End:            1003,
		TopHash:        sh,
	}
	ser, err := h.Serialize()
	r.NoError(err)

	h1, err := DeserializeFileHeader(ser)
	r.NoError(err)
	r.Equal(h, h1)

	for _, v := range []struct {
		height uint64
		hash   hash.Hash256
		equal  bool
	}{
		{h.End + 1, sh, false},
		{h.End, hash.ZeroHash256, false},
		{h.End, sh, true},
	} {
		ser1, err := h.createHeaderBytes(v.height, v.hash)
		r.NoError(err)
		r.EqualValues(1003, h.End)
		r.Equal(v.equal, bytes.Compare(ser1, ser) == 0)
	}
}

func TestFileHeader(t *testing.T) {
	testHeader := func(kv db.KVStore, t *testing.T) {
		r := require.New(t)
		r.NotNil(kv)

		ctx := context.Background()
		r.NoError(kv.Start(ctx))
		defer kv.Stop(ctx)

		h, err := ReadFileHeader(kv, headerDataNs, fileHeaderKey)
		r.Equal(db.ErrNotExist, errors.Cause(err))
		r.Nil(h)

		h = &FileHeader{
			Version:        FileV2,
			Compressor:     "test",
			BlockStoreSize: 32,
			Start:          3,
			End:            1003,
			TopHash:        hash.Hash256{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		}
		r.NoError(WriteHeader(kv, headerDataNs, fileHeaderKey, h))

		h1, err := ReadFileHeader(kv, headerDataNs, fileHeaderKey)
		r.NoError(err)
		r.Equal(h, h1)
	}

	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-header")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(t, testPath)
	}()

	cfg := config.Default.DB
	cfg.DbPath = testPath
	t.Run("test file header", func(t *testing.T) {
		testHeader(db.NewBoltDB(cfg), t)
	})
}
