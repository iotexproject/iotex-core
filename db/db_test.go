// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/testutil"
)

var (
	bucket1 = "test_ns1"
	bucket2 = "test_ns2"
	testK1  = [3][]byte{[]byte("key_1"), []byte("key_2"), []byte("key_3")}
	testV1  = [3][]byte{[]byte("value_1"), []byte("value_2"), []byte("value_3")}
	testK2  = [3][]byte{[]byte("key_4"), []byte("key_5"), []byte("key_6")}
	testV2  = [3][]byte{[]byte("value_4"), []byte("value_5"), []byte("value_6")}
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

	path := "test-kv-store.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(t, err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	for _, v := range []KVStore{
		NewMemKVStore(),
		NewBoltDB(cfg),
	} {
		t.Run("test put get", func(t *testing.T) {
			testKVStorePutGet(v, t)
		})
	}

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
		kvboltDB := kvStore.(*BoltDB)
		err = kvboltDB.batchPutForceFail(bucket1, testK1[:], testV[:])
		assert.NotNil(err)

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
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(t, err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	t.Run("test rollback", func(t *testing.T) {
		testBatchRollback(NewBoltDB(cfg), t)
	})

}

func TestDBInMemBatchCommit(t *testing.T) {
	require := require.New(t)
	kvStore := NewMemKVStore()
	ctx := context.Background()
	b := batch.NewBatch()

	require.NoError(kvStore.Start(ctx))
	defer func() {
		require.NoError(kvStore.Stop(ctx))
	}()

	require.NoError(kvStore.Put(bucket1, testK1[0], testV1[1]))
	require.NoError(kvStore.Put(bucket2, testK2[1], testV2[0]))
	require.NoError(kvStore.Put(bucket1, testK1[2], testV1[0]))
	b.Put(bucket1, testK1[0], testV1[0], "")
	value, err := kvStore.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[1], value)
	value, err = kvStore.Get(bucket2, testK2[1])
	require.NoError(err)
	require.Equal(testV2[0], value)
	require.NoError(kvStore.WriteBatch(b))
	value, err = kvStore.Get(bucket1, testK1[0])
	require.NoError(err)
	require.Equal(testV1[0], value)
}

func TestDBBatch(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		require := require.New(t)
		ctx := context.Background()
		batch := batch.NewBatch()

		require.NoError(kvStore.Start(ctx))
		defer func() {
			require.NoError(kvStore.Stop(ctx))
		}()

		require.NoError(kvStore.Put(bucket1, testK1[0], testV1[1]))
		require.NoError(kvStore.Put(bucket2, testK2[1], testV2[0]))
		require.NoError(kvStore.Put(bucket1, testK1[2], testV1[0]))

		batch.Put(bucket1, testK1[0], testV1[0], "")
		batch.Put(bucket2, testK2[1], testV2[1], "")
		value, err := kvStore.Get(bucket1, testK1[0])
		require.NoError(err)
		require.Equal(testV1[1], value)

		value, err = kvStore.Get(bucket2, testK2[1])
		require.NoError(err)
		require.Equal(testV2[0], value)
		require.NoError(kvStore.WriteBatch(batch))
		batch.Clear()

		value, err = kvStore.Get(bucket1, testK1[0])
		require.NoError(err)
		require.Equal(testV1[0], value)

		value, err = kvStore.Get(bucket2, testK2[1])
		require.NoError(err)
		require.Equal(testV2[1], value)

		value, err = kvStore.Get(bucket1, testK1[2])
		require.NoError(err)
		require.Equal(testV1[0], value)

		batch.Put(bucket1, testK1[0], testV1[1], "")
		require.NoError(kvStore.WriteBatch(batch))
		batch.Clear()

		require.Equal(0, batch.Size())

		value, err = kvStore.Get(bucket2, testK2[1])
		require.NoError(err)
		require.Equal(testV2[1], value)

		value, err = kvStore.Get(bucket1, testK1[0])
		require.NoError(err)
		require.Equal(testV1[1], value)

		require.NoError(kvStore.WriteBatch(batch))
		batch.Clear()

		batch.Put(bucket1, testK1[2], testV1[2], "")
		require.NoError(kvStore.WriteBatch(batch))
		batch.Clear()

		value, err = kvStore.Get(bucket1, testK1[2])
		require.NoError(err)
		require.Equal(testV1[2], value)

		value, err = kvStore.Get(bucket2, testK2[1])
		require.NoError(err)
		require.Equal(testV2[1], value)

		batch.Clear()
		batch.Put(bucket1, testK1[2], testV1[2], "")
		batch.Delete(bucket2, testK2[1], "")
		require.NoError(kvStore.WriteBatch(batch))
		batch.Clear()

		value, err = kvStore.Get(bucket1, testK1[2])
		require.NoError(err)
		require.Equal(testV1[2], value)

		_, err = kvStore.Get(bucket2, testK2[1])
		require.Error(err)
	}

	path := "test-batch-commit.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(t, err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	for _, v := range []KVStore{
		NewMemKVStore(),
		NewBoltDB(cfg),
	} {
		t.Run("test batch", func(t *testing.T) {
			testBatchRollback(v, t)
		})
	}
}

