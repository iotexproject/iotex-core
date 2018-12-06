// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"github.com/iotexproject/iotex-core/pkg/hash"
)

type (
	// KVStoreCache is a local cache of batched <k, v> for fast query
	KVStoreCache interface {
		// Read retrieves a record
		Read(hash.CacheHash) ([]byte, error)
		// Write puts a record into cache
		Write(hash.CacheHash, []byte)
		// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
		WriteIfNotExist(hash.CacheHash, []byte) error
		// Evict deletes a record from cache
		Evict(hash.CacheHash)
		// Clear clear the cache
		Clear()
	}

	// kvCache implements KVStoreCache interface
	kvCache struct {
		cache   map[hash.CacheHash][]byte // local cache of batched <k, v> for fast query
		deleted map[hash.CacheHash]struct{}
	}
)

// NewKVCache returns a KVCache
func NewKVCache() KVStoreCache {
	c := kvCache{
		cache:   make(map[hash.CacheHash][]byte),
		deleted: make(map[hash.CacheHash]struct{}),
	}
	return &c
}

// Read retrieves a record
func (c *kvCache) Read(k hash.CacheHash) ([]byte, error) {
	if v, ok := c.cache[k]; ok {
		return v, nil
	}
	if _, ok := c.deleted[k]; ok {
		return nil, ErrAlreadyDeleted
	}
	return nil, ErrNotExist
}

// Write puts a record into cache
func (c *kvCache) Write(k hash.CacheHash, v []byte) {
	c.cache[k] = v
	delete(c.deleted, k)
}

// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
func (c *kvCache) WriteIfNotExist(k hash.CacheHash, v []byte) error {
	if _, ok := c.cache[k]; ok {
		return ErrAlreadyExist
	}
	c.cache[k] = v
	delete(c.deleted, k)
	return nil
}

// Evict deletes a record from cache
func (c *kvCache) Evict(k hash.CacheHash) {
	delete(c.cache, k)
	c.deleted[k] = struct{}{}
}

// Clear clear the cache
func (c *kvCache) Clear() {
	c.cache = nil
	c.deleted = nil
	c.cache = make(map[hash.CacheHash][]byte)
	c.deleted = make(map[hash.CacheHash]struct{})
}
