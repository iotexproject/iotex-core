package factory

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state/factory/erigonstore"
)

// erigonWorkingSetStoreForSimulate is a working set store that uses erigon as the main store
// and falls back to the statedb for certain operations. It is used for simulating transactions
// without actually committing them to the erigon store.
type erigonWorkingSetStoreForSimulate struct {
	writer
	store       workingSetStore // fallback to statedb for staking, rewarding and poll
	erigonStore workingSetStore
}

func newErigonWorkingSetStoreForSimulate(store workingSetStore, erigonStore workingSetStore) *erigonWorkingSetStoreForSimulate {
	return &erigonWorkingSetStoreForSimulate{
		store:       store,
		erigonStore: erigonStore,
		writer:      newWorkingSetStoreWithSecondary(store, erigonStore),
	}
}

func (store *erigonWorkingSetStoreForSimulate) Start(context.Context) error {
	return nil
}

func (store *erigonWorkingSetStoreForSimulate) Stop(context.Context) error {
	return nil
}

func (store *erigonWorkingSetStoreForSimulate) GetObject(ns string, key []byte, obj any) error {
	storage := erigonstore.ObjectContractStorage(obj)
	if storage != nil {
		return store.erigonStore.GetObject(ns, key, obj)
	}
	return store.store.GetObject(ns, key, obj)
}

func (store *erigonWorkingSetStoreForSimulate) States(ns string, keys [][]byte, obj any) ([][]byte, [][]byte, error) {
	storage := erigonstore.ObjectContractStorage(obj)
	if storage != nil {
		return store.erigonStore.States(ns, keys, obj)
	}
	// currently only used for staking & poll, no need to read from erigon
	return store.store.States(ns, keys, obj)
}

func (store *erigonWorkingSetStoreForSimulate) Finalize(_ context.Context) error {
	return nil
}

func (store *erigonWorkingSetStoreForSimulate) FinalizeTx(ctx context.Context) error {
	return nil
}

func (store *erigonWorkingSetStoreForSimulate) Digest() hash.Hash256 {
	return store.store.Digest()
}

func (store *erigonWorkingSetStoreForSimulate) Commit(context.Context, uint64) error {
	// do nothing for dryrun
	return nil
}

func (store *erigonWorkingSetStoreForSimulate) Close() {
	store.erigonStore.Close()
}

func (store *erigonWorkingSetStoreForSimulate) CreateGenesisStates(ctx context.Context) error {
	return nil
}

func (store *erigonWorkingSetStoreForSimulate) KVStore() db.KVStore {
	return nil
}
