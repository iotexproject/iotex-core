// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// protocolID is the protocol ID
const protocolID = "staking"

// Protocol defines the protocol of handling staking
type Protocol struct {
	addr address.Address
}

// NewProtocol instantiates the protocol of staking
func NewProtocol() *Protocol {
	h := hash.Hash160b([]byte(protocolID))
	addr, err := address.FromBytes(h[:])
	if err != nil {
		log.L().Panic("Error when constructing the address of staking protocol", zap.Error(err))
	}

	return &Protocol{addr: addr}
}

// Handle handles a staking message
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.StakeCreate:
		return p.handleStakeCreate(ctx, act, sm)
	case *action.StakeUnstake:
		return p.handleStakeUnstake(ctx, act, sm)
	case *action.StakeWithdraw:
		return p.handleStakeWithdraw(ctx, act, sm)
	case *action.StakeChangeCandidate:
		return p.handleStakeChangeCandidate(ctx, act, sm)
	case *action.StakeTransferOwnership:
		return p.handleStakeTransferOwnership(ctx, act, sm)
	case *action.StakeAdd:
		return p.handleStakeAddDeposit(ctx, act, sm)
	case *action.StakeAgain:
		return p.handleStakeRestake(ctx, act, sm)
	}
	return nil, nil
}

// Validate validates a staking message
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	//TODO
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateReader, []byte, ...[]byte) ([]byte, error) {
	//TODO
	return nil, protocol.ErrUnimplemented
}

// Register registers the protocol with a unique ID
func (p *Protocol) Register(r *protocol.Registry) error {
	return r.Register(protocolID, p)
}

// ForceRegister registers the protocol with a unique ID and force replacing the previous protocol if it exists
func (p *Protocol) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(protocolID, p)
}

func (p *Protocol) handleStakeCreate(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleStakeUnstake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleStakeWithdraw(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleStakeChangeCandidate(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleStakeTransferOwnership(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleStakeAddDeposit(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleStakeRestake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}
