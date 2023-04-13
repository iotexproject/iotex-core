// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
)

type (
	bucketStateManager struct {
		protocol.StateManager
		config bucketStateConfig
	}

	bucketStateConfig struct {
		bucketNamespace  string
		totalBucketKey   []byte
		bucketKeygenFunc func(*VoteBucket) []byte
	}
)

var _executorBucketStateConfig = bucketStateConfig{
	bucketNamespace: _executionStakingNameSpace,
	totalBucketKey:  TotalBucketKey,
	bucketKeygenFunc: func(bucket *VoteBucket) []byte {
		return bucketKey(bucket.Index)
	},
}

func newExecutorBucketStateManager(sm protocol.StateManager) *bucketStateManager {
	return &bucketStateManager{
		StateManager: sm,
		config:       _executorBucketStateConfig,
	}
}

func (bsm *bucketStateManager) add(bucket *VoteBucket) (uint64, error) {
	// Get total bucket count
	var tc totalBucketCount
	if _, err := bsm.State(
		&tc,
		protocol.NamespaceOption(bsm.config.bucketNamespace),
		protocol.KeyOption(bsm.config.totalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}
	// Add index inside bucket and put
	index := tc.Count()
	bucket.Index = index
	if _, err := bsm.PutState(
		bucket,
		protocol.NamespaceOption(bsm.config.bucketNamespace),
		protocol.KeyOption(bsm.config.bucketKeygenFunc(bucket))); err != nil {
		return 0, err
	}
	// Update total bucket count
	tc.count++
	_, err := bsm.PutState(
		&tc,
		protocol.NamespaceOption(bsm.config.bucketNamespace),
		protocol.KeyOption(bsm.config.totalBucketKey))
	return index, err
}
