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

	"github.com/pkg/errors"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
)

func (p *Protocol) handleCreateStake(ctx context.Context, act *action.CreateStake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	staker, gasFee, err := fetchCaller(ctx, sm, act.Amount())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
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

	bucketIndex, err := NewBucketIndex(index, candidate.Owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new bucket index")
	}
	if err := putBucketIndex(sm, actionCtx.Caller, bucketIndex); err != nil {
		return nil, errors.Wrap(err, "failed to put bucket index")
	}

	// update candidate
	weightedVote := p.calculateVoteWeight(bucket, false)
	if err := candidate.AddVote(weightedVote); err != nil {
		return nil, errors.Wrapf(err, "failed to add vote for candidate %s", candidate.Owner.String())
	}
	if err := putCandidate(sm, candidate.Owner, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of candidate %s", candidate.Owner.String())
	}

	p.inMemCandidates.Put(candidate)

	// update staker balance
	if err := staker.SubBalance(act.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of staker %s", actionCtx.Caller.String())
	}
	// put updated staker's account state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), staker); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}

	return p.settleAction(ctx, sm, gasFee)
}

func (p *Protocol) handleUnstake(ctx context.Context, act *action.Unstake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	_, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, isSelfStaking, err := p.fetchBucket(sm, actionCtx.Caller, act.BucketIndex())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch bucket by address %s", actionCtx.Caller.String())
	}
	if bucket.Owner != actionCtx.Caller {
		return nil, fmt.Errorf("bucket owner does not match action caller, bucket owner %s, action caller %s",
			bucket.Owner.String(), actionCtx.Caller.String())
	}
	// update bucket
	bucket.UnstakeStartTime = blkCtx.BlockTimeStamp

	// remove candidate if the bucket is self staking, otherwise update candidate's vote
	if isSelfStaking {
		if err := delCandidate(sm, bucket.Candidate); err != nil {
			return nil, errors.Wrapf(err, "failed to delete candidate %s", bucket.Candidate.String())
		}
		p.inMemCandidates.Delete(bucket.Candidate)
	} else {
		candidate := p.inMemCandidates.GetByOwner(bucket.Candidate)
		if candidate == nil {
			return nil, errors.Wrap(ErrInvalidOwner, "cannot find candidate in candidate center")
		}
		weightedVote := p.calculateVoteWeight(bucket, false)
		if err := candidate.SubVote(weightedVote); err != nil {
			return nil, errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String())
		}
		if err := putCandidate(sm, bucket.Candidate, candidate); err != nil {
			return nil, errors.Wrapf(err, "failed to put state of candidate %s", bucket.Candidate.String())
		}

		p.inMemCandidates.Put(candidate)
	}

	return p.settleAction(ctx, sm, gasFee)
}

func (p *Protocol) handleWithdrawStake(ctx context.Context, act *action.WithdrawStake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	bucket, isSelfStaking, err := p.fetchBucket(sm, actionCtx.Caller, act.BucketIndex())
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get bucket by address %s", actionCtx.Caller.String())
	}
	if bucket.Owner != actionCtx.Caller {
		return nil, fmt.Errorf("bucket owner does not match action caller, bucket owner %s, action caller %s",
			bucket.Owner.String(), actionCtx.Caller.String())
	}

	withdrawer, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	// check unstake time
	if bucket.UnstakeStartTime.Unix() == 0 {
		return nil, errors.New("bucket has not been unstaked")
	}
	if blkCtx.BlockTimeStamp.Before(bucket.UnstakeStartTime.Add(p.voteCal.WithdrawWaitingPeriod)) {
		return nil, fmt.Errorf("stake is not ready to withdraw, current time %s, required time %s",
			blkCtx.BlockTimeStamp, bucket.UnstakeStartTime.Add(p.voteCal.WithdrawWaitingPeriod))
	}

	// delete bucket and bucket index
	if err := delBucket(sm, bucket.Candidate, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete bucket for candidate %s", bucket.Candidate.String())
	}
	if !isSelfStaking {
		if err := delBucketIndex(sm, bucket.Owner, act.BucketIndex()); err != nil {
			return nil, errors.Wrapf(err, "failed to delete bucket index for voter %s", bucket.Owner.String())
		}
	}

	// update withdrawer balance
	if err := withdrawer.AddBalance(bucket.StakedAmount); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of withdrawer %s", actionCtx.Caller.String())
	}
	// put updated withdrawer's account state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), withdrawer); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}

	return p.settleAction(ctx, sm, gasFee)
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

	registrationFee := unit.ConvertIotxToRau(100)

	caller, gasFee, err := fetchCaller(ctx, sm, new(big.Int).Add(act.Amount(), registrationFee))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	owner := actCtx.Caller
	if act.OwnerAddress() != nil {
		owner = act.OwnerAddress()
	}
	bucket := NewVoteBucket(owner, owner, act.Amount(), act.Duration(), blkCtx.BlockTimeStamp, act.AutoStake())
	bucketIdx, err := putBucket(sm, owner, bucket)
	if err != nil {
		return nil, err
	}

	bucketIndex, err := NewBucketIndex(bucketIdx, owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new bucket index")
	}
	if err := putBucketIndex(sm, owner, bucketIndex); err != nil {
		return nil, errors.Wrap(err, "failed to put bucket index")
	}

	c := NewCandidate(owner, act.OperatorAddress(), act.RewardAddress(), act.Name(), bucketIdx, act.Amount())
	c.Votes = p.calculateVoteWeight(bucket, true)

	if err := putCandidate(sm, c.Owner, c); err != nil {
		return nil, err
	}

	// update caller balance
	if err := caller.SubBalance(new(big.Int).Add(act.Amount(), registrationFee)); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of staker %s", actCtx.Caller.String())
	}
	// put updated caller's account state to trie
	if err := accountutil.StoreAccount(sm, actCtx.Caller.String(), caller); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actCtx.Caller.String())
	}
	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, err
	}

	p.inMemCandidates.Delete(owner)
	p.inMemCandidates.Put(c)
	return receipt, nil
}

