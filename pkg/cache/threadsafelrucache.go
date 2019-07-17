// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package cache

import (
	"sync"

	"github.com/golang/groupcache/lru"
)

// ThreadSafeLruCache defines a lru cache which is thread safe
type ThreadSafeLruCache struct {
	cache *lru.Cache
	mutex sync.RWMutex
}

// NewThreadSafeLruCache returns a thread safe lru cache with fix size
func NewThreadSafeLruCache(maxEntries int) *ThreadSafeLruCache {
	return &ThreadSafeLruCache{
		cache: lru.New(maxEntries),
	}
}

// Add adds a value to the cache.
func (c *ThreadSafeLruCache) Add(key lru.Key, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache.Add(key, value)
}

// Get looks up a key's value from the cache.
func (c *ThreadSafeLruCache) Get(key lru.Key) (value interface{}, ok bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.cache.Get(key)
}

// Remove removes the provided key from the cache.
func (c *ThreadSafeLruCache) Remove(key lru.Key) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache.Remove(key)
}

// RemoveOldest removes the oldest item from the cache.
func (c *ThreadSafeLruCache) RemoveOldest() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache.RemoveOldest()
}

// Len returns the number of items in the cache.
func (c *ThreadSafeLruCache) Len() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.cache.Len()
}

// Clear purges all stored items from the cache.
func (c *ThreadSafeLruCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache.Clear()
}
