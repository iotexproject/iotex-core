// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package trie

import (
	"context"
)

// KVStore defines an interface for storing trie data as key-value pair
type KVStore interface {
	// Start starts the KVStore
	Start(context.Context) error
	// Stop stops the KVStore
	Stop(context.Context) error
	// Put puts key, value pair into KVStore
	Put([]byte, []byte) error
	// Delete deletes record from KVStore by key
	Delete([]byte) error
	// Get gets the value from KVStore by key
	Get([]byte) ([]byte, error)
}

type mKeyType [32]byte

type inMemKVStore struct {
	kvpairs map[mKeyType][]byte
}

func castKeyType(k []byte) mKeyType {
	var c mKeyType
	copy(c[:], k)

	return c
}

// newInMemKVStore defines a kv store in memory
func newInMemKVStore() KVStore {
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
