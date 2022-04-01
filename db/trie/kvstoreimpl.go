// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
)

var (
	_trieKeystoreMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_trie_keystore",
			Help: "IoTeX Trie Keystore",
		},
		[]string{"type"},
	)
)

func init() {
	prometheus.MustRegister(_trieKeystoreMtc)
}

type (
	// kvStoreImpl defines a kvStore with fixed bucket and cache layer for trie.
	// It may be used in other cases as well
	kvStoreImpl struct {
		lc     lifecycle.Lifecycle
		bucket string
		dao    db.KVStoreBasic
	}

	mKeyType [32]byte

	inMemKVStore struct {
		kvpairs map[mKeyType][]byte
	}
)

func castKeyType(k []byte) mKeyType {
	var c mKeyType
	copy(c[:], k)

	return c
}

// NewMemKVStore defines a kv store in memory
func NewMemKVStore() KVStore {
	return &inMemKVStore{kvpairs: map[mKeyType][]byte{}}
}

func (s *inMemKVStore) Start(ctx context.Context) error {
	return nil
}

func (s *inMemKVStore) Stop(ctx context.Context) error {
	return nil
}

func (s *inMemKVStore) Put(k []byte, v []byte) error {
	dbKey := castKeyType(k)
	s.kvpairs[dbKey] = v

	return nil
}

func (s *inMemKVStore) Get(k []byte) ([]byte, error) {
	dbKey := castKeyType(k)
	v, ok := s.kvpairs[dbKey]
	if !ok {
		return nil, ErrNotExist
	}
	return v, nil
}

func (s *inMemKVStore) Delete(k []byte) error {
	dbKey := castKeyType(k)
	delete(s.kvpairs, dbKey)

	return nil
}

func (s *inMemKVStore) Purge(tag, k []byte) error {
	return nil
}

// NewKVStore creates a new KVStore
func NewKVStore(bucket string, dao db.KVStoreBasic) (KVStore, error) {
	s := &kvStoreImpl{
		bucket: bucket,
		dao:    dao,
	}
	s.lc.Add(s.dao)

	return s, nil
}

// Start starts the kv store
func (s *kvStoreImpl) Start(ctx context.Context) error {
	return s.lc.OnStart(ctx)
}

// Stop stops the kv store
func (s *kvStoreImpl) Stop(ctx context.Context) error {
	return s.lc.OnStop(ctx)
}

// Delete deletes key
func (s *kvStoreImpl) Delete(key []byte) error {
	_trieKeystoreMtc.WithLabelValues("delete").Inc()

	err := s.dao.Delete(s.bucket, key)
	if errors.Cause(err) == db.ErrNotExist {
		return errors.Wrap(ErrNotExist, err.Error())
	}

	return err
}

// Put puts value for key
func (s *kvStoreImpl) Put(key, value []byte) error {
	_trieKeystoreMtc.WithLabelValues("put").Inc()
	return s.dao.Put(s.bucket, key, value)
}

// Get gets value of key
func (s *kvStoreImpl) Get(key []byte) ([]byte, error) {
	_trieKeystoreMtc.WithLabelValues("get").Inc()
	value, err := s.dao.Get(s.bucket, key)
	if errors.Cause(err) == db.ErrNotExist {
		return nil, errors.Wrapf(ErrNotExist, err.Error())
	}

	return value, err
}
