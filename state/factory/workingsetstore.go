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
		PutObject(ns string, key []byte, object any) (err error)
		GetObject(ns string, key []byte, object any) error
		DeleteObject(ns string, key []byte, object any) error
		// States reads a set of states. keys selects specific keys (nil means "all
		// keys in ns"); scan, when non-nil, instead asks for an ordered, bounded
		// range scan. The two are mutually exclusive and validated by the caller.
		States(ns string, object any, keys [][]byte, scan *db.RangeScan) (state.Iterator, error)
		Commit(context.Context, uint64) error
		Digest() hash.Hash256
		Finalize(context.Context) error
		FinalizeTx(context.Context) error
		Snapshot() int
		RevertSnapshot(int) error
		ResetSnapshots()
		Close()
		CreateGenesisStates(context.Context) error
		ErigonStore() (any, error)
	}

	stateDBWorkingSetStore struct {
		lock sync.Mutex
		// TODO: handle committed flag properly in the functions
		committed  bool
		readBuffer bool
		flusher    db.KVStoreFlusher
	}
)

// KVStore() hands this store out as the base KVStore of a derived working set, so
// it has to keep answering ordered range scans. Assert it at compile time rather
// than discovering it from a failed type assertion at block-production time.
var _ db.KVStoreWithRangeScan = (*stateDBWorkingSetStore)(nil)

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

func (store *stateDBWorkingSetStore) PutObject(ns string, key []byte, obj any) error {
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
	if err := store.flusher.KVStoreWithBuffer().Put(ns, key, value); err != nil {
		return errors.Wrap(err, "failed to put value")
	}
	if !store.committed {
		return nil
	}
	return store.flusher.Flush()
}

func (store *stateDBWorkingSetStore) DeleteObject(ns string, key []byte, obj any) error {
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

func (store *stateDBWorkingSetStore) GetObject(ns string, key []byte, obj any) error {
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

// statesKVStore picks which KVStore a read goes to. States() and ScanRange() MUST
// share it: ScanRange exists so that this store can sit in the base-store slot of a
// derived working set's flusher (see (*workingSet).NewWorkingSet), and a range scan
// that read a different source than States would answer the same question two ways
// depending on how the caller got here -- which is a state divergence, not a
// performance detail.
func (store *stateDBWorkingSetStore) statesKVStore() db.KVStore {
	if store.readBuffer {
		// TODO: after the 180 HF, we can revert readBuffer, and always go this case
		return store.flusher.KVStoreWithBuffer()
	}
	return store.flusher.BaseKVStore()
}

func (store *stateDBWorkingSetStore) States(ns string, obj any, keys [][]byte, scan *db.RangeScan) (state.Iterator, error) {
	keys, values, err := readStates(store.statesKVStore(), ns, keys, scan)
	if err != nil {
		return nil, err
	}
	return state.NewIterator(keys, values)
}

// ScanRange makes *stateDBWorkingSetStore a db.KVStoreWithRangeScan.
//
// This is required, not optional. KVStore() returns the store itself, so when
// (*stateDB).Mint lags the chain tip and derives the next working set from the
// cached parent one, this store becomes the base KVStore of the child's
// kvStoreWithBuffer. Without this method the child's type assertion to
// db.KVStoreWithRangeScan fails and every range scan on the proposer's working set
// errors, while validators -- who build their working set on a committed DAO --
// answer it fine. Same block, two different results: a fork.
//
// Semantics are the KVStoreWithRangeScan contract in full; they are inherited
// unchanged from whichever store statesKVStore() selects, which is the same source
// States() reads.
func (store *stateDBWorkingSetStore) ScanRange(ns string, min, max []byte, limit int) ([][]byte, [][]byte, error) {
	kvStore := store.statesKVStore()
	scanner, ok := kvStore.(db.KVStoreWithRangeScan)
	if !ok {
		// Deliberately not softened to an empty result. This is a node-local
		// capability fact, not chain state: it can differ between the proposer and
		// the validators of the same block, so an empty answer here would turn a
		// build/config problem into a divergence.
		return nil, nil, errors.Wrapf(db.ErrNotSupported, "kvstore %T does not support ScanRange", kvStore)
	}
	return scanner.ScanRange(ns, min, max, limit)
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

func (store *stateDBWorkingSetStore) ErigonStore() (any, error) {
	return nil, errors.Wrap(state.ErrErigonStoreNotSupported, "failed to get erigon store")
}

// CaptureWriteQueue returns a snapshot of all entries in the write queue.
// Must be called BEFORE Commit() which flushes and clears the buffer.
func (store *stateDBWorkingSetStore) CaptureWriteQueue() []WriteQueueEntry {
	kvb := store.flusher.KVStoreWithBuffer()
	size := kvb.Size()
	entries := make([]WriteQueueEntry, 0, size)
	for i := 0; i < size; i++ {
		wi, err := kvb.Entry(i)
		if err != nil {
			continue
		}
		entries = append(entries, WriteQueueEntry{
			WriteType: uint8(wi.WriteType()),
			Namespace: wi.Namespace(),
			Key:       append([]byte(nil), wi.Key()...),
			Value:     append([]byte(nil), wi.Value()...),
		})
	}
	return entries
}

// WriteQueueEntry is a captured state mutation from the write queue.
type WriteQueueEntry struct {
	WriteType uint8 // 0=Put, 1=Delete
	Namespace string
	Key       []byte
	Value     []byte
}
