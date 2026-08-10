// Copyright (c) 2022 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

type valueBytes []byte

func (v *valueBytes) Serialize() ([]byte, error) {
	return *v, nil
}

func (v *valueBytes) Deserialize(data []byte) error {
	*v = data
	return nil
}

func TestStateDBWorkingSetStore(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	inMemStore := db.NewMemKVStore()
	flusher, err := db.NewKVStoreFlusher(inMemStore, batch.NewCachedBatch())
	require.NoError(err)
	store := newStateDBWorkingSetStore(flusher, true)
	require.NotNil(store)
	require.NoError(store.Start(ctx))
	namespace := "namespace"
	key1 := []byte("key1")
	value1 := valueBytes("value1")
	key2 := []byte("key2")
	value2 := valueBytes("value2")
	key3 := []byte("key3")
	value3 := valueBytes("value3")
	t.Run("test kvstore feature", func(t *testing.T) {
		var value valueBytes
		err := store.GetObject(namespace, key1, &value)
		require.Error(err)
		require.NoError(store.DeleteObject(namespace, key1, nil))
		require.NoError(store.PutObject(namespace, key1, &value1))
		var valueInStore valueBytes
		err = store.GetObject(namespace, key1, &valueInStore)
		require.NoError(err)
		require.True(bytes.Equal(value1, valueInStore))
		sn1 := store.Snapshot()
		require.NoError(store.PutObject(namespace, key2, &value2))
		err = store.GetObject(namespace, key2, &valueInStore)
		require.NoError(err)
		require.True(bytes.Equal(value2, valueInStore))
		store.Snapshot()
		require.NoError(store.PutObject(namespace, key3, &value3))
		err = store.GetObject(namespace, key3, &valueInStore)
		require.NoError(err)
		require.True(bytes.Equal(value3, valueInStore))
		iter, err := store.States(namespace, nil, [][]byte{key1, key2, key3}, nil)
		require.NoError(err)
		require.Equal(3, iter.Size())
		var valuesInStore []valueBytes
		for {
			vb := valueBytes{}
			if _, err := iter.Next(&vb); err == nil {
				valuesInStore = append(valuesInStore, vb)
			} else {
				break
			}
		}
		require.True(bytes.Equal(value1, valuesInStore[0]))
		require.True(bytes.Equal(value2, valuesInStore[1]))
		require.True(bytes.Equal(value3, valuesInStore[2]))
		t.Run("test digest", func(t *testing.T) {
			h := store.Digest()
			require.Equal("e1f83be0a44ae601061724990036b8a40edbf81cffc639657c9bb2c5d384defa", hex.EncodeToString(h[:]))
		})
		sn3 := store.Snapshot()
		require.NoError(store.DeleteObject(namespace, key1, nil))
		err = store.GetObject(namespace, key1, &valueInStore)
		require.Error(err)
		iter, err = store.States(namespace, &valueInStore, [][]byte{key1, key2, key3}, nil)
		require.NoError(err)
		require.Equal(3, iter.Size())
		valuesInStore = []valueBytes{}
		for {
			vb := valueBytes{}
			switch _, err := iter.Next(&vb); errors.Cause(err) {
			case state.ErrNilValue:
				valuesInStore = append(valuesInStore, nil)
				continue
			case nil:
				valuesInStore = append(valuesInStore, vb)
				continue
			}
			break
		}
		require.Nil(valuesInStore[0])
		require.True(bytes.Equal(value2, valuesInStore[1]))
		require.True(bytes.Equal(value3, valuesInStore[2]))
		require.NoError(store.RevertSnapshot(sn3))
		err = store.GetObject(namespace, key1, &valueInStore)
		require.NoError(err)
		require.NoError(store.RevertSnapshot(sn1))
		require.True(bytes.Equal(value1, valueInStore))
		err = store.GetObject(namespace, key2, &valueInStore)
		require.Error(err)
	})
	t.Run("finalize & commit", func(t *testing.T) {
		height := uint64(100)
		ctx := context.Background()
		var value valueBytes
		err := store.GetObject(AccountKVNamespace, []byte(CurrentHeightKey), &value)
		require.Error(err)
		_, err = inMemStore.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.Error(err)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight: height,
		})
		require.NoError(store.Finalize(ctx))
		var heightInStore valueBytes
		err = store.GetObject(AccountKVNamespace, []byte(CurrentHeightKey), &heightInStore)
		require.NoError(err)
		require.True(bytes.Equal(heightInStore, byteutil.Uint64ToBytes(height)))
		_, err = inMemStore.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.Error(err)
		require.NoError(store.Commit(ctx, 0))
		heightInStore, err = inMemStore.Get(AccountKVNamespace, []byte(CurrentHeightKey))
		require.NoError(err)
		require.True(bytes.Equal(heightInStore, byteutil.Uint64ToBytes(height)))
	})
	require.NoError(store.Stop(ctx))
}

func TestStateDBWorkingSetStoreDumpWriteQueue(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	flusher, err := db.NewKVStoreFlusher(db.NewMemKVStore(), batch.NewCachedBatch())
	require.NoError(err)
	store := newStateDBWorkingSetStore(flusher, true)
	require.NoError(store.Start(ctx))
	defer func() { require.NoError(store.Stop(ctx)) }()

	namespace := "namespace"
	value1 := valueBytes("value1")
	require.NoError(store.PutObject(namespace, []byte("key1"), &value1))
	require.NoError(store.DeleteObject(namespace, []byte("key2"), nil))

	var buf bytes.Buffer
	require.NoError(store.DumpWriteQueue(&buf))
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")

	// one line per queued write, in queue order — this ordering is what Digest()
	// hashes over, so it is the thing a mismatch diff needs to line up on
	require.Len(lines, 2)
	require.True(strings.HasPrefix(lines[0], "0\t"))
	require.True(strings.HasPrefix(lines[1], "1\t"))
	for i, key := range []string{"key1", "key2"} {
		fields := strings.Split(lines[i], "\t")
		require.Len(fields, 6)
		require.Equal(namespace, fields[2])
		require.Equal(hex.EncodeToString([]byte(key)), fields[3])
	}
	// the put carries its value, the delete does not
	require.Equal("6", strings.Split(lines[0], "\t")[5])
	require.Equal("0", strings.Split(lines[1], "\t")[5])

	// dumping twice must give identical output, otherwise diffing two binaries'
	// dumps would report spurious divergence
	var buf2 bytes.Buffer
	require.NoError(store.DumpWriteQueue(&buf2))
	require.Equal(buf.String(), buf2.String())

	// a differing value shows up as a differing line while ns/key stay equal —
	// the exact signal used to localise a delta state digest mismatch
	flusher2, err := db.NewKVStoreFlusher(db.NewMemKVStore(), batch.NewCachedBatch())
	require.NoError(err)
	store2 := newStateDBWorkingSetStore(flusher2, true)
	require.NoError(store2.Start(ctx))
	defer func() { require.NoError(store2.Stop(ctx)) }()
	other := valueBytes("valueX")
	require.NoError(store2.PutObject(namespace, []byte("key1"), &other))
	var buf3 bytes.Buffer
	require.NoError(store2.DumpWriteQueue(&buf3))
	line := strings.Split(strings.TrimRight(buf3.String(), "\n"), "\n")[0]
	require.NotEqual(lines[0], line)
	require.Equal(strings.Split(lines[0], "\t")[3], strings.Split(line, "\t")[3]) // same key
	require.NotEqual(strings.Split(lines[0], "\t")[4], strings.Split(line, "\t")[4])
}

func TestFactoryWorkingSetStore(t *testing.T) {
	// TODO: add unit test for factory working set store
}