func (p *Protocol) handleCandidateUpdate(ctx context.Context, act *action.CandidateUpdate, sm protocol.StateManager) (*action.Receipt, error) {
	actCtx := protocol.MustGetActionCtx(ctx)

	_, gasFee, err := fetchCaller(ctx, sm, new(big.Int))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	// only owner can update candidate
	c := p.inMemCandidates.GetByOwner(actCtx.Caller)
	if c == nil {
		return nil, ErrInvalidOwner
	}

	if len(act.Name()) != 0 {
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

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, err
	}

	p.inMemCandidates.Delete(c.Owner)
	p.inMemCandidates.Put(c)
	return receipt, nil
}

// settleAccount deposits gas fee and updates caller's nonce
func (p *Protocol) settleAction(
	ctx context.Context,
	sm protocol.StateManager,
	gasFee *big.Int,
) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	if blkCtx.GasLimit < actionCtx.IntrinsicGas {
		return nil, errors.Wrap(action.ErrHitGasLimit, "block gas limit exceeded")
	}
	if err := p.depositGas(ctx, sm, gasFee); err != nil {
		return nil, errors.Wrap(err, "failed to deposit gas")
	}
	if err := p.increaseNonce(sm, actionCtx.Caller, actionCtx.Nonce); err != nil {
		return nil, errors.Wrap(err, "failed to update nonce")
	}
	return &action.Receipt{
		Status:          uint64(iotextypes.ReceiptStatus_Success),
		BlockHeight:     blkCtx.BlockHeight,
		ActionHash:      actionCtx.ActionHash,
		GasConsumed:     actionCtx.IntrinsicGas,
		ContractAddress: p.addr.String(),
	}, nil
}

func (p *Protocol) increaseNonce(sm protocol.StateManager, addr address.Address, nonce uint64) error {
	acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(addr.Bytes()))
	if err != nil {
		return err
	}
	// TODO: this check shouldn't be necessary
	if nonce > acc.Nonce {
		acc.Nonce = nonce
	}
	return accountutil.StoreAccount(sm, addr.String(), acc)
}

func (p *Protocol) fetchBucket(sm protocol.StateReader, voter address.Address, index uint64) (*VoteBucket, bool, error) {
	var candAddr address.Address
	var isSelfStaking bool
	candidate := p.inMemCandidates.GetBySelfStakingIndex(index)
	if candidate != nil {
		candAddr = candidate.Owner
		isSelfStaking = true
	} else {
		bucketIndices, err := getBucketIndices(sm, voter)
		if err != nil {
			return nil, false, err
		}
		candAddr = bucketIndices.findCandidate(index)
		if candAddr == nil {
			return nil, false, fmt.Errorf("failed to find candidate address from bucket indices of voter %s", voter.String())
		}
	}
	bucket, err := getBucket(sm, candAddr, index)
	if err != nil {
		return nil, false, err
	}
	return bucket, isSelfStaking, nil
}

func fetchCaller(ctx context.Context, sm protocol.StateReader, amount *big.Int) (*state.Account, *big.Int, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)

	caller, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load or create the account of caller %s", actionCtx.Caller.String())
	}
	gasFee := big.NewInt(0).Mul(actionCtx.GasPrice, big.NewInt(0).SetUint64(actionCtx.IntrinsicGas))
	// check caller's balance
	if big.NewInt(0).Add(amount, gasFee).Cmp(caller.Balance) == 1 {
		return nil, nil, errors.Wrapf(
			state.ErrNotEnoughBalance,
			"caller %s balance %s, required amount %s",
			actionCtx.Caller.String(),
			caller.Balance,
			big.NewInt(0).Add(amount, gasFee),
		)
	}
	return caller, gasFee, nil
}
