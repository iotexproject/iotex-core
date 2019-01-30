// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package producer

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

// Protocol defines the protocol of block producer fund operation and block producer rewarding process.
type Protocol struct {
	admin address.Address
}

// NewProtocol instantiates a block producer protocol instance
func NewProtocol(admin address.Address) *Protocol {
	return &Protocol{
		admin: admin,
	}
}

// Handle handles the actions on the block producer protocol
func (p *Protocol) Handle(
	ctx context.Context,
	act action.Action,
	sm protocol.StateManager,
) (*action.Receipt, error) {
	return nil, nil
}

// Validate validates the actions on the block producer protocol
func (p *Protocol) Validate(
	ctx context.Context,
	act action.Action,
) error {
	vaCtx, ok := protocol.GetValidateActionsCtx(ctx)
	if !ok {
		return errors.New("miss action validation context")
	}
	switch act := act.(type) {
	case *SetBlockReward:
		if err := p.assertAdminPermission(vaCtx); err != nil {
			return err
		}
		if err := p.assertAmount(act.Amount()); err != nil {
			return err
		}
	case *SetEpochReward:
		if err := p.assertAdminPermission(vaCtx); err != nil {
			return err
		}
		if err := p.assertAmount(act.Amount()); err != nil {
			return err
		}
	}
	return nil
}

func (p *Protocol) Donate(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
	data []byte,
) error {
	return nil
}

func (p *Protocol) TotalBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	return nil, nil
}

func (p *Protocol) AvailableBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	return nil, nil
}

func (p *Protocol) Claim(
	ctx context.Context,
	sm protocol.StateManager,
	amount *big.Int,
) error {
	return nil
}

func (p *Protocol) UnclaimedBalance(
	ctx context.Context,
	sm protocol.StateManager,
) (*big.Int, error) {
	return nil, nil
}

func (p *Protocol) SettleBlockReward(
	ctx context.Context,
	sm protocol.StateManager) error {
	return nil
}

func (p *Protocol) SettleEpochReward(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	return nil
}
