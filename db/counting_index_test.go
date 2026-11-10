// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

func TestNewCountingIndex(t *testing.T) {
	require := require.New(t)

	_, err := NewCountingIndexNX(nil, []byte{1})
	require.Equal(ErrInvalid, errors.Cause(err))
	_, err = NewCountingIndexNX(NewMemKVStore(), nil)
	require.Equal(ErrInvalid, errors.Cause(err))
	_, err = NewCountingIndexNX(NewMemKVStore(), []byte{})
	require.Equal(ErrInvalid, errors.Cause(err))
}

func TestCountingIndex(t *testing.T) {
	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		require.NoError(kv.Start(context.Background()))
		defer func() {
			require.NoError(kv.Stop(context.Background()))
		}()

		bucket := []byte("test")
		_, err := GetCountingIndex(kv, bucket)
		require.Equal(ErrNotExist, errors.Cause(err))

		index, err := NewCountingIndexNX(kv, bucket)
		require.NoError(err)
		require.Equal(uint64(0), index.Size())

		// write 200 entries in batch mode
		for i := 0; i < 200; i++ {
			h := hash.Hash160b([]byte(strconv.Itoa(i)))
			require.NoError(index.Add(h[:], true))
		}
		// cannot Add() before Commit() in batch mode
		require.Equal(ErrInvalid, errors.Cause(index.Add([]byte{1}, false)))
		require.NoError(index.Commit())
		require.EqualValues(200, index.Size())
		// cannot get > size
		_, err = index.Get(index.Size())
		require.Equal(ErrNotExist, errors.Cause(err))
		k, err := index.Get(10)
		require.NoError(err)
		h := hash.Hash160b([]byte(strconv.Itoa(10)))
		require.Equal(h[:], k)
		index.Close()

		// re-open the bucket
		index, err = GetCountingIndex(kv, bucket)
		require.NoError(err)
		// write another 100 entries
		for i := 200; i < 250; i++ {
			h := hash.Hash160b([]byte(strconv.Itoa(i)))
			require.NoError(index.Add(h[:], false))
		}
		require.EqualValues(250, index.Size())

		// use external batch
		require.Equal(ErrInvalid, index.Finalize())
		b := batch.NewBatch()
		require.NoError(index.UseBatch(b))
		for i := 250; i < 300; i++ {
			h := hash.Hash160b([]byte(strconv.Itoa(i)))
			require.NoError(index.Add(h[:], true))
		}
		require.NoError(index.Finalize())
		cIndex, ok := index.(*countingIndex)
		require.True(ok)
		require.NoError(cIndex.kvStore.WriteBatch(b))
		require.EqualValues(300, index.Size())

		_, err = index.Range(248, 0)
		require.Equal(ErrInvalid, errors.Cause(err))
		_, err = index.Range(248, 53)
		require.Equal(ErrInvalid, errors.Cause(err))

		// last key
		v, err := index.Range(299, 1)
		require.NoError(err)
		require.Equal(1, len(v))
		h = hash.Hash160b([]byte(strconv.Itoa(299)))
		require.Equal(h[:], v[0])

		// first 5 keys
		v, err = index.Range(0, 5)
		require.NoError(err)
		require.Equal(5, len(v))
		for i := range v {
			h := hash.Hash160b([]byte(strconv.Itoa(i)))
			require.Equal(h[:], v[i])
		}

		// last 40 keys
		v, err = index.Range(260, 40)
		require.NoError(err)
		require.Equal(40, len(v))
		for i := range v {
			h := hash.Hash160b([]byte(strconv.Itoa(260 + i)))
			require.Equal(h[:], v[i])
		}
		index.Close()

		// re-open the bucket, verify size = 300
		index1, err := GetCountingIndex(kv, bucket)
		require.NoError(err)
		require.EqualValues(300, index1.Size())

		// revert last 40 keys
		err = index1.Revert(0)
		require.Equal(ErrInvalid, errors.Cause(err))
		err = index1.Revert(index1.Size() + 1)
		require.Equal(ErrInvalid, errors.Cause(err))
		require.NoError(index1.Revert(40))
		require.EqualValues(260, index1.Size())

		// last 40 keys
		_, err = index1.Range(220, 41)
		require.Equal(ErrInvalid, errors.Cause(err))
		v, err = index1.Range(220, 40)
		require.NoError(err)
		require.Equal(40, len(v))
		for i := range v {
			h := hash.Hash160b([]byte(strconv.Itoa(220 + i)))
			require.Equal(h[:], v[i])
		}
	}

	path := "test-counting.bolt"
	testPath, err := testutil.PathOfTempFile(path)
	require.NoError(t, err)
	defer testutil.CleanupPath(testPath)
	cfg := DefaultConfig
	cfg.DbPath = testPath

	for _, v := range []KVStore{
		NewMemKVStore(),
		NewBoltDB(cfg),
	} {
		t.Run("test counting index", func(t *testing.T) {
			testFunc(v, t)
		})
	}
}

const (
	Tenants = 10000
	Keys    = 200
)

