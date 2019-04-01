// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	bucket1 = "test_ns1"
	bucket2 = "test_ns2"
	bucket3 = "test_ns3"
	testK1  = [3][]byte{[]byte("key_1"), []byte("key_2"), []byte("key_3")}
	testV1  = [3][]byte{[]byte("value_1"), []byte("value_2"), []byte("value_3")}
	testK2  = [3][]byte{[]byte("key_4"), []byte("key_5"), []byte("key_6")}
	testV2  = [3][]byte{[]byte("value_4"), []byte("value_5"), []byte("value_6")}
	cfg     = config.Default.DB
)

func TestKVStorePutGet(t *testing.T) {
	testKVStorePutGet := func(kvStore KVStore, t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()

		assert.Nil(kvStore.Start(ctx))
		defer func() {
			err := kvStore.Stop(ctx)
			assert.Nil(err)
		}()

		assert.Nil(kvStore.Put(bucket1, []byte("key"), []byte("value")))
		value, err := kvStore.Get(bucket1, []byte("key"))
		assert.Nil(err)
		assert.Equal([]byte("value"), value)
		value, err = kvStore.Get("test_ns_1", []byte("key"))
		assert.NotNil(err)
		assert.Nil(value)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.NotNil(err)
		assert.Nil(value)
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		testKVStorePutGet(NewMemKVStore(), t)
	})

	path := "test-kv-store.bolt"
	cfg.DbPath = path
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testKVStorePutGet(NewOnDiskDB(cfg), t)
	})

	path = "test-kv-store.badger"
	cfg.DbPath = path
	cfg.UseBadgerDB = true
	t.Run("Badger DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testKVStorePutGet(NewOnDiskDB(cfg), t)
	})
}

func TestBatchRollback(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()

		assert.Nil(kvStore.Start(ctx))
		defer func() {
			err := kvStore.Stop(ctx)
			assert.Nil(err)
		}()

		assert.Nil(kvStore.Put(bucket1, testK1[0], testV1[0]))
		value, err := kvStore.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
		assert.Nil(kvStore.Put(bucket1, testK1[1], testV1[1]))
		value, err = kvStore.Get(bucket1, testK1[1])
		assert.Nil(err)
		assert.Equal(testV1[1], value)
		assert.Nil(kvStore.Put(bucket1, testK1[2], testV1[2]))
		value, err = kvStore.Get(bucket1, testK1[2])
		assert.Nil(err)
		assert.Equal(testV1[2], value)

		testV := [3][]byte{[]byte("value1.1"), []byte("value2.1"), []byte("value3.1")}
		if kvboltDB, ok := kvStore.(*boltDB); ok {
			err = kvboltDB.batchPutForceFail(bucket1, testK1[:], testV[:])
			assert.NotNil(err)
		} else if kvbadgerDB, ok := kvStore.(*boltDB); ok {
			err = kvbadgerDB.batchPutForceFail(bucket1, testK1[:], testV[:])
			assert.NotNil(err)
		}

		value, err = kvStore.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
		value, err = kvStore.Get(bucket1, testK1[1])
		assert.Nil(err)
		assert.Equal(testV1[1], value)
		value, err = kvStore.Get(bucket1, testK1[2])
		assert.Nil(err)
		assert.Equal(testV1[2], value)
	}

	path := "test-batch-rollback.bolt"
	cfg.DbPath = path
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBatchRollback(NewOnDiskDB(cfg), t)
	})

	path = "test-batch-rollback.badger"
	cfg.DbPath = path
	cfg.UseBadgerDB = true
	t.Run("Badger DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBatchRollback(NewOnDiskDB(cfg), t)
	})
}

func TestDBInMemBatchCommit(t *testing.T) {
	require := require.New(t)
	kvStore := NewMemKVStore()
	ctx := context.Background()
	batch := NewBatch()

	require.Nil(kvStore.Start(ctx))
	defer func() {
		err := kvStore.Stop(ctx)
		require.Nil(err)
	}()

	require.Nil(kvStore.Put(bucket1, testK1[0], testV1[1]))
	require.Nil(kvStore.Put(bucket2, testK2[1], testV2[0]))
	require.Nil(kvStore.Put(bucket1, testK1[2], testV1[0]))
	batch.Put(bucket1, testK1[0], testV1[0], "")
	value, err := kvStore.Get(bucket1, testK1[0])
	require.Nil(err)
	require.Equal(testV1[1], value)
	value, err = kvStore.Get(bucket2, testK2[1])
	require.Nil(err)
	require.Equal(testV2[0], value)
	require.Nil(kvStore.Commit(batch))
	value, err = kvStore.Get(bucket1, testK1[0])
	require.Nil(err)
	require.Equal(testV1[0], value)
}

