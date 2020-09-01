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

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// ErrBucketNotExist indicates certain bucket does not exist in db
	ErrBucketNotExist = errors.New("bucket not exist in DB")
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrIO indicates the generic error of DB I/O operation
	ErrIO = errors.New("DB I/O operation error")
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

// Filter returns <k, v> pair in a bucket that meet the condition
func (m *memKVStore) Filter(namespace string, c Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	// TODO: implement filter for memKVStore
	return nil, nil, errors.New("in-memory KVStore does not support Filter()")
}

// WriteBatch commits a batch
func (m *memKVStore) WriteBatch(b batch.KVStoreBatch) (e error) {
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
		switch write.WriteType() {
		case batch.Put:
			if err := m.Put(write.Namespace(), write.Key(), write.Value()); err != nil {
				e = err
			}
		case batch.Delete:
			if err := m.Delete(write.Namespace(), write.Key()); err != nil {
				e = err
			}
		}
		if e != nil {
			break
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