func TestBulk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		require.NoError(kv.Start(context.Background()))
		defer func() {
			require.NoError(kv.Stop(context.Background()))
		}()

		// create 10000 tenants
		for i := 0; i < Tenants; i++ {
			h := hash.Hash160b([]byte(strconv.Itoa(i)))
			tenant, err := NewCountingIndexNX(kv, h[:])
			require.NoError(err)

			for i := 0; i < Keys; i++ {
				h := hash.Hash160b([]byte(strconv.Itoa(i)))
				require.NoError(tenant.Add(h[:], true))
			}
			require.NoError(tenant.Commit())
			tenant.Close()
			log.L().Info(fmt.Sprintf("write tenant %d:\n", i))
		}
	}

	cfg := DefaultConfig
	cfg.DbPath = "test-bulk.dat"
	t.Run("Bolt DB", func(t *testing.T) {
		testutil.CleanupPath(cfg.DbPath)
		testFunc(NewBoltDB(cfg), t)
	})
}

func TestCheckBulk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	testFunc := func(kv KVStore, t *testing.T) {
		require := require.New(t)

		require.NoError(kv.Start(context.Background()))
		defer func() {
			require.NoError(kv.Stop(context.Background()))
		}()

		// verify 1000 tenants
		for i := 0; i < Tenants; i++ {
			h := hash.Hash160b([]byte(strconv.Itoa(i)))
			index, err := GetCountingIndex(kv, h[:])
			require.NoError(err)
			require.EqualValues(Keys, index.Size())

			value, err := index.Range(0, Keys)
			require.NoError(err)
			require.EqualValues(Keys, len(value))

			for i := range value {
				h := hash.Hash160b([]byte(strconv.Itoa(i)))
				require.Equal(h[:], value[i])
			}
			log.L().Info(fmt.Sprintf("verify tenant: %d\n", i))
		}
	}

	cfg := DefaultConfig
	cfg.DbPath = "test-bulk.dat"
	t.Run("Bolt DB", func(t *testing.T) {
		defer testutil.CleanupPath(cfg.DbPath)
		testFunc(NewBoltDB(cfg), t)
	})
}

// failGetKVStore fails reads on demand
type failGetKVStore struct {
	KVStoreWithRange
	failGet bool
}

func (f *failGetKVStore) Get(ns string, key []byte) ([]byte, error) {
	if f.failGet {
		return nil, errors.New("injected read fault")
	}
	return f.KVStoreWithRange.Get(ns, key)
}

func TestCountingIndexAddToBatch(t *testing.T) {
	r := require.New(t)
	kv := NewMemKVStore().(KVStoreWithRange)
	index, err := NewCountingIndexNX(kv, []byte("test"))
	r.NoError(err)
	r.NoError(index.Add([]byte("v0"), false))

	b := batch.NewBatch()
	apply := index.AddToBatch(b, []byte("v1"), []byte("v2"), []byte("v3"))
	// the index is not changed before the batch is committed
	r.EqualValues(1, index.Size())
	r.NoError(kv.WriteBatch(b))
	r.EqualValues(1, index.Size())
	apply()
	r.EqualValues(4, index.Size())
	values, err := index.Range(0, 4)
	r.NoError(err)
	r.Equal([][]byte{[]byte("v0"), []byte("v1"), []byte("v2"), []byte("v3")}, values)

	// the new size is persisted
	reloaded, err := GetCountingIndex(kv, []byte("test"))
	r.NoError(err)
	r.EqualValues(4, reloaded.Size())

	// adding no value keeps the size
	b = batch.NewBatch()
	apply = index.AddToBatch(b)
	r.NoError(kv.WriteBatch(b))
	apply()
	r.EqualValues(4, index.Size())
}

func TestNewCountingIndexNXReadError(t *testing.T) {
	r := require.New(t)
	kv := &failGetKVStore{KVStoreWithRange: NewMemKVStore().(KVStoreWithRange)}
	index, err := NewCountingIndexNX(kv, []byte("test"))
	r.NoError(err)
	r.NoError(index.Add([]byte("v0"), false))
	r.NoError(index.Add([]byte("v1"), false))

	// a failed read is not taken as a missing index
	kv.failGet = true
	_, err = NewCountingIndexNX(kv, []byte("test"))
	r.Error(err)
	kv.failGet = false
	index, err = NewCountingIndexNX(kv, []byte("test"))
	r.NoError(err)
	r.EqualValues(2, index.Size())
}

func TestLoadCountingIndex(t *testing.T) {
	r := require.New(t)
	kv := &failGetKVStore{KVStoreWithRange: NewMemKVStore().(KVStoreWithRange)}

	// a missing index is empty, and is not created
	index, err := LoadCountingIndex(kv, []byte("test"))
	r.NoError(err)
	r.Zero(index.Size())
	_, err = kv.Get("test", CountKey)
	r.ErrorIs(err, ErrNotExist)

	// it is created once a batch is committed
	b := batch.NewBatch()
	index.AddToBatch(b, []byte("v0"), []byte("v1"))
	r.NoError(kv.WriteBatch(b))
	index, err = LoadCountingIndex(kv, []byte("test"))
	r.NoError(err)
	r.EqualValues(2, index.Size())

	// a failed read is returned
	kv.failGet = true
	_, err = LoadCountingIndex(kv, []byte("test"))
	r.Error(err)

	_, err = LoadCountingIndex(nil, []byte("test"))
	r.ErrorIs(err, ErrInvalid)
	_, err = LoadCountingIndex(kv, nil)
	r.ErrorIs(err, ErrInvalid)
}
