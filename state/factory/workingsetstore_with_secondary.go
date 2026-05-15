package factory

import (
	"context"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
	"github.com/iotexproject/iotex-core/v2/state"
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
	// Get(string, []byte) ([]byte, error)
	GetObject(string, []byte, any) error
	States(string, any, [][]byte) (state.Iterator, error)
	Digest() hash.Hash256
	// Filter(string, db.Condition, []byte, []byte) ([][]byte, [][]byte, error)
}

type writer interface {
	PutObject(ns string, key []byte, obj any) error
	DeleteObject(ns string, key []byte, obj any) error
	Snapshot() int
	RevertSnapshot(snapshot int) error
	ResetSnapshots()
	CreateGenesisStates(context.Context) error
}

// treat erigon as 3rd output, still read from statedb
// it's used for PutBlock, generating historical states in erigon
type workingSetStoreWithSecondary struct {
	reader
	writer          workingSetStore
	writerSecondary workingSetStore
	snMap           map[int]int
}

func newWorkingSetStoreWithSecondary(store workingSetStore, erigonStore workingSetStore) *workingSetStoreWithSecondary {
	return &workingSetStoreWithSecondary{
		reader:          store,
		writer:          store,
		writerSecondary: erigonStore,
		snMap:           make(map[int]int),
	}
}

func (store *workingSetStoreWithSecondary) Start(context.Context) error {
	return nil
}

func (store *workingSetStoreWithSecondary) Stop(context.Context) error {
	return nil
}

func (store *workingSetStoreWithSecondary) Finalize(ctx context.Context) error {
	if err := store.writer.Finalize(ctx); err != nil {
		return err
	}
	return store.writerSecondary.Finalize(ctx)
}

func (store *workingSetStoreWithSecondary) FinalizeTx(ctx context.Context) error {
	if err := store.writer.FinalizeTx(ctx); err != nil {
		return err
	}
	return store.writerSecondary.FinalizeTx(ctx)
}

func (store *workingSetStoreWithSecondary) PutObject(ns string, key []byte, obj any) error {
	if err := store.writer.PutObject(ns, key, obj); err != nil {
		return err
	}
	return store.writerSecondary.PutObject(ns, key, obj)
}

func (store *workingSetStoreWithSecondary) DeleteObject(ns string, key []byte, obj any) error {
	if err := store.writer.DeleteObject(ns, key, obj); err != nil {
		return err
	}
	return store.writerSecondary.DeleteObject(ns, key, obj)
}

func (store *workingSetStoreWithSecondary) Commit(ctx context.Context, retention uint64) error {
	// Commit to secondary store first, then commit to main store
	// This ensures that if the secondary store fails, the main store is not committed
	// the main store can be committed independently, but the secondary store cannot
	t1 := time.Now()
	if err := store.writerSecondary.Commit(ctx, retention); err != nil {
		return err
	}
	perfMtc.WithLabelValues("commiterigon").Set(float64(time.Since(t1).Nanoseconds()))
	t2 := time.Now()
	err := store.writer.Commit(ctx, retention)
	perfMtc.WithLabelValues("commitstore").Set(float64(time.Since(t2).Nanoseconds()))
	return err
}

func (store *workingSetStoreWithSecondary) Snapshot() int {
	sn := store.writer.Snapshot()
	isn := store.writerSecondary.Snapshot()
	store.snMap[sn] = isn
	return sn
}

func (store *workingSetStoreWithSecondary) RevertSnapshot(sn int) error {
	if err := store.writer.RevertSnapshot(sn); err != nil {
		return err
	}
	if isn, ok := store.snMap[sn]; ok {
		if err := store.writerSecondary.RevertSnapshot(isn); err != nil {
			return err
		}
		delete(store.snMap, sn)
	} else {
		log.S().Panicf("no isn for sn %d", sn)
	}
	return nil
}

func (store *workingSetStoreWithSecondary) ResetSnapshots() {
	store.writer.ResetSnapshots()
	store.writerSecondary.ResetSnapshots()
	store.snMap = make(map[int]int)
}

func (store *workingSetStoreWithSecondary) Close() {
	store.writer.Close()
	store.writerSecondary.Close()
}

func (store *workingSetStoreWithSecondary) CreateGenesisStates(ctx context.Context) error {
	if err := store.writer.CreateGenesisStates(ctx); err != nil {
		return err
	}
	return store.writerSecondary.CreateGenesisStates(ctx)
}

func (store *workingSetStoreWithSecondary) GetObject(ns string, key []byte, obj any) error {
	return store.reader.GetObject(ns, key, obj)
}

func (store *workingSetStoreWithSecondary) States(ns string, obj any, keys [][]byte) (state.Iterator, error) {
	return store.reader.States(ns, obj, keys)
}

func (store *workingSetStoreWithSecondary) KVStore() db.KVStore {
	return nil
}

func (store *workingSetStoreWithSecondary) ErigonStore() (any, error) {
	return store.writerSecondary, nil
}
