// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"strings"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state"
)

var (
	// ErrUnimplemented indicates a method is not implemented yet
	ErrUnimplemented = errors.New("method is unimplemented")
)

// Protocol defines the protocol interfaces atop IoTeX blockchain
type Protocol interface {
	ActionValidator
	ActionHandler
	ReadState(context.Context, StateManager, []byte, ...[]byte) ([]byte, error)
}

// ActionValidator is the interface of validating an action
type ActionValidator interface {
	Validate(context.Context, action.Action) error
}

// ActionEnvelopeValidator is the interface of validating an action
type ActionEnvelopeValidator interface {
	Validate(context.Context, action.SealedEnvelope) error
}

// ActionHandler is the interface for the action handlers. For each incoming action, the assembled actions will be
// called one by one to process it. ActionHandler implementation is supposed to parse the sub-type of the action to
// decide if it wants to handle this action or not.
type ActionHandler interface {
	Handle(context.Context, action.Action, StateManager) (*action.Receipt, error)
}

// ChainManager defines the blockchain interface
type ChainManager interface {
	// ChainID returns the chain ID
	ChainID() uint32
	// CandidatesByHeight returns the candidate list by a given height
	CandidatesByHeight(height uint64) ([]*state.Candidate, error)
	// ProductivityByEpoch returns the number of produced blocks per delegate in an epoch
	ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error)
	// SimulateExecution simulates a running of smart contract operation
	SimulateExecution(caller address.Address, ex *action.Execution) ([]byte, *action.Receipt, error)
}

// StateManager defines the state DB interface atop IoTeX blockchain
type StateManager interface {
	// Accounts
	Height() uint64
	Snapshot() int
	Revert(int) error
	// General state
	State(hash.Hash160, interface{}) error
	PutState(hash.Hash160, interface{}) error
	DelState(pkHash hash.Hash160) error
	GetDB() db.KVStore
	GetCachedBatch() db.CachedBatch
}

// MockChainManager mocks ChainManager interface
type MockChainManager struct {
}

// Nonce mocks base method
func (m *MockChainManager) Nonce(addr string) (uint64, error) {
	if strings.EqualFold("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", addr) {
		return 0, errors.New("MockChainManager nonce error")
	}
	return 2, nil
}

// ChainID return chain ID
func (m *MockChainManager) ChainID() uint32 {
	return 0
}

// CandidatesByHeight returns the candidate list by a given height
func (m *MockChainManager) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	return nil, nil
}

// ProductivityByEpoch returns the number of produced blocks per delegate in an epoch
func (m *MockChainManager) ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error) {
	return 0, nil, nil
}

// SimulateExecution simulates a running of smart contract operation
func (m *MockChainManager) SimulateExecution(caller address.Address, ex *action.Execution) ([]byte, *action.Receipt, error) {
	return nil, nil, nil
}
