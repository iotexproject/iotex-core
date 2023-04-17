// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
)

type (
	bucketStateManager struct {
		protocol.StateManager
		config      *bucketStateConfig
		totalAmount *totalAmount
	}

	bucketStateConfig struct {
		bucketNamespace   string
		totalBucketKey    []byte
		bucketPoolAddrKey []byte
		bucketKeygenFunc  func(*VoteBucket) []byte
		bucketPoolAddr    string
	}

	bucketViewData struct {
		totalAmount *totalAmount
	}
)

var (
	_executorBucketStateConfig = bucketStateConfig{
		bucketNamespace:   _executionStakingNameSpace,
		totalBucketKey:    TotalBucketKey,
		bucketPoolAddrKey: _bucketPoolAddrKey,
		bucketKeygenFunc: func(bucket *VoteBucket) []byte {
			return bucketKey(bucket.Index)
		},
		// TODO (iip-16): change to the another address, separate from the delegate bucket pool address
		bucketPoolAddr: address.StakingBucketPoolAddr,
	}
)

func newExecutorBucketStateManager(sm protocol.StateManager) (*bucketStateManager, error) {
	bsm := &bucketStateManager{
		StateManager: sm,
		config:       &_executorBucketStateConfig,
		totalAmount: &totalAmount{
			amount: big.NewInt(0),
		},
	}

	view, err := readView(sm)
	if err == nil {
		bsm.totalAmount = view.ebsmView.totalAmount
	} else if total, err := loadBucketPoolFromStateDB(sm, bsm.config); err != nil {
		return nil, err
	} else {
		bsm.totalAmount = total
	}

	return bsm, nil
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
	if _, err := bsm.PutState(
		&tc,
		protocol.NamespaceOption(bsm.config.bucketNamespace),
		protocol.KeyOption(bsm.config.totalBucketKey)); err != nil {
		return 0, err
	}

	return index, nil
}

func (bsm *bucketStateManager) depositPool(amount *big.Int, newBucket bool) error {
	bsm.totalAmount.AddBalance(amount, newBucket)
	if _, err := bsm.PutState(bsm.totalAmount, protocol.NamespaceOption(bsm.config.bucketNamespace), protocol.KeyOption(bsm.config.bucketPoolAddrKey)); err != nil {
		return err
	}
	return nil
}

func (bsm *bucketStateManager) view() *bucketViewData {
	return &bucketViewData{
		totalAmount: bsm.totalAmount,
	}
}

func createExecutionBucketView(sr protocol.StateReader) (*bucketViewData, error) {
	return createBucketView(sr, &_executorBucketStateConfig)
}

func createBucketView(sr protocol.StateReader, config *bucketStateConfig) (*bucketViewData, error) {
	totalAmount, err := loadBucketPoolFromStateDB(sr, config)
	if err != nil {
		return nil, err
	}
	return &bucketViewData{
		totalAmount: totalAmount,
	}, nil
}

func loadBucketPoolFromStateDB(sr protocol.StateReader, config *bucketStateConfig) (*totalAmount, error) {
	ta := totalAmount{
		amount: big.NewInt(0),
	}
	_, err := sr.State(
		&ta,
		protocol.NamespaceOption(config.bucketNamespace),
		protocol.KeyOption(config.bucketPoolAddrKey),
	)
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return &ta, nil
		}
		return nil, err
	}
	return &ta, err
}
