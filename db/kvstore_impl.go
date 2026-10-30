// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"strings"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

var (
	// ErrBucketNotExist indicates certain bucket does not exist in db
	ErrBucketNotExist = errors.New("bucket not exist in DB")
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrIO indicates the generic error of DB I/O operation
	ErrIO = errors.New("DB I/O operation error")
	// ErrNotSupported indicates that api is not supported
	ErrNotSupported = errors.New("not supported")
)

const (
	_keyDelimiter = "."
)

// memKVStore is the in-memory implementation of KVStore for testing purpose
type memKVStore struct {
	data   cache.LRUCache
	bucket cache.LRUCache
}

// NewMemKVStore instantiates an in-memory KV store
func NewMemKVStore() KVStoreForRangeIndex {
	return &memKVStore{
		bucket: cache.NewThreadSafeLruCache(0),
		data:   cache.NewThreadSafeLruCache(0),
	}
}

func (m *memKVStore) Start(_ context.Context) error { return nil }

func (m *memKVStore) Stop(_ context.Context) error { return nil }

// Put inserts a <key, value> record
func (m *memKVStore) Put(namespace string, key, value []byte) error {
	if _, ok := m.bucket.Get(namespace); !ok {
		m.bucket.Add(namespace, struct{}{})
	}
	m.data.Add(namespace+_keyDelimiter+string(key), value)
	return nil
}

func (m *memKVStore) Insert(name []byte, key uint64, value []byte) error {
	return ErrNotSupported
}

func (m *memKVStore) Purge(name []byte, key uint64) error {
	return ErrNotSupported
}

func (m *memKVStore) Remove(name []byte, key uint64) error {
	return ErrNotSupported
}

func (m *memKVStore) SeekPrev(name []byte, key uint64) ([]byte, error) {
	return nil, ErrNotSupported
}

func (m *memKVStore) SeekNext(name []byte, key uint64) ([]byte, error) {
	return nil, ErrNotSupported
}

// Get retrieves a record
func (m *memKVStore) Get(namespace string, key []byte) ([]byte, error) {
	if _, ok := m.bucket.Get(namespace); !ok {
		return nil, errors.Wrapf(ErrNotExist, "namespace = %x doesn't exist", []byte(namespace))
	}
	value, _ := m.data.Get(namespace + _keyDelimiter + string(key))
	if value != nil {
		return value.([]byte), nil
	}
	return nil, errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
}

// Get retrieves a record
func (m *memKVStore) Range(namespace string, key []byte, count uint64) ([][]byte, error) {
	if _, ok := m.bucket.Get(namespace); !ok {
		return nil, errors.Wrapf(ErrNotExist, "namespace = %s doesn't exist", namespace)
	}
	value := make([][]byte, count)
	start := byteutil.BytesToUint64BigEndian(key)
	for i := uint64(0); i < count; i++ {
		key = byteutil.Uint64ToBytesBigEndian(start + i)
		v, _ := m.data.Get(namespace + _keyDelimiter + string(key))
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
	m.data.Remove(namespace + _keyDelimiter + string(key))
	return nil
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (m *memKVStore) Filter(namespace string, c Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	// TODO: implement filter for memKVStore
	return nil, nil, errors.New("in-memory KVStore does not support Filter()")
}

// ScanRange returns up to limit <k, v> pairs in [min, max), ascending by bytes.Compare(k).
// See KVStoreWithRangeScan for the exact semantics, which must stay identical across engines.
//
// NOTE on the storage key encoding: entries are keyed as
// namespace + _keyDelimiter + string(key), and neither the namespace nor the key
// is escaped. The delimiter is therefore ambiguous whenever a namespace contains
// it: namespace "a.b" with key "c" and namespace "a" with key "b.c" occupy the
// same map entry. That ambiguity is pre-existing -- Put/Get/Delete already alias
// the same way -- and it is not introduced here; ScanRange inherits it. It does
// not corrupt ORDERING, because within a fixed namespace the prefix is constant
// and byte order over the suffix equals byte order over the key. It can produce
// FALSE MATCHES: scanning namespace "a" would surface a key "b.c" that really
// belongs to namespace "a.b". memKVStore is a test-only store (all production
// paths use bolt or pebble, where namespaces are physically separate buckets /
// hash-prefixed keys), and none of the namespaces in state/tables.go contain
// ".", so this is reported rather than papered over.
func (m *memKVStore) ScanRange(namespace string, min, max []byte, limit int) ([][]byte, [][]byte, error) {
	if emptyScanRange(min, max) {
		return nil, nil, nil
	}
	var (
		prefix = namespace + _keyDelimiter
		keys   [][]byte
		values [][]byte
	)
	// a missing namespace simply yields no match, which is an empty result
	m.data.Range(func(k cache.Key, v interface{}) bool {
		ks, ok := k.(string)
		if !ok || !strings.HasPrefix(ks, prefix) {
			return true
		}
		key := []byte(ks[len(prefix):])
		if !inScanRange(key, min, max) {
			return true
		}
		value, ok := v.([]byte)
		if !ok {
			return true
		}
		keys = append(keys, key)
		values = append(values, copyBytes(value))
		return true
	})
	k, v := sortAndTruncateScan(keys, values, limit)
	return k, v, nil
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
	return nil, ErrNotSupported
}

// GetKeyByPrefix retrieves all keys those with const prefix
func (m *memKVStore) GetKeyByPrefix(namespace, prefix []byte) ([][]byte, error) {
	return nil, ErrNotSupported
}
