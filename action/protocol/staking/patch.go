// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	_metaNS     = "meta"
	_nameNS     = "name"
	_operatorNS = "operator"
	_start      = "start"
	_tip        = "tip"
)

var ErrPatchNotInitialized = errors.New("patch has not been initialized yet")

// Patch is the patch of staking protocol
type Patch struct {
	lock        sync.RWMutex
	initialized bool
	startHeight uint64
	tipHeight   uint64
	store       db.KVStore
}

// NewPatch creates a new staking patch with a kvstore
func NewPatch(store db.KVStore) *Patch {
	return &Patch{
		initialized: false,
		store:       store,
	}
}

func (patch *Patch) read(ns string, height uint64) (CandidateList, error) {
	data, err := patch.store.Get(_nameNS, byteutil.Uint64ToBytes(height))
	switch errors.Cause(err) {
	case db.ErrNotExist:
		return nil, nil
	case nil:
		var list CandidateList
		if err := list.Deserialize(data); err != nil {
			return nil, err
		}
		return list, nil
	default:
		return nil, err
	}
}

// Start starts the patch
func (patch *Patch) Start(ctx context.Context) error {
	patch.lock.Lock()
	defer patch.lock.Unlock()

	if err := patch.store.Start(ctx); err != nil {
		return err
	}
	startHeight, err := patch.store.Get(_metaNS, []byte(_start))
	switch errors.Cause(err) {
	case db.ErrBucketNotExist, db.ErrNotExist:
	case nil:
		patch.initialized = true
		patch.startHeight = byteutil.BytesToUint64(startHeight)
	default:
		return err
	}
	tipHeight, err := patch.store.Get(_metaNS, []byte(_tip))
	switch errors.Cause(err) {
	case db.ErrBucketNotExist, db.ErrNotExist:
		if patch.initialized {
			return errors.New("tip height is not set")
		}
	case nil:
		if !patch.initialized {
			return errors.New("invalid tip height")
		}
		patch.tipHeight = byteutil.BytesToUint64(tipHeight)
	default:
		return err
	}

	return nil
}

// Stop stops the patch
func (patch *Patch) Stop(ctx context.Context) error {
	patch.lock.Lock()
	defer patch.lock.Unlock()

	return patch.store.Stop(ctx)
}

// Info returns the start height and the tip height
func (patch *Patch) Info() (uint64, uint64, error) {
	patch.lock.RLock()
	defer patch.lock.RUnlock()
	if !patch.initialized {
		return 0, 0, ErrPatchNotInitialized
	}

	return patch.startHeight, patch.tipHeight, nil
}

// Read reads CandidateList by name and CandidateList by operator of given height
func (patch *Patch) Read(height uint64) (CandidateList, CandidateList, error) {
	patch.lock.RLock()
	defer patch.lock.RUnlock()
	if !patch.initialized {
		return nil, nil, ErrPatchNotInitialized
	}
	if height < patch.startHeight || height > patch.tipHeight {
		return nil, nil, errors.Errorf("%d is out of range [%d, %d]", height, patch.startHeight, patch.tipHeight)
	}
	for h := height; h >= patch.startHeight; h-- {
		listByName, err := patch.read(_nameNS, h)
		if err != nil {
			return nil, nil, err
		}
		if listByName == nil {
			continue
		}
		listByOperator, err := patch.read(_operatorNS, h)
		if err != nil {
			return nil, nil, err
		}
		if listByOperator == nil {
			return nil, nil, errors.Errorf("data corrupted at %d", h)
		}
		return listByName, listByOperator, nil
	}

	return nil, nil, errors.Errorf("failed to read data of %d", height)
}

// Write writes CandidateList by name and CandidateList by operator into store
func (patch *Patch) Write(height uint64, listByName, listByOperator CandidateList) error {
	patch.lock.Lock()
	defer patch.lock.Unlock()
	b := batch.NewBatch()
	switch {
	case !patch.initialized:
		b.Put(_metaNS, []byte(_start), byteutil.Uint64ToBytes(height), "failed to write start height")
	case height < patch.startHeight:
		return errors.Errorf("new height %d is lower than start height %d", height, patch.startHeight)
	case height < patch.tipHeight:
		return errors.Errorf("new tip height %d is lower than current tip %d", height, patch.tipHeight)
	}
	b.Put(_metaNS, []byte(_tip), byteutil.Uint64ToBytes(height), "failed to write tip height")
	bytesByName, err := listByName.Serialize()
	if err != nil {
		return err
	}
	b.Put(_nameNS, byteutil.Uint64ToBytes(height), bytesByName, "failed to write candidate list by name")
	bytesByOperator, err := listByOperator.Serialize()
	if err != nil {
		return err
	}
	b.Put(_operatorNS, byteutil.Uint64ToBytes(height), bytesByOperator, "failed to write candidate list by operator")
	if err := patch.store.WriteBatch(b); err != nil {
		return err
	}
	switch {
	case !patch.initialized:
		patch.startHeight = height
		patch.tipHeight = height
		patch.initialized = true
	case patch.tipHeight < height:
		patch.tipHeight = height
	}

	return nil
}
