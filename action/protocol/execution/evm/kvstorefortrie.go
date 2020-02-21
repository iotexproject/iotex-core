// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/state"
)

type kvStoreForTrie struct {
	nsOpt protocol.StateOption
	sm    protocol.StateManager
}

func newKVStoreForTrieWithStateManager(ns string, sm protocol.StateManager) trie.KVStore {
	return &kvStoreForTrie{nsOpt: protocol.NamespaceOption(ns), sm: sm}
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
	_, err := kv.sm.PutState(sb, protocol.KeyOption(key), kv.nsOpt)

	return err
}

func (kv *kvStoreForTrie) Delete(key []byte) error {
	_, err := kv.sm.DelState(protocol.KeyOption(key), kv.nsOpt)
	switch errors.Cause(err) {
	case state.ErrStateNotExist:
		return errors.Wrapf(db.ErrNotExist, "failed to find key %x", key)
	}
	return err
}

func (kv *kvStoreForTrie) Get(key []byte) ([]byte, error) {
	var value SerializableBytes
	_, err := kv.sm.State(&value, protocol.KeyOption(key), kv.nsOpt)
	switch errors.Cause(err) {
	case state.ErrStateNotExist:
		return nil, errors.Wrapf(db.ErrNotExist, "failed to find key %x", key)
	}
	return value, err
}
