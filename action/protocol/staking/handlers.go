// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
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
	_, err := getCandidate(sm, actCtx.Caller)
	if err == nil || errors.Cause(err) != state.ErrStateNotExist {
		return nil, errors.New("already exist")
	}

	if p.inMemCandidates.Contains(act.Name()) {
		return nil, errors.New("name already in use")
	}

	owner := actCtx.Caller
	if act.OwnerAddress() != nil {
		owner = act.OwnerAddress()
	}

	c := NewCandidate(owner, act.OperatorAddress(), act.RewardAddress(), act.Name(), act.Amount())
	if err := putCandidate(sm, c.Owner, c); err != nil {
		return nil, err
	}

	if err := p.inMemCandidates.Put(c); err != nil {
		return nil, err
	}

	// TODO create self staking bucket

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
	c, err := getCandidate(sm, actCtx.Caller)
	if err != nil {
		return nil, err
	}
	if len(act.Name()) != 0 {
		p.inMemCandidates.Delete(c.Name)
		c.Name = act.Name()
	}

	if act.OperatorAddress() != nil {
		c.Operator = act.OperatorAddress()
	}

	if act.RewardAddress() != nil {
		c.Reward = act.RewardAddress()
	}

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
