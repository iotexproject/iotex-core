// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// ErrBucketNotExist indicates certain bucket does not exist in db
	ErrBucketNotExist = errors.New("bucket not exist in DB")
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyDeleted indicates the key has been deleted
	ErrAlreadyDeleted = errors.New("already deleted from DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
	// ErrIO indicates the generic error of DB I/O operation
	ErrIO = errors.New("DB I/O operation error")
)

type (
	// KVStore is the interface of KV store.
	KVStore interface {
		lifecycle.StartStopper

		// Put insert or update a record identified by (namespace, key)
		Put(string, []byte, []byte) error
		// Get gets a record by (namespace, key)
		Get(string, []byte) ([]byte, error)
		// Delete deletes a record by (namespace, key)
		Delete(string, []byte) error
		// Commit commits a batch
		Commit(KVStoreBatch) error
	}

	// KVStoreWithRange is KVStore with Range() API
	KVStoreWithRange interface {
		KVStore
		// Range gets a range of records by (namespace, key, count)
		Range(string, []byte, uint64) ([][]byte, error)
	}

	// KVStoreWithBucketFillPercent is KVStore with option to set bucket fill percent
	KVStoreWithBucketFillPercent interface {
		KVStore
		// SetBucketFillPercent sets specified fill percent for a bucket
		SetBucketFillPercent(string, float64) error
	}

	// KVStoreForRangeIndex is KVStore for range index
	KVStoreForRangeIndex interface {
		KVStore
		// Insert inserts a value into the index
		Insert([]byte, uint64, []byte) error
		// Seek returns value by the key
		Seek([]byte, uint64) ([]byte, error)
		// Remove removes an existing key
		Remove([]byte, uint64) error
		// Purge deletes an existing key and all keys before it
		Purge([]byte, uint64) error
		// GetBucketByPrefix retrieves all bucket those with const namespace prefix
		GetBucketByPrefix([]byte) ([][]byte, error)
		// GetKeyByPrefix retrieves all keys those with const prefix
		GetKeyByPrefix(namespace, prefix []byte) ([][]byte, error)
	}
)

const (
	keyDelimiter = "."
)

// memKVStore is the in-memory implementation of KVStore for testing purpose
type memKVStore struct {
	data   *sync.Map
	bucket *sync.Map
}

// NewMemKVStore instantiates an in-memory KV store
func NewMemKVStore() KVStore {
	return &memKVStore{
		bucket: &sync.Map{},
		data:   &sync.Map{},
	}
}

func (m *memKVStore) Start(_ context.Context) error { return nil }

func (m *memKVStore) Stop(_ context.Context) error { return nil }

// Put inserts a <key, value> record
func (m *memKVStore) Put(namespace string, key, value []byte) error {
	_, _ = m.bucket.LoadOrStore(namespace, struct{}{})
	m.data.Store(namespace+keyDelimiter+string(key), value)
	return nil
}

// Get retrieves a record
func (m *memKVStore) Get(namespace string, key []byte) ([]byte, error) {
	if _, ok := m.bucket.Load(namespace); !ok {
		return nil, errors.Wrapf(ErrNotExist, "namespace = %x doesn't exist", []byte(namespace))
	}
	value, _ := m.data.Load(namespace + keyDelimiter + string(key))
	if value != nil {
		return value.([]byte), nil
	}
	return nil, errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
}

// Get retrieves a record
func (m *memKVStore) Range(namespace string, key []byte, count uint64) ([][]byte, error) {
	if _, ok := m.bucket.Load(namespace); !ok {
		return nil, errors.Wrapf(ErrNotExist, "namespace = %s doesn't exist", namespace)
	}
	value := make([][]byte, count)
	start := byteutil.BytesToUint64BigEndian(key)
	for i := uint64(0); i < count; i++ {
		key = byteutil.Uint64ToBytesBigEndian(start + i)
		v, _ := m.data.Load(namespace + keyDelimiter + string(key))
		if v == nil {
			return nil, errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
		}
		value[i] = make([]byte, len(v.([]byte)))
		copy(value[i], v.([]byte))
	}
	return value, nil
}

// Delete deletes a record
func (m *memKVStore) Delete(namespace string, key []byte) error {
	m.data.Delete(namespace + keyDelimiter + string(key))
	return nil
}

// Commit commits a batch
func (m *memKVStore) Commit(b KVStoreBatch) (e error) {
	succeed := false
	b.Lock()
	defer func() {
		if succeed {
			// clear the batch if commit succeeds
			b.ClearAndUnlock()
		} else {
			b.Unlock()
		}
	}()
	for i := 0; i < b.Size(); i++ {
		write, err := b.Entry(i)
		if err != nil {
			return err
		}
		if write.writeType == Put {
			if err := m.Put(write.namespace, write.key, write.value); err != nil {
				e = err
				break
			}
		} else if write.writeType == Delete {
			if err := m.Delete(write.namespace, write.key); err != nil {
				e = err
				break
			}
		}
	}
	if e == nil {
		succeed = true
	}

	return e
}

// GetBucketByPrefix retrieves all bucket those with const namespace prefix
func (m *memKVStore) GetBucketByPrefix(namespace []byte) ([][]byte, error) {
	return nil, nil
}

// GetKeyByPrefix retrieves all keys those with const prefix
func (m *memKVStore) GetKeyByPrefix(namespace, prefix []byte) ([][]byte, error) {
	return nil, nil
}
