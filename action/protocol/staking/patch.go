// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

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

// Patch is the patch of staking protocol
type Patch struct {
	startHeight uint64
	endHeight   uint64
	store       db.KVStore
}

// NewPatch creates a new staking patch with a kvstore
func NewPatch(store db.KVStore) *Patch {
	return &Patch{
		store: store,
	}
}

func (patch *Patch) read(ns string, height uint64) (CandidateList, error) {
	data, err := patch.store.Get(ns, byteutil.Uint64ToBytesBigEndian(height))
	switch errors.Cause(err) {
	case db.ErrNotExist:
		// return nil to indicate need to continue reading at diff heights
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
	return patch.store.Start(ctx)
}

// Stop stops the patch
func (patch *Patch) Stop(ctx context.Context) error {
	return patch.store.Stop(ctx)
}

// WriteStartHeight writes the start height
func (patch *Patch) WriteStartHeight(height uint64) error {
	return patch.store.Put(_metaNS, []byte(_start), byteutil.Uint64ToBytesBigEndian(height))
}

// Info returns the start height and the tip height
func (patch *Patch) Info() (uint64, uint64, error) {
	startHeight, err := patch.store.Get(_metaNS, []byte(_start))
	if err != nil {
		return 0, 0, err
	}
	tipHeight, err := patch.store.Get(_metaNS, []byte(_tip))
	if err != nil {
		return 0, 0, err
	}
	return byteutil.BytesToUint64(startHeight), byteutil.BytesToUint64(tipHeight), nil
}

// Read reads CandidateList by name and CandidateList by operator of given height
func (patch *Patch) Read(height uint64) (CandidateList, CandidateList, error) {
	if height < patch.startHeight || height > patch.endHeight {
		return nil, nil, errors.Errorf("%d is out of range [%d, %d]", height, patch.startHeight, patch.endHeight)
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
	b := batch.NewBatch()
	bytesByName, err := listByName.Serialize()
	if err != nil {
		return err
	}
	b.Put(_nameNS, byteutil.Uint64ToBytesBigEndian(height), bytesByName, "failed to write candidate list by name")
	bytesByOperator, err := listByOperator.Serialize()
	if err != nil {
		return err
	}
	b.Put(_operatorNS, byteutil.Uint64ToBytesBigEndian(height), bytesByOperator, "failed to write candidate list by operator")
	b.Put(_metaNS, []byte(_tip), byteutil.Uint64ToBytesBigEndian(height), "failed to write tip height")

	return patch.store.WriteBatch(b)
}
