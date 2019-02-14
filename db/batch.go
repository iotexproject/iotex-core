// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"sync"

	"github.com/iotexproject/iotex-core/pkg/log"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/pkg/hash"
)

type (
	// KVStoreBatch defines a batch buffer interface that stages Put/Delete entries in sequential order
	// To use it, first start a new batch
	// b := NewBatch()
	// and keep batching Put/Delete operation into it
	// b.Put(bucket, k, v)
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
		// truncate the write queue
		truncate(int)
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
		// Digest of the cached batch
		Digest() hash.Hash256
		// clone clones the cached batch
		clone() CachedBatch
	}

	// cachedBatch implements the CachedBatch interface
	cachedBatch struct {
		lock sync.RWMutex
		KVStoreBatch
		KVStoreCache
		tag        int            // latest snapshot + 1
		batchShots []int          // snapshots of batch are merely size of write queue at time of snapshot
		cacheShots []KVStoreCache // snapshots of cache
	}
)

const (
	// Put indicate the type of write operation to be Put
	Put int32 = iota
	// Delete indicate the type of write operation to be Delete
	Delete int32 = 1
)

func (wi *writeInfo) serialize() []byte {
	bytes := make([]byte, 0)
	bytes = append(bytes, []byte(wi.namespace)...)
	bytes = append(bytes, wi.key...)
	bytes = append(bytes, wi.value...)
	return bytes
}

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
		return nil, errors.Wrap(ErrIO, "index out of range")
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

// truncate the write queue
func (b *baseKVStoreBatch) truncate(size int) {
	b.writeQueue = b.writeQueue[:size]
}

//======================================
// CachedBatch implementation
//======================================

// NewCachedBatch returns a new cached batch buffer
func NewCachedBatch() CachedBatch {
	return &cachedBatch{
		KVStoreBatch: NewBatch(),
		KVStoreCache: NewKVCache(),
		batchShots:   make([]int, 0),
		cacheShots:   make([]KVStoreCache, 0),
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
	cb.batchShots = nil
	cb.cacheShots = nil
	cb.batchShots = make([]int, 0)
	cb.cacheShots = make([]KVStoreCache, 0)
}

// Put inserts a <key, value> record
func (cb *cachedBatch) Put(namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	h := cb.hash(namespace, key)
	cb.Write(h, value)
	cb.batch(Put, namespace, key, value, errorFormat, errorArgs)
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
	cb.batchShots = nil
	cb.cacheShots = nil
	cb.batchShots = make([]int, 0)
	cb.cacheShots = make([]KVStoreCache, 0)
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
	// save a copy of current batch/cache
	cb.batchShots = append(cb.batchShots, cb.Size())
	cb.cacheShots = append(cb.cacheShots, cb.KVStoreCache.Clone())
	return cb.tag
}

// Revert sets the cached batch to the state at the given snapshot
func (cb *cachedBatch) Revert(snapshot int) error {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	// throw error if the snapshot number does not exist
	if snapshot < 0 || snapshot >= cb.tag {
		return errors.Wrapf(ErrIO, "invalid snapshot number = %d", snapshot)
	}
	cb.tag = snapshot + 1
	cb.batchShots = cb.batchShots[:cb.tag]
	cb.KVStoreBatch.truncate(cb.batchShots[snapshot])
	cb.cacheShots = cb.cacheShots[:cb.tag]
	cb.KVStoreCache = nil
	cb.KVStoreCache = cb.cacheShots[snapshot]
	return nil
}

func (cb *cachedBatch) Digest() hash.Hash256 {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	// 1. This could be improved by being processed in parallel
	// 2. Digest could be replaced by merkle root if we need proof
	bytes := make([]byte, 0)
	for i := 0; i < cb.Size(); i++ {
		wi, err := cb.Entry(i)
		if err != nil {
			log.S().Panic("Batch entry %d doesn't exist", i)
		}
		bytes = append(bytes, wi.serialize()...)
	}
	return hash.Hash256b(bytes)
}

//======================================
// private functions
//======================================
func (cb *cachedBatch) hash(namespace string, key []byte) hash.Hash160 {
	return hash.Hash160b(append([]byte(namespace), key...))
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
