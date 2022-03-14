package testdb

import (
	"bytes"
	"context"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
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
func NewMockStateManager(ctrl *gomock.Controller) protocol.StateManager {
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	var h uint64
	kv := NewMockKVStore(ctrl)
	dk := protocol.NewDock()
	view := protocol.View{}
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
			var fv [][]byte
			if cfg.Keys == nil {
				_, fv, err = kv.Filter(cfg.Namespace, func(k, v []byte) bool {
					return true
				}, nil, nil)
				if err != nil {
					return 0, nil, state.ErrStateNotExist
				}
			} else {
				for _, key := range cfg.Keys {
					value, err := kv.Get(cfg.Namespace, key)
					switch errors.Cause(err) {
					case db.ErrNotExist, db.ErrBucketNotExist:
						fv = append(fv, nil)
					case nil:
						fv = append(fv, value)
					default:
						return 0, nil, err
					}
				}
			}
			return 0, state.NewIterator(fv), nil
		},
	).AnyTimes()
	sm.EXPECT().Height().DoAndReturn(
		func() (uint64, error) {
			return h, nil
		},
	).AnyTimes()
	sm.EXPECT().Load(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns, key string, v interface{}) error {
			return dk.Load(ns, key, v)
		},
	).AnyTimes()
	sm.EXPECT().Unload(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns, key string, v interface{}) error {
			return dk.Unload(ns, key, v)
		},
	).AnyTimes()
	sm.EXPECT().ReadView(gomock.Any()).DoAndReturn(
		func(name string) (interface{}, error) {
			if v, hit := view[name]; hit {
				return v, nil
			}
			return nil, protocol.ErrNoName
		},
	).AnyTimes()
	sm.EXPECT().WriteView(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, v interface{}) error {
			view[name] = v
			return nil
		},
	).AnyTimes()
	// use Snapshot() to simulate workingset.Reset()
	sm.EXPECT().Snapshot().DoAndReturn(
		func() int {
			dk.Reset()
			return 0
		},
	).AnyTimes()

	return sm
}
