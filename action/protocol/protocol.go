// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/hash"
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
	// GetChainID returns the chain ID
	ChainID() uint32
	// GetHashByHeight returns Block's hash by height
	GetHashByHeight(height uint64) (hash.Hash256, error)
	// StateByAddr returns account of a given address
	StateByAddr(address string) (*state.Account, error)
	// Nonce returns the nonce if the account exists
	Nonce(addr string) (uint64, error)
	// CandidatesByHeight returns the candidate list by a given height
	CandidatesByHeight(height uint64) ([]*state.Candidate, error)
	// ProductivityByEpoch returns the number of produced blocks per delegate in an epoch
	ProductivityByEpoch(epochNum uint64) (uint64, map[string]uint64, error)
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
