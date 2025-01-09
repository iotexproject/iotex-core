// Copyright (c) 2024 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

type stateDBWorkingSetStore struct {
	*workingSetStoreCommon
	readBuffer bool
}

func newStateDBWorkingSetStore(view protocol.View, flusher db.KVStoreFlusher, readBuffer bool) workingSetStore {
	return &stateDBWorkingSetStore{
		workingSetStoreCommon: &workingSetStoreCommon{
			flusher: flusher,
			view:    view,
		},
		readBuffer: readBuffer,
	}
}

func (store *stateDBWorkingSetStore) Start(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) Stop(context.Context) error {
	return nil
}

func (store *stateDBWorkingSetStore) Get(ns string, key []byte) ([]byte, error) {
	data, err := store.flusher.KVStoreWithBuffer().Get(ns, key)
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return nil, errors.Wrapf(state.ErrStateNotExist, "failed to get state of ns = %x and key = %x", ns, key)
		}
		return nil, err
	}
	return data, nil
}

func (store *stateDBWorkingSetStore) States(ns string, keys [][]byte) ([][]byte, [][]byte, error) {
	if store.readBuffer {
		// TODO: after the 180 HF, we can revert readBuffer, and always go this case
		return readStates(store.flusher.KVStoreWithBuffer(), ns, keys)
	}
	return readStates(store.flusher.BaseKVStore(), ns, keys)
}

func (store *stateDBWorkingSetStore) Finalize(_ context.Context, height uint64) error {
	// Persist current chain Height
	store.flusher.KVStoreWithBuffer().MustPut(
		AccountKVNamespace,
		[]byte(CurrentHeightKey),
		byteutil.Uint64ToBytes(height),
	)
	return nil
}

func (store *stateDBWorkingSetStore) Close() {}
