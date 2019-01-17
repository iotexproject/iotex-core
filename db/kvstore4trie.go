// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	trieKeystoreMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_trie_keystore",
			Help: "IoTeX Trie Keystore",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(trieKeystoreMtc)
}

// KVStoreForTrie defines a kvstore with fixed bucket and cache layer for trie.
// It may be used in other cases as well
type KVStoreForTrie struct {
	lc     lifecycle.Lifecycle
	bucket string
	dao    KVStore
	cb     CachedBatch
}

// Option defines an interface to initialize the kv store
type Option func(*KVStoreForTrie) error

// CachedBatchOption defines a way to set the cache layer for db
func CachedBatchOption(cb CachedBatch) Option {
	return func(kvStore *KVStoreForTrie) error {
		kvStore.cb = cb
		return nil
	}
}

// NewKVStoreForTrie creates a new KVStoreForTrie
func NewKVStoreForTrie(bucket string, dao KVStore, options ...Option) (*KVStoreForTrie, error) {
	s := &KVStoreForTrie{bucket: bucket, dao: dao}
	for _, opt := range options {
		if err := opt(s); err != nil {
			return nil, err
		}
	}
	if s.cb == nil {
		// always have a cache layer
		s.cb = NewCachedBatch()
	}
	s.lc.Add(s.dao)
	return s, nil
}

// Start starts the kv store
func (s *KVStoreForTrie) Start(ctx context.Context) error {
	return s.lc.OnStart(ctx)
}

// Stop stops the kv store
func (s *KVStoreForTrie) Stop(ctx context.Context) error {
	return s.lc.OnStop(ctx)
}

// Delete deletes key
func (s *KVStoreForTrie) Delete(key []byte) error {
	trieKeystoreMtc.WithLabelValues("delete").Inc()
	s.cb.Delete(s.bucket, key, "failed to delete key %x", key)
	// TODO: bug, need to mark key as deleted
	return nil
}

// Put puts value for key
func (s *KVStoreForTrie) Put(key []byte, value []byte) error {
	trieKeystoreMtc.WithLabelValues("put").Inc()
	s.cb.Put(s.bucket, key, value, "failed to put key %x value %x", key, value)
	return nil
}

// Get gets value of key
func (s *KVStoreForTrie) Get(key []byte) ([]byte, error) {
	trieKeystoreMtc.WithLabelValues("get").Inc()
	v, err := s.cb.Get(s.bucket, key)
	if err != nil {
		if v, err = s.dao.Get(s.bucket, key); err != nil {
			return nil, errors.Wrapf(err, "failed to get key %x", key)
		}
		// TODO: put it back to cache
	}
	return v, err
}

// Flush flushs the data in cache layer to db
func (s *KVStoreForTrie) Flush() error {
	return s.dao.Commit(s.cb)
}
