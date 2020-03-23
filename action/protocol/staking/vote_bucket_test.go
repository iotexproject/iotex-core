// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

const (
	stateDBPath = "stateDB.test"
)

func newMockKVStore(ctrl *gomock.Controller) db.KVStore {
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
			vns, _ := vmap[ns]
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
			vns, _ := vmap[ns]
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

func newMockStateManager(ctrl *gomock.Controller) protocol.StateManager {
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	var h uint64
	kv := newMockKVStore(ctrl)
	dk := protocol.NewDock()
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
	sm.EXPECT().DelState(gomock.Any(), gomock.Any()).DoAndReturn(
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
			if cfg.Cond == nil {
				cfg.Cond = func(k, v []byte) bool {
					return true
				}
			}
			_, fv, err := kv.Filter(cfg.Namespace, cfg.Cond, cfg.MinKey, cfg.MaxKey)
			if err != nil {
				return 0, nil, state.ErrStateNotExist
			}
			return 0, state.NewIterator(fv), nil
		},
	).AnyTimes()
	sm.EXPECT().ConfirmedHeight().DoAndReturn(
		func() uint64 {
			return h
		},
	).AnyTimes()
	sm.EXPECT().Load(gomock.Any(), gomock.Any()).DoAndReturn(
		func(name string, v interface{}) error {
			return dk.Load(name, v)
		},
	).AnyTimes()
	sm.EXPECT().Unload(gomock.Any()).DoAndReturn(
		func(name string) (interface{}, error) {
			return dk.Unload(name)
		},
	).AnyTimes()
	sm.EXPECT().Push().DoAndReturn(
		func() error {
			return dk.Push()
		},
	).AnyTimes()

	return sm
}

func TestGetPutStaking(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)
	sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)

	tests := []struct {
		name  hash.Hash160
		index uint64
	}{
		{
			hash.BytesToHash160([]byte{1, 2, 3, 4}),
			0,
		},
		{
			hash.BytesToHash160([]byte{1, 2, 3, 4}),
			1,
		},
		{
			hash.BytesToHash160([]byte{2, 3, 4, 5}),
			2,
		},
		{
			hash.BytesToHash160([]byte{2, 3, 4, 5}),
			3,
		},
	}

	// put buckets and get
	for _, e := range tests {
		addr, _ := address.FromBytes(e.name[:])
		_, err := getBucket(sm, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		vb := NewVoteBucket(addr, identityset.Address(1), big.NewInt(2100000000), 21*uint32(e.index+1), time.Now(), true)

		count, err := getTotalBucketCount(sm)
		require.NoError(err)
		require.Equal(e.index, count)
		count, err = putBucket(sm, vb)
		require.NoError(err)
		require.Equal(e.index, count)
		count, err = getTotalBucketCount(sm)
		require.NoError(err)
		require.Equal(e.index+1, count)
		vb1, err := getBucket(sm, e.index)
		require.NoError(err)
		require.Equal(e.index, vb1.Index)
		require.Equal(vb, vb1)
	}

	vb, err := getBucket(sm, 2)
	require.NoError(err)
	vb.AutoStake = false
	vb.StakedAmount.Sub(vb.StakedAmount, big.NewInt(100))
	require.NoError(updateBucket(sm, 2, vb))
	vb1, err := getBucket(sm, 2)
	require.NoError(err)
	require.Equal(vb, vb1)

	// delete buckets and get
	for _, e := range tests {
		require.NoError(delBucket(sm, e.index))
		_, err := getBucket(sm, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
