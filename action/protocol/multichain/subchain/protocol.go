// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package subchain

import (
	"context"
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state/factory"
)

// ProtocolID is the protocol ID
// TODO: it works only for one instance per protocol definition now
const ProtocolID = "multi-chain_sub-chain"

// Protocol defines the protocol to handle multi-chain actions on sub-chain
type Protocol struct {
	chainID uint32
	// TODO: explorer dependency deleted at #1085, need to add api related params
	sf factory.Factory
}

// NewProtocol constructs a sub-chain protocol on sub-chain
func NewProtocol(chain blockchain.Blockchain) *Protocol {
	return &Protocol{
		chainID: chain.ChainID(),
		sf:      chain.GetFactory(),
	}
}

// Handle handles how to mutate the state db given the multi-chain action on sub-chain
func (p *Protocol) Handle(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.SettleDeposit:
		if err := p.validateDeposit(act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit settlement action")
		}
		if err := p.mutateDeposit(ctx, act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit settlement action")
		}
	}
	// TODO: consider add receipt later
	return nil, nil
}

// Validate validates the multi-chain action on sub-chain
func (p *Protocol) Validate(_ context.Context, act action.Action) error {
	switch act := act.(type) {
	case *action.SettleDeposit:
		if err := p.validateDeposit(act, nil); err != nil {
			return errors.Wrapf(err, "error when validating deposit settlement action")
		}
	}
	return nil
}

// ReadState read the state on blockchain via protocol
func (p *Protocol) ReadState(context.Context, protocol.StateManager, []byte, ...[]byte) ([]byte, error) {
	return nil, protocol.ErrUnimplemented
}

func (p *Protocol) validateDeposit(deposit *action.SettleDeposit, sm protocol.StateManager) error {
	// Validate main-chain state
	// TODO: explorer dependency deleted at #1085, need to revive by migrating to api
	return nil
}

func (p *Protocol) mutateDeposit(ctx context.Context, deposit *action.SettleDeposit, sm protocol.StateManager) error {
	raCtx := protocol.MustGetRunActionsCtx(ctx)

	// Update the deposit index
	depositAddr := depositAddress(deposit.Index())
	var depositIndex DepositIndex
	if err := sm.PutState(depositAddr, &depositIndex); err != nil {
		return err
	}

	// Update the action owner
	owner, err := accountutil.LoadOrCreateAccount(sm, raCtx.Caller.String(), big.NewInt(0))
	if err != nil {
		return err
	}
	accountutil.SetNonce(deposit, owner)
	if err := accountutil.StoreAccount(sm, raCtx.Caller.String(), owner); err != nil {
		return err
	}

	// Update the deposit recipient
	recipient, err := accountutil.LoadOrCreateAccount(sm, deposit.Recipient(), big.NewInt(0))
	if err != nil {
		return err
	}
	if err := recipient.AddBalance(deposit.Amount()); err != nil {
		return err
	}
	return accountutil.StoreAccount(sm, deposit.Recipient(), recipient)
}

func depositAddress(index uint64) hash.Hash160 {
	return hash.Hash160b([]byte(fmt.Sprintf("depositToSubChain.%d", index)))
}
