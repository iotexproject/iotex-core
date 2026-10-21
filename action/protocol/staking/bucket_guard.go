// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/v2/state"
)

type (
	// BucketGuard is the on-chain QSB guard record for a native bucket.
	BucketGuard = bucketguard.Guard

	// BucketGuardStateManager reads and writes QSB guard records.
	BucketGuardStateManager struct {
		protocol.StateManager
		*BucketGuardStateReader
	}

	// BucketGuardStateReader reads QSB guard records.
	BucketGuardStateReader struct {
		protocol.StateReader
	}
)

func bucketGuardKey(index uint64) []byte {
	return append([]byte{_bucketGuard}, byteutil.Uint64ToBytesBigEndian(index)...)
}

// NewBucketGuardStateManager creates a guard state manager.
func NewBucketGuardStateManager(sm protocol.StateManager) *BucketGuardStateManager {
	return &BucketGuardStateManager{
		StateManager:           sm,
		BucketGuardStateReader: NewBucketGuardStateReader(sm),
	}
}

// Put writes a guard record for a bucket.
func (bsm *BucketGuardStateManager) Put(index uint64, guard *BucketGuard) error {
	if guard == nil {
		return errors.New("nil bucket guard")
	}
	_, err := bsm.PutState(guard, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(bucketGuardKey(index)))
	return err
}

// Delete removes a guard record, including a tombstone.
func (bsm *BucketGuardStateManager) Delete(index uint64) error {
	_, err := bsm.DelState(
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(bucketGuardKey(index)),
		protocol.ObjectOption(&BucketGuard{}),
	)
	return err
}

// NewBucketGuardStateReader creates a guard state reader.
func NewBucketGuardStateReader(sr protocol.StateReader) *BucketGuardStateReader {
	return &BucketGuardStateReader{StateReader: sr}
}

// Get returns the guard record for a bucket. An absent record returns nil, nil.
func (bsr *BucketGuardStateReader) Get(index uint64) (*BucketGuard, error) {
	guard := BucketGuard{}
	if _, err := bsr.State(&guard, protocol.NamespaceOption(_stakingNameSpace), protocol.KeyOption(bucketGuardKey(index))); err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			return nil, nil
		}
		return nil, err
	}
	return &guard, nil
}
