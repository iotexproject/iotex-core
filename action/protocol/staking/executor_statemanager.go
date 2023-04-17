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
	ExecutorTypeProposer ExecutorType = 1 << iota
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
		executorNamespace  string
		executorKeygenFunc func(*Executor) []byte
	}

	executorViewData struct {
		executors map[ExecutorType]map[string]*Executor
	}
)

var (
	_executorStateConfig = executorStateConfig{
		executorNamespace: _executorNameSpace,
		executorKeygenFunc: func(e *Executor) []byte {
			return e.Operator.Bytes()
		},
	}
)

func createExecutorView(sr protocol.StateReader) (*executorViewData, error) {
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

func (esm *executorStateManager) upsert(e *Executor) error {
	candMap, ok := esm.executors[e.Type]
	if !ok {
		candMap = make(map[string]*Executor)
		esm.executors[e.Type] = candMap
	}
	candMap[e.Operator.String()] = e

	_, err := esm.PutState(e, protocol.NamespaceOption(esm.config.executorNamespace), protocol.KeyOption(esm.config.executorKeygenFunc(e)))
	return err
}

func (esm *executorStateManager) get(etype ExecutorType, operator address.Address) (e *Executor, ok bool) {
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
