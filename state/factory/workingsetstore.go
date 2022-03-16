// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"fmt"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/zap"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/db/trie/mptrie"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

type (
	workingSetStore interface {
		db.KVStoreBasic
		Commit() error
		States(string, [][]byte) ([][]byte, error)
		Digest() hash.Hash256
		Finalize(uint64) error
		Snapshot() int
		RevertSnapshot(int) error
		ResetSnapshots()
		ReadView(string) (interface{}, error)
		WriteView(string, interface{}) error
	}
	stateDBWorkingSetStore struct {
		view    protocol.View
		flusher db.KVStoreFlusher
	}
	factoryWorkingSetStore struct {
		view      protocol.View
		flusher   db.KVStoreFlusher
		tlt       trie.TwoLayerTrie
		trieRoots map[int][]byte
	}
)

func newStateDBWorkingSetStore(view protocol.View, flusher db.KVStoreFlusher) workingSetStore {
	return &stateDBWorkingSetStore{
		flusher: flusher,
		view:    view,
	}
}

func newFactoryWorkingSetStore(view protocol.View, flusher db.KVStoreFlusher) (workingSetStore, error) {
	tlt, err := newTwoLayerTrie(ArchiveTrieNamespace, flusher.KVStoreWithBuffer(), ArchiveTrieRootKey, true)
	if err != nil {
		return nil, err
	}

	return &factoryWorkingSetStore{
		flusher:   flusher,
		view:      view,
		tlt:       tlt,
		trieRoots: make(map[int][]byte),
	}, nil
}

func (store *stateDBWorkingSetStore) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) Stop(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) ReadView(name string) (interface{}, error) {
	return store.view.Read(name)
}

func (store *stateDBWorkingSetStore) WriteView(name string, value interface{}) error {
	return store.view.Write(name, value)
}

func (store *stateDBWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	data, err := store.flusher.KVStoreWithBuffer().Get(ns, key)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, errors.Wrapf(state.ErrStateNotExist, "failed to get state of ns = %x and key = %x", ns, key)
		}
		return nil, err
	}
	return data, nil
}

func (store *stateDBWorkingSetStore) Put(ns string, key []byte, value []byte) error {
	store.flusher.KVStoreWithBuffer().MustPut(ns, key, value)
	return nil
}

func (store *stateDBWorkingSetStore) Delete(ns string, key []byte) error {
	store.flusher.KVStoreWithBuffer().MustDelete(ns, key)
	return nil
}

func (store *stateDBWorkingSetStore) States(ns string, keys [][]byte) ([][]byte, error) {
	return readStates(store.flusher.KVStoreWithBuffer(), ns, keys)
}

func (store *stateDBWorkingSetStore) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *stateDBWorkingSetStore) Finalize(height uint64) error {
	// Persist current chain Height
	store.flusher.KVStoreWithBuffer().MustPut(
		AccountKVNamespace,
		[]byte(CurrentHeightKey),
		byteutil.Uint64ToBytes(height),
	)
	return nil
}

func (store *stateDBWorkingSetStore) Commit() error {
	return store.flusher.Flush()
}

func (store *stateDBWorkingSetStore) Snapshot() int {
	return store.flusher.KVStoreWithBuffer().Snapshot()
}

func (store *stateDBWorkingSetStore) RevertSnapshot(snapshot int) error {
	return store.flusher.KVStoreWithBuffer().RevertSnapshot(snapshot)
}

func (store *stateDBWorkingSetStore) ResetSnapshots() {
	store.flusher.KVStoreWithBuffer().ResetSnapshots()
}

func (store *factoryWorkingSetStore) Start(ctx context.Context) error {
	return store.tlt.Start(ctx)
}

func (store *factoryWorkingSetStore) Stop(ctx context.Context) error {
	return store.tlt.Stop(ctx)
}

func (store *factoryWorkingSetStore) ReadView(name string) (interface{}, error) {
	return store.view.Read(name)
}

func (store *factoryWorkingSetStore) WriteView(name string, value interface{}) error {
	return store.view.Write(name, value)
}

func (store *factoryWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	return readState(store.tlt, ns, key)
}

func (store *factoryWorkingSetStore) Put(ns string, key []byte, value []byte) error {
	store.flusher.KVStoreWithBuffer().MustPut(ns, key, value)
	nsHash := hash.Hash160b([]byte(ns))

	return store.tlt.Upsert(nsHash[:], toLegacyKey(key), value)
}

func (store *factoryWorkingSetStore) Delete(ns string, key []byte) error {
	store.flusher.KVStoreWithBuffer().MustDelete(ns, key)
	nsHash := hash.Hash160b([]byte(ns))

	err := store.tlt.Delete(nsHash[:], toLegacyKey(key))
	if errors.Cause(err) == trie.ErrNotExist {
		return errors.Wrapf(state.ErrStateNotExist, "key %x doesn't exist in namespace %x", key, nsHash)
	}
	return err
}

func (store *factoryWorkingSetStore) States(ns string, keys [][]byte) ([][]byte, error) {
	values := [][]byte{}
	if keys == nil {
		iter, err := mptrie.NewLayerTwoLeafIterator(store.tlt, namespaceKey(ns), legacyKeyLen())
		if err != nil {
			return nil, err
		}
		for {
			_, value, err := iter.Next()
			if err == trie.ErrEndOfIterator {
				break
			}
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
	} else {
		for _, key := range keys {
			value, err := readState(store.tlt, ns, key)
			switch errors.Cause(err) {
			case state.ErrStateNotExist:
				values = append(values, nil)
			case nil:
				values = append(values, value)
			default:
				return nil, err
			}
		}
	}
	return values, nil
}
func (store *factoryWorkingSetStore) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *factoryWorkingSetStore) Finalize(h uint64) error {
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

func (store *factoryWorkingSetStore) Commit() error {
	dbBatchSizelMtc.WithLabelValues().Set(float64(store.flusher.KVStoreWithBuffer().Size()))
	return store.flusher.Flush()
}

func (store *factoryWorkingSetStore) Snapshot() int {
	rh, err := store.tlt.RootHash()
	if err != nil {
		log.L().Panic("failed to do snapshot", zap.Error(err))
	}
	s := store.flusher.KVStoreWithBuffer().Snapshot()
	store.trieRoots[s] = rh
	return s
}

func (store *factoryWorkingSetStore) RevertSnapshot(snapshot int) error {
	if err := store.flusher.KVStoreWithBuffer().RevertSnapshot(snapshot); err != nil {
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
	store.flusher.KVStoreWithBuffer().ResetSnapshots()
	store.trieRoots = make(map[int][]byte)
}
