package db

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/cache"
	"github.com/iotexproject/iotex-core/db/batch"
)

// kvStoreWithCache is an implementation of KVStore, wrapping kvstore with LRU caches of latest states
type kvStoreWithCache struct {
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
	for _, sc := range kvc.stateCaches {
		sc.Clear()
	}
	return kvc.store.Stop(ctx)
}

// Put inserts a <key, value> record into stateCaches and kvstore
func (kvc *kvStoreWithCache) Put(namespace string, key, value []byte) (err error) {
	kvc.putStateCaches(namespace, key, value)
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
	return kvc.store.Get(namespace, key)
}

// Filter returns <k, v> pair in a bucket that meet the condition
func (kvc *kvStoreWithCache) Filter(namespace string, cond Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
	return kvc.store.Filter(namespace, cond, minKey, maxKey)
}

// Delete deletes a record from statecaches if exists, and from kvstore
func (kvc *kvStoreWithCache) Delete(namespace string, key []byte) (err error) {
	kvc.deleteStateCaches(namespace, key)
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
			kvc.putStateCaches(ns, write.Key(), write.Value())
		} else if write.WriteType() == batch.Delete {
			kvc.deleteStateCaches(ns, write.Key())
		}
	}
	return kvc.store.WriteBatch(kvsb)
}

// ======================================
// private functions
// ======================================

func (kvc *kvStoreWithCache) putStateCaches(namespace string, key, value []byte) {
	// store on stateCaches
	if sc, ok := kvc.stateCaches[namespace]; ok {
		sc.Add(string(key), value)
	} else {
		sc := cache.NewThreadSafeLruCache(kvc.cacheSize)
		sc.Add(string(key), value)
		kvc.stateCaches[namespace] = sc
	}
	return
}

func (kvc *kvStoreWithCache) deleteStateCaches(namespace string, key []byte) {
	// remove on stateCaches
	if sc, ok := kvc.stateCaches[namespace]; ok {
		sc.Remove(string(key))
	} else {
		sc := cache.NewThreadSafeLruCache(kvc.cacheSize)
		kvc.stateCaches[namespace] = sc
	}
	return
}

func (kvc *kvStoreWithCache) getStateCaches(namespace string, key []byte) ([]byte, error) {
	// look up stateCachces
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
