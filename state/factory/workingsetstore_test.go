// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/stretchr/testify/require"
)

func TestStateDBWorkingSetStore(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	view := protocol.View{}
	inMemStore := db.NewMemKVStore()
	flusher, err := db.NewKVStoreFlusher(inMemStore, batch.NewCachedBatch())
	require.NoError(err)
	store := newStateDBWorkingSetStore(view, flusher)
	require.NotNil(store)
	require.NoError(store.Start(ctx))
	name := "name"
	viewValue := "value"
	namespace := "namespace"
	key1 := []byte("key1")
	value1 := []byte("value1")
	key2 := []byte("key2")
	value2 := []byte("value2")
	key3 := []byte("key3")
	value3 := []byte("value3")
	t.Run("test view", func(t *testing.T) {
		_, err := store.ReadView(name)
		require.Error(err)
		require.NoError(store.WriteView(name, viewValue))
		valueInView, err := store.ReadView(name)
		require.NoError(err)
		require.Equal(valueInView, viewValue)
	})
	t.Run("test kvstore feature", func(t *testing.T) {
		_, err := store.Get(namespace, key1)
		require.Error(err)
		require.NoError(store.Delete(namespace, key1))
		require.NoError(store.Put(namespace, key1, value1))
		valueInStore, err := store.Get(namespace, key1)
		require.NoError(err)
		require.True(bytes.Equal(value1, valueInStore))
		sn1 := store.Snapshot()
		require.NoError(store.Put(namespace, key2, value2))
		valueInStore, err = store.Get(namespace, key2)
		require.NoError(err)
		require.True(bytes.Equal(value2, valueInStore))
		store.Snapshot()
		require.NoError(store.Put(namespace, key3, value3))
		valueInStore, err = store.Get(namespace, key3)
		require.NoError(err)
		require.True(bytes.Equal(value3, valueInStore))
		valuesInStore, err := store.States(namespace, [][]byte{key1, key2, key3})
		require.Equal(3, len(valuesInStore))
		require.True(bytes.Equal(value1, valuesInStore[0]))
		require.True(bytes.Equal(value2, valuesInStore[1]))
		require.True(bytes.Equal(value3, valuesInStore[2]))
		t.Run("test digest", func(t *testing.T) {
			h := store.Digest()
			require.Equal("e1f83be0a44ae601061724990036b8a40edbf81cffc639657c9bb2c5d384defa", hex.EncodeToString(h[:]))
		})
		sn3 := store.Snapshot()
		require.NoError(store.Delete(namespace, key1))
		_, err = store.Get(namespace, key1)
		require.Error(err)
		valuesInStore, err = store.States(namespace, [][]byte{key1, key2, key3})
		require.Equal(3, len(valuesInStore))
		require.Nil(valuesInStore[0])
		require.True(bytes.Equal(value2, valuesInStore[1]))
		require.True(bytes.Equal(value3, valuesInStore[2]))
		require.NoError(store.RevertSnapshot(sn3))
		valueInStore, err = store.Get(namespace, key1)
		require.NoError(err)
		require.NoError(store.RevertSnapshot(sn1))
		require.True(bytes.Equal(value1, valueInStore))
		_, err = store.Get(namespace, key2)
		require.Error(err)
	})
	t.Run("finalize & commit", func(t *testing.T) {
		height := uint64(100)
		_, err := store.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.Error(err)
		_, err = inMemStore.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.Error(err)
		require.NoError(store.Finalize(height))
		heightInStore, err := store.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.NoError(err)
		require.True(bytes.Equal(heightInStore, byteutil.Uint64ToBytes(height)))
		_, err = inMemStore.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.Error(err)
		require.NoError(store.Commit())
		heightInStore, err = inMemStore.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.NoError(err)
		require.True(bytes.Equal(heightInStore, byteutil.Uint64ToBytes(height)))
	})
	require.NoError(store.Stop(ctx))
}

func TestFactoryWorkingSetStore(t *testing.T) {
	// TODO: add unit test for factory working set store
}
