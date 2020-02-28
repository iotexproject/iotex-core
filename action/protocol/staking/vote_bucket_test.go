// Copyright (c) 2020 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
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
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
)

const (
	stateDBPath = "stateDB.test"
)

func newMockKVStore(ctrl *gomock.Controller) db.KVStore {
	kv := db.NewMockKVStore(ctrl)
	kmap := make(map[hash.Hash160][]byte)
	vmap := make(map[hash.Hash160][]byte)

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
			h := hash.Hash160b(append([]byte(ns), k...))
			key := make([]byte, len(k))
			copy(key, k)
			value := make([]byte, len(v))
			copy(value, v)
			kmap[h] = key
			vmap[h] = value
			return nil
		},
	).AnyTimes()
	kv.EXPECT().Get(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, k []byte) ([]byte, error) {
			h := hash.Hash160b(append([]byte(ns), k...))
			v, ok := vmap[h]
			if ok {
				return v, nil
			}
			return nil, db.ErrNotExist
		},
	).AnyTimes()
	kv.EXPECT().Delete(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, k []byte) error {
			h := hash.Hash160b(append([]byte(ns), k...))
			delete(kmap, h)
			delete(vmap, h)
			return nil
		},
	).AnyTimes()
	kv.EXPECT().WriteBatch(gomock.Any()).Return(nil).AnyTimes()
	var fk, fv [][]byte
	kv.EXPECT().Filter(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ns string, cond db.Condition) ([][]byte, [][]byte, error) {
			for h, k := range kmap {
				v := vmap[h]
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
	kv := newMockKVStore(ctrl)
	sm.EXPECT().State(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
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
	sm.EXPECT().PutState(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(
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

	return sm
}

func TestGetPutStaking(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)
	sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(factory.StakingNameSpace),
		protocol.KeyOption(factory.TotalBucketKey),
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
		_, err := getBucket(sm, addr, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))

		vb := NewVoteBucket(addr, identityset.Address(1), big.NewInt(2100000000), 21*uint32(e.index+1), time.Now(), true)

		count, err := getTotalBucketCount(sm)
		require.NoError(err)
		require.Equal(e.index, count)
		require.NoError(putBucket(sm, addr, vb))
		count, err = getTotalBucketCount(sm)
		require.NoError(err)
		require.Equal(e.index+1, count)
		vb1, err := getBucket(sm, addr, e.index)
		require.NoError(err)
		require.Equal(vb, vb1)
		require.Equal(vb.Owner, vb1.Owner)
	}

	// delete buckets and get
	for _, e := range tests {
		addr, _ := address.FromBytes(e.name[:])
		require.NoError(delBucket(sm, addr, e.index))
		_, err := getBucket(sm, addr, e.index)
		require.Equal(state.ErrStateNotExist, errors.Cause(err))
	}
}
