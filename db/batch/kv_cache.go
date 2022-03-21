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
		// Append appends caches
		Append(...KVStoreCache) error
	}

	// kvCacheKey is the key for 2D Map cache
	kvCacheKey struct {
		key1 string
		key2 string
	}

	node struct {
		value   []byte
		deleted bool
	}

	// kvCache implements KVStoreCache interface
	kvCache struct {
		cache map[string]map[string]*node // local cache of batched <k, v> for fast query
	}
)

// NewKVCache returns a KVCache
func NewKVCache() KVStoreCache {
	return &kvCache{
		cache: make(map[string]map[string]*node),
	}
}

// Read retrieves a record
func (c *kvCache) Read(key *kvCacheKey) ([]byte, error) {
	if ns, ok := c.cache[key.key1]; ok {
		if node, ok := ns[key.key2]; ok {
			if node.deleted {
				return nil, ErrAlreadyDeleted
			}
			return node.value, nil
		}
	}
	return nil, ErrNotExist
}

// Write puts a record into cache
func (c *kvCache) Write(key *kvCacheKey, v []byte) {
	if _, ok := c.cache[key.key1]; !ok {
		c.cache[key.key1] = make(map[string]*node)
	}
	c.cache[key.key1][key.key2] = &node{
		value:   v,
		deleted: false,
	}
}

// WriteIfNotExist puts a record into cache only if it doesn't exist, otherwise return ErrAlreadyExist
func (c *kvCache) WriteIfNotExist(key *kvCacheKey, v []byte) error {
	if _, ok := c.cache[key.key1]; !ok {
		c.cache[key.key1] = make(map[string]*node)
	}
	if node, ok := c.cache[key.key1][key.key2]; ok && !node.deleted {
		return ErrAlreadyExist
	}
	c.cache[key.key1][key.key2] = &node{
		value:   v,
		deleted: false,
	}
	return nil
}

// Evict deletes a record from cache
func (c *kvCache) Evict(key *kvCacheKey) {
	if _, ok := c.cache[key.key1]; !ok {
		c.cache[key.key1] = make(map[string]*node)
	}
	c.cache[key.key1][key.key2] = &node{
		value:   nil,
		deleted: true,
	}
}

// Clear clear the cache
func (c *kvCache) Clear() {
	c.cache = make(map[string]map[string]*node)
}

func (c *kvCache) Append(caches ...KVStoreCache) error {
	// c should be written in order
	for _, cc := range caches {
		cc, ok := cc.(*kvCache)
		if !ok {
			return ErrUnexpectedType
		}
		for key1, ns := range cc.cache {
			if _, ok := c.cache[key1]; !ok {
				c.cache[key1] = make(map[string]*node)
			}
			for key2, node := range ns {
				c.cache[key1][key2] = node
			}
		}
	}
	return nil
}
