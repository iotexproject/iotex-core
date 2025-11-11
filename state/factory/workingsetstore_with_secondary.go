package factory

import (
	"bytes"
	"context"
	"strings"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
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
	writer           workingSetStore
	writerSecondary  workingSetStore
	snMap            map[int]int
	consistencyCheck bool
}

func newWorkingSetStoreWithSecondary(store workingSetStore, erigonStore workingSetStore, consistencyCheck bool) *workingSetStoreWithSecondary {
	return &workingSetStoreWithSecondary{
		reader:           store,
		writer:           store,
		writerSecondary:  erigonStore,
		snMap:            make(map[int]int),
		consistencyCheck: consistencyCheck,
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
	return store.GetObjectWithValidate(ns, key, key, obj)
}

func (store *workingSetStoreWithSecondary) GetObjectWithValidate(ns string, key, erigonKey []byte, obj any) error {
	if !store.consistencyCheck {
		return store.reader.GetObject(ns, key, obj)
	}
	// consistency check between two stores
	cco, ok := obj.(interface {
		New() any
		ConsistentEqual(other any) bool
	})
	if !ok {
		return store.reader.GetObject(ns, key, obj)
	}
	other := cco.New()
	err := store.reader.GetObject(ns, key, obj)
	errOther := store.writerSecondary.GetObject(ns, erigonKey, other)
	if errors.Cause(err) != errors.Cause(errOther) {
		prefix := state.PollCandidatesPrefix
		if ns == state.AccountKVNamespace && errors.Cause(err) == state.ErrStateNotExist && strings.HasPrefix(string(erigonKey), prefix) {
			// special case for legacy candidate list
			return err
		}
		log.S().Panicf("inconsistent existence %T for ns %s key %x: %v vs %v", obj, ns, key, err, errOther)
		return errors.Errorf("inconsistent existence %T for ns %s key %x: %v vs %v", obj, ns, key, err, errOther)
	}
	if err != nil {
		return err
	}
	if !cco.ConsistentEqual(other) {
		log.S().Panicf("inconsistent object for ns %s key %x: %T vs %T", ns, key, cco, other)
		return errors.Errorf("inconsistent object for ns %s key %x: %+v vs %+v", ns, key, cco, other)
	}
	return nil
}

func (store *workingSetStoreWithSecondary) States(ns string, obj any, keys [][]byte) (state.Iterator, error) {
	iter, err := store.reader.States(ns, obj, keys)
	if !store.consistencyCheck {
		return iter, err
	}
	cco, ok := obj.(interface {
		New() any
		ConsistentEqual(other any) bool
	})
	if !ok {
		return iter, err
	}
	otherIter, errOther := store.writerSecondary.States(ns, obj, keys)
	if errors.Cause(err) != errors.Cause(errOther) {
		log.S().Panicf("inconsistent existence for ns %s keys %x: %v vs %v", ns, keys, err, errOther)
		return nil, errors.Errorf("inconsistent existence for ns %s keys %x: %v vs %v", ns, keys, err, errOther)
	}
	if err != nil {
		return iter, err
	}
	var (
		_keys, otherKeys [][]byte
		objs, otherObjs  []any
	)
	for {
		newObj := cco.New()
		key, iterErr := iter.Next(newObj)
		if iterErr != nil {
			break
		}
		objs = append(objs, newObj)
		_keys = append(_keys, key)
	}
	for {
		newObj := cco.New()
		key, iterErr := otherIter.Next(newObj)
		if iterErr != nil {
			break
		}
		otherObjs = append(otherObjs, newObj)
		otherKeys = append(otherKeys, key)
	}
	if len(objs) != len(otherObjs) {
		log.S().Panicf("inconsistent number of objects for ns %s keys %x: %d vs %d", ns, _keys, len(objs), len(otherObjs))
		return nil, errors.Errorf("inconsistent number of objects for ns %s keys %x: %d vs %d", ns, keys, len(objs), len(otherObjs))
	}
	for i := range objs {
		if !bytes.Equal(_keys[i], otherKeys[i]) {
			log.S().Panicf("inconsistent keys for ns %s: %x vs %x", ns, _keys[i], otherKeys[i])
			return nil, errors.Errorf("inconsistent keys for ns %s: %x vs %x", ns, _keys[i], otherKeys[i])
		}
		cco := objs[i].(interface {
			ConsistentEqual(other any) bool
		})
		if !cco.ConsistentEqual(otherObjs[i]) {
			log.S().Panicf("inconsistent object for ns %s key %x: %+v vs %+v", ns, _keys[i], objs[i], otherObjs[i])
			return nil, errors.Errorf("inconsistent object for ns %s key %x: %+v vs %+v", ns, _keys[i], objs[i], otherObjs[i])
		}
	}
	return store.reader.States(ns, obj, keys)
}

func (store *workingSetStoreWithSecondary) KVStore() db.KVStore {
	return nil
}

func (store *workingSetStoreWithSecondary) ErigonStore() (any, error) {
	return store.writerSecondary, nil
}
