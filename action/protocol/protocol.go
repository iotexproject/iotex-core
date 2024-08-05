// Copyright (c) 2019 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/pkg/log"
)

var (
	// ErrUnimplemented indicates a method is not implemented yet
	ErrUnimplemented = errors.New("method is unimplemented")
)

const (
	// SystemNamespace is the namespace to store system information such as candidates/probationList/unproductiveDelegates
	SystemNamespace = "System"
)

// Protocol defines the protocol interfaces atop IoTeX blockchain
type Protocol interface {
	ActionHandler
	ReadState(context.Context, StateReader, []byte, ...[]byte) ([]byte, uint64, error)
	Register(*Registry) error
	ForceRegister(*Registry) error
	Name() string
}

// Starter starts the protocol
type Starter interface {
	Start(context.Context, StateReader) (interface{}, error)
}

// GenesisStateCreator creates some genesis states
type GenesisStateCreator interface {
	CreateGenesisStates(context.Context, StateManager) error
}

// PreStatesCreator creates preliminary states for state manager
type PreStatesCreator interface {
	CreatePreStates(context.Context, StateManager) error
}

// PreCommitter performs pre-commit action of the protocol
type PreCommitter interface {
	PreCommit(context.Context, StateManager) error
}

// Committer performs commit action of the protocol
type Committer interface {
	Commit(context.Context, StateManager) error
}

// PostSystemActionsCreator creates a list of system actions to be appended to block actions
type PostSystemActionsCreator interface {
	CreatePostSystemActions(context.Context, StateReader) ([]action.Envelope, error)
}

// ActionValidator is the interface of validating an action
type ActionValidator interface {
	Validate(context.Context, action.Action, StateReader) error
}

// ActionHandler is the interface for the action handlers. For each incoming action, the assembled actions will be
// called one by one to process it. ActionHandler implementation is supposed to parse the sub-type of the action to
// decide if it wants to handle this action or not.
type ActionHandler interface {
	Handle(context.Context, action.Action, StateManager) (*action.Receipt, error)
}

type (
	Options struct {
		ValueBigInt *big.Int
	}

	Option func(*Options)
)

func BurnGasOption(burnAmount *big.Int) Option {
	return func(opts *Options) {
		opts.ValueBigInt = burnAmount
	}
}

// DepositGas deposits gas to rewarding pool and burns baseFee
type DepositGas func(context.Context, StateManager, *big.Int, ...Option) ([]*action.TransactionLog, error)

// View stores the view for all protocols
type View map[string]interface{}

func (view View) Read(name string) (interface{}, error) {
	if v, hit := view[name]; hit {
		return v, nil
	}
	return nil, ErrNoName
}

func (view View) Write(name string, v interface{}) error {
	view[name] = v
	return nil
}

// HashStringToAddress generates the contract address from the protocolID of each protocol
func HashStringToAddress(str string) address.Address {
	h := hash.Hash160b([]byte(str))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of account protocol", zap.Error(err))
	}
	return addr
}

func SplitGas(ctx context.Context, tx action.TxDynamicGas, usedGas uint64) (*big.Int, *big.Int, error) {
	var (
		baseFee = MustGetBlockchainCtx(ctx).Tip.BaseFee
		gas     = new(big.Int).SetUint64(usedGas)
	)
	priority, err := action.EffectiveGasTip(tx, baseFee)
	if err != nil {
		return nil, nil, err
	}
	if baseFee == nil {
		return priority.Mul(priority, gas), nil, nil
	}
	// after enabling EIP-1559, fee is split into 2 parts
	// priority fee goes to the rewarding pool (or block producer) as before
	// base fee will be burnt
	base := new(big.Int).Set(baseFee)
	return priority.Mul(priority, gas), base.Mul(base, gas), nil
}
