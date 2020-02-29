// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func (p *Protocol) handleCreateStake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleUnstake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleWithdrawStake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleChangeCandidate(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleTransferStake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleDepositToStake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleRestake(ctx context.Context, act action.Action, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleCandidateRegister(ctx context.Context, act *action.CandidateRegister, sm protocol.StateManager) (*action.Receipt, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	owner := actCtx.Caller
	if act.OwnerAddress() != nil {
		owner = act.OwnerAddress()
	}

	// owner address is the unique ID for candidate
	if p.inMemCandidates.ContainsOwner(act.OwnerAddress()) {
		return nil, ErrAlreadyExist
	}

	// TODO create self staking bucket
	bucketIdx := uint64(0)

	c := NewCandidate(owner, act.OperatorAddress(), act.RewardAddress(), act.Name(), bucketIdx, act.Amount())
	if err := putCandidate(sm, c.Owner, c); err != nil {
		return nil, err
	}

	if err := p.inMemCandidates.Put(c); err != nil {
		return nil, err
	}

	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actCtx.ActionHash,
		GasConsumed:     actCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}, nil
}

func (p *Protocol) handleCandidateUpdate(ctx context.Context, act *action.CandidateUpdate, sm protocol.StateManager) (*action.Receipt, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	// only owner can update candidate
	if !p.inMemCandidates.ContainsOwner(actCtx.Caller) {
		return nil, ErrInvalidOwner
	}
	c := p.inMemCandidates.GetByOwner(actCtx.Caller)

	// cannot collide with existing name
	if act.Name() != c.Name && p.inMemCandidates.ContainsName(act.Name()) {
		return nil, ErrInvalidCanName
	}

	// cannot collide with existing operator address
	if act.OperatorAddress() != c.Operator && p.inMemCandidates.ContainsOperator(act.OperatorAddress()) {
		return nil, ErrInvalidOperator
	}

	// nothing changes, simply return
	if act.Name() == c.Name && act.OperatorAddress() == c.Operator && act.RewardAddress() == c.Reward {
		return &action.Receipt{
			Status:          uint64(iotextypes.ReceiptStatus_Success),
			BlockHeight:     blkCtx.BlockHeight,
			ActionHash:      actCtx.ActionHash,
			GasConsumed:     actCtx.IntrinsicGas,
			ContractAddress: p.addr.String(),
		}, nil
	}

	// keep a copy of current candidate before updating
	oldCand := c.Clone()
	c.Name = act.Name()
	c.Operator = act.OperatorAddress()
	c.Reward = act.RewardAddress()

	if err := putCandidate(sm, c.Owner, c); err != nil {
		return nil, err
	}

	// delete the current and update new into candidate center
	p.inMemCandidates.Delete(oldCand.Name)
	if err := p.inMemCandidates.Put(c); err != nil {
		return nil, err
	}

	return &action.Receipt{
		Status:      uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight: blkCtx.BlockHeight,
		ActionHash:  actCtx.ActionHash,
		// TODO: update real gas
		GasConsumed:     actCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}, nil
}
