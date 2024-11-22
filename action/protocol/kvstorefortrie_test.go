// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/v2/state"
)

type inMemStateManager struct {
	store map[string]map[string][]byte
}

func newInMemStateManager() *inMemStateManager {
	return &inMemStateManager{
		store: map[string]map[string][]byte{},
	}
}

func (imsm *inMemStateManager) Height() (uint64, error) {
	return 0, nil
}

func (imsm *inMemStateManager) State(s interface{}, opts ...StateOption) (uint64, error) {
	cfg, err := CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	value, ok := imsm.store[cfg.Namespace][hex.EncodeToString(cfg.Key)]
	if !ok {
		return 0, state.ErrStateNotExist
	}
	state.Deserialize(s, value)
	return 0, nil
}

func (imsm *inMemStateManager) States(...StateOption) (uint64, state.Iterator, error) {
	return 0, nil, nil
}

func (imsm *inMemStateManager) ReadView(string) (interface{}, error) {
	return nil, nil
}

func (imsm *inMemStateManager) Snapshot() int {
	return 0
}

func (imsm *inMemStateManager) Revert(int) error {
	return nil
}

func (imsm *inMemStateManager) PutState(s interface{}, opts ...StateOption) (uint64, error) {
	cfg, err := CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	value, err := state.Serialize(s)
	if err != nil {
		return 0, err
	}
	if _, ok := imsm.store[cfg.Namespace]; !ok {
		imsm.store[cfg.Namespace] = map[string][]byte{}
	}
	imsm.store[cfg.Namespace][hex.EncodeToString(cfg.Key)] = value

	return 0, nil
}

func (imsm *inMemStateManager) DelState(opts ...StateOption) (uint64, error) {
	cfg, err := CreateStateConfig(opts...)
	if err != nil {
		return 0, err
	}
	if _, ok := imsm.store[cfg.Namespace][hex.EncodeToString(cfg.Key)]; !ok {
		return 0, state.ErrStateNotExist
	}
	delete(imsm.store[cfg.Namespace], hex.EncodeToString(cfg.Key))

	return 0, nil
}

func (imsm *inMemStateManager) WriteView(string, interface{}) error {
	return nil
}

func (imsm *inMemStateManager) ProtocolDirty(string) bool {
	return false
}

func (imsm *inMemStateManager) Load(string, string, interface{}) error {
	return nil
}

func (imsm *inMemStateManager) Unload(string, string, interface{}) error {
	return nil
}

func (imsm *inMemStateManager) Reset() {
}

func TestKVStoreForTrie(t *testing.T) {
	require := require.New(t)
	ns := "namespace"
	key := []byte("key")
	value := SerializableBytes("value")
	sm := newInMemStateManager()
	var kvstore trie.KVStore
	kvstore = NewKVStoreForTrieWithStateManager(ns, sm)
	require.NoError(kvstore.Start(context.Background()))
	require.NoError(kvstore.Stop(context.Background()))
	_, err := kvstore.Get(key)
	require.Error(err)
	require.NoError(kvstore.Delete(key))
	require.NoError(kvstore.Put(key, value))
	fromStore, err := kvstore.Get(key)
	require.NoError(err)
	require.True(bytes.Equal(fromStore, value))
	require.NoError(kvstore.Delete(key))
	_, err = kvstore.Get(key)
	require.Error(err)
	require.NoError(kvstore.Put(key, value))
	kvstore = NewKVStoreForTrieWithStateReader(ns, sm)
	require.NoError(kvstore.Start(context.Background()))
	require.NoError(kvstore.Stop(context.Background()))
	require.Error(kvstore.Put(key, value), "not implemented")
	require.Error(kvstore.Delete(key), "not implemented")
	_, err = kvstore.Get([]byte("not exist"))
	require.Error(err)
	fromStore, err = kvstore.Get(key)
	require.NoError(err)
	require.True(bytes.Equal(fromStore, value))

}

func TestHistoryKVStoreForTrie(t *testing.T) {
	r := require.New(t)
	ns := "namespace"
	key := []byte("key")
	value := SerializableBytes("value")
	sm := newInMemStateManager()
	kvstore := NewKVStoreForTrieWithStateManager(ns, sm)

	trie, err := mptrie.New(mptrie.KVStoreOption(kvstore), mptrie.KeyLengthOption(3))
	r.NoError(err)
	r.NoError(trie.Start(context.Background()))
	defer trie.Stop(context.Background())

	root0, err := trie.RootHash()
	r.NoError(err)
	t.Logf("root: %x\n", root0)

	r.NoError(trie.Upsert(key, value))
	root1, err := trie.RootHash()
	r.NoError(err)
	t.Logf("root: %x\n", root1)

	r.NoError(trie.Upsert(key, SerializableBytes("value2")))
	root2, err := trie.RootHash()
	r.NoError(err)
	t.Logf("root: %x\n", root2)

	r.NoError(trie.SetRootHash(root1))
	for _, opt := range kvstore.Stales() {
		for _, o := range opt {
			t.Logf("stale: %#v\n", o)
		}
	}
}
