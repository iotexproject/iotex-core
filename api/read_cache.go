package api

import (
	"encoding/json"
	"sync/atomic"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/pkg/log"
)

type (
	// ReadKey represents a read key
	ReadKey struct {
		Name   string   `json:"name,omitempty"`
		Height string   `json:"height,omitempty"`
		Method []byte   `json:"method,omitempty"`
		Args   [][]byte `json:"args,omitempty"`
	}

	// ReadCache stores read results
	ReadCache struct {
		total, hit atomic.Int64
		c          *ttl.Cache
	}
)

// Hash returns the hash of key's json string
func (k *ReadKey) Hash() hash.Hash160 {
	b, _ := json.Marshal(k)
	return hash.Hash160b(b)
}

// NewReadCache returns a new read cache
func NewReadCache() *ReadCache {
	c, _ := ttl.NewCache()
	return &ReadCache{
		c: c,
	}
}

// Get reads according to key
func (rc *ReadCache) Get(key hash.Hash160) ([]byte, bool) {
	total := rc.total.Add(1)
	d, ok := rc.c.Get(key)
	if !ok {
		return nil, false
	}
	if hit := rc.hit.Add(1); hit%100 == 0 {
		log.Logger("api").Info("API cache hit", zap.Int64("total", total), zap.Int64("hit", hit))
	}
	return d.([]byte), true
}

// Put writes according to key
func (rc *ReadCache) Put(key hash.Hash160, value []byte) {
	rc.c.Set(key, value)
}

// Clear clears the cache
func (rc *ReadCache) Clear() {
	rc.c.Reset()
}
