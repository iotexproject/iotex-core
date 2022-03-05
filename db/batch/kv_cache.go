// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package batch

import (
	"github.com/iotexproject/go-pkgs/hash"
)

type (
	// KVStoreCache is a local cache of batched <k, v> for fast query
	KVStoreCache interface {
		// Read retrieves a record
		Read(hash160 hash.Hash160) ([]byte, error)
		// Write puts a record into cache
		Write(hash.Hash160, []byte)
		// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
		WriteIfNotExist(hash.Hash160, []byte) error
		// Evict deletes a record from cache
		Evict(hash.Hash160)
		// Clear clear the cache
		Clear()
	}

	// kvCache implements KVStoreCache interface
	kvCache struct {
		cache   map[hash.Hash160][]byte // local cache of batched <k, v> for fast query
		deleted map[hash.Hash160]struct{}
	}
)

// NewKVCache returns a KVCache
func NewKVCache() KVStoreCache {
	c := kvCache{
		cache:   make(map[hash.Hash160][]byte),
		deleted: make(map[hash.Hash160]struct{}),
	}
	return &c
}

// Read retrieves a record
func (c *kvCache) Read(k hash.Hash160) ([]byte, error) {
	if v, ok := c.cache[k]; ok {
		return v, nil
	}
	if _, ok := c.deleted[k]; ok {
		return nil, ErrAlreadyDeleted
	}
	return nil, ErrNotExist
}

// Write puts a record into cache
func (c *kvCache) Write(k hash.Hash160, v []byte) {
	c.cache[k] = v
	delete(c.deleted, k)
}

// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
func (c *kvCache) WriteIfNotExist(k hash.Hash160, v []byte) error {
	if _, ok := c.cache[k]; ok {
		return ErrAlreadyExist
	}
	c.cache[k] = v
	delete(c.deleted, k)
	return nil
}

// Evict deletes a record from cache
func (c *kvCache) Evict(k hash.Hash160) {
	delete(c.cache, k)
	c.deleted[k] = struct{}{}
}

// Clear clear the cache
func (c *kvCache) Clear() {
	c.cache = nil
	c.deleted = nil
	c.cache = make(map[hash.Hash160][]byte)
	c.deleted = make(map[hash.Hash160]struct{})
}
