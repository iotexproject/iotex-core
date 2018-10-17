// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// KVStoreBatch defines a batch buffer interface that stages Put/Delete entries in sequential order
	// To use it, first start a new batch
	// b := NewBatch()
	// and keep batching Put/Delete operation into it
	// b.Put(bucket, k, v)
	// b.PutIfNotExists(bucket, k, v)
	// b.Delete(bucket, k, v)
	// once it's done, call KVStore interface's Commit() to persist to underlying DB
	// KVStore.Commit(b)
	KVStoreBatch interface {
		// Lock locks the batch
		Lock()
		// Unlock unlocks the batch
		Unlock()
		// Put insert or update a record identified by (namespace, key)
		Put(string, []byte, []byte, string, ...interface{}) error
		// PutIfNotExists puts a record only if (namespace, key) doesn't exist, otherwise return ErrAlreadyExist
		PutIfNotExists(string, []byte, []byte, string, ...interface{}) error
		// Delete deletes a record by (namespace, key)
		Delete(string, []byte, string, ...interface{}) error
		// Size returns the size of batch
		Size() int
		// Entry returns the entry at the index
		Entry(int) (*writeInfo, error)
		// Clear clears entries staged in batch
		Clear() error
		// internal clear called by Clear()
		clear() error
	}

	// writeInfo is the struct to store Put/Delete operation info
	writeInfo struct {
		writeType   int32
		namespace   string
		key         []byte
		value       []byte
		errorFormat string
		errorArgs   interface{}
	}

	// baseKVStoreBatch is the base implementation of KVStoreBatch
	baseKVStoreBatch struct {
		mutex      sync.RWMutex
		writeQueue []writeInfo
	}

	// CachedBatch derives from Batch interface
	// A local cache is added to provide fast retrieval of pending Put/Delete entries
	CachedBatch interface {
		KVStoreBatch
		// Get gets a record by (namespace, key)
		Get(string, []byte) ([]byte, error)
	}

	// cachedBatch implements the CachedBatch interface
	cachedBatch struct {
		baseKVStoreBatch
		cache map[hash.PKHash][]byte // local cache of batched <k, v> for fast query
	}
)

const (
	// Put indicate the type of write operation to be Put
	Put int32 = iota
	// Delete indicate the type of write operation to be Delete
	Delete int32 = 1
	// PutIfNotExists indicate the type of write operation to be PutIfNotExists
	PutIfNotExists int32 = 2
)

// NewBatch returns a batch
func NewBatch() KVStoreBatch {
	return &baseKVStoreBatch{}
}

// Lock locks the batch
func (b *baseKVStoreBatch) Lock() {
	b.mutex.Lock()
}

// Unlock unlocks the batch
func (b *baseKVStoreBatch) Unlock() {
	b.mutex.Unlock()
}

// Put inserts a <key, value> record
func (b *baseKVStoreBatch) Put(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.batch(Put, namespace, key, value, errorFormat, errorArgs)
	return nil
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (b *baseKVStoreBatch) PutIfNotExists(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.batch(PutIfNotExists, namespace, key, value, errorFormat, errorArgs)
	return nil
}

// Delete deletes a record
func (b *baseKVStoreBatch) Delete(namespace string, key []byte, errorFormat string, errorArgs ...interface{}) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.batch(Delete, namespace, key, nil, errorFormat, errorArgs)
}

// Size returns the size of batch
func (b *baseKVStoreBatch) Size() int {
	return len(b.writeQueue)
}

// Entry returns the entry at the index
func (b *baseKVStoreBatch) Entry(index int) (*writeInfo, error) {
	if index < 0 || index >= len(b.writeQueue) {
		return nil, errors.Wrap(ErrInvalidDB, "index out of range")
	}
	return &b.writeQueue[index], nil
}

// Clear clear write queue
func (b *baseKVStoreBatch) Clear() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.clear()
}

func (b *baseKVStoreBatch) batch(op int32, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	b.writeQueue = append(
		b.writeQueue,
		writeInfo{
			writeType:   op,
			namespace:   namespace,
			key:         key,
			value:       value,
			errorFormat: errorFormat,
			errorArgs:   errorArgs,
		})
	return nil
}

func (b *baseKVStoreBatch) clear() error {
	b.writeQueue = nil
	return nil
}

//======================================
// CachedBatch implementation
//======================================

// NewCachedBatch returns a new cached batch buffer
func NewCachedBatch() CachedBatch {
	return &cachedBatch{
		cache: make(map[hash.PKHash][]byte),
	}
}

// Put inserts a <key, value> record
func (cb *cachedBatch) Put(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.cache[cb.hash(namespace, key)] = value
	return cb.batch(Put, namespace, key, value, errorFormat, errorArgs)
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (cb *cachedBatch) PutIfNotExists(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	if _, ok := cb.cache[cb.hash(namespace, key)]; ok {
		return ErrAlreadyExist
	}
	cb.cache[cb.hash(namespace, key)] = value
	return cb.batch(PutIfNotExists, namespace, key, value, errorFormat, errorArgs)
}

// Delete deletes a record
func (cb *cachedBatch) Delete(namespace string, key []byte, errorFormat string, errorArgs ...interface{}) error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	delete(cb.cache, cb.hash(namespace, key))
	return cb.batch(Delete, namespace, key, nil, errorFormat, errorArgs)
}

// Clear clear the cached batch buffer
func (cb *cachedBatch) Clear() error {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	return cb.clear()
}

// Get retrieves a record
func (cb *cachedBatch) Get(namespace string, key []byte) ([]byte, error) {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	if v, ok := cb.cache[cb.hash(namespace, key)]; ok {
		return v, nil
	}
	return nil, ErrNotExist
}

//======================================
// private functions
//======================================
func (cb *cachedBatch) hash(namespace string, key []byte) hash.PKHash {
	stream := []byte(namespace)
	stream = append(stream, key...)
	return byteutil.BytesTo20B(hash.Hash160b(stream))
}

func (cb *cachedBatch) clear() error {
	cb.cache = nil
	cb.cache = make(map[hash.PKHash][]byte)
	return cb.baseKVStoreBatch.clear()
}
