// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

var (
	// ErrUnimplemented indicates a method is not implemented yet
	ErrUnimplemented = errors.New("method is unimplemented")
)

// Protocol defines the protocol interfaces atop IoTeX blockchain
type Protocol interface {
	ActionValidator
	ActionHandler
	ReadState(context.Context, StateReader, []byte, ...[]byte) ([]byte, error)
	Register(*Registry) error
	ForceRegister(*Registry) error
}

// GenesisStateCreator creates some genesis states
type GenesisStateCreator interface {
	CreateGenesisStates(context.Context, StateManager) error
}

// PreStatesCreator creates state manager
type PreStatesCreator interface {
	CreatePreStates(context.Context, StateManager) error
}

// PostSystemActionsCreator creates a list of system actions to be appended to block actions
type PostSystemActionsCreator interface {
	CreatePostSystemActions(context.Context) ([]action.Envelope, error)
}

// ActionValidator is the interface of validating an action
type ActionValidator interface {
	Validate(context.Context, action.Action) error
}

// ActionEnvelopeValidator is the interface of validating an SealedEnvelope action
type ActionEnvelopeValidator interface {
	Validate(context.Context, action.SealedEnvelope) error
}

// ActionHandler is the interface for the action handlers. For each incoming action, the assembled actions will be
// called one by one to process it. ActionHandler implementation is supposed to parse the sub-type of the action to
// decide if it wants to handle this action or not.
type ActionHandler interface {
	Handle(context.Context, action.Action, StateManager) (*action.Receipt, error)
}
