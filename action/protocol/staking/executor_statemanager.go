// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
	"github.com/pkg/errors"
)

const (
	// ExecutorTypeProposer is the type of proposer
	ExecutorTypeProposer ExecutorType = iota
)

type (
	// ExecutorType is the type of an executor
	ExecutorType uint32

	// executorStateManager manages the states of executors
	// not thread-safe
	executorStateManager struct {
		protocol.StateManager
		config    executorStateConfig
		executors map[ExecutorType]map[string]*Executor
	}

	executorStateConfig struct {
		// bucket state config
		bucketNamespace  string
		totalBucketKey   []byte
		bucketKeygenFunc func(*VoteBucket) []byte
		// executor state config
		executorNamespace  string
		executorKeygenFunc func(*Executor) []byte
	}

	executorViewData struct {
		executors map[ExecutorType]map[string]*Executor
	}
)

var _executorStateConfig = executorStateConfig{
	bucketNamespace: _executionStakingNameSpace,
	totalBucketKey:  TotalBucketKey,
	bucketKeygenFunc: func(bucket *VoteBucket) []byte {
		return bucketKey(bucket.Index)
	},
	executorNamespace: _executorNameSpace,
	executorKeygenFunc: func(e *Executor) []byte {
		return e.Operator.Bytes()
	},
}

func createExecutorBaseView(sr protocol.StateReader) (*executorViewData, error) {
	executors, err := loadExecutorFromStateDB(sr, &_executorStateConfig)
	if err != nil {
		return nil, err
	}
	return &executorViewData{
		executors: executors,
	}, nil
}

func loadExecutorFromStateDB(sr protocol.StateReader, config *executorStateConfig) (map[ExecutorType]map[string]*Executor, error) {
	executors := make(map[ExecutorType]map[string]*Executor)
	_, iter, err := sr.States(protocol.NamespaceOption(config.executorNamespace))
	if err != nil {
		if errors.Is(err, state.ErrStateNotExist) {
			return executors, nil
		}
		return nil, err
	}
	cands := []*Executor{}
	for i := 0; i < iter.Size(); i++ {
		c := &Executor{}
		if err := iter.Next(c); err != nil {
			return nil, errors.Wrapf(err, "failed to deserialize execution candidate")
		}
		cands = append(cands, c)
	}
	for i := range cands {
		candMap, ok := executors[cands[i].Type]
		if !ok {
			candMap = make(map[string]*Executor)
			executors[cands[i].Type] = candMap
		}
		candMap[cands[i].Operator.String()] = cands[i]
	}
	return executors, nil
}

func newExecutorStateManager(sm protocol.StateManager) (*executorStateManager, error) {
	esm := &executorStateManager{
		StateManager: sm,
		config:       _executorStateConfig,
		executors:    make(map[ExecutorType]map[string]*Executor),
	}
	view, err := readView(sm)
	if err == nil {
		esm.executors = view.esmView.executors
	} else if executors, err := loadExecutorFromStateDB(esm, &esm.config); err != nil {
		return nil, err
	} else {
		esm.executors = executors
	}
	return esm, nil
}

func (esm *executorStateManager) putBucket(bucket *VoteBucket) (uint64, error) {
	// Get total bucket count
	var tc totalBucketCount
	if _, err := esm.State(
		&tc,
		protocol.NamespaceOption(esm.config.bucketNamespace),
		protocol.KeyOption(esm.config.totalBucketKey)); err != nil && errors.Cause(err) != state.ErrStateNotExist {
		return 0, err
	}
	// Add index inside bucket and put
	index := tc.Count()
	bucket.Index = index
	if _, err := esm.PutState(
		bucket,
		protocol.NamespaceOption(esm.config.bucketNamespace),
		protocol.KeyOption(esm.config.bucketKeygenFunc(bucket))); err != nil {
		return 0, err
	}
	// Update total bucket count
	tc.count++
	_, err := esm.PutState(
		&tc,
		protocol.NamespaceOption(esm.config.bucketNamespace),
		protocol.KeyOption(esm.config.totalBucketKey))
	return index, err
}

func (esm *executorStateManager) putExecutor(e *Executor) error {
	candMap, ok := esm.executors[e.Type]
	if !ok {
		candMap = make(map[string]*Executor)
		esm.executors[e.Type] = candMap
	}
	candMap[e.Operator.String()] = e

	_, err := esm.PutState(e, protocol.NamespaceOption(esm.config.executorNamespace), protocol.KeyOption(esm.config.executorKeygenFunc(e)))
	return err
}

func (esm *executorStateManager) getExecutor(etype ExecutorType, operator address.Address) (e *Executor, ok bool) {
	candMap, ok := esm.executors[etype]
	if !ok {
		return nil, false
	}
	e, ok = candMap[operator.String()]
	return
}

func (esm *executorStateManager) view() *executorViewData {
	return &executorViewData{
		executors: esm.executors,
	}
}
