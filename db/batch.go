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
	// if commit succeeds, the batch is cleared
	// otherwise the batch is kept intact (so batch user can figure out what’s wrong and attempt re-commit later)
	KVStoreBatch interface {
		// Lock locks the batch
		Lock()
		// Unlock unlocks the batch
		Unlock()
		// ClearAndUnlock clears the write queue and unlocks the batch
		ClearAndUnlock()
		// Put insert or update a record identified by (namespace, key)
		Put(string, []byte, []byte, string, ...interface{})
		// PutIfNotExists puts a record only if (namespace, key) doesn't exist, otherwise return ErrAlreadyExist
		PutIfNotExists(string, []byte, []byte, string, ...interface{}) error
		// Delete deletes a record by (namespace, key)
		Delete(string, []byte, string, ...interface{})
		// Size returns the size of batch
		Size() int
		// Entry returns the entry at the index
		Entry(int) (*writeInfo, error)
		// Clear clears entries staged in batch
		Clear()
		// CloneBatch clones the batch
		CloneBatch() KVStoreBatch
		// batch puts an entry into the write queue
		batch(op int32, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{})
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
		// Snapshot takes a snapshot of current cached batch
		Snapshot() int
		// Revert sets the cached batch to the state at the given snapshot
		Revert(int) error
		// clone clones the cached batch
		clone() CachedBatch
		// kvBatch returns the embedded KVStoreBatch
		kvBatch() KVStoreBatch
		// kvCache returns the embedded KVStoreCache
		kvCache() KVStoreCache
	}

	// cachedBatch implements the CachedBatch interface
	cachedBatch struct {
		lock sync.RWMutex
		KVStoreBatch
		KVStoreCache
		tag       int                 // latest snapshot + 1
		snapshots map[int]CachedBatch // saved snapshots
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

// ClearAndUnlock clears the write queue and unlocks the batch
func (b *baseKVStoreBatch) ClearAndUnlock() {
	defer b.mutex.Unlock()
	b.writeQueue = nil
}

// Put inserts a <key, value> record
func (b *baseKVStoreBatch) Put(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.batch(Put, namespace, key, value, errorFormat, errorArgs)
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (b *baseKVStoreBatch) PutIfNotExists(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.batch(PutIfNotExists, namespace, key, value, errorFormat, errorArgs)
	return nil
}

// Delete deletes a record
func (b *baseKVStoreBatch) Delete(namespace string, key []byte, errorFormat string, errorArgs ...interface{}) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.batch(Delete, namespace, key, nil, errorFormat, errorArgs)
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
func (b *baseKVStoreBatch) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.writeQueue = nil
}

// CloneBatch clones the batch
func (b *baseKVStoreBatch) CloneBatch() KVStoreBatch {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	c := baseKVStoreBatch{
		writeQueue: make([]writeInfo, b.Size()),
	}
	// clone the writeQueue
	copy(c.writeQueue, b.writeQueue)
	return &c
}

// batch puts an entry into the write queue
func (b *baseKVStoreBatch) batch(op int32, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
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
}

//======================================
// CachedBatch implementation
//======================================

// NewCachedBatch returns a new cached batch buffer
func NewCachedBatch() CachedBatch {
	return &cachedBatch{
		KVStoreBatch: NewBatch(),
		KVStoreCache: NewKVCache(),
		snapshots:    make(map[int]CachedBatch),
	}
}

// Lock locks the batch
func (cb *cachedBatch) Lock() {
	cb.lock.Lock()
}

// Unlock unlocks the batch
func (cb *cachedBatch) Unlock() {
	cb.lock.Unlock()
}

// ClearAndUnlock clears the write queue and unlocks the batch
func (cb *cachedBatch) ClearAndUnlock() {
	defer cb.lock.Unlock()
	cb.KVStoreCache.Clear()
	cb.KVStoreBatch.Clear()
	// clear all saved snapshots
	cb.tag = 0
	cb.snapshots = nil
	cb.snapshots = make(map[int]CachedBatch)
}

// Put inserts a <key, value> record
func (cb *cachedBatch) Put(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	h := cb.hash(namespace, key)
	cb.Write(h, value)
	cb.batch(Put, namespace, key, value, errorFormat, errorArgs)
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (cb *cachedBatch) PutIfNotExists(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) error {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	// TODO: bug, this is not a valid check whether the instance exists
	h := cb.hash(namespace, key)
	if err := cb.WriteIfNotExist(h, value); err != nil {
		return err
	}
	cb.batch(PutIfNotExists, namespace, key, value, errorFormat, errorArgs)
	return nil
}

// Delete deletes a record
func (cb *cachedBatch) Delete(namespace string, key []byte, errorFormat string, errorArgs ...interface{}) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	h := cb.hash(namespace, key)
	cb.Evict(h)
	cb.batch(Delete, namespace, key, nil, errorFormat, errorArgs)
}

// Clear clear the cached batch buffer
func (cb *cachedBatch) Clear() {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	cb.KVStoreCache.Clear()
	cb.KVStoreBatch.Clear()
	// clear all saved snapshots
	cb.tag = 0
	cb.snapshots = nil
	cb.snapshots = make(map[int]CachedBatch)
}

// Get retrieves a record
func (cb *cachedBatch) Get(namespace string, key []byte) ([]byte, error) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	h := cb.hash(namespace, key)
	return cb.Read(h)
}

// Snapshot takes a snapshot of current cached batch
func (cb *cachedBatch) Snapshot() int {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	defer func() { cb.tag++ }()
	// save a clone of current batch/cache
	cb.snapshots[cb.tag] = cb.clone()
	return cb.tag
}

// Revert sets the cached batch to the state at the given snapshot
func (cb *cachedBatch) Revert(snapshot int) error {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	// throw error if the snapshot number does not exist
	if _, ok := cb.snapshots[snapshot]; !ok {
		return errors.Wrapf(ErrInvalidDB, "invalid snapshot number = %d", snapshot)
	}
	cb.KVStoreBatch = nil
	cb.KVStoreCache = nil
	cb.KVStoreBatch = cb.snapshots[snapshot].kvBatch()
	cb.KVStoreCache = cb.snapshots[snapshot].kvCache()
	return nil
}

//======================================
// private functions
//======================================
func (cb *cachedBatch) hash(namespace string, key []byte) hash.CacheHash {
	stream := hash.Hash160b([]byte(namespace))
	stream = append(stream, key...)
	return byteutil.BytesToCacheHash(hash.Hash160b(stream))
}

// clone clones the batch
func (cb *cachedBatch) clone() CachedBatch {
	// note it only clones the current internal batch and cache, to be used by Revert() later
	// it does not clone the entire cachedBatch struct itself
	return &cachedBatch{
		KVStoreBatch: cb.CloneBatch(),
		KVStoreCache: cb.KVStoreCache.Clone(),
	}
}

// kvBatch returns the embedded KVStoreBatch
func (cb *cachedBatch) kvBatch() KVStoreBatch {
	return cb.KVStoreBatch
}

// kvCache returns the embedded KVStoreCache
func (cb *cachedBatch) kvCache() KVStoreCache {
	return cb.KVStoreCache
}
