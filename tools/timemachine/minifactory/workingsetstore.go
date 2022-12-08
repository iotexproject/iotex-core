// Copyright (c) 2022 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package minifactory

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
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
		view       protocol.View
		flusher    db.KVStoreFlusher
		readBuffer bool
	}
	factoryWorkingSetStore struct {
		view      protocol.View
		flusher   db.KVStoreFlusher
		tlt       trie.TwoLayerTrie
		trieRoots map[int][]byte
	}
)

func newStateDBWorkingSetStore(view protocol.View, flusher db.KVStoreFlusher, readBuffer bool) workingSetStore {
	return &stateDBWorkingSetStore{
		flusher:    flusher,
		view:       view,
		readBuffer: readBuffer,
	}
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
	return nil, factory.ErrNotSupported
}

func (store *stateDBWorkingSetStore) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *stateDBWorkingSetStore) Finalize(height uint64) error {
	// Persist current chain Height
	store.flusher.KVStoreWithBuffer().MustPut(
		factory.AccountKVNamespace,
		[]byte(factory.CurrentHeightKey),
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
