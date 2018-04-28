// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided ‘as is’ and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"sync"

	"github.com/boltdb/bolt"
	"github.com/iotexproject/iotex-core/common/service"
)

// KVStore is the interface of KV store.
type KVStore interface {
	service.Service
	// Put insert or update a record identified by (namespace, key)
	Put(string, []byte, []byte) error
	// Put puts a record only if (namespace, key) doesn't exist
	PutIfNotExists(string, []byte, []byte) error
	// Get gets a record by (namespace, key)
	Get(string, []byte) ([]byte, error)
}

const (
	keyDelimiter = "."
)

// memKVStore is the in-memory implementation of KVStore for testing purpose
type memKVStore struct {
	service.AbstractService
	data sync.Map
}

// NewMemKVStore instantiates an in-memory KV store
func NewMemKVStore() KVStore {
	return &memKVStore{}
}

func (m *memKVStore) Put(namespace string, key []byte, value []byte) error {
	m.data.Store(namespace+keyDelimiter+string(key), value)
	return nil
}

func (m *memKVStore) PutIfNotExists(namespace string, key []byte, value []byte) error {
	_, ok := m.data.Load(namespace + keyDelimiter + string(key))
	if !ok {
		m.data.Store(namespace+keyDelimiter+string(key), value)
	}
	return nil
}

func (m *memKVStore) Get(namespace string, key []byte) ([]byte, error) {
	value, _ := m.data.Load(namespace + keyDelimiter + string(key))
	if value != nil {
		return value.([]byte), nil
	}
	return nil, nil
}

const (
	fileMode = 0600
)

// boltDB is KVStore implementation based bolt DB
type boltDB struct {
	service.AbstractService
	db      *bolt.DB
	path    string
	options *bolt.Options
}

// NewBoltDB instantiates a boltdb based KV store
func NewBoltDB(path string, options *bolt.Options) KVStore {
	return &boltDB{path: path, options: options}
}

func (b *boltDB) Start() error {
	db, err := bolt.Open(b.path, fileMode, b.options)
	if err != nil {
		return err
	}
	b.db = db
	return nil
}

func (b *boltDB) Stop() error {
	return b.db.Close()
}

func (b *boltDB) Put(namespace string, key []byte, value []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
		if err != nil {
			return err
		}
		return bucket.Put(key, value)
	})
}

func (b *boltDB) PutIfNotExists(namespace string, key []byte, value []byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
		if err != nil {
			return err
		}
		if bucket.Get(key) == nil {
			return bucket.Put(key, value)
		}
		return nil
	})
}

func (b *boltDB) Get(namespace string, key []byte) ([]byte, error) {
	var value []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket == nil {
			return nil
		}
		value = bucket.Get(key)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}
