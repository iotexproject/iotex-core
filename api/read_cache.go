package api

import (
	"encoding/json"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/log"
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
		total, hit int
		lock       sync.RWMutex
		bins       map[hash.Hash160][]byte
	}
)

// Hash returns the hash of key's json string
func (k *ReadKey) Hash() hash.Hash160 {
	b, _ := json.Marshal(k)
	return hash.Hash160b(b)
}

// NewReadCache returns a new read cache
func NewReadCache() *ReadCache {
	return &ReadCache{
		bins: make(map[hash.Hash160][]byte),
	}
}

// Get reads according to key
func (rc *ReadCache) Get(key hash.Hash160) ([]byte, bool) {
	rc.lock.RLock()
	defer rc.lock.RUnlock()

	rc.total++
	d, ok := rc.bins[key]
	if !ok {
		return nil, false
	}
	rc.hit++
	if rc.hit%100 == 0 {
		log.L().Info("API cache hit", zap.Int("total", rc.total), zap.Int("hit", rc.hit))
	}
	return d, true
}

// Put writes according to key
func (rc *ReadCache) Put(key hash.Hash160, value []byte) {
	rc.lock.Lock()
	rc.bins[key] = value
	rc.lock.Unlock()
}

// Clear clears the cache
func (rc *ReadCache) Clear() {
	rc.lock.Lock()
	rc.bins = nil
	rc.bins = make(map[hash.Hash160][]byte)
	rc.lock.Unlock()
}

// Respond implements the Responder interface
func (rc *ReadCache) Respond(*block.Block) error {
	// invalidate the cache at every new block
	rc.Clear()
	return nil
}

// Exit implements the Responder interface
func (rc *ReadCache) Exit() {
	rc.Clear()
}
