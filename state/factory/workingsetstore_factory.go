// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/trie"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

type factoryWorkingSetStore struct {
	*workingSetStoreCommon
	tlt       trie.TwoLayerTrie
	trieRoots map[int][]byte
}

func newFactoryWorkingSetStore(view protocol.View, flusher db.KVStoreFlusher) (workingSetStore, error) {
	tlt, err := newTwoLayerTrie(ArchiveTrieNamespace, flusher.KVStoreWithBuffer(), ArchiveTrieRootKey, true)
	if err != nil {
		return nil, err
	}

	return &factoryWorkingSetStore{
		workingSetStoreCommon: &workingSetStoreCommon{
			flusher: flusher,
			view:    view,
		},
		tlt:       tlt,
		trieRoots: make(map[int][]byte),
	}, nil
}

func newFactoryWorkingSetStoreAtHeight(view protocol.View, flusher db.KVStoreFlusher, height uint64) (workingSetStore, error) {
	rootKey := fmt.Sprintf("%s-%d", ArchiveTrieRootKey, height)
	tlt, err := newTwoLayerTrie(ArchiveTrieNamespace, flusher.KVStoreWithBuffer(), rootKey, false)
	if err != nil {
		return nil, err
	}

	return &factoryWorkingSetStore{
		workingSetStoreCommon: &workingSetStoreCommon{
			flusher: flusher,
			view:    view,
		},
		tlt:       tlt,
		trieRoots: make(map[int][]byte),
	}, nil
}

func (store *factoryWorkingSetStore) Start(ctx context.Context) error {
	return store.tlt.Start(ctx)
}

func (store *factoryWorkingSetStore) Stop(ctx context.Context) error {
	return store.tlt.Stop(ctx)
}

func (store *factoryWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	return readStateFromTLT(store.tlt, ns, key)
}

func (store *factoryWorkingSetStore) Put(ns string, key []byte, value []byte) error {
	store.workingSetStoreCommon.Put(ns, key, value)
	nsHash := hash.Hash160b([]byte(ns))

	return store.tlt.Upsert(nsHash[:], toLegacyKey(key), value)
}

func (store *factoryWorkingSetStore) Delete(ns string, key []byte) error {
	store.workingSetStoreCommon.Delete(ns, key)
	nsHash := hash.Hash160b([]byte(ns))

	err := store.tlt.Delete(nsHash[:], toLegacyKey(key))
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "key %x doesn't exist in namespace %x", key, nsHash)
	}
	return err
}

func (store *factoryWorkingSetStore) States(ns string, keys [][]byte) ([][]byte, [][]byte, error) {
	return readStatesFromTLT(store.tlt, ns, keys)
}

func (store *factoryWorkingSetStore) Finalize(_ context.Context, h uint64) error {
	rootHash, err := store.tlt.RootHash()
	if err != nil {
		return err
	}
	store.flusher.KVStoreWithBuffer().MustPut(AccountKVNamespace, []byte(CurrentHeightKey), byteutil.Uint64ToBytes(h))
	store.flusher.KVStoreWithBuffer().MustPut(ArchiveTrieNamespace, []byte(ArchiveTrieRootKey), rootHash)
	// Persist the historical accountTrie's root hash
	store.flusher.KVStoreWithBuffer().MustPut(
		ArchiveTrieNamespace,
		[]byte(fmt.Sprintf("%s-%d", ArchiveTrieRootKey, h)),
		rootHash,
	)
	return nil
}

func (store *factoryWorkingSetStore) FinalizeTx(context.Context) error {
	return nil
}

func (store *factoryWorkingSetStore) Snapshot() int {
	rh, err := store.tlt.RootHash()
	if err != nil {
		log.L().Panic("failed to do snapshot", zap.Error(err))
	}
	s := store.workingSetStoreCommon.Snapshot()
	store.trieRoots[s] = rh
	return s
}

func (store *factoryWorkingSetStore) RevertSnapshot(snapshot int) error {
	if err := store.workingSetStoreCommon.RevertSnapshot(snapshot); err != nil {
		return err
	}
	root, ok := store.trieRoots[snapshot]
	if !ok {
		// this should not happen, b/c we save the trie root on a successful return of Snapshot(), but check anyway
		return errors.Wrapf(trie.ErrInvalidTrie, "failed to get trie root for snapshot = %d", snapshot)
	}
	return store.tlt.SetRootHash(root[:])
}

func (store *factoryWorkingSetStore) ResetSnapshots() {
	store.workingSetStoreCommon.ResetSnapshots()
	store.trieRoots = make(map[int][]byte)
}

func (store *factoryWorkingSetStore) Close() {}
