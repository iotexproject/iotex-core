// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"sync"

	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
)

// KVStoreBatch is the interface of KV store.
type KVStoreBatch interface {
	// Put insert or update a record identified by (namespace, key)
	Put(string, []byte, []byte) error
	// PutIfNotExists puts a record only if (namespace, key) doesn't exist, otherwise return ErrAlreadyExist
	PutIfNotExists(string, []byte, []byte) error
	// Delete deletes a record by (namespace, key)
	Delete(string, []byte) error
	// Clear clear batch write queue
	Clear() error
	// Commit commit queued write to db
	Commit() error
}

const (
	// Put indicate the type of write operation to be Put
	Put int32 = iota
	// Delete indicate the type of write operation to be Delete
	Delete int32 = 1
	// PutIfNotExists indicate the type of write operation to be PutIfNotExists
	PutIfNotExists int32 = 2
)

// writeInfo is the struct to store write operation info
type writeInfo struct {
	writeType int32
	namespace string
	key       []byte
	value     []byte
}

// baseKVStoreBatch is the base class of KVStoreBatch
type baseKVStoreBatch struct {
	writeQueue []writeInfo
}

// NewBaseKVStoreBatch instantiates an in-memory KV store
func NewBaseKVStoreBatch() KVStoreBatch {
	return &baseKVStoreBatch{}
}

// Put inserts a <key, value> record
func (b *baseKVStoreBatch) Put(namespace string, key []byte, value []byte) error {
	b.writeQueue = append(b.writeQueue, writeInfo{writeType: Put, namespace: namespace, key: key, value: value})
	return nil
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (b *baseKVStoreBatch) PutIfNotExists(namespace string, key []byte, value []byte) error {
	b.writeQueue = append(b.writeQueue, writeInfo{writeType: PutIfNotExists, namespace: namespace, key: key, value: value})
	return nil
}

// Delete deletes a record
func (b *baseKVStoreBatch) Delete(namespace string, key []byte) error {
	b.writeQueue = append(b.writeQueue, writeInfo{writeType: Delete, namespace: namespace, key: key})
	return nil
}

// Clear clear write queue
func (b *baseKVStoreBatch) Clear() error {
	b.writeQueue = nil
	return nil
}

// Send queued write. This function is not really transactional. we think we don't need in mem db
func (b *baseKVStoreBatch) Commit() error {
	panic("not implement")
}

// memKVStoreBatch is the in-memory implementation of KVStore for testing purpose
type memKVStoreBatch struct {
	baseKVStoreBatch
	data *sync.Map
}

// NewMemKVStoreBatch instantiates an in-memory KV store
func NewMemKVStoreBatch(data *sync.Map) KVStoreBatch {
	return &memKVStoreBatch{data: data}
}

// Send queued write. This function is not really transactional. we think we don't need in mem db
func (m *memKVStoreBatch) Commit() error {
	for _, write := range m.writeQueue {
		if write.writeType == Put {
			m.data.Store(write.namespace+keyDelimiter+string(write.key), write.value)
		} else if write.writeType == PutIfNotExists {
			_, ok := m.data.Load(write.namespace + keyDelimiter + string(write.key))
			if !ok {
				m.data.Store(write.namespace+keyDelimiter+string(write.key), write.value)
				return nil
			}
			return ErrAlreadyExist
		} else if write.writeType == Delete {
			m.data.Delete(write.namespace + keyDelimiter + string(write.key))
		}
	}

	return nil
}

type boltDBBatch struct {
	baseKVStoreBatch
	bdb *boltDB
}

// NewBoltDBBatch instantiates a boltdb based KV store
func NewBoltDBBatch(bdb *boltDB) KVStoreBatch {
	return &boltDBBatch{bdb: bdb}
}

// Commit queued write
func (b *boltDBBatch) Commit() error {
	return b.bdb.db.Update(func(tx *bolt.Tx) error {
		for _, write := range b.writeQueue {
			if write.writeType == Put {
				bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
				if err != nil {
					return err
				}
				if err := bucket.Put(write.key, write.value); err != nil {
					return err
				}
			} else if write.writeType == PutIfNotExists {
				bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
				if err != nil {
					return err
				}
				if bucket.Get(write.key) == nil {
					if err := bucket.Put(write.key, write.value); err != nil {
						return err
					}
				} else {
					return ErrAlreadyExist
				}
			} else if write.writeType == Delete {
				bucket := tx.Bucket([]byte(write.namespace))
				if bucket == nil {
					return errors.Wrapf(bolt.ErrBucketNotFound, "bucket = %s", write.namespace)
				}
				if err := bucket.Delete(write.key); err != nil {
					return err
				}
			}
		}
		// clear queues
		b.Clear()

		return nil
	})
}
