// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"github.com/iotexproject/go-pkgs/hash"

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
		flusher db.KVStoreFlusher
	}
)

func (store *workingSetStoreCommon) Filter(ns string, cond db.Condition, start, limit []byte) ([][]byte, [][]byte, error) {
	return store.flusher.KVStoreWithBuffer().Filter(ns, cond, start, limit)
}

func (store *workingSetStoreCommon) WriteBatch(bat batch.KVStoreBatch) error {
	return store.flusher.KVStoreWithBuffer().WriteBatch(bat)
}

func (store *workingSetStoreCommon) Put(ns string, key []byte, value []byte) error {
	store.flusher.KVStoreWithBuffer().MustPut(ns, key, value)
	return nil
}

func (store *workingSetStoreCommon) Delete(ns string, key []byte) error {
	store.flusher.KVStoreWithBuffer().MustDelete(ns, key)
	return nil
}

func (store *workingSetStoreCommon) Digest() hash.Hash256 {
	return hash.Hash256b(store.flusher.SerializeQueue())
}

func (store *workingSetStoreCommon) Commit() error {
	_dbBatchSizelMtc.WithLabelValues().Set(float64(store.flusher.KVStoreWithBuffer().Size()))
	return store.flusher.Flush()
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
