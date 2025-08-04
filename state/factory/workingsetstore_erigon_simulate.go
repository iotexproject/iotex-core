package factory

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
)

// erigonWorkingSetStoreForSimulate is a working set store that uses erigon as the main store
// and falls back to the statedb for certain operations. It is used for simulating transactions
// without actually committing them to the erigon store.
type erigonWorkingSetStoreForSimulate struct {
	writer
	store       workingSetStore // fallback to statedb for staking, rewarding and poll
	erigonStore *erigonWorkingSetStore
}

func newErigonWorkingSetStoreForSimulate(store workingSetStore, erigonStore *erigonWorkingSetStore) *erigonWorkingSetStoreForSimulate {
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
	if _, ok := obj.(ContractStorage); ok {
		return store.erigonStore.GetObject(ns, key, obj)
	}
	if _, ok := obj.(*state.Account); !ok && ns == AccountKVNamespace {
		return store.store.GetObject(ns, key, obj)
	}
	switch ns {
	case AccountKVNamespace, evm.CodeKVNameSpace:
		return store.erigonStore.GetObject(ns, key, obj)
	default:
		return store.store.GetObject(ns, key, obj)
	}
}

func (store *erigonWorkingSetStoreForSimulate) Get(ns string, key []byte) ([]byte, error) {
	switch ns {
	case AccountKVNamespace, evm.CodeKVNameSpace:
		return store.erigonStore.Get(ns, key)
	default:
		return store.store.Get(ns, key)
	}
}

func (store *erigonWorkingSetStoreForSimulate) States(ns string, keys [][]byte, obj any) ([][]byte, [][]byte, error) {
	if _, ok := obj.(ContractStorage); ok {
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

func (store *erigonWorkingSetStoreForSimulate) Filter(ns string, cond db.Condition, start, limit []byte) ([][]byte, [][]byte, error) {
	return store.store.Filter(ns, cond, start, limit)
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
