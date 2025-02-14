package factory

import (
	"context"
	"fmt"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/execution/evm"
)

var (
	perfMtc = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "iotex_factory_perf",
			Help: "iotex factory perf",
		},
		[]string{"topic"},
	)
)

type reader interface {
	Get(string, []byte) ([]byte, error)
	States(string, [][]byte) ([][]byte, [][]byte, error)
	Digest() hash.Hash256
	ReadView(string) (interface{}, error)
}

type writer interface {
	WriteView(name string, value interface{}) error
	Put(ns string, key []byte, value []byte) error
	Delete(ns string, key []byte) error
	Snapshot() int
	RevertSnapshot(snapshot int) error
	ResetSnapshots()
}

// treat erigon as 3rd output, still read from statedb
// it's used for PutBlock, generating historical states in erigon
type stateDBWorkingSetStoreWithErigonOutput struct {
	reader
	store       *stateDBWorkingSetStore
	erigonStore *erigonStore
	snMap       map[int]int
}

func init() {
}

func newStateDBWorkingSetStoreWithErigonOutput(store *stateDBWorkingSetStore, erigonStore *erigonStore) *stateDBWorkingSetStoreWithErigonOutput {
	return &stateDBWorkingSetStoreWithErigonOutput{
		reader:      store,
		store:       store,
		erigonStore: erigonStore,
		snMap:       make(map[int]int),
	}
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Stop(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Finalize(ctx context.Context, height uint64) error {
	if err := store.store.Finalize(ctx, height); err != nil {
		return err
	}
	return store.erigonStore.finalize(ctx, height, uint64(protocol.MustGetBlockCtx(ctx).BlockTimeStamp.Unix()))
}

func (store *stateDBWorkingSetStoreWithErigonOutput) FinalizeTx(ctx context.Context) error {
	return store.erigonStore.finalizeTx(ctx)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) WriteView(name string, value interface{}) error {
	return store.store.WriteView(name, value)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Put(ns string, key []byte, value []byte) error {
	if err := store.store.Put(ns, key, value); err != nil {
		return err
	}
	return store.erigonStore.put(ns, key, value)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Delete(ns string, key []byte) error {
	// delete won't happen in account and contract
	return store.store.Delete(ns, key)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Commit(ctx context.Context) error {
	t1 := time.Now()
	if err := store.store.Commit(ctx); err != nil {
		return err
	}
	perfMtc.WithLabelValues("commitstore").Set(float64(time.Since(t1).Nanoseconds()))
	t2 := time.Now()
	err := store.erigonStore.commit(ctx)
	perfMtc.WithLabelValues("commiterigon").Set(float64(time.Since(t2).Nanoseconds()))
	return err
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Snapshot() int {
	sn := store.store.Snapshot()
	isn := store.erigonStore.snapshot()
	store.snMap[sn] = isn
	return sn
}

func (store *stateDBWorkingSetStoreWithErigonOutput) RevertSnapshot(sn int) error {
	store.store.RevertSnapshot(sn)
	if isn, ok := store.snMap[sn]; ok {
		store.erigonStore.revertToSnapshot(isn)
		delete(store.snMap, sn)
	} else {
		panic(fmt.Sprintf("no isn for sn %d", sn))
	}
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonOutput) ResetSnapshots() {
	store.store.ResetSnapshots()
	store.snMap = make(map[int]int)
}

func (store *stateDBWorkingSetStoreWithErigonOutput) Close() {
	store.erigonStore.rollback()
}

// used in historical states query
// account & contract read & write on erigon
type stateDBWorkingSetStoreWithErigonDryrun struct {
	writer
	store       *stateDBWorkingSetStore // fallback to statedb for staking, rewarding and poll
	erigonStore *erigonStore
}

func newStateDBWorkingSetStoreWithErigonDryrun(store *stateDBWorkingSetStore, erigonStore *erigonStore) *stateDBWorkingSetStoreWithErigonDryrun {
	return &stateDBWorkingSetStoreWithErigonDryrun{
		store:       store,
		erigonStore: erigonStore,
		writer:      newStateDBWorkingSetStoreWithErigonOutput(store, erigonStore),
	}
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Stop(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Get(ns string, key []byte) ([]byte, error) {
	switch ns {
	case AccountKVNamespace, evm.CodeKVNameSpace:
		return store.erigonStore.get(ns, key)
	default:
		return store.store.Get(ns, key)
	}
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) States(ns string, keys [][]byte) ([][]byte, [][]byte, error) {
	// currently only used for staking & poll, no need to read from erigon
	return store.store.States(ns, keys)
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Finalize(_ context.Context, height uint64) error {
	// do nothing for dryrun
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) FinalizeTx(ctx context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) ReadView(name string) (interface{}, error) {
	// only used for staking
	return store.store.ReadView(name)
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Digest() hash.Hash256 {
	return store.store.Digest()
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Commit(context.Context) error {
	// do nothing for dryrun
	return nil
}

func (store *stateDBWorkingSetStoreWithErigonDryrun) Close() {
	store.erigonStore.rollback()
}

// func (store *stateDBWorkingSetStoreWithErigonDryrun) WriteView(name string, value interface{}) error {
// 	return store.outer.WriteView(name, value)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) Put(ns string, key []byte, value []byte) error {
// 	return store.outer.Put(ns, key, value)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) Delete(ns string, key []byte) error {
// 	return store.outer.Delete(ns, key)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) Snapshot() int {
// 	return store.outer.Snapshot()
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) RevertSnapshot(sn int) error {
// 	return store.outer.RevertSnapshot(sn)
// }

// func (store *stateDBWorkingSetStoreWithErigonDryrun) ResetSnapshots() {
// 	store.outer.ResetSnapshots()
// }
