// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/boltdb/bolt"
	"github.com/pkg/errors"
)

// KVStoreBatch is the interface of Batch KVStore.
// It buffers the Put/Delete operations and persist to DB in a single transaction upon Commit()
type KVStoreBatch interface {
	// Put insert or update a record identified by (namespace, key)
	Put(string, []byte, []byte, string, ...interface{}) error
	// PutIfNotExists puts a record only if (namespace, key) doesn't exist, otherwise return ErrAlreadyExist
	PutIfNotExists(string, []byte, []byte, string, ...interface{}) error
	// Delete deletes a record by (namespace, key)
	Delete(string, []byte, string, ...interface{}) error
	// Clear clear batch write queue
	Clear() error
	// Commit commit queued write to db
	Commit() error
	// KVStore returns the underlying KVStore
	KVStore() KVStore
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
	writeType   int32
	namespace   string
	key         []byte
	value       []byte
	errorFormat string
	errorArgs   interface{}
}

// baseKVStoreBatch is the base class of KVStoreBatch
type baseKVStoreBatch struct {
	writeQueue []writeInfo
}

// Put inserts a <key, value> record
func (b *baseKVStoreBatch) Put(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	b.writeQueue = append(b.writeQueue, writeInfo{writeType: Put, namespace: namespace,
		key: key, value: value, errorFormat: errorFormat, errorArgs: errorArgs})
	return nil
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (b *baseKVStoreBatch) PutIfNotExists(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	b.writeQueue = append(b.writeQueue, writeInfo{writeType: PutIfNotExists, namespace: namespace,
		key: key, value: value, errorFormat: errorFormat, errorArgs: errorArgs})
	return nil
}

// Delete deletes a record
func (b *baseKVStoreBatch) Delete(namespace string, key []byte, errorFormat string, errorArgs ...interface{}) error {
	b.writeQueue = append(b.writeQueue, writeInfo{writeType: Delete, namespace: namespace,
		key: key, errorFormat: errorFormat, errorArgs: errorArgs})
	return nil
}

// Clear clear write queue
func (b *baseKVStoreBatch) Clear() error {
	b.writeQueue = nil
	return nil
}

// Commit needs to be implemented by derived class
func (b *baseKVStoreBatch) Commit() error {
	panic("not implement")
}

//======================================
// memKVStoreBatch is the in-memory implementation of KVStoreBatch for testing purpose
//======================================
type memKVStoreBatch struct {
	baseKVStoreBatch
	kv *memKVStore
}

// NewMemKVStoreBatch instantiates an in-memory KV store
func NewMemKVStoreBatch(kv *memKVStore) KVStoreBatch {
	return &memKVStoreBatch{kv: kv}
}

// Commit persists pending writes to DB and clears the write queue
func (m *memKVStoreBatch) Commit() error {
	for _, write := range m.writeQueue {
		if write.writeType == Put {
			m.kv.Put(write.namespace, write.key, write.value)
		} else if write.writeType == PutIfNotExists {
			if err := m.kv.PutIfNotExists(write.namespace, write.key, write.value); err != nil {
				return err
			}
		} else if write.writeType == Delete {
			m.kv.Delete(write.namespace, write.key)
		}
	}
	// clear queues
	return m.Clear()
}

// KVStore returns the underlying KVStore
func (m *memKVStoreBatch) KVStore() KVStore {
	return m.kv
}

//======================================
// boltDBBatch is the DB implementation of KVStoreBatch
//======================================
type boltDBBatch struct {
	baseKVStoreBatch
	bdb *boltDB
}

// NewBoltDBBatch instantiates a boltdb based KV store
func NewBoltDBBatch(bdb *boltDB) KVStoreBatch {
	return &boltDBBatch{bdb: bdb}
}

// Commit persists pending writes to DB and clears the write queue
func (b *boltDBBatch) Commit() error {
	var err error
	numRetries := b.bdb.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		err = b.bdb.db.Update(func(tx *bolt.Tx) error {
			for _, write := range b.writeQueue {
				if write.writeType == Put {
					bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
					if err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
					if err := bucket.Put(write.key, write.value); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				} else if write.writeType == PutIfNotExists {
					bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
					if err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
					if bucket.Get(write.key) == nil {
						if err := bucket.Put(write.key, write.value); err != nil {
							return errors.Wrapf(err, write.errorFormat, write.errorArgs)
						}
					} else {
						return ErrAlreadyExist
					}
				} else if write.writeType == Delete {
					bucket := tx.Bucket([]byte(write.namespace))
					if bucket == nil {
						return nil
					}
					if err := bucket.Delete(write.key); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				}
			}

			return nil
		})

		if err == nil || err == ErrAlreadyExist {
			break
		}
	}

	// clear queues
	b.Clear()

	return err
}

// KVStore returns the underlying KVStore
func (b *boltDBBatch) KVStore() KVStore {
	return b.bdb
}
