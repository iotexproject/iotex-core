// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package batch

import (
	"sync"

	"github.com/pkg/errors"
)

const (
	// resetSnapshotIgnoreThreshold is the threshold of snapshot number to ignore reset snapshots
	resetSnapshotIgnoreThreshold = 10
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
		lock         sync.RWMutex
		kvStoreBatch *baseKVStoreBatch
		tag          int            // latest snapshot + 1
		batchShots   []int          // snapshots of batch are merely size of write queue at time of snapshot
		caches       []KVStoreCache // snapshots of cache
		keyTags      map[kvCacheKey]*kvCacheValue
		tagKeys      [][]kvCacheKey
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
func (b *baseKVStoreBatch) Put(namespace string, key, value []byte, errorMessage string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.batch(Put, namespace, key, value, errorMessage)
}

func (b *baseKVStoreBatch) Append(kvb KVStoreBatch) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	kvb.Lock()
	defer kvb.Unlock()
	for i := range kvb.Size() {
		wi, err := b.Entry(i)
		if err != nil {
			panic(err)
		}
		switch wi.writeType {
		case Put:
			b.batch(Put, wi.namespace, wi.key, wi.value, wi.errorMessage)
		case Delete:
			b.batch(Delete, wi.namespace, wi.key, nil, wi.errorMessage)
		default:
			panic("unexpected write type")
		}
	}
}

