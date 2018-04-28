// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotexproject/iotex-core/common/utils"
)

func TestKVStorePutGet(t *testing.T) {
	testKVStorePutGet := func(kvStore KVStore, t *testing.T) {
		err := kvStore.Init()
		assert.Nil(t, err)
		err = kvStore.Start()
		assert.Nil(t, err)
		defer func() {
			err = kvStore.Stop()
			assert.Nil(t, err)
		}()

		err = kvStore.Put("test_ns", []byte("key"), []byte("value"))
		assert.Nil(t, err)
		value, err := kvStore.Get("test_ns", []byte("key"))
		assert.Nil(t, err)
		assert.Equal(t, "value", string(value))
		value, err = kvStore.Get("test_ns_1", []byte("key"))
		assert.Nil(t, err)
		assert.Nil(t, value)
		value, err = kvStore.Get("test_ns", []byte("key_1"))
		assert.Nil(t, err)
		assert.Nil(t, value)

		err = kvStore.PutIfNotExists("test_ns", []byte("key_1"), []byte("value_1"))
		assert.Nil(t, err)
		value, err = kvStore.Get("test_ns", []byte("key_1"))
		assert.Nil(t, err)
		assert.Equal(t, "value_1", string(value))

		err = kvStore.PutIfNotExists("test_ns", []byte("key_1"), []byte("value_2"))
		assert.Nil(t, err)
		value, err = kvStore.Get("test_ns", []byte("key_1"))
		assert.Nil(t, err)
		assert.Equal(t, "value_1", string(value))
	}

	t.Run("In-memory KV Store", func(t *testing.T) {
		testKVStorePutGet(NewMemKVStore(), t)
	})

	path := "/tmp/test-kv-store-" + string(rand.Int())
	t.Run("Bolt DB", func(t *testing.T) {
		cleanup := func() {
			if utils.FileExists(path) {
				err := os.Remove(path)
				assert.Nil(t, err)
			}
		}

		cleanup()
		defer cleanup()
		testKVStorePutGet(NewBoltDB(path, nil), t)
	})
}
