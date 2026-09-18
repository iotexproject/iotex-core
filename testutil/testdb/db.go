package testdb

import (
	"bytes"
	"context"
	"sort"

	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// NewMockKVStore returns a in memory KVStore.
func NewMockKVStore(ctrl *gomock.Controller) db.KVStore {
	kv := db.NewMockKVStore(ctrl)
	kmap := make(map[string]map[hash.Hash160][]byte)
	vmap := make(map[string]map[hash.Hash160][]byte)

	kv.EXPECT().Start(gomock.Any()).Return(nil).AnyTimes()
	kv.EXPECT().Stop(gomock.Any()).DoAndReturn(
		func(ctx context.Context) error {
			kmap = nil
			vmap = nil
			return nil
		},
	).AnyTimes()
	kv.EXPECT().Put(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, k []byte, v []byte) error {
			kns, ok := kmap[ns]
			if !ok {
				kns = make(map[hash.Hash160][]byte)
				kmap[ns] = kns
			}
			vns, ok := vmap[ns]
			if !ok {
				vns = make(map[hash.Hash160][]byte)
				vmap[ns] = vns
			}
			h := hash.Hash160b(k)
			key := make([]byte, len(k))
			copy(key, k)
			value := make([]byte, len(v))
			copy(value, v)
			kns[h] = key
			vns[h] = value
			return nil
		},
	).AnyTimes()
	kv.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, k []byte) ([]byte, error) {
			vns, ok := vmap[ns]
			if !ok {
				return nil, db.ErrBucketNotExist
			}
			v, ok := vns[hash.Hash160b(k)]
			if ok {
				return v, nil
			}
			return nil, db.ErrNotExist
		},
	).AnyTimes()
	kv.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, k []byte) error {
			kns, ok := kmap[ns]
			if !ok {
				return db.ErrBucketNotExist
			}
			vns := vmap[ns]
			h := hash.Hash160b(k)
			delete(kns, h)
			delete(vns, h)
			return nil
		},
	).AnyTimes()
	kv.EXPECT().WriteBatch(gomock.Any()).Return(nil).AnyTimes()
	var fk, fv [][]byte
	kv.EXPECT().Filter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, cond db.Condition, minKey, maxKey []byte) ([][]byte, [][]byte, error) {
			// clear filter result
			fk = fk[:0]
			fv = fv[:0]
			kns, ok := kmap[ns]
			if !ok {
				return nil, nil, db.ErrBucketNotExist
			}
			vns := vmap[ns]
			checkMin := len(minKey) > 0
			checkMax := len(maxKey) > 0
			for h, k := range kns {
				if checkMin && bytes.Compare(k, minKey) == -1 {
					continue
				}
				if checkMax && bytes.Compare(k, maxKey) == 1 {
					continue
				}
				v := vns[h]
				if cond(k, v) {
					key := make([]byte, len(k))
					copy(key, k)
					value := make([]byte, len(v))
					copy(value, v)
					fk = append(fk, key)
					fv = append(fv, value)
				}
			}
			return fk, fv, nil
		},
	).AnyTimes()
	return kv
}

// NewMockStateManager returns a in memory StateManager.
func NewMockStateManager(ctrl *gomock.Controller) *mock_chainmanager.MockStateManager {
	sm := NewMockStateManagerWithoutHeightFunc(ctrl)
	sm.EXPECT().Height().Return(uint64(0), nil).AnyTimes()

	return sm
}