// Delete deletes a record
func (b *baseKVStoreBatch) Delete(namespace string, key []byte, errorMessage string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.batch(Delete, namespace, key, nil, errorMessage)
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
	// 1. Digest could be replaced by merkle root if we need proof
	b.mutex.Lock()
	defer b.mutex.Unlock()

	var (
		serialisedBytes = make([][]byte, len(b.writeQueue))
		wg              = sync.WaitGroup{}
	)

	wg.Add(len(b.writeQueue))
	for i, wi := range b.writeQueue {
		go func(i int, info *WriteInfo) {
			defer wg.Done()
			if filter != nil && filter(info) {
				return
			}

			idx := i
			var data []byte
			if serialize != nil {
				data = serialize(info)
			} else {
				data = info.Serialize()
			}

			serialisedBytes[idx] = data
		}(i, wi)
	}
	wg.Wait()

	var returnedBytes []byte
	for _, sb := range serialisedBytes {
		returnedBytes = append(returnedBytes, sb...)
	}

	return returnedBytes
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
func (b *baseKVStoreBatch) batch(op WriteType, namespace string, key, value []byte, errorMessage string) {
	b.writeQueue = append(
		b.writeQueue,
		&WriteInfo{
			writeType:    op,
			namespace:    namespace,
			key:          key,
			value:        value,
			errorMessage: errorMessage,
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
	cb := &cachedBatch{
		kvStoreBatch: newBaseKVStoreBatch(),
	}
	cb.clear()

	return cb
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
	cb.clear()
}

func (cb *cachedBatch) currentCache() KVStoreCache {
	return cb.caches[len(cb.caches)-1]
}

func (cb *cachedBatch) clear() {
	cb.kvStoreBatch.Clear()
	cb.tag = 0
	cb.batchShots = make([]int, 0)
	cb.caches = []KVStoreCache{NewKVCache()}
	cb.keyTags = map[kvCacheKey]*kvCacheValue{}
	cb.tagKeys = [][]kvCacheKey{{}}
}

func (cb *cachedBatch) touchKey(h kvCacheKey) {
	tags, ok := cb.keyTags[h]
	if !ok {
		cb.keyTags[h] = newkvCacheValue([]int{cb.tag})
		cb.tagKeys[cb.tag] = append(cb.tagKeys[cb.tag], h)
		return
	}
	if tags.last() != cb.tag {
		tags.append(cb.tag)
		cb.tagKeys[cb.tag] = append(cb.tagKeys[cb.tag], h)
	}
}

// Put inserts a <key, value> record
func (cb *cachedBatch) Put(namespace string, key, value []byte, errorMessage string) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	cb.put(namespace, key, value, errorMessage)
}

func (cb *cachedBatch) put(namespace string, key, value []byte, errorMessage string) {
	h := cb.hash(namespace, key)
	cb.touchKey(h)
	cb.currentCache().Write(&h, value)
	cb.kvStoreBatch.batch(Put, namespace, key, value, errorMessage)
}

func (cb *cachedBatch) Append(b KVStoreBatch) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	b.Lock()
	defer b.Unlock()
	for i := range b.Size() {
		wi, err := b.Entry(i)
		if err != nil {
			panic(err)
		}
		switch wi.writeType {
		case Put:
			cb.put(wi.namespace, wi.key, wi.value, wi.errorMessage)
		case Delete:
			cb.delete(wi.namespace, wi.key, wi.errorMessage)
		default:
			panic("unexpected write type")
		}
	}
}

// Delete deletes a record
func (cb *cachedBatch) Delete(namespace string, key []byte, errorMessage string) {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	cb.delete(namespace, key, errorMessage)
}

func (cb *cachedBatch) delete(namespace string, key []byte, errorMessage string) {
	h := cb.hash(namespace, key)
	cb.touchKey(h)
	cb.currentCache().Evict(&h)
	cb.kvStoreBatch.batch(Delete, namespace, key, nil, errorMessage)
}

// Clear clear the cached batch buffer
func (cb *cachedBatch) Clear() {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	cb.clear()
}

// Get retrieves a record
func (cb *cachedBatch) Get(namespace string, key []byte) ([]byte, error) {
	cb.lock.RLock()
	defer cb.lock.RUnlock()
	h := cb.hash(namespace, key)
	var v []byte
	err := ErrNotExist
	if tags, ok := cb.keyTags[h]; ok {
		for i := tags.len() - 1; i >= 0; i-- {
			v, err = cb.caches[tags.getAt(i)].Read(&h)
			if errors.Cause(err) == ErrNotExist {
				continue
			}
			break
		}
	}
	return v, err
}

// Snapshot takes a snapshot of current cached batch
func (cb *cachedBatch) Snapshot() int {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	defer func() { cb.tag++ }()
	// save a copy of current batch/cache
	cb.batchShots = append(cb.batchShots, cb.kvStoreBatch.Size())
	cb.caches = append(cb.caches, NewKVCache())
	cb.tagKeys = append(cb.tagKeys, []kvCacheKey{})
	return cb.tag
}

func (cb *cachedBatch) RevertSnapshot(snapshot int) error {
	cb.lock.Lock()
	defer cb.lock.Unlock()
	// throw error if the snapshot number does not exist
	if snapshot < 0 || snapshot >= cb.tag {
		return errors.Wrapf(ErrOutOfBound, "invalid snapshot number = %d", snapshot)
	}
	cb.tag = snapshot + 1
	cb.batchShots = cb.batchShots[:cb.tag]
	cb.kvStoreBatch.truncate(cb.batchShots[snapshot])
	cb.caches = cb.caches[:cb.tag+1]
	cb.caches[cb.tag].Clear()
	for tag := cb.tag; tag < len(cb.tagKeys); tag++ {
		keys := cb.tagKeys[tag]
		for _, key := range keys {
			kv := cb.keyTags[key]
			kv.pop()
			if kv.len() == 0 {
				delete(cb.keyTags, key)
			}
		}
	}
	cb.tagKeys = cb.tagKeys[:cb.tag+1]
	cb.tagKeys[cb.tag] = []kvCacheKey{}
	return nil
}

func (cb *cachedBatch) ResetSnapshots() {
	cb.lock.Lock()
	defer cb.lock.Unlock()

	// ignore reset snapshots if the snapshot number is less than threshold
	if cb.tag <= resetSnapshotIgnoreThreshold {
		return
	}
	cb.tag = 0
	cb.batchShots = nil
	cb.batchShots = make([]int, 0)
	if len(cb.caches) > 1 {
		if err := cb.caches[0].Append(cb.caches[1:]...); err != nil {
			panic(errors.Wrap(err, "failed to reset snapshots"))
		}
		cb.caches = cb.caches[:1]
	}
	keys := make([]kvCacheKey, 0, len(cb.keyTags))
	for key, v := range cb.keyTags {
		v.reset()
		keys = append(keys, key)
	}
	cb.tagKeys = [][]kvCacheKey{keys}
}

func (cb *cachedBatch) CheckFillPercent(ns string) (float64, bool) {
	return cb.kvStoreBatch.CheckFillPercent(ns)
}

func (cb *cachedBatch) AddFillPercent(ns string, percent float64) {
	cb.kvStoreBatch.AddFillPercent(ns, percent)
}

func (cb *cachedBatch) hash(namespace string, key []byte) kvCacheKey {
	return kvCacheKey{namespace, string(key)}
}
