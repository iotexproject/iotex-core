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

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

// Protocol defines the protocol to handle multi-chain actions on sub-chain
type Protocol struct {
	chainID      uint32
	mainChainAPI explorer.Explorer
	sf           factory.Factory
}

// NewProtocol constructs a sub-chain protocol on sub-chain
func NewProtocol(chain blockchain.Blockchain, mainChainAPI explorer.Explorer) *Protocol {
	return &Protocol{
		chainID:      chain.ChainID(),
		mainChainAPI: mainChainAPI,
		sf:           chain.GetFactory(),
	}
}

// Handle handles how to mutate the state db given the multi-chain action on sub-chain
func (p *Protocol) Handle(_ context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	switch act := act.(type) {
	case *action.SettleDeposit:
		if err := p.validateDeposit(act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit settlement action")
		}
		if err := p.mutateDeposit(act, sm); err != nil {
			return nil, errors.Wrapf(err, "error when handling deposit settlement action")
		}
	}
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

func (p *Protocol) validateDeposit(deposit *action.SettleDeposit, sm protocol.StateManager) error {
	// Validate main-chain state
	// TODO: this may not be the type safe casting if index is greater than 2^63
	depositsOnMainChain, err := p.mainChainAPI.GetDeposits(int64(p.chainID), int64(deposit.Index()), 1)
	if err != nil {
		return err
	}
	if len(depositsOnMainChain) != 1 {
		return fmt.Errorf("%d deposits found instead of 1", len(depositsOnMainChain))
	}
	depositOnMainChain := depositsOnMainChain[0]
	if depositOnMainChain.Confirmed {
		return fmt.Errorf("deposit %d is already confirmed", deposit.Index())
	}

	// Validate sub-chain state
	var depositIndex DepositIndex
	addr := depositAddress(deposit.Index())
	if sm == nil {
		err = p.sf.State(addr, &depositIndex)
	} else {
		err = sm.State(addr, &depositIndex)
	}
	switch errors.Cause(err) {
	case nil:
		return fmt.Errorf("deposit %d is already settled", deposit.Index())
	case state.ErrStateNotExist:
		return nil
	default:
		return errors.Wrapf(err, "error when loading state of %x", addr)
	}
}

func (p *Protocol) mutateDeposit(deposit *action.SettleDeposit, sm protocol.StateManager) error {
	// Update the deposit index
	depositAddr := depositAddress(deposit.Index())
	var depositIndex DepositIndex
	if err := sm.PutState(depositAddr, &depositIndex); err != nil {
		return err
	}

	// Update the action owner
	owner, err := account.LoadOrCreateAccountState(sm, deposit.Sender(), big.NewInt(0))
	if err != nil {
		return err
	}
	account.SetNonce(deposit, owner)
	ownerPKHash, err := srcAddressPKHash(deposit.Sender())
	if err != nil {
		return err
	}
	if err := sm.PutState(ownerPKHash, owner); err != nil {
		return err
	}

	// Update the deposit recipient
	recipient, err := account.LoadOrCreateAccountState(sm, deposit.Recipient(), big.NewInt(0))
	if err != nil {
		return err
	}
	if err := recipient.AddBalance(deposit.Amount()); err != nil {
		return err
	}
	recipientPKHash, err := srcAddressPKHash(deposit.Recipient())
	if err != nil {
		return err
	}
	return sm.PutState(recipientPKHash, recipient)
}

func depositAddress(index uint64) hash.PKHash {
	return byteutil.BytesTo20B(hash.Hash160b([]byte(fmt.Sprintf("depositToSubChain.%d", index))))
}

func srcAddressPKHash(srcAddr string) (hash.PKHash, error) {
	addr, err := address.IotxAddressToAddress(srcAddr)
	if err != nil {
		return hash.ZeroPKHash, errors.Wrapf(err, "cannot get the public key hash of address %s", srcAddr)
	}
	return byteutil.BytesTo20B(addr.Payload()), nil
}
