// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"sync"

	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// CachedKVStore is the interface of cached KVStore.
	// It builds on top of the KVStoreBatch interface to re-use its batched write feature. In addition, a local
	// cache is added to provide retrieval of pending Put entries.
	CachedKVStore interface {
		KVStore
		// Clear clear batch write queue
		Clear() error
		// Commit commit queued write to db
		Commit() error
		// KVStore returns the underlying KVStore
		KVStore() KVStore
	}

	// cachedKVStore implements the base class of CachedKVStore
	cachedKVStore struct {
		KVStoreBatch
		mutex sync.RWMutex
		cache map[hash.AddrHash][]byte // local cache of batched <k, v> for fast query
		kv    KVStore                  // underlying KV store
	}
)

// NewCachedKVStore instantiates an in-memory CachedKVStore
func NewCachedKVStore(kv KVStore) CachedKVStore {
	c := cachedKVStore{
		KVStoreBatch: kv.Batch(),
		cache:        make(map[hash.AddrHash][]byte),
		kv:           kv,
	}
	return &c
}

func (c *cachedKVStore) Start(ctx context.Context) error { return c.kv.Start(ctx) }

func (c *cachedKVStore) Stop(ctx context.Context) error { return c.kv.Stop(ctx) }

// Put inserts a <key, value> record
func (c *cachedKVStore) Put(namespace string, key, value []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache[c.hash(namespace, key)] = value
	return c.KVStoreBatch.Put(namespace, key, value, "failed to put key = %x", key)
}

// PutIfNotExists inserts a <key, value> record only if it does not exist yet, otherwise return ErrAlreadyExist
func (c *cachedKVStore) PutIfNotExists(namespace string, key, value []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.get(namespace, key) != nil {
		return ErrAlreadyExist
	}
	c.cache[c.hash(namespace, key)] = value
	return c.KVStoreBatch.PutIfNotExists(namespace, key, value, "failed to put non-existing key = %x", key)
}

// Get retrieves a record
func (c *cachedKVStore) Get(namespace string, key []byte) ([]byte, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if v := c.get(namespace, key); v != nil {
		return v, nil
	}
	return c.kv.Get(namespace, key)
}

// Delete deletes a record
func (c *cachedKVStore) Delete(namespace string, key []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.KVStoreBatch.Delete(namespace, key, "failed to delete key = %x", key)
}

// Clear clear write queue
func (c *cachedKVStore) Clear() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.KVStoreBatch.Clear(); err != nil {
		return err
	}
	return c.clear()
}

// Commit persists pending writes to DB and clears the local cache
func (c *cachedKVStore) Commit() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.KVStoreBatch.Commit(); err != nil {
		return err
	}
	return c.clear()
}

// Batch returns the batch api object
func (c *cachedKVStore) Batch() KVStoreBatch {
	return c.KVStoreBatch
}

// KVStore returns the underlying KVStore
func (c *cachedKVStore) KVStore() KVStore {
	return c.kv
}

//======================================
// private functions
//======================================
func (c *cachedKVStore) hash(namespace string, key []byte) hash.AddrHash {
	stream := []byte(namespace)
	stream = append(stream, key...)
	return byteutil.BytesTo20B(hash.Hash160b(stream))
}

func (c *cachedKVStore) get(namespace string, key []byte) []byte {
	if v, ok := c.cache[c.hash(namespace, key)]; ok {
		return v
	}
	return nil
}

func (c *cachedKVStore) clear() error {
	c.cache = nil
	c.cache = make(map[hash.AddrHash][]byte)
	return nil
}
