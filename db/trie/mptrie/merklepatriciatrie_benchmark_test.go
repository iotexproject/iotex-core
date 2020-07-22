package mptrie

import (
	"context"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/testutil"
)

// BenchmarkTrieGet-4                	   29383	     44256 ns/op	   19754 B/op	     269 allocs/op
// BenchmarkTrieGetWithAsync-4       	 1214578	      1022 ns/op	     160 B/op	       5 allocs/op
// BenchmarkTrieGetDisk-4            	   26509	     49948 ns/op	   25371 B/op	     305 allocs/op
// BenchmarkTrieGetDiskWithAsync-4   	 1156329	      1101 ns/op	     160 B/op	       5 allocs/op
// BenchmarkUpsertLE-4               	   10000	    156839 ns/op	   41624 B/op	     434 allocs/op
// BenchmarkUpsertBE-4               	    9362	    251458 ns/op	   79795 B/op	     900 allocs/op
func BenchmarkTrie_Get(b *testing.B)            { benchTrieGet(b, false, false) }
func BenchmarkTrie_GetWithAsync(b *testing.B)   { benchTrieGet(b, true, false) }
func BenchmarkTrie_GetDB(b *testing.B)          { benchTrieGet(b, false, true) }
func BenchmarkTrie_GetDBWithAsync(b *testing.B) { benchTrieGet(b, true, true) }
func BenchmarkTrie_UpsertLE(b *testing.B)       { benchTrieUpsert(b, binary.LittleEndian) }
func BenchmarkTrie_UpsertBE(b *testing.B)       { benchTrieUpsert(b, binary.BigEndian) }

const (
	benchElemCount = 20000
	keyLength      = 32
)

func benchTrieGet(b *testing.B, async, withDB bool) {
	var (
		require = require.New(b)
		opts    = []Option{KeyLengthOption(keyLength)}
		flush   func() error
	)
	if async {
		opts = append(opts, AsyncOption())
	}
	if withDB {
		testPath, err := testutil.PathOfTempFile(fmt.Sprintf("test-kv-store-%t.bolt", async))
		require.NoError(err)
		cfg := config.Default.DB
		cfg.DbPath = testPath
		dao := db.NewBoltDB(cfg)
		flusher, err := db.NewKVStoreFlusher(dao, batch.NewCachedBatch())
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

	key := make([]byte, keyLength)
	for i := 0; i < benchElemCount; i++ {
		binary.LittleEndian.PutUint64(key, uint64(i))
		require.NoError(tr.Upsert(key, key))
	}
	binary.LittleEndian.PutUint64(key, uint64(benchElemCount)/2)
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
		opts    = []Option{KeyLengthOption(keyLength)}
	)
	trie, err := New(opts...)
	require.NoError(err)
	require.NoError(trie.Start(context.Background()))
	defer require.NoError(trie.Stop(context.Background()))
	k := make([]byte, keyLength)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.PutUint64(k, uint64(i))
		trie.Upsert(k, k)
	}
	b.StopTimer()
	b.ReportAllocs()
}
