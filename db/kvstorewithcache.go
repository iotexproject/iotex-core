package db

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/log"
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
	if err := kvc.store.Put(namespace, key, value); err != nil {
		return err
	}
	kvc.updateStateCachesIfExists(namespace, key, value)
	return nil
}

// Get retrieves a <key, value> record from stateCaches, and if not exists, retrieves from kvstore
func (kvc *kvStoreWithCache) Get(namespace string, key []byte) ([]byte, error) {
	if cachedData, isExist := kvc.getStateCaches(namespace, key); isExist {
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
	if err := kvc.store.Delete(namespace, key); err != nil {
		return err
	}
	kvc.deleteStateCaches(namespace, key)
	return nil
}

// WriteBatch commits a batch into stateCaches and kvstore
func (kvc *kvStoreWithCache) WriteBatch(kvsb batch.KVStoreBatch) (err error) {
	if err := kvc.store.WriteBatch(kvsb); err != nil {
		return err
	}
	kvsb.Lock()
	defer kvsb.ClearAndUnlock()
	for i := 0; i < kvsb.Size(); i++ {
		write, e := kvsb.Entry(i)
		if e != nil {
			return e
		}
		ns := write.Namespace()
		switch write.WriteType() {
		case batch.Put:
			kvc.updateStateCachesIfExists(ns, write.Key(), write.Value())
		case batch.Delete:
			kvc.deleteStateCaches(ns, write.Key())
		}
	}
	return nil
}

// ======================================
// private functions
// ======================================

// store on stateCaches
func (kvc *kvStoreWithCache) putStateCaches(namespace string, key, value []byte) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		sc.Add(hex.EncodeToString(key), value)
	} else {
		sc := cache.NewThreadSafeLruCache(kvc.cacheSize)
		sc.Add(hex.EncodeToString(key), value)
		kvc.stateCaches[namespace] = sc
	}
}

// update on stateCaches if the key exists
func (kvc *kvStoreWithCache) updateStateCachesIfExists(namespace string, key, value []byte) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		if _, ok := sc.Get(hex.EncodeToString(key)); ok {
			sc.Add(hex.EncodeToString(key), value)
		}
	}
}

// remove on stateCaches if the key exists
func (kvc *kvStoreWithCache) deleteStateCaches(namespace string, key []byte) {
	kvc.mutex.Lock()
	defer kvc.mutex.Unlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		sc.Remove(hex.EncodeToString(key))
	}
}

// look up stateCachces
func (kvc *kvStoreWithCache) getStateCaches(namespace string, key []byte) ([]byte, bool) {
	kvc.mutex.RLock()
	defer kvc.mutex.RUnlock()
	if sc, ok := kvc.stateCaches[namespace]; ok {
		if data, ok := sc.Get(hex.EncodeToString(key)); ok {
			if byteData, ok := data.([]byte); ok {
				return byteData, true
			}
			log.L().Fatal("failed to convert interface{} to bytes from stateCaches")
			return nil, true
		}
	}
	return nil, false
}
