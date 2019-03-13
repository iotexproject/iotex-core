// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package mainchain

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/address"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
)

// DepositAddress returns the deposit address (20-byte)
func DepositAddress(subChainAddr []byte, depositIndex uint64) hash.Hash160 {
	var stream []byte
	stream = append(stream, subChainAddr...)
	stream = append(stream, []byte(".deposit.")...)
	temp := make([]byte, 8)
	enc.MachineEndian.PutUint64(temp, depositIndex)
	stream = append(stream, temp...)
	return hash.Hash160b(stream)
}

// Deposit returns the deposit record
func (p *Protocol) Deposit(subChainAddr address.Address, depositIndex uint64) (*Deposit, error) {
	key := DepositAddress(subChainAddr.Bytes(), depositIndex)
	var deposit Deposit
	if err := p.sf.State(key, &deposit); err != nil {
		return nil, errors.Wrapf(err, "error when loading state of %x", key)
	}
	return &deposit, nil
}

func (p *Protocol) handleDeposit(
	ctx context.Context,
	deposit *action.CreateDeposit,
	sm protocol.StateManager,
) (*action.Receipt, error) {
	raCtx := protocol.MustGetRunActionsCtx(ctx)
	account, subChainInOp, err := p.validateDeposit(raCtx.Caller, deposit, sm)
	if err != nil {
		return nil, err
	}
	return p.mutateDeposit(raCtx.Caller, deposit, account, subChainInOp, sm)
}

func (p *Protocol) validateDeposit(
	caller address.Address,
	deposit *action.CreateDeposit,
	sm protocol.StateManager,
) (*state.Account, InOperation, error) {
	cost, err := deposit.Cost()
	if err != nil {
		return nil, InOperation{}, errors.Wrap(err, "error when getting deposit's cost")
	}
	account, err := p.accountWithEnoughBalance(caller.String(), cost, sm)
	if err != nil {
		return nil, InOperation{}, err
	}
	subChainsInOp, err := p.subChainsInOperation(sm)
	if err != nil {
		return nil, InOperation{}, err
	}
	inOp, ok := subChainsInOp.Get(deposit.ChainID())
	if !ok {
		return nil, InOperation{}, errors.Errorf("address %s is not on a sub-chain in operation", deposit.Recipient())
	}
	return account, inOp, nil
}

func (p *Protocol) mutateDeposit(
	caller address.Address,
	deposit *action.CreateDeposit,
	acct *state.Account,
	subChainInOp InOperation,
	sm protocol.StateManager,
) (*action.Receipt, error) {
	// Subtract the balance from sender account
	acct.Balance = big.NewInt(0).Sub(acct.Balance, deposit.Amount())
	// TODO: this is not right, but currently the actions in a block is not processed according to the nonce
	accountutil.SetNonce(deposit, acct)
	if err := accountutil.StoreAccount(sm, caller.String(), acct); err != nil {
		return nil, err
	}

	// Update sub-chain state
	addr, err := address.FromBytes(subChainInOp.Addr)
	if err != nil {
		return nil, err
	}
	subChain, err := p.SubChain(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "error when getting the state of sub-chain %d", subChain.ChainID)
	}
	depositIndex := subChain.DepositCount
	subChain.DepositCount++
	if err := sm.PutState(hash.BytesToHash160(addr.Bytes()), subChain); err != nil {
		return nil, err
	}

	// Insert deposit state
	recipient, err := address.FromString(deposit.Recipient())
	if err != nil {
		return nil, err
	}
	if err := sm.PutState(
		DepositAddress(subChainInOp.Addr, depositIndex),
		&Deposit{
			Amount:    deposit.Amount(),
			Addr:      recipient.Bytes(),
			Confirmed: false,
		},
	); err != nil {
		return nil, err
	}

	var value [8]byte
	enc.MachineEndian.PutUint64(value[:], depositIndex)
	gas, err := deposit.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	receipt := action.Receipt{
		ReturnValue:     value[:],
		Status:          0,
		ActHash:         deposit.Hash(),
		GasConsumed:     gas,
		ContractAddress: addr.String(),
	}
	return &receipt, nil
}
