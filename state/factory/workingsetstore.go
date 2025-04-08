// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	workingSetStore interface {
		db.KVStore
		Commit() error
		States(string, [][]byte) ([][]byte, [][]byte, error)
		Digest() hash.Hash256
		Finalize(uint64) error
		Snapshot() int
		RevertSnapshot(int) error
		ResetSnapshots()
	}

	stateDBWorkingSetStore struct {
		lock sync.Mutex
		// TODO: handle committed flag properly in the functions
		committed  bool
		readBuffer bool
		flusher    db.KVStoreFlusher
	}
)

func newStateDBWorkingSetStore(flusher db.KVStoreFlusher, readBuffer bool) workingSetStore {
	return &stateDBWorkingSetStore{
		flusher:    flusher,
		readBuffer: readBuffer,
	}
}

func (store *stateDBWorkingSetStore) Filter(ns string, cond db.Condition, start, limit []byte) ([][]byte, [][]byte, error) {
	return store.flusher.KVStoreWithBuffer().Filter(ns, cond, start, limit)
}

func (store *stateDBWorkingSetStore) WriteBatch(bat batch.KVStoreBatch) error {
	store.lock.Lock()
	defer store.lock.Unlock()
	if err := store.flusher.KVStoreWithBuffer().WriteBatch(bat); err != nil {
		return errors.Wrap(err, "failed to write batch")
	}
	if !store.committed {
		return nil
	}
	return store.flusher.Flush()
}

func (store *stateDBWorkingSetStore) Put(ns string, key []byte, value []byte) error {
	store.lock.Lock()
	defer store.lock.Unlock()
	if err := store.flusher.KVStoreWithBuffer().Put(ns, key, value); err != nil {
		return errors.Wrap(err, "failed to put value")
	}
	if !store.committed {
		return nil
	}
	return store.flusher.Flush()
}

func (store *stateDBWorkingSetStore) Delete(ns string, key []byte) error {
	store.lock.Lock()
	defer store.lock.Unlock()
	if err := store.flusher.KVStoreWithBuffer().Delete(ns, key); err != nil {
		return errors.Wrap(err, "failed to delete value")
	}
	if !store.committed {
		return nil
	}
	return store.flusher.Flush()
}

func (store *stateDBWorkingSetStore) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *stateDBWorkingSetStore) Commit() error {
	store.lock.Lock()
	defer store.lock.Unlock()
	if store.committed {
		return errors.New("working set store already committed")
	}
	_dbBatchSizelMtc.WithLabelValues().Set(float64(store.flusher.KVStoreWithBuffer().Size()))
	if err := store.flusher.Flush(); err != nil {
		return errors.Wrap(err, "failed to commit working set store")
	}
	store.committed = true
	return nil
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

func (store *stateDBWorkingSetStore) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) Stop(context.Context) error {
	return nil
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

func (store *stateDBWorkingSetStore) States(ns string, keys [][]byte) ([][]byte, [][]byte, error) {
	if store.readBuffer {
		// TODO: after the 180 HF, we can revert readBuffer, and always go this case
		return readStates(store.flusher.KVStoreWithBuffer(), ns, keys)
	}
	return readStates(store.flusher.BaseKVStore(), ns, keys)
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
