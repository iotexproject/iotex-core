package blockdao

import (
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestCompressDataBytes(t *testing.T) {
	r := require.New(t)

	// cannot compress nil nor decompress empty byte slice
	_, err := compressDatabytes(nil, false)
	r.Equal(ErrDataCorruption, err)
	_, err = decompressDatabytes([]byte{})
	r.Equal(ErrDataCorruption, err)

	zero := hash.ZeroHash256
	blkHash, _ := hex.DecodeString("22cd0c2d1f7d65298cec7599e2d0e3c650dd8b4ed2b1c816d909026c60d785b2")
	compressTests := [][]byte{
		[]byte{},
		zero[:],
		blkHash,
		[]byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`1234567890-=~!@#$%^&*()_+å∫ç∂´´©˙ˆˆ˚¬µ˜˜πœ®ß†¨¨∑≈¥Ω[]',./{}|:<>?"),
	}
	for _, ser := range compressTests {
		for _, compress := range []bool{false, true} {
			v, err := compressDatabytes(ser, compress)
			r.NoError(err)
			if compress {
				r.EqualValues(_compressed, v[len(v)-1])
			} else {
				r.EqualValues(_normal, v[len(v)-1])
			}

			ser1, err := decompressDatabytes(v)
			r.NoError(err)
			r.Equal(ser, ser1)
			v[len(v)-1] = _compressed + 1
			v, err = decompressDatabytes(v)
			r.Equal(ErrDataCorruption, err)
		}
	}
}

func TestNewFileDAO(t *testing.T) {
	r := require.New(t)

	testPath, err := testutil.PathOfTempFile("test-newfd")
	r.NoError(err)
	testutil.CleanupPath(t, testPath)
	defer func() {
		testutil.CleanupPath(t, testPath)
	}()
	cfg := config.Default.DB
	cfg.DbPath = testPath

	fd, err := newFileDAONew(db.NewBoltDB(cfg), 2, cfg)
	r.NoError(err)
	r.NotNil(fd)
	ctx := context.Background()
	r.NoError(fd.Start(ctx))
	defer fd.Stop(ctx)

	// new file does not use legacy's namespaces
	newFd, ok := fd.(*fileDAONew)
	r.True(ok)
	for _, v := range []string{
		blockNS,
		blockHeaderNS,
		blockBodyNS,
		blockFooterNS,
		receiptsNS,
	} {
		_, err = newFd.kvStore.Get(v, []byte{})
		r.Error(err)
		r.True(strings.HasPrefix(err.Error(), "bucket = "+hex.EncodeToString([]byte(v))+" doesn't exist"))
	}

	// test counting index add []byte
	log := &block.BlkTransactionLog{}
	ser, err := compressDatabytes(log.Serialize(), false)
	r.NoError(err)
	r.Equal([]byte{_normal}, ser)
	r.NoError(addOneEntryToBatch(newFd.hashStore, ser, newFd.batch))
	r.NoError(newFd.kvStore.WriteBatch(newFd.batch))
	v, err := newFd.hashStore.Get(0)
	r.NoError(err)
	r.Equal(ser, v)
	v, err = decompressDatabytes(v)
	r.NoError(err)
	r.Equal([]byte{}, v)
}
