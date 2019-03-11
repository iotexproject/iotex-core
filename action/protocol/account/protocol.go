// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/address"
)

// ProtocolID is the protocol ID
// TODO: it works only for one instance per protocol definition now
const ProtocolID = "account"

// Protocol defines the protocol of handling account
type Protocol struct{}

// NewProtocol instantiates the protocol of account
func NewProtocol() *Protocol { return &Protocol{} }

// Handle handles an account
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.Transfer:
		return p.handleTransfer(ctx, act, sm)
	}
	return nil, nil
}

// Validate validates an account
func (p *Protocol) Validate(ctx context.Context, act action.Action) error {
	switch act := act.(type) {
	case *action.Transfer:
		if err := p.validateTransfer(ctx, act); err != nil {
			return errors.Wrap(err, "error when validating transfer action")
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

// Initialize initializes the protocol by setting the initial balances to some addresses
func (p *Protocol) Initialize(
	ctx context.Context,
	sm protocol.StateManager,
	addrs []address.Address,
	amounts []*big.Int,
) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	if err := p.assertZeroBlockHeight(raCtx.BlockHeight); err != nil {
		return err
	}
	if err := p.assertEqualLength(addrs, amounts); err != nil {
		return err
	}
	if err := p.assertAmounts(amounts); err != nil {
		return err
	}
	for i, addr := range addrs {
		if _, err := accountutil.LoadOrCreateAccount(sm, addr.String(), amounts[i]); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) assertZeroBlockHeight(height uint64) error {
	if height != 0 {
		return errors.Errorf("current block height %d is not zero", height)
	}
	return nil
}

func (p *Protocol) assertEqualLength(addrs []address.Address, amounts []*big.Int) error {
	if len(addrs) != len(amounts) {
		return errors.Errorf(
			"address slice length %d and amounts slice length %d don't match",
			len(addrs),
			len(amounts),
		)
	}
	return nil
}

func (p *Protocol) assertAmounts(amounts []*big.Int) error {
	for _, amount := range amounts {
		if amount.Cmp(big.NewInt(0)) >= 0 {
			return nil
		}
		return errors.Errorf("account amount %s shouldn't be negative", amount.String())
	}
	return nil
}
