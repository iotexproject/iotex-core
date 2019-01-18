// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyDeleted indicates the key has been deleted
	ErrAlreadyDeleted = errors.New("already deleted from DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
	// ErrIO indicates the generic error of DB I/O operation
	ErrIO = errors.New("DB I/O operation error")
)

// KVStore is the interface of KV store.
type KVStore interface {
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
		return nil, errors.Wrapf(ErrNotExist, "namespace = %s doesn't exist", namespace)
	}
	value, _ := m.data.Load(namespace + keyDelimiter + string(key))
	if value != nil {
		return value.([]byte), nil
	}
	return nil, errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
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

// NewOnDiskDB instantiates an on-disk KV store
func NewOnDiskDB(cfg config.DB) KVStore {
	if cfg.UseBadgerDB {
		return &badgerDB{db: nil, path: cfg.DbPath, config: cfg}
	}
	return &boltDB{db: nil, path: cfg.DbPath, config: cfg}
}
