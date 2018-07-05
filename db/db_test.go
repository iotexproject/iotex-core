// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/test/util"
)

var (
	bucket = "test_ns"
	testK  = [3][]byte{[]byte("key_1"), []byte("key_2"), []byte("key_3")}
	testV  = [3][]byte{[]byte("value_1"), []byte("value_2"), []byte("value_3")}
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

		err = kvStore.Put(bucket, []byte("key"), []byte("value"))
		assert.Nil(err)
		value, err := kvStore.Get(bucket, []byte("key"))
		assert.Nil(err)
		assert.Equal([]byte("value"), value)
		value, err = kvStore.Get("test_ns_1", []byte("key"))
		assert.NotNil(err)
		assert.Nil(value)
		value, err = kvStore.Get(bucket, testK[0])
		assert.NotNil(err)
		assert.Nil(value)

		err = kvStore.PutIfNotExists(bucket, testK[0], testV[0])
		assert.Nil(err)
		value, err = kvStore.Get(bucket, testK[0])
		assert.Nil(err)
		assert.Equal(testV[0], value)

		err = kvStore.PutIfNotExists(bucket, testK[0], testV[1])
		assert.NotNil(err)
		value, err = kvStore.Get(bucket, testK[0])
		assert.Nil(err)
		assert.Equal(testV[0], value)
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		testKVStorePutGet(NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + string(rand.Int())
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

		err = kvboltDB.Put(bucket, testK[0], testV[0])
		assert.Nil(err)
		value, err := kvboltDB.Get(bucket, testK[0])
		assert.Nil(err)
		assert.Equal(testV[0], value)
		err = kvboltDB.Put(bucket, testK[1], testV[1])
		assert.Nil(err)
		value, err = kvboltDB.Get(bucket, testK[1])
		assert.Nil(err)
		assert.Equal(testV[1], value)
		err = kvboltDB.Put(bucket, testK[2], testV[2])
		assert.Nil(err)
		value, err = kvboltDB.Get(bucket, testK[2])
		assert.Nil(err)
		assert.Equal(testV[2], value)

		testV1 := [3][]byte{[]byte("value1.1"), []byte("value2.1"), []byte("value3.1")}

		err = kvboltDB.batchPutForceFail(bucket, testK[:], testV1[:])
		assert.NotNil(err)

		value, err = kvboltDB.Get(bucket, testK[0])
		assert.Nil(err)
		assert.Equal(testV[0], value)
		value, err = kvboltDB.Get(bucket, testK[1])
		assert.Nil(err)
		assert.Equal(testV[1], value)
		value, err = kvboltDB.Get(bucket, testK[2])
		assert.Nil(err)
		assert.Equal(testV[2], value)
	}

	path := "/tmp/test-batch-rollback-" + string(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		util.CleanupPath(t, path)
		defer util.CleanupPath(t, path)
		testBatchRollback(NewBoltDB(path, nil), t)
	})
}
