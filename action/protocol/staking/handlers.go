// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
)

func (p *Protocol) handleCreateStake(ctx context.Context, act *action.CreateStake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	staker, err := accountutil.LoadOrCreateAccount(sm, actionCtx.Caller.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of staker %s", actionCtx.Caller.String())
	}

	// update staker balance
	if err := staker.SubBalance(act.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of staker %s", actionCtx.Caller.String())
	}
	// put updated staker's account state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), staker); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}

	if err := p.settleAccount(ctx, sm); err != nil {
		return nil, errors.Wrapf(err, "failed to settle the account of staker %s", actionCtx.Caller.String())
	}

	// Create new bucket and bucket index
	candidate := p.inMemCandidates.GetByName(act.Candidate())
	if candidate == nil {
		return nil, errors.Wrap(ErrInvalidCanName, "cannot find candidate in candidate center")
	}
	bucket := NewVoteBucket(candidate.Owner, actionCtx.Caller, act.Amount(), act.Duration(), blkCtx.BlockTimeStamp, act.AutoStake())
	index, err := putBucket(sm, candidate.Owner, bucket)
	if err != nil {
		return nil, errors.Wrap(err, "failed to put bucket")
	}

	bucketIndex, err := NewBucketIndex(index, act.Candidate())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new bucket index")
	}
	if err := putBucketIndex(sm, actionCtx.Caller, bucketIndex); err != nil {
		return nil, errors.Wrap(err, "failed to put bucket index")
	}

	// update candidate
	weightedVote := p.calculateVoteWeight(bucket, false)
	if err := candidate.AddVote(weightedVote); err != nil {
		return nil, errors.Wrapf(err, "failed to add vote for candidate %s", candidate.Name)
	}
	if err := putCandidate(sm, candidate.Owner, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put candidate %s", candidate.Name)
	}

	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}, nil
}

func (p *Protocol) handleUnstake(ctx context.Context, act *action.Unstake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	if err := p.settleAccount(ctx, sm); err != nil {
		return nil, errors.Wrapf(err, "failed to settle the account of unstaker %s", actionCtx.Caller.String())
	}

	bucket, err := getBucketByVoter(sm, actionCtx.Caller, act.BucketIndex())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bucket by voter %s", actionCtx.Caller.String())
	}
	// update bucket
	bucket.UnstakeStartTime = blkCtx.BlockTimeStamp

	// remove candidate if the bucket is self staking, otherwise update candidate's vote
	if p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()) {
		p.inMemCandidates.Delete(bucket.Candidate)
		if err := delCandidate(sm, bucket.Candidate); err != nil {
			return nil, errors.Wrapf(err, "failed to delete candidate %s", bucket.Candidate.String())
		}
	} else {
		candidate := p.inMemCandidates.GetByOwner(bucket.Candidate)
		if candidate == nil {
			return nil, errors.Wrap(ErrInvalidOwner, "cannot find candidate in candidate center")
		}
		weightedVote := p.calculateVoteWeight(bucket, false)
		candidate.SubVote(weightedVote)
		if err := putCandidate(sm, bucket.Candidate, candidate); err != nil {
			return nil, errors.Wrapf(err, "failed to put candidate %s", bucket.Candidate.String())
		}
	}

	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}, nil
}

func (p *Protocol) handleWithdrawStake(ctx context.Context, act *action.WithdrawStake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	bucket, err := getBucketByVoter(sm, actionCtx.Caller, act.BucketIndex())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bucket by voter %s", actionCtx.Caller.String())
	}

	// check unstake time
	if bucket.UnstakeStartTime.Unix() == 0 {
		return nil, errors.New("bucket has not been unstaked")
	}
	if blkCtx.BlockTimeStamp.Before(bucket.UnstakeStartTime.Add(p.voteCal.WithdrawWaitingPeriod)) {
		return nil, fmt.Errorf("stake is not ready to withdraw, current time %s, required time %s",
			blkCtx.BlockTimeStamp, bucket.UnstakeStartTime.Add(p.voteCal.WithdrawWaitingPeriod))
	}

	withdrawer, err := accountutil.LoadOrCreateAccount(sm, actionCtx.Caller.String())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load or create the account of withdrawer %s", actionCtx.Caller.String())
	}
	// update withdrawer balance
	if err := withdrawer.AddBalance(bucket.StakedAmount); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of withdrawer %s", actionCtx.Caller.String())
	}
	// put updated withdrawer's account state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), withdrawer); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}

	if err := p.settleAccount(ctx, sm); err != nil {
		return nil, errors.Wrapf(err, "failed to settle the account of withdrawer %s", actionCtx.Caller.String())
	}

	// delete bucket and bucket index
	if err := delBucket(sm, bucket.Candidate, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete bucket for candidate %s", bucket.Candidate.String())
	}
	if err := delBucketIndex(sm, bucket.Owner, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete bucket index for voter %s", bucket.Owner.String())
	}

	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}, nil
}

func (p *Protocol) handleChangeCandidate(ctx context.Context, act *action.ChangeCandidate, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleTransferStake(ctx context.Context, act *action.TransferStake, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleDepositToStake(ctx context.Context, act *action.DepositToStake, sm protocol.StateManager) (*action.Receipt, error) {
	// TODO
	return nil, nil
}

func (p *Protocol) handleRestake(ctx context.Context, act *action.Restake, sm protocol.StateManager) (*action.Receipt, error) {
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

	bucket := NewVoteBucket(owner, owner, act.Amount(), act.Duration(), time.Now(), act.AutoStake())
	bucketIdx, err := putBucket(sm, owner, bucket)
	if err != nil {
		return nil, err
	}

	c := NewCandidate(owner, act.OperatorAddress(), act.RewardAddress(), act.Name(), bucketIdx, act.Amount())
	c.Votes = p.calculateVoteWeight(bucket, true)

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
		c.Name = act.Name()
	}

	// cannot collide with existing operator address
	if act.OperatorAddress() != nil {
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

// settleAccount deposits gas fee and updates caller's nonce
func (p *Protocol) settleAccount(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	if blkCtx.GasLimit < actionCtx.IntrinsicGas {
		return errors.Wrap(action.ErrHitGasLimit, "block gas limit exceeded")
	}
	gasFee := big.NewInt(0).Mul(actionCtx.GasPrice, big.NewInt(0).SetUint64(actionCtx.IntrinsicGas))
	if err := p.depositGas(ctx, sm, gasFee); err != nil {
		return errors.Wrap(err, "failed to deposit gas")
	}
	return p.increaseNonce(sm, actionCtx.Caller, actionCtx.Nonce)
}

func (p *Protocol) increaseNonce(sm protocol.StateManager, addr address.Address, nonce uint64) error {
	acc, err := accountutil.LoadOrCreateAccount(sm, addr.String())
	if err != nil {
		return err
	}
	// TODO: this check shouldn't be necessary
	if nonce > acc.Nonce {
		acc.Nonce = nonce
	}
	return accountutil.StoreAccount(sm, addr.String(), acc)
}

func getBucketByVoter(sm protocol.StateReader, voter address.Address, index uint64) (*VoteBucket, error) {
	bucketIndices, err := getBucketIndices(sm, voter)
	if err != nil {
		return nil, err
	}
	candAddr := bucketIndices.findCandidate(index)
	if candAddr == nil {
		return nil, fmt.Errorf("failed to find candidate address from bucket indices of voter %s", voter.String())
	}
	return getBucket(sm, candAddr, index)
}
