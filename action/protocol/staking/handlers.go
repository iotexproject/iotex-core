// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"time"

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

	// cannot collide with existing owner
	if p.inMemCandidates.ContainsOwner(act.OwnerAddress()) {
		return nil, ErrAlreadyExist
	}

	// cannot collide with existing name
	if p.inMemCandidates.ContainsName(act.Name()) {
		return nil, ErrInvalidCanName
	}

	// cannot collide with existing operator address
	if p.inMemCandidates.ContainsOperator(act.OperatorAddress()) {
		return nil, ErrInvalidOperator
	}

	bucket := NewVoteBucket(owner, owner, act.Amount(), act.Duration(), time.Now(), act.AutoStake())
	bucketIdx, err := putBucket(sm, owner, bucket)
	if err != nil {
		return nil, err
	}

	c := NewCandidate(owner, act.OperatorAddress(), act.RewardAddress(), act.Name(), bucketIdx, act.Amount())
	c.Votes = calculateVoteWeight(bucket, true)

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
	c := p.inMemCandidates.GetByOwner(actCtx.Caller)
	if c == nil {
		return nil, ErrInvalidOwner
	}

	// cannot collide with existing name
	if len(act.Name()) != 0 {
		if act.Name() != c.Name && p.inMemCandidates.ContainsName(act.Name()) {
			return nil, ErrInvalidCanName
		}
		c.Name = act.Name()
	}

	// cannot collide with existing operator address
	if act.OperatorAddress() != nil {
		if act.OperatorAddress() != c.Operator && p.inMemCandidates.ContainsOperator(act.OperatorAddress()) {
			return nil, ErrInvalidOperator
		}
		c.Operator = act.OperatorAddress()
	}

	if act.RewardAddress() != nil {
		c.Reward = act.RewardAddress()
	}

	if err := putCandidate(sm, c.Owner, c); err != nil {
		return nil, err
	}

	// delete the current and update new into candidate center
	p.inMemCandidates.Delete(c.Owner)
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
