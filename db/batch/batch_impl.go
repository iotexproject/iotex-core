// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package batch

import (
	"sync"

	"github.com/pkg/errors"
)

type (
	// baseKVStoreBatch is the base implementation of KVStoreBatch
	baseKVStoreBatch struct {
		mutex      sync.RWMutex
		fillLock   sync.RWMutex
		writeQueue []*WriteInfo
		fill       map[string]float64
	}

	// cachedBatch implements the CachedBatch interface
	cachedBatch struct {
		lock sync.RWMutex
		KVStoreCache
		kvStoreBatch *baseKVStoreBatch
		tag          int            // latest snapshot + 1
		batchShots   []int          // snapshots of batch are merely size of write queue at time of snapshot
		cacheShots   []KVStoreCache // snapshots of cache
	}
)

func newBaseKVStoreBatch() *baseKVStoreBatch {
	return &baseKVStoreBatch{
		fill: make(map[string]float64),
	}
}

// NewBatch returns a batch
func NewBatch() KVStoreBatch {
	return newBaseKVStoreBatch()
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

	b.fillLock.Lock()
	defer b.fillLock.Unlock()
	for k := range b.fill {
		delete(b.fill, k)
	}
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
func (b *baseKVStoreBatch) Entry(index int) (*WriteInfo, error) {
	if index < 0 || index >= len(b.writeQueue) {
		return nil, errors.Wrap(ErrOutOfBound, "index out of range")
	}
	return b.writeQueue[index], nil
}

func (b *baseKVStoreBatch) SerializeQueue(serialize WriteInfoSerialize, filter WriteInfoFilter) []byte {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	// 1. This could be improved by being processed in parallel
	// 2. Digest could be replaced by merkle root if we need proof
	bytes := make([]byte, 0)
	for _, wi := range b.writeQueue {
		if filter != nil && filter(wi) {
			continue
		}
		if serialize != nil {
			bytes = append(bytes, serialize(wi)...)
		} else {
			bytes = append(bytes, wi.Serialize()...)
		}
	}
	return bytes
}

// Clear clear write queue
func (b *baseKVStoreBatch) Clear() {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.writeQueue = nil

	b.fillLock.Lock()
	defer b.fillLock.Unlock()
	for k := range b.fill {
		delete(b.fill, k)
	}
}

func (b *baseKVStoreBatch) Translate(wit WriteInfoTranslate) KVStoreBatch {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if wit == nil {
		c := &baseKVStoreBatch{
			writeQueue: make([]*WriteInfo, b.Size()),
		}
		// clone the writeQueue
		copy(c.writeQueue, b.writeQueue)
		return c
	}
	c := &baseKVStoreBatch{
		writeQueue: []*WriteInfo{},
	}
	for _, wi := range b.writeQueue {
		newWi := wit(wi)
		if newWi != nil {
			c.writeQueue = append(c.writeQueue, newWi)
		}
	}

	return c
}

func (b *baseKVStoreBatch) CheckFillPercent(ns string) (float64, bool) {
	b.fillLock.RLock()
	defer b.fillLock.RUnlock()
	p, ok := b.fill[ns]
	return p, ok
}

func (b *baseKVStoreBatch) AddFillPercent(ns string, percent float64) {
	b.fillLock.Lock()
	defer b.fillLock.Unlock()
	b.fill[ns] = percent
}

// batch puts an entry into the write queue
func (b *baseKVStoreBatch) batch(op WriteType, namespace string, key, value []byte, errorFormat string, errorArgs ...interface{}) {
	b.writeQueue = append(
		b.writeQueue,
		&WriteInfo{
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

////////////////////////////////////////
// CachedBatch implementation
////////////////////////////////////////

// NewCachedBatch returns a new cached batch buffer
func NewCachedBatch() CachedBatch {
	return &cachedBatch{
		kvStoreBatch: newBaseKVStoreBatch(),
		KVStoreCache: NewKVCache(),
		batchShots:   make([]int, 0),
		cacheShots:   make([]KVStoreCache, 0),
	}
}

func (cb *cachedBatch) Translate(wit WriteInfoTranslate) KVStoreBatch {
	return cb.kvStoreBatch.Translate(wit)
}

func (cb *cachedBatch) Entry(i int) (*WriteInfo, error) {
	return cb.kvStoreBatch.Entry(i)
}

func (cb *cachedBatch) SerializeQueue(serialize WriteInfoSerialize, filter WriteInfoFilter) []byte {
	return cb.kvStoreBatch.SerializeQueue(serialize, filter)
}

func (cb *cachedBatch) Size() int {
	return cb.kvStoreBatch.Size()
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
	cb.kvStoreBatch.Clear()
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
	cb.kvStoreBatch.batch(Put, namespace, key, value, errorFormat, errorArgs)
}

// Delete deletes a record
func (cb *cachedBatch) Delete(namespace string, key []byte, errorFormat string, errorArgs ...interface{}) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	h := cb.hash(namespace, key)
	cb.Evict(h)
	cb.kvStoreBatch.batch(Delete, namespace, key, nil, errorFormat, errorArgs)
}

// Clear clear the cached batch buffer
func (cb *cachedBatch) Clear() {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	cb.KVStoreCache.Clear()
	cb.kvStoreBatch.Clear()
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
	cb.batchShots = append(cb.batchShots, cb.kvStoreBatch.Size())
	cb.cacheShots = append(cb.cacheShots, cb.KVStoreCache.Clone())
	return cb.tag
}

// Revert sets the cached batch to the state at the given snapshot
func (cb *cachedBatch) Revert(snapshot int) error {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	// throw error if the snapshot number does not exist
	if snapshot < 0 || snapshot >= cb.tag {
		return errors.Wrapf(ErrOutOfBound, "invalid snapshot number = %d", snapshot)
	}
	cb.tag = snapshot + 1
	cb.batchShots = cb.batchShots[:cb.tag]
	cb.kvStoreBatch.truncate(cb.batchShots[snapshot])
	cb.cacheShots = cb.cacheShots[:cb.tag]
	cb.KVStoreCache = nil
	cb.KVStoreCache = cb.cacheShots[snapshot]
	return nil
}

func (cb *cachedBatch) CheckFillPercent(ns string) (float64, bool) {
	return cb.kvStoreBatch.CheckFillPercent(ns)
}

func (cb *cachedBatch) AddFillPercent(ns string, percent float64) {
	cb.kvStoreBatch.AddFillPercent(ns, percent)
}

func (cb *cachedBatch) hash(namespace string, key []byte) *KvCacheKey {
	return &KvCacheKey{namespace, string(key)}
}