func TestDBBatch(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()
		batch := NewBatch()

		require.Nil(kvStore.Start(ctx))
		defer func() {
			err := kvStore.Stop(ctx)
			require.Nil(err)
		}()

		require.Nil(kvStore.Put(bucket1, testK1[0], testV1[1]))
		require.Nil(kvStore.Put(bucket2, testK2[1], testV2[0]))
		require.Nil(kvStore.Put(bucket1, testK1[2], testV1[0]))

		batch.Put(bucket1, testK1[0], testV1[0], "")
		batch.Put(bucket2, testK2[1], testV2[1], "")
		value, err := kvStore.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[1], value)

		value, err = kvStore.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[0], value)
		require.Nil(kvStore.Commit(batch))

		value, err = kvStore.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[0], value)

		value, err = kvStore.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		value, err = kvStore.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[0], value)

		batch.Put(bucket1, testK1[0], testV1[1], "")
		require.NoError(kvStore.Commit(batch))

		require.Equal(0, batch.Size())

		value, err = kvStore.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		value, err = kvStore.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[1], value)

		require.Nil(kvStore.Commit(batch))

		batch.Put(bucket1, testK1[2], testV1[2], "")
		require.NoError(kvStore.Commit(batch))

		value, err = kvStore.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[2], value)

		value, err = kvStore.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		batch.Clear()
		batch.Put(bucket1, testK1[2], testV1[2], "")
		batch.Delete(bucket2, testK2[1], "")
		require.Nil(kvStore.Commit(batch))

		value, err = kvStore.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[2], value)

		_, err = kvStore.Get(bucket2, testK2[1])
		require.NotNil(err)
	}

	path := "test-batch-commit.bolt"
	cfg.DbPath = path
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBatchRollback(NewOnDiskDB(cfg), t)
	})

	path = "test-batch-commit.badger"
	cfg.DbPath = path
	cfg.UseBadgerDB = true
	t.Run("Badger DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testBatchRollback(NewOnDiskDB(cfg), t)
	})
}

func TestCacheKV(t *testing.T) {
	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		require.Nil(kv.Start(context.Background()))
		defer func() {
			err := kv.Stop(context.Background())
			require.Nil(err)
		}()

		cb := NewCachedBatch()
		cb.Put(bucket1, testK1[0], testV1[0], "")
		v, _ := cb.Get(bucket1, testK1[0])
		require.Equal(testV1[0], v)
		cb.Clear()
		require.Equal(0, cb.Size())
		_, err := cb.Get(bucket1, testK1[0])
		require.Error(err)
		cb.Put(bucket2, testK2[2], testV2[2], "")
		v, _ = cb.Get(bucket2, testK2[2])
		require.Equal(testV2[2], v)
		// put testK1[1] with a new value
		cb.Put(bucket1, testK1[1], testV1[2], "")
		v, _ = cb.Get(bucket1, testK1[1])
		require.Equal(testV1[2], v)
		// delete a non-existing entry is OK
		cb.Delete(bucket2, []byte("notexist"), "")
		require.Nil(kv.Commit(cb))

		v, _ = kv.Get(bucket1, testK1[1])
		require.Equal(testV1[2], v)

		cb = NewCachedBatch()
		require.NoError(kv.Commit(cb))
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		kv := NewMemKVStore()
		testFunc(kv, t)
	})

	path := "test-cache-kv.bolt"
	cfg.DbPath = path
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testFunc(NewOnDiskDB(cfg), t)
	})

	path = "test-cache-kv.badger"
	cfg.DbPath = path
	cfg.UseBadgerDB = true
	t.Run("Badger DB", func(t *testing.T) {
		testutil.CleanupPath(t, path)
		defer testutil.CleanupPath(t, path)
		testFunc(NewOnDiskDB(cfg), t)
	})
}
