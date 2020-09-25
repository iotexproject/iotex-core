package db

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/iotex-core/db/batch"
)

// kvStoreWithCache is an implementation of KVStore, wrapping kvstore with LRU caches of latest states
type kvStoreWithCache struct {
	mutex       sync.RWMutex // lock for stateCaches
	store       KVStore
	stateCaches map[string]*cache.ThreadSafeLruCache // map having lru cache to store current states for speed up
	cacheSize   int
}

// NewKvStoreWithCache wraps kvstore with stateCaches
func NewKvStoreWithCache(kvstore KVStore, cacheSize int) KVStore {
	return &kvStoreWithCache{
		store:       kvstore,
		stateCaches: make(map[string]*cache.ThreadSafeLruCache),
		cacheSize:   cacheSize,
	}
}

// Start starts the kvStoreWithCache
func (kvc *kvStoreWithCache) Start(ctx context.Context) error {
	return kvc.store.Start(ctx)
}

// Stop stops the kvStoreWithCache
func (kvc *kvStoreWithCache) Stop(ctx context.Context) error {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	for _, sc := range kvc.stateCaches {
		sc.Clear()
	}
	return kvc.store.Stop(ctx)
}

// Put inserts a <key, value> record into stateCaches and kvstore
func (kvc *kvStoreWithCache) Put(namespace string, key, value []byte) (err error) {
	kvc.updateStateCachesIfExists(namespace, key, value)
	return kvc.store.Put(namespace, key, value)
}

// Get retrieves a <key, value> record from stateCaches, and if not exists, retrieves from kvstore
func (kvc *kvStoreWithCache) Get(namespace string, key []byte) ([]byte, error) {
	cachedData, err := kvc.getStateCaches(namespace, key)
	if err != nil {
		return nil, err
	}
	if cachedData != nil {
		return cachedData, nil
	}
	kvStoreData, err := kvc.store.Get(namespace, key)
	if err != nil {
		return nil, err
	}
	// in case of read-miss, put into statecaches
	kvc.putStateCaches(namespace, key, kvStoreData)
	return kvStoreData, nil
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (kvc *kvStoreWithCache) Filter(namespace string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	return kvc.store.Filter(namespace, cond, minKey, maxKey)
}

// Delete deletes a record from statecaches if exists, and from kvstore
func (kvc *kvStoreWithCache) Delete(namespace string, key []byte) (err error) {
	kvc.deleteStateCachesIfExists(namespace, key)
	return kvc.store.Delete(namespace, key)
}

// WriteBatch commits a batch into stateCaches and kvstore
func (kvc *kvStoreWithCache) WriteBatch(kvsb batch.KVStoreBatch) (err error) {
	for i := 0; i < kvsb.Size(); i++ {
		write, e := kvsb.Entry(i)
		if e != nil {
			return e
		}
		ns := write.Namespace()
		if write.WriteType() == batch.Put {
			kvc.updateStateCachesIfExists(ns, write.Key(), write.Value())
		} else if write.WriteType() == batch.Delete {
			kvc.deleteStateCachesIfExists(ns, write.Key())
		}
	}
	return kvc.store.WriteBatch(kvsb)
}

// ======================================
// private functions
// ======================================

// store on stateCaches
func (kvc *kvStoreWithCache) putStateCaches(namespace string, key, value []byte) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		sc.Add(string(key), value)
	} else {
		sc := cache.NewThreadSafeLruCache(kvc.cacheSize)
		sc.Add(string(key), value)
		kvc.stateCaches[namespace] = sc
	}
	return
}

// update on stateCaches if the key exists
func (kvc *kvStoreWithCache) updateStateCachesIfExists(namespace string, key, value []byte) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		if _, ok := sc.Get(string(key)); ok {
			sc.Add(string(key), value)
		}
	}
	return
}

// remove on stateCaches if the key exists
func (kvc *kvStoreWithCache) deleteStateCachesIfExists(namespace string, key []byte) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		sc.Remove(string(key))
	}
	return
}

// look up stateCachces
func (kvc *kvStoreWithCache) getStateCaches(namespace string, key []byte) ([]byte, error) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		if data, ok := sc.Get(string(key)); ok {
			if byteData, ok := data.([]byte); ok {
				return byteData, nil
			}
			return nil, errors.New("failed to convert interface{} to bytes from stateCaches")
		}
	}
	return nil, nil
}
