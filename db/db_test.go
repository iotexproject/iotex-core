// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"math/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/util"
)

var (
	bucket1 = "test_ns1"
	bucket2 = "test_ns2"
	bucket3 = "test_ns3"
	testK1  = [3][]byte{[]byte("key_1"), []byte("key_2"), []byte("key_3")}
	testV1  = [3][]byte{[]byte("value_1"), []byte("value_2"), []byte("value_3")}
	testK2  = [3][]byte{[]byte("key_4"), []byte("key_5"), []byte("key_6")}
	testV2  = [3][]byte{[]byte("value_4"), []byte("value_5"), []byte("value_6")}
)

func TestKVStorePutGet(t *testing.T) {
	testKVStorePutGet := func(kvStore KVStore, t *testing.T) {
		assert := assert.New(t)
		ctx := context.Background()

		err := kvStore.Start(ctx)
		assert.Nil(err)
		defer func() {
			err = kvStore.Stop(ctx)
			assert.Nil(err)
		}()

		err = kvStore.Put(bucket1, []byte("key"), []byte("value"))
		assert.Nil(err)
		value, err := kvStore.Get(bucket1, []byte("key"))
		assert.Nil(err)
		assert.Equal([]byte("value"), value)
		value, err = kvStore.Get("test_ns_1", []byte("key"))
		assert.NotNil(err)
		assert.Nil(value)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.NotNil(err)
		assert.Nil(value)

		err = kvStore.PutIfNotExists(bucket1, testK1[0], testV1[0])
		assert.Nil(err)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)

		err = kvStore.PutIfNotExists(bucket1, testK1[0], testV1[1])
		assert.NotNil(err)
		value, err = kvStore.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		testKVStorePutGet(NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + strconv.Itoa(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		util.CleanupPath(t, path)
		defer util.CleanupPath(t, path)
		testKVStorePutGet(NewBoltDB(path, nil), t)
	})
}

func TestBatchRollback(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		assert := assert.New(t)

		ctx := context.Background()
		kvboltDB := kvStore.(*boltDB)

		err := kvboltDB.Start(ctx)
		assert.Nil(err)
		defer func() {
			err = kvboltDB.Stop(ctx)
			assert.Nil(err)
		}()

		err = kvboltDB.Put(bucket1, testK1[0], testV1[0])
		assert.Nil(err)
		value, err := kvboltDB.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
		err = kvboltDB.Put(bucket1, testK1[1], testV1[1])
		assert.Nil(err)
		value, err = kvboltDB.Get(bucket1, testK1[1])
		assert.Nil(err)
		assert.Equal(testV1[1], value)
		err = kvboltDB.Put(bucket1, testK1[2], testV1[2])
		assert.Nil(err)
		value, err = kvboltDB.Get(bucket1, testK1[2])
		assert.Nil(err)
		assert.Equal(testV1[2], value)

		testV := [3][]byte{[]byte("value1.1"), []byte("value2.1"), []byte("value3.1")}

		err = kvboltDB.batchPutForceFail(bucket1, testK1[:], testV[:])
		assert.NotNil(err)

		value, err = kvboltDB.Get(bucket1, testK1[0])
		assert.Nil(err)
		assert.Equal(testV1[0], value)
		value, err = kvboltDB.Get(bucket1, testK1[1])
		assert.Nil(err)
		assert.Equal(testV1[1], value)
		value, err = kvboltDB.Get(bucket1, testK1[2])
		assert.Nil(err)
		assert.Equal(testV1[2], value)
	}

	path := "/tmp/test-batch-rollback-" + strconv.Itoa(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		util.CleanupPath(t, path)
		defer util.CleanupPath(t, path)
		testBatchRollback(NewBoltDB(path, nil), t)
	})
}

func TestDBBatch(t *testing.T) {
	testBatchRollback := func(kvStore KVStore, t *testing.T) {
		require := require.New(t)

		ctx := context.Background()
		kvboltDB := kvStore.(*boltDB)
		batch := boltDBBatch{bdb: kvboltDB}

		err := kvboltDB.Start(ctx)
		require.Nil(err)
		defer func() {
			err = kvboltDB.Stop(ctx)
			require.Nil(err)
		}()

		err = kvboltDB.Put(bucket1, testK1[0], testV1[1])
		require.Nil(err)

		err = kvboltDB.Put(bucket2, testK2[1], testV2[0])
		require.Nil(err)

		err = kvboltDB.Put(bucket1, testK1[2], testV1[0])
		require.Nil(err)

		err = batch.Put(bucket1, testK1[0], testV1[0], "")
		require.Nil(err)
		err = batch.Put(bucket2, testK2[1], testV2[1], "")
		require.Nil(err)

		value, err := kvboltDB.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[1], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[0], value)

		err = batch.Commit()
		require.Nil(err)

		value, err = kvboltDB.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[0], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		value, err = kvboltDB.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[0], value)

		err = batch.Put(bucket1, testK1[0], testV1[1], "")
		require.Nil(err)
		err = batch.PutIfNotExists(bucket2, testK2[1], testV2[0], "")
		require.Nil(err)
		err = batch.Commit()
		require.Equal(err, ErrAlreadyExist)
		err = batch.Clear()
		require.Nil(err)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		value, err = kvboltDB.Get(bucket1, testK1[0])
		require.Nil(err)
		require.Equal(testV1[0], value)

		err = batch.PutIfNotExists(bucket3, testK2[0], testV2[0], "")
		require.Nil(err)
		err = batch.Commit()
		require.Nil(err)

		value, err = kvboltDB.Get(bucket3, testK2[0])
		require.Nil(err)
		require.Equal(testV2[0], value)

		err = batch.Put(bucket1, testK1[2], testV1[2], "")
		require.Nil(err)
		// we did not set key in bucket3 yet, so this operation will fail and
		// cause transaction rollback
		err = batch.PutIfNotExists(bucket3, testK2[0], testV2[1], "")
		require.Nil(err)
		err = batch.Commit()
		require.NotNil(err)

		value, err = kvboltDB.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[0], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.Nil(err)
		require.Equal(testV2[1], value)

		err = batch.Clear()
		require.Nil(err)
		err = batch.Put(bucket1, testK1[2], testV1[2], "")
		require.Nil(err)
		err = batch.Delete(bucket2, testK2[1], "")
		require.Nil(err)
		err = batch.Commit()
		require.Nil(err)

		value, err = kvboltDB.Get(bucket1, testK1[2])
		require.Nil(err)
		require.Equal(testV1[2], value)

		value, err = kvboltDB.Get(bucket2, testK2[1])
		require.NotNil(err)
	}

	t.Run("Bolt DB", func(t *testing.T) {
		path := "/tmp/test-batch-rollback-" + strconv.Itoa(rand.Int())
		util.CleanupPath(t, path)
		defer util.CleanupPath(t, path)
		testBatchRollback(NewBoltDB(path, nil), t)
	})
}