// NewMockStateManagerWithoutHeightFunc returns a in memory StateManager without default height function.
func NewMockStateManagerWithoutHeightFunc(ctrl *gomock.Controller) *mock_chainmanager.MockStateManager {
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	kv := NewMockKVStore(ctrl)
	views := protocol.NewViews()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).DoAndReturn(
		func(s interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			value, err := kv.Get(cfg.Namespace, cfg.Key)
			if err != nil {
				return 0, state.ErrStateNotExist
			}
			ss, ok := s.(state.Deserializer)
			if !ok {
				return 0, errors.New("state is not a deserializer")
			}
			return 0, ss.Deserialize(value)
		},
	).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).DoAndReturn(
		func(s interface{}, opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			ss, ok := s.(state.Serializer)
			if !ok {
				return 0, errors.New("state is not a serializer")
			}
			value, err := ss.Serialize()
			if err != nil {
				return 0, err
			}
			return 0, kv.Put(cfg.Namespace, cfg.Key, value)
		},
	).AnyTimes()
	sm.EXPECT().DelState(gomock.Any()).DoAndReturn(
		func(opts ...protocol.StateOption) (uint64, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, err
			}
			return 0, kv.Delete(cfg.Namespace, cfg.Key)
		},
	).AnyTimes()
	sm.EXPECT().States(gomock.Any()).DoAndReturn(
		func(opts ...protocol.StateOption) (uint64, state.Iterator, error) {
			cfg, err := protocol.CreateStateConfig(opts...)
			if err != nil {
				return 0, nil, err
			}
			var fk [][]byte
			var fv [][]byte
			if cfg.Keys == nil {
				// The real state factory serves a bounded States() scan in
				// ascending key order over the half-open interval
				// [RangeMin, RangeMax), then applies Limit. The in-memory KV
				// returns Go-map order, so reproduce the contract here --
				// otherwise a caller that relies on ordering (the IIP-59 shard
				// walk asserts strictly ascending keys) passes against a real
				// database and fails, nondeterministically, against this mock.
				fk, fv, err = kv.Filter(cfg.Namespace, func(k, v []byte) bool {
					if cfg.RangeMin != nil && bytes.Compare(k, cfg.RangeMin) < 0 {
						return false
					}
					if cfg.RangeMax != nil && bytes.Compare(k, cfg.RangeMax) >= 0 {
						return false
					}
					return true
				}, nil, nil)
				if err != nil {
					return 0, nil, state.ErrStateNotExist
				}
				fk, fv = sortByKeyAscending(fk, fv)
				if cfg.Limit > 0 && len(fk) > cfg.Limit {
					fk, fv = fk[:cfg.Limit], fv[:cfg.Limit]
				}
			} else {
				for _, key := range cfg.Keys {
					value, err := kv.Get(cfg.Namespace, key)
					switch errors.Cause(err) {
					case db.ErrNotExist, db.ErrBucketNotExist:
						fv = append(fv, nil)
						fk = append(fk, key)
					case nil:
						fv = append(fv, value)
						fk = append(fk, key)
					default:
						return 0, nil, err
					}
				}
			}
			iter, err := state.NewIterator(fk, fv)
			if err != nil {
				return 0, nil, err
			}
			return 0, iter, nil
		},
	).AnyTimes()
	// sm.EXPECT().Height().Return(uint64(0), nil).AnyTimes()
	sm.EXPECT().ReadView(gomock.Any()).DoAndReturn(
		func(name string) (interface{}, error) {
			if v, err := views.Read(name); err == nil {
				return v, nil
			}
			return nil, protocol.ErrNoName
		},
	).AnyTimes()
	sm.EXPECT().WriteView(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, v protocol.View) error {
			views.Write(name, v)
			return nil
		},
	).AnyTimes()
	// use Snapshot() to simulate workingset.Reset()
	sm.EXPECT().Snapshot().DoAndReturn(
		func() int {
			return 0
		},
	).AnyTimes()
	// Deliberately no default Revert() expectation here, unlike Snapshot()
	// above. gomock resolves a call against the first *unexhausted* matching
	// expectation in declaration order, and one installed by this constructor
	// is declared before anything a test writes -- an AnyTimes() Revert here
	// would swallow every call and leave a test's own
	// `EXPECT().Revert(...).Times(1)` permanently unmet. Tests that drive a
	// handler down its rollback path declare their own; AllowRevert is the
	// shorthand for the ones that only need the call not to abort.

	return sm
}

// AllowRevert accepts any number of Revert calls on sm and does nothing.
//
// This mock keeps no undo log, so it could not roll state back even if it
// wanted to: what the expectation buys is the ability to drive a handler down
// a failure path that reverts without gomock aborting on an unexpected call.
// No test may depend on revert semantics through it.
//
// Call it only from tests that assert nothing about Revert. Because gomock
// matches expectations in declaration order, this one shadows any narrower
// Revert expectation declared after it.
func AllowRevert(sm *mock_chainmanager.MockStateManager) {
	sm.EXPECT().Revert(gomock.Any()).Return(nil).AnyTimes()
}

// sortByKeyAscending reorders a (keys, values) pair into ascending key order,
// keeping the two slices aligned.
func sortByKeyAscending(keys [][]byte, values [][]byte) ([][]byte, [][]byte) {
	order := make([]int, len(keys))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return bytes.Compare(keys[order[a]], keys[order[b]]) < 0
	})
	sortedKeys := make([][]byte, len(keys))
	sortedValues := make([][]byte, len(values))
	for i, j := range order {
		sortedKeys[i] = keys[j]
		if j < len(values) {
			sortedValues[i] = values[j]
		}
	}
	return sortedKeys, sortedValues
}
