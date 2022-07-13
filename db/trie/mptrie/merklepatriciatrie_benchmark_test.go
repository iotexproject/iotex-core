package mptrie

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/util/randutil"
	"github.com/iotexproject/iotex-core/testutil"
)

func BenchmarkTrie_Get(b *testing.B)            { benchTrieGet(b, false, false) }
func BenchmarkTrie_GetWithAsync(b *testing.B)   { benchTrieGet(b, true, false) }
func BenchmarkTrie_GetDB(b *testing.B)          { benchTrieGet(b, false, true) }
func BenchmarkTrie_GetDBWithAsync(b *testing.B) { benchTrieGet(b, true, true) }
func BenchmarkTrie_UpsertLE(b *testing.B)       { benchTrieUpsert(b, binary.LittleEndian) }
func BenchmarkTrie_UpsertBE(b *testing.B)       { benchTrieUpsert(b, binary.BigEndian) }

const (
	_benchElemCount = 20000
	_keyLength      = 32
)

func benchTrieGet(b *testing.B, async, withDB bool) {
	var (
		require = require.New(b)
		opts    = []Option{KeyLengthOption(_keyLength)}
		flush   func() error
	)
	if async {
		opts = append(opts, AsyncOption())
	}
	if withDB {
		testPath, err := testutil.PathOfTempFile(fmt.Sprintf("test-kv-store-%t.bolt", async))
		require.NoError(err)
		defer testutil.CleanupPath(testPath)
		cfg := db.DefaultConfig
		cfg.DbPath = testPath
		dao := db.NewBoltDB(cfg)
		flusher, err := db.NewKVStoreFlusher(dao, batch.NewCachedBatch())
		require.NoError(err)
		flusherKV := flusher.KVStoreWithBuffer()
		flush = flusher.Flush
		kvStore, err := trie.NewKVStore("test", flusherKV)
		require.NoError(err)
		require.NoError(kvStore.Start(context.Background()))
		opts = append(opts, KVStoreOption(kvStore))
	}
	tr, err := New(opts...)
	require.NoError(err)
	require.NoError(tr.Start(context.Background()))
	defer require.NoError(tr.Stop(context.Background()))

	key := make([]byte, _keyLength)
	for i := 0; i < _benchElemCount; i++ {
		binary.LittleEndian.PutUint64(key, uint64(i))
		require.NoError(tr.Upsert(key, key))
	}
	binary.LittleEndian.PutUint64(key, uint64(_benchElemCount)/2)
	if withDB {
		require.NoError(flush())
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tr.Get(key)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func benchTrieUpsert(b *testing.B, e binary.ByteOrder) {
	var (
		require = require.New(b)
		opts    = []Option{KeyLengthOption(_keyLength)}
	)
	trie, err := New(opts...)
	require.NoError(err)
	require.NoError(trie.Start(context.Background()))
	defer require.NoError(trie.Stop(context.Background()))
	k := make([]byte, _keyLength)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.PutUint64(k, uint64(i))
		trie.Upsert(k, k)
	}
	b.StopTimer()
	b.ReportAllocs()
}

func BenchmarkTrie_UpsertWithAsync(b *testing.B) {
	tr, _, callback := initTrie(2)
	defer callback()

	keys := [][]byte{}
	for i := 0; i < 10; i++ {
		for j := 0; j < 256; j++ {
			key := []byte{byte(i), byte(j)}
			keys = append(keys, key)
		}
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for _, v := range keys {
			tr.Upsert(v, []byte{})
		}
	}
}

func BenchmarkTrie_RootHashWithAsync(b *testing.B) {
	tr, flush, callback := initTrie(2)
	defer callback()

	keys := [][]byte{}
	for i := 0; i < 10; i++ {
		for j := 0; j < 256; j++ {
			key := []byte{byte(i), byte(j)}
			keys = append(keys, key)
		}
	}
	for _, v := range keys {
		tr.Upsert(v, []byte{})
	}
	tr.RootHash()
	flush()

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		tr.Upsert(keys[randutil.Intn(len(keys))], []byte{})
		tr.RootHash()
	}
}

func initTrie(keyLen int) (trie.Trie, func() error, func()) {
	testPath, err := testutil.PathOfTempFile("test-kv-store.bolt")
	if err != nil {
		panic(err)
	}
	cfg := db.DefaultConfig
	cfg.DbPath = testPath
	dao := db.NewBoltDB(cfg)
	flusher, err := db.NewKVStoreFlusher(dao, batch.NewCachedBatch())
	if err != nil {
		panic(err)
	}
	flusherKV := flusher.KVStoreWithBuffer()
	kvStore, err := trie.NewKVStore("test", flusherKV)
	if err != nil {
		panic(err)
	}
	err = kvStore.Start(context.Background())
	if err != nil {
		panic(err)
	}
	opts := []Option{KeyLengthOption(keyLen), AsyncOption(), KVStoreOption(kvStore)}
	tr, err := New(opts...)
	if err != nil {
		panic(err)
	}
	err = tr.Start(context.Background())
	if err != nil {
		panic(err)
	}
	return tr, flusher.Flush, func() {
		tr.Stop(context.Background())
		testutil.CleanupPath(testPath)
	}
}
