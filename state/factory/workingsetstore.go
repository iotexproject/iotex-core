// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
)

type (
	workingSetStore interface {
		db.KVStoreBasic
		GetDirty(string, []byte) ([]byte, bool)
		Commit() error
		States(string, [][]byte) ([][]byte, [][]byte, error)
		Digest() hash.Hash256
		Finalize(uint64) error
		Snapshot() int
		RevertSnapshot(int) error
		ResetSnapshots()
		ReadView(string) (interface{}, error)
		WriteView(string, interface{}) error
	}
	workingSetStoreCommon struct {
		view    protocol.View
		flusher db.KVStoreFlusher
	}
)

func (store *workingSetStoreCommon) ReadView(name string) (interface{}, error) {
	return store.view.Read(name)
}

func (store *workingSetStoreCommon) WriteView(name string, value interface{}) error {
	return store.view.Write(name, value)
}

func (store *workingSetStoreCommon) GetDirty(ns string, key []byte) ([]byte, bool) {
	return store.flusher.KVStoreWithBuffer().GetDirty(ns, key)
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
