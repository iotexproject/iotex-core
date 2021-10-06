package api

import (
	"encoding/json"
	"sync"

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
		bins       map[string][]byte
	}
)

// String returns the key as a string
func (k *ReadKey) String() string {
	b, _ := json.Marshal(k)
	return string(b)
}

// NewReadCache returns a new read cache
func NewReadCache() *ReadCache {
	return &ReadCache{
		bins: make(map[string][]byte),
	}
}

// Get reads according to key
func (rc *ReadCache) Get(key string) ([]byte, bool) {
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
func (rc *ReadCache) Put(key string, value []byte) {
	rc.lock.Lock()
	rc.bins[key] = value
	rc.lock.Unlock()
}

// Clear clears the cache
func (rc *ReadCache) Clear() {
	rc.lock.Lock()
	rc.bins = nil
	rc.bins = make(map[string][]byte)
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
