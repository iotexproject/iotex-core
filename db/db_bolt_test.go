// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/testutil"
)

func TestBoltDB_NilDB_DoesNotPanic(t *testing.T) {
	r := require.New(t)

	cfg := DefaultConfig
	kv := NewBoltDB(cfg)
	_, err := kv.Get("namespace", []byte("test"))
	r.Errorf(err, "db hasn't started")

	r.Errorf(kv.Delete("test", []byte("delete_test")), "db hasn't started")
}

func TestBucketExists(t *testing.T) {
	r := require.New(t)
	testPath, err := testutil.PathOfTempFile("test-bucket")
	r.NoError(err)
	defer func() {
		testutil.CleanupPath(t, testPath)
	}()

	cfg := DefaultConfig
	cfg.DbPath = testPath
	kv := NewBoltDB(cfg)
	ctx := context.Background()
	r.NoError(kv.Start(ctx))
	defer kv.Stop(ctx)
	r.False(kv.BucketExists("name"))
	r.NoError(kv.Put("name", []byte("key"), []byte{}))
	r.True(kv.BucketExists("name"))
}

func BenchmarkBoltDB_Get(b *testing.B) {
	runBenchmark := func(b *testing.B, size int) {
		path, err := testutil.PathOfTempFile("boltdb")
		require.NoError(b, err)
		db := BoltDB{
			path:   path,
			config: DefaultConfig,
		}
		db.Start(context.Background())
		defer db.Stop(context.Background())

		key := []byte("key")
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(rand.Int())
		}
		require.NoError(b, db.Put("ns", key, data))

		b.ResetTimer()
		for n := 0; n < b.N; n++ {
			b.StartTimer()
			_, err := db.Get("ns", key)
			b.StopTimer()
			require.NoError(b, err)
		}
	}

	b.Run("100", func(b *testing.B) {
		runBenchmark(b, 100)
	})
	b.Run("10000", func(b *testing.B) {
		runBenchmark(b, 100)
	})
	b.Run("1000000", func(b *testing.B) {
		runBenchmark(b, 100)
	})
}
