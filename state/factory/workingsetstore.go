// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"sync"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
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
	workingSetStoreCommon struct {
		lock sync.Mutex
		// TODO: handle committed flag properly in the functions
		committed bool
		flusher   db.KVStoreFlusher
	}
)

func (store *workingSetStoreCommon) Filter(ns string, cond db.Condition, start, limit []byte) ([][]byte, [][]byte, error) {
	return store.flusher.KVStoreWithBuffer().Filter(ns, cond, start, limit)
}

func (store *workingSetStoreCommon) WriteBatch(bat batch.KVStoreBatch) error {
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

func (store *workingSetStoreCommon) Put(ns string, key []byte, value []byte) error {
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

func (store *workingSetStoreCommon) Delete(ns string, key []byte) error {
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

func (store *workingSetStoreCommon) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *workingSetStoreCommon) Commit() error {
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

func (store *workingSetStoreCommon) Snapshot() int {
	return store.flusher.KVStoreWithBuffer().Snapshot()
}

func (store *workingSetStoreCommon) RevertSnapshot(snapshot int) error {
	return store.flusher.KVStoreWithBuffer().RevertSnapshot(snapshot)
}

func (store *workingSetStoreCommon) ResetSnapshots() {
	store.flusher.KVStoreWithBuffer().ResetSnapshots()
}
