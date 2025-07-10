package factory

import (
	"context"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/log"
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
	GetFromStateDB(string, []byte) ([]byte, error)
	States(string, [][]byte) ([][]byte, [][]byte, error)
	Digest() hash.Hash256
	Filter(string, db.Condition, []byte, []byte) ([][]byte, [][]byte, error)
}

type writer interface {
	Put(ns string, key []byte, value []byte) error
	Delete(ns string, key []byte) error
	Snapshot() int
	RevertSnapshot(snapshot int) error
	ResetSnapshots()
	WriteBatch(batch.KVStoreBatch) error
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

func (store *workingSetStoreWithSecondary) WriteBatch(batch batch.KVStoreBatch) error {
	if err := store.writer.WriteBatch(batch); err != nil {
		return err
	}
	return store.writerSecondary.WriteBatch(batch)
}

func (store *workingSetStoreWithSecondary) Put(ns string, key []byte, value []byte) error {
	if err := store.writer.Put(ns, key, value); err != nil {
		return err
	}
	return store.writerSecondary.Put(ns, key, value)
}

func (store *workingSetStoreWithSecondary) Delete(ns string, key []byte) error {
	if err := store.writer.Delete(ns, key); err != nil {
		return err
	}
	return store.writerSecondary.Delete(ns, key)
}

func (store *workingSetStoreWithSecondary) Commit(ctx context.Context, retention uint64) error {
	t1 := time.Now()
	if err := store.writer.Commit(ctx, retention); err != nil {
		return err
	}
	perfMtc.WithLabelValues("commitstore").Set(float64(time.Since(t1).Nanoseconds()))
	t2 := time.Now()
	err := store.writerSecondary.Commit(ctx, retention)
	perfMtc.WithLabelValues("commiterigon").Set(float64(time.Since(t2).Nanoseconds()))
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
