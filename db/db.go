// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"sync"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	// ErrInvalidDB indicates invalid operation attempted to Blockchain database
	ErrInvalidDB = errors.New("invalid DB operation")
	// ErrNotExist indicates certain item does not exist in Blockchain database
	ErrNotExist = errors.New("not exist in DB")
	// ErrAlreadyExist indicates certain item already exists in Blockchain database
	ErrAlreadyExist = errors.New("already exist in DB")
)

// KVStore is the interface of KV store.
type KVStore interface {
	lifecycle.StartStopper

	// Put insert or update a record identified by (namespace, key)
	Put(string, []byte, []byte) error
	// Put puts a record only if (namespace, key) doesn't exist, otherwise return ErrAlreadyExist
	PutIfNotExists(string, []byte, []byte) error
	// Get gets a record by (namespace, key)
	Get(string, []byte) ([]byte, error)
	// Delete deletes a record by (namespace, key)
	Delete(string, []byte) error
	// Batch return a kv store batch api object
	Batch() KVStoreBatch
}

const (
	keyDelimiter = "."
)

// memKVStore is the in-memory implementation of KVStore for testing purpose
type memKVStore struct {
	data   *sync.Map
	bucket map[string]struct{}
}

// NewMemKVStore instantiates an in-memory KV store
func NewMemKVStore() KVStore {
	return &memKVStore{
		bucket: make(map[string]struct{}),
		data:   &sync.Map{},
	}
}

func (m *memKVStore) Start(_ context.Context) error { return nil }

func (m *memKVStore) Stop(_ context.Context) error { return nil }

// Put inserts a <key, value> record
func (m *memKVStore) Put(namespace string, key []byte, value []byte) error {
	m.bucket[namespace] = struct{}{}
	m.data.Store(namespace+keyDelimiter+string(key), value)
	return nil
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (m *memKVStore) PutIfNotExists(namespace string, key []byte, value []byte) error {
	m.bucket[namespace] = struct{}{}
	_, loaded := m.data.LoadOrStore(namespace+keyDelimiter+string(key), value)
	if loaded {
		return ErrAlreadyExist
	}

	return nil
}

// Get retrieves a record
func (m *memKVStore) Get(namespace string, key []byte) ([]byte, error) {
	if _, ok := m.bucket[namespace]; !ok {
		return nil, errors.Wrapf(bolt.ErrBucketNotFound, "bucket = %s", namespace)
	}
	value, _ := m.data.Load(namespace + keyDelimiter + string(key))
	if value != nil {
		return value.([]byte), nil
	}
	return nil, errors.Wrapf(ErrNotExist, "key = %x", key)
}

// Delete deletes a record
func (m *memKVStore) Delete(namespace string, key []byte) error {
	m.data.Delete(namespace + keyDelimiter + string(key))
	return nil
}

// Batch return a kv store batch api object
func (m *memKVStore) Batch() KVStoreBatch {
	return NewMemKVStoreBatch(m.data, &m.bucket)
}

const fileMode = 0600

// boltDB is KVStore implementation based bolt DB
type boltDB struct {
	mutex  sync.RWMutex
	db     *bolt.DB
	path   string
	config *config.DB
}

// NewBoltDB instantiates a boltdb based KV store
func NewBoltDB(path string, cfg *config.DB) KVStore {
	return &boltDB{db: nil, path: path, config: cfg}
}

// Start opens the BoltDB (creates new file if not existing yet)
func (b *boltDB) Start(_ context.Context) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.db != nil {
		return nil
	}

	db, err := bolt.Open(b.path, fileMode, nil)
	if err != nil {
		return err
	}
	b.db = db
	return nil
}

// Stop closes the BoltDB
func (b *boltDB) Stop(_ context.Context) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if b.db != nil {
		err := b.db.Close()
		b.db = nil
		return err
	}
	return nil
}

// Put inserts a <key, value> record
func (b *boltDB) Put(namespace string, key []byte, value []byte) error {
	var err error
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		err = b.db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
			if err != nil {
				return err
			}
			return bucket.Put(key, value)
		})

		if err == nil {
			break
		}
	}

	return err
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (b *boltDB) PutIfNotExists(namespace string, key []byte, value []byte) error {
	var err error
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		err = b.db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
			if err != nil {
				return err
			}
			if bucket.Get(key) == nil {
				return bucket.Put(key, value)
			}
			return ErrAlreadyExist
		})

		if err == nil || err == ErrAlreadyExist {
			break
		}
	}

	return err
}

// Get retrieves a record
func (b *boltDB) Get(namespace string, key []byte) ([]byte, error) {
	var value []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket == nil {
			return errors.Wrapf(bolt.ErrBucketNotFound, "bucket = %s", namespace)
		}
		value = bucket.Get(key)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if value == nil {
		err = errors.Wrapf(ErrNotExist, "key = %x", key)
	}
	return value, err
}

// Delete deletes a record
func (b *boltDB) Delete(namespace string, key []byte) error {
	var err error
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		err = b.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(namespace))
			if bucket == nil {
				return nil
			}
			return bucket.Delete(key)
		})

		if err == nil {
			break
		}
	}

	return err
}

// Batch return a kv store batch api object
func (b *boltDB) Batch() KVStoreBatch {
	return NewBoltDBBatch(b)
}

//======================================
// private functions
//======================================

// intentionally fail to test DB can successfully rollback
func (b *boltDB) batchPutForceFail(namespace string, key [][]byte, value [][]byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
		if err != nil {
			return err
		}
		if len(key) != len(value) {
			return errors.Wrap(ErrInvalidDB, "batch put <k, v> size not match")
		}
		for i := 0; i < len(key); i++ {
			if err := bucket.Put(key[i], value[i]); err != nil {
				return err
			}
			// intentionally fail to test DB can successfully rollback
			if i == len(key)-1 {
				return errors.Wrapf(ErrInvalidDB, "force fail to test DB rollback")
			}
		}
		return nil
	})
}
