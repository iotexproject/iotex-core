// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"

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
}

// NewKVStoreForTrie creates a new KVStoreForTrie
func NewKVStoreForTrie(bucket string, dao KVStore) (*KVStoreForTrie, error) {
	s := &KVStoreForTrie{
		bucket: bucket,
		dao:    dao,
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
	// TODO: bug, need to mark key as deleted

	return s.dao.Delete(s.bucket, key)
}

// Put puts value for key
func (s *KVStoreForTrie) Put(key, value []byte) error {
	trieKeystoreMtc.WithLabelValues("put").Inc()
	return s.dao.Put(s.bucket, key, value)
}

// Get gets value of key
func (s *KVStoreForTrie) Get(key []byte) ([]byte, error) {
	trieKeystoreMtc.WithLabelValues("get").Inc()
	return s.dao.Get(s.bucket, key)
}
