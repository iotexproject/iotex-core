package mptrie

import (
	"context"
	"fmt"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

// new version
func benchmarkTrie(async bool, b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping TestPressure in short mode.")
	}
	require := require.New(b)
	testPath, err := testutil.PathOfTempFile(fmt.Sprintf("test-kv-store-%t.bolt", async))
	require.NoError(err)
	cfg := config.Default.DB
	cfg.DbPath = testPath
	dao := db.NewBoltDB(cfg)
	flusher, err := db.NewKVStoreFlusher(dao, batch.NewCachedBatch())
	flusherKV := flusher.KVStoreWithBuffer()
	kv, err := trie.NewKVStore("test", flusherKV)
	require.NoError(err)
	require.NoError(kv.Start(context.Background()))
	var tr trie.Trie
	if async {
		tr, err = New(KVStoreOption(kv), KeyLengthOption(20), AsyncOption())
	} else {
		tr, err = New(KVStoreOption(kv), KeyLengthOption(20))
	}
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	seed := 0
	b.ResetTimer()
	// insert 128k entries
	var k [32]byte
	k[0] = byte(seed)
	c := 0
	for c = 0; c < b.N<<2; c++ {
		k = hash.Hash256b(k[:])
		v := testV[k[0]&7]
		if _, err := tr.Get(k[:20]); err == nil {
			b.Logf("Warning: collision on k %x", k[:20])
			break
		}
		require.NoError(tr.Upsert(k[:20], v))
		vb, err := tr.Get(k[:20])
		require.NoError(err)
		require.Equal(v, vb)
	}
	_, err = tr.RootHash()
	require.NoError(err)
	require.False(tr.IsEmpty())
	// delete 128k entries
	var d [32]byte
	d[0] = byte(seed)
	for i := 0; i < c; i++ {
		d = hash.Hash256b(d[:])
		if i%2 == 0 {
			require.NoError(tr.Delete(d[:20]))
			_, err = tr.Get(d[:20])
			require.Equal(trie.ErrNotExist, errors.Cause(err))
		}
	}
	_, err = tr.RootHash()
	require.NoError(err)
	b.Log("upsert times:", b.N<<2, "buffer size:", flusherKV.Size())
	b.StopTimer()
	require.NoError(flusher.Flush())
	require.NoError(tr.Stop(context.Background()))
	require.NoError(kv.Stop(context.Background()))
}

func BenchmarkTrie(b *testing.B) {
	b.Run("without-async", func(b *testing.B) {
		benchmarkTrie(false, b)
	})
	b.Run("with-async", func(b *testing.B) {
		benchmarkTrie(true, b)
	})
}