func TestCacheKV(t *testing.T) {
	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		require.NoError(kv.Start(context.Background()))
		defer func() {
			require.NoError(kv.Stop(context.Background()))
		}()

		cb := batch.NewCachedBatch()
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
		require.NoError(kv.WriteBatch(cb))

		v, _ = kv.Get(bucket1, testK1[1])
		require.Equal(testV1[2], v)

		cb = batch.NewCachedBatch()
		require.NoError(kv.WriteBatch(cb))
	}

	path := "test-cache-kv.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(t, err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	for _, v := range []KVStore{
		NewMemKVStore(),
		NewBoltDB(cfg),
	} {
		t.Run("test cache kv", func(t *testing.T) {
			testFunc(v, t)
		})
	}
}

func TestDeleteBucket(t *testing.T) {
	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		require.NoError(kv.Start(context.Background()))
		defer func() {
			require.NoError(kv.Stop(context.Background()))
		}()

		require.NoError(kv.Put(bucket1, testK1[0], testV1[0]))
		v, err := kv.Get(bucket1, testK1[0])
		require.NoError(err)
		require.Equal(testV1[0], v)

		require.NoError(kv.Put(bucket2, testK1[0], testV1[0]))
		v, err = kv.Get(bucket2, testK1[0])
		require.NoError(err)
		require.Equal(testV1[0], v)

		require.NoError(kv.Delete(bucket1, nil))
		v, err = kv.Get(bucket1, testK1[0])
		require.Equal(ErrNotExist, errors.Cause(err))
		require.Equal([]uint8([]byte(nil)), v)

		v, _ = kv.Get(bucket2, testK1[0])
		require.Equal(testV1[0], v)
	}

	path := "test-delete.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(t, err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	t.Run("test delete bucket", func(t *testing.T) {
		testFunc(NewBoltDB(cfg), t)
	})
}

func TestFilter(t *testing.T) {
	require := require.New(t)

	testFunc := func(kv KVStore, t *testing.T) {
		require.NoError(kv.Start(context.Background()))
		defer func() {
			require.NoError(kv.Stop(context.Background()))
		}()

		tests := []struct {
			ns     string
			prefix []byte
		}{
			{
				bucket1,
				[]byte("test"),
			},
			{
				bucket1,
				[]byte("come"),
			},
			{
				bucket2,
				[]byte("back"),
			},
		}

		// add 100 keys with each prefix
		b := batch.NewBatch()
		numKey := 100
		for _, e := range tests {
			for i := 0; i < numKey; i++ {
				k := append(e.prefix, byteutil.Uint64ToBytesBigEndian(uint64(i))...)
				v := hash.Hash256b(k)
				b.Put(e.ns, k, v[:], "")
			}
		}
		require.NoError(kv.WriteBatch(b))

		_, _, err := kv.Filter("nonamespace", func(k, v []byte) bool {
			return bytes.HasPrefix(k, v)
		}, nil, nil)
		require.Equal(ErrBucketNotExist, errors.Cause(err))

		// filter using func with no match
		fk, fv, err := kv.Filter(tests[0].ns, func(k, v []byte) bool {
			return bytes.HasPrefix(k, tests[2].prefix)
		}, nil, nil)
		require.Nil(fk)
		require.Nil(fv)
		require.Equal(ErrNotExist, errors.Cause(err))

		// filter out <k, v> pairs
		for _, e := range tests {
			fk, fv, err := kv.Filter(e.ns, func(k, v []byte) bool {
				return bytes.HasPrefix(k, e.prefix)
			}, nil, nil)
			require.NoError(err)
			require.Equal(numKey, len(fk))
			require.Equal(numKey, len(fv))
			for i := range fk {
				k := append(e.prefix, byteutil.Uint64ToBytesBigEndian(uint64(i))...)
				require.Equal(fk[i], k)
				v := hash.Hash256b(k)
				require.Equal(fv[i], v[:])
			}
		}

		// filter with min/max key
		for _, e := range tests {
			min := 9
			max := 91
			minKey := append(e.prefix, byteutil.Uint64ToBytesBigEndian(uint64(min))...)
			maxKey := append(e.prefix, byteutil.Uint64ToBytesBigEndian(uint64(max))...)
			fk, fv, err := kv.Filter(e.ns, func(k, v []byte) bool {
				return bytes.HasPrefix(k, e.prefix)
			}, minKey, maxKey)
			require.NoError(err)
			require.Equal(max-min+1, len(fk))
			require.Equal(max-min+1, len(fv))
			for i := range fk {
				k := append(e.prefix, byteutil.Uint64ToBytesBigEndian(uint64(i+min))...)
				require.Equal(fk[i], k)
				v := hash.Hash256b(k)
				require.Equal(fv[i], v[:])
			}
		}
	}

	path := "test-filter.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	t.Run("test filter", func(t *testing.T) {
		testFunc(NewBoltDB(cfg), t)
	})
}
