// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/state"
)

type (
	kvStoreForTrie struct {
		nsOpt StateOption
		sm    StateManager
	}
	kvStoreForTrieWithStateReader struct {
		nsOpt StateOption
		sr    StateReader
	}
)

// NewKVStoreForTrieWithStateManager creates a trie.KVStore with state manager
func NewKVStoreForTrieWithStateManager(ns string, sm StateManager) trie.KVStore {
	return &kvStoreForTrie{nsOpt: NamespaceOption(ns), sm: sm}
}

func (kv *kvStoreForTrie) Start(context.Context) error {
	return nil
}

func (kv *kvStoreForTrie) Stop(context.Context) error {
	return nil
}

func (kv *kvStoreForTrie) Put(key []byte, value []byte) error {
	var sb SerializableBytes
	if err := sb.Deserialize(value); err != nil {
		return err
	}
	_, err := kv.sm.PutState(sb, KeyOption(key), kv.nsOpt)

	return err
}

func (kv *kvStoreForTrie) Delete(key []byte) error {
	_, err := kv.sm.DelState(KeyOption(key), kv.nsOpt)
	if errors.Cause(err) == state.ErrStateNotExist {
		return nil
	}
	return err
}

func (kv *kvStoreForTrie) Get(key []byte) ([]byte, error) {
	var value SerializableBytes
	_, err := kv.sm.State(&value, KeyOption(key), kv.nsOpt)
	switch errors.Cause(err) {
	case state.ErrStateNotExist:
		return nil, errors.Wrapf(db.ErrNotExist, "failed to find key %x", key)
	}
	return value, err
}

// NewKVStoreForTrieWithStateReader creates a trie.KVStore with state reader
func NewKVStoreForTrieWithStateReader(ns string, sr StateReader) trie.KVStore {
	return &kvStoreForTrieWithStateReader{nsOpt: NamespaceOption(ns), sr: sr}
}

func (kv *kvStoreForTrieWithStateReader) Start(context.Context) error {
	return nil
}

func (kv *kvStoreForTrieWithStateReader) Stop(context.Context) error {
	return nil
}

func (kv *kvStoreForTrieWithStateReader) Put(key []byte, value []byte) error {
	return errors.New("not implemented")
}

func (kv *kvStoreForTrieWithStateReader) Delete(key []byte) error {
	return errors.New("not implemented")
}

func (kv *kvStoreForTrieWithStateReader) Get(key []byte) ([]byte, error) {
	var value SerializableBytes
	_, err := kv.sr.State(&value, KeyOption(key), kv.nsOpt)
	switch errors.Cause(err) {
	case state.ErrStateNotExist:
		return nil, errors.Wrapf(db.ErrNotExist, "failed to find key %x", key)
	}
	return value, err
}
