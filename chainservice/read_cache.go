// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package chainservice

import (
	"encoding/json"

	"github.com/iotexproject/go-pkgs/cache/ttl"
	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

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
	rc.total++
	d, ok := rc.c.Get(key)
	if !ok {
		return nil, false
	}
	rc.hit++
	if rc.hit%100 == 0 {
		log.Logger("api").Info("API cache hit", zap.Int("total", rc.total), zap.Int("hit", rc.hit))
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
