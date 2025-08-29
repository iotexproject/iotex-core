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

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	workingSetStore interface {
		Start(context.Context) error
		Stop(context.Context) error
		KVStore() db.KVStore
		PutObject(ns string, key []byte, object any, secondaryOnly bool) (err error)
		GetObject(ns string, key []byte, object any, secondaryOnly bool) error
		DeleteObject(ns string, key []byte, object any, secondaryOnly bool) error
		States(ns string, keys [][]byte, object any, secondaryOnly bool) ([][]byte, [][]byte, error)
		Commit(context.Context, uint64) error
		Digest() hash.Hash256
		Finalize(context.Context) error
		FinalizeTx(context.Context) error
		Snapshot() int
		RevertSnapshot(int) error
		ResetSnapshots()
		Close()
		CreateGenesisStates(context.Context) error
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

func (store *stateDBWorkingSetStore) PutObject(ns string, key []byte, obj any, secondaryOnly bool) error {
	if secondaryOnly {
		return nil
	}
	store.lock.Lock()
	defer store.lock.Unlock()
	value, err := state.Serialize(obj)
	if err != nil {
		return errors.Wrapf(err, "failed to serialize object of ns = %x and key = %x", ns, key)
	}
	return store.putKV(ns, key, value)
}

func (store *stateDBWorkingSetStore) Put(ns string, key []byte, value []byte) error {
	store.lock.Lock()
	defer store.lock.Unlock()
	return store.putKV(ns, key, value)
}

func (store *stateDBWorkingSetStore) putKV(ns string, key []byte, value []byte) error {
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

func (store *stateDBWorkingSetStore) DeleteObject(ns string, key []byte, obj any, secondaryOnly bool) error {
	if secondaryOnly {
		return nil
	}
	return store.Delete(ns, key)
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

func (store *stateDBWorkingSetStore) Commit(_ context.Context, _ uint64) error {
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

func (store *stateDBWorkingSetStore) GetObject(ns string, key []byte, obj any, secondaryOnly bool) error {
	if secondaryOnly {
		return errors.Wrap(state.ErrStateNotExist, "working set store not support secondary only")
	}
	v, err := store.getKV(ns, key)
	if err != nil {
		return err
	}
	return state.Deserialize(obj, v)
}

func (store *stateDBWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	return store.getKV(ns, key)
}

func (store *stateDBWorkingSetStore) getKV(ns string, key []byte) ([]byte, error) {
	data, err := store.flusher.KVStoreWithBuffer().Get(ns, key)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, errors.Wrapf(state.ErrStateNotExist, "failed to get state of ns = %x and key = %x", ns, key)
		}
		return nil, err
	}
	return data, nil
}

func (store *stateDBWorkingSetStore) States(ns string, keys [][]byte, obj any, secondaryOnly bool) ([][]byte, [][]byte, error) {
	if secondaryOnly {
		return nil, nil, errors.Wrap(state.ErrStateNotExist, "working set store not support secondary only")
	}
	if store.readBuffer {
		// TODO: after the 180 HF, we can revert readBuffer, and always go this case
		return readStates(store.flusher.KVStoreWithBuffer(), ns, keys)
	}
	return readStates(store.flusher.BaseKVStore(), ns, keys)
}

func (store *stateDBWorkingSetStore) Finalize(ctx context.Context) error {
	height := protocol.MustGetBlockCtx(ctx).BlockHeight
	// Persist current chain Height
	store.flusher.KVStoreWithBuffer().MustPut(
		AccountKVNamespace,
		[]byte(CurrentHeightKey),
		byteutil.Uint64ToBytes(height),
	)
	return nil
}

func (store *stateDBWorkingSetStore) FinalizeTx(_ context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) Close() {}

func (store *stateDBWorkingSetStore) CreateGenesisStates(ctx context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) KVStore() db.KVStore {
	return store
}
