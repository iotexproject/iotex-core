// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package batch

type (
	// KVStoreCache is a local cache of batched <k, v> for fast query
	KVStoreCache interface {
		// Read retrieves a record
		Read(*kvCacheKey) ([]byte, error)
		// Write puts a record into cache
		Write(*kvCacheKey, []byte)
		// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
		WriteIfNotExist(*kvCacheKey, []byte) error
		// Evict deletes a record from cache
		Evict(*kvCacheKey)
		// Clear clear the cache
		Clear()
		// Clone clones the cache
		Clone() KVStoreCache
	}

	// kvCacheKey is the key for 2D Map cache
	kvCacheKey struct {
		key1 string
		key2 string
	}

	// kvCache implements KVStoreCache interface
	kvCache struct {
		cache      map[string]map[string][]byte // local cache of batched <k, v> for fast query
		deletedMap map[string]map[string]struct{}
	}
)

// NewKVCache returns a KVCache
func NewKVCache() KVStoreCache {
	return &kvCache{
		cache:      make(map[string]map[string][]byte),
		deletedMap: make(map[string]map[string]struct{}),
	}
}

// Read retrieves a record
func (c *kvCache) Read(key *kvCacheKey) ([]byte, error) {
	if v := c.cacheGet(key); v != nil {
		return v, nil
	}
	if c.deletedMapGet(key) {
		return nil, ErrAlreadyDeleted
	}
	return nil, ErrNotExist
}

// Write puts a record into cache
func (c *kvCache) Write(key *kvCacheKey, v []byte) {
	c.cachePut(key, v)
	c.deletedMapDelete(key)
}

// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
func (c *kvCache) WriteIfNotExist(key *kvCacheKey, v []byte) error {
	if v := c.cacheGet(key); v != nil {
		return ErrAlreadyExist
	}
	c.cachePut(key, v)
	c.deletedMapDelete(key)
	return nil
}

// Evict deletes a record from cache
func (c *kvCache) Evict(key *kvCacheKey) {
	c.cacheDelete(key)
	c.deletedMapPut(key)
}

// Clear clear the cache
func (c *kvCache) Clear() {
	c.cacheClear()
	c.deletedMapClear()
}

// Clone clones the cache
func (c *kvCache) Clone() KVStoreCache {
	clone := kvCache{
		cache:      make(map[string]map[string][]byte),
		deletedMap: make(map[string]map[string]struct{}),
	}
	// clone entries in map
	for k1, v1 := range c.cache {
		clone.cache[k1] = make(map[string][]byte)
		for k2, v2 := range v1 {
			clone.cache[k1][k2] = v2
		}
	}
	for k1, v1 := range c.deletedMap {
		clone.deletedMap[k1] = make(map[string]struct{})
		for k2, v2 := range v1 {
			clone.deletedMap[k1][k2] = v2
		}
	}
	return &clone
}

func (c *kvCache) cachePut(key *kvCacheKey, v []byte) {
	if _, ok := c.cache[key.key1]; !ok {
		c.cache[key.key1] = make(map[string][]byte)
	}
	c.cache[key.key1][key.key2] = v
}

func (c *kvCache) cacheGet(key *kvCacheKey) []byte {
	if v1, ok := c.cache[key.key1]; ok {
		if v2, ok := v1[key.key2]; ok {
			return v2
		}
	}
	return nil
}

func (c *kvCache) cacheDelete(key *kvCacheKey) {
	if v1, ok := c.cache[key.key1]; ok {
		delete(v1, key.key2)
		if len(v1) == 0 {
			delete(c.cache, key.key1)
		}
	}
}

func (c *kvCache) cacheClear() {
	for k1 := range c.cache {
		for k2 := range c.cache[k1] {
			delete(c.cache[k1], k2)
		}
		delete(c.cache, k1)
	}
}

func (c *kvCache) deletedMapPut(key *kvCacheKey) {
	if _, ok := c.deletedMap[key.key1]; !ok {
		c.deletedMap[key.key1] = make(map[string]struct{})
	}
	c.deletedMap[key.key1][key.key2] = struct{}{}
}

func (c *kvCache) deletedMapGet(key *kvCacheKey) bool {
	if v1, ok := c.deletedMap[key.key1]; ok {
		if _, ok := v1[key.key2]; ok {
			return true
		}
	}
	return false
}

func (c *kvCache) deletedMapDelete(key *kvCacheKey) {
	if v1, ok := c.deletedMap[key.key1]; ok {
		delete(v1, key.key2)
		if len(v1) == 0 {
			delete(c.deletedMap, key.key1)
		}
	}
}

func (c *kvCache) deletedMapClear() {
	for k1 := range c.deletedMap {
		for k2 := range c.deletedMap[k1] {
			delete(c.deletedMap[k1], k2)
		}
		delete(c.deletedMap, k1)
	}
}
