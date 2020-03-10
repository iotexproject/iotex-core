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
	if _, err := putBucketAndIndex(sm, bucket); err != nil {
		return nil, errors.Wrap(err, "failed to put bucket")
	}

	// update candidate
	weightedVote := p.calculateVoteWeight(bucket, false)
	if err := candidate.AddVote(weightedVote); err != nil {
		return nil, errors.Wrapf(err, "failed to add vote for candidate %s", candidate.Owner.String())
	}
	if err := putCandidate(sm, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of candidate %s", candidate.Owner.String())
	}

	// update staker balance
	if err := staker.SubBalance(act.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of staker %s", actionCtx.Caller.String())
	}
	// put updated staker's account state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), staker); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to settle action")
	}
	if err := p.inMemCandidates.Upsert(candidate); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (p *Protocol) handleUnstake(ctx context.Context, act *action.Unstake, sm protocol.StateManager) (*action.Receipt, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)

	_, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, err := p.fetchBucket(ctx, sm, act.BucketIndex(), true, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}

	// update bucket
	bucket.UnstakeStartTime = blkCtx.BlockTimeStamp
	if err := updateBucket(sm, act.BucketIndex(), bucket); err != nil {
		return nil, errors.Wrapf(err, "failed to update bucket for voter %s", bucket.Owner)
	}

	candidate := p.inMemCandidates.GetByOwner(bucket.Candidate)
	if candidate == nil {
		return nil, errors.Wrap(ErrInvalidOwner, "cannot find candidate in candidate center")
	}
	weightedVote := p.calculateVoteWeight(bucket, false)
	if err := candidate.SubVote(weightedVote); err != nil {
		return nil, errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String())
	}
	// clear candidate's self stake if the bucket is self staking
	if p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()) {
		candidate.SelfStake = big.NewInt(0)
	}
	if err := putCandidate(sm, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of candidate %s", bucket.Candidate.String())
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to settle action")
	}
	if err := p.inMemCandidates.Upsert(candidate); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (p *Protocol) handleWithdrawStake(ctx context.Context, act *action.WithdrawStake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	withdrawer, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, err := p.fetchBucket(ctx, sm, act.BucketIndex(), true, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}

	// check unstake time
	if bucket.UnstakeStartTime.Unix() == 0 {
		return nil, errors.New("bucket has not been unstaked")
	}
	if blkCtx.BlockTimeStamp.Before(bucket.UnstakeStartTime.Add(p.config.WithdrawWaitingPeriod)) {
		return nil, fmt.Errorf("stake is not ready to withdraw, current time %s, required time %s",
			blkCtx.BlockTimeStamp, bucket.UnstakeStartTime.Add(p.config.WithdrawWaitingPeriod))
	}

	// delete bucket and bucket index
	if err := delBucket(sm, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete bucket for candidate %s", bucket.Candidate.String())
	}
	if err := delCandBucketIndex(sm, bucket.Candidate, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete bucket index for candidate %s", bucket.Candidate.String())
	}
	if err := delVoterBucketIndex(sm, bucket.Owner, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete bucket index for voter %s", bucket.Owner.String())
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
	candidate := p.inMemCandidates.GetByName(act.Candidate())
	if candidate == nil {
		return nil, errors.Wrap(ErrInvalidCanName, "cannot find candidate in candidate center")
	}

	_, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, err := p.fetchBucket(ctx, sm, act.BucketIndex(), true, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}

	prevCandidate := p.inMemCandidates.GetByOwner(bucket.Candidate)
	if prevCandidate == nil {
		return nil, errors.Wrap(ErrInvalidOwner, "cannot find candidate in candidate center")
	}

	// update bucket index
	if err := delCandBucketIndex(sm, bucket.Candidate, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete candidate bucket index for candidate %s", bucket.Candidate.String())
	}
	if err := putCandBucketIndex(sm, candidate.Owner, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to put candidate bucket index for candidate %s", candidate.Owner.String())
	}
	// update bucket
	bucket.Candidate = candidate.Owner
	if err := updateBucket(sm, act.BucketIndex(), bucket); err != nil {
		return nil, errors.Wrapf(err, "failed to update bucket for voter %s", bucket.Owner)
	}

	weightedVotes := p.calculateVoteWeight(bucket, false)

	// update previous candidate
	if err := prevCandidate.SubVote(weightedVotes); err != nil {
		return nil, errors.Wrapf(err, "failed to subtract vote for previous candidate %s", prevCandidate.Owner.String())
	}
	if err := putCandidate(sm, prevCandidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of previous candidate %s", prevCandidate.Owner.String())
	}

	// update current candidate
	if err := candidate.AddVote(weightedVotes); err != nil {
		return nil, errors.Wrapf(err, "failed to add vote for candidate %s", candidate.Owner.String())
	}
	if err := putCandidate(sm, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of candidate %s", candidate.Owner.String())
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to settle action")
	}
	if err := p.inMemCandidates.Upsert(prevCandidate); err != nil {
		return nil, err
	}
	if err := p.inMemCandidates.Upsert(candidate); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (p *Protocol) handleTransferStake(ctx context.Context, act *action.TransferStake, sm protocol.StateManager) (*action.Receipt, error) {
	_, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, err := p.fetchBucket(ctx, sm, act.BucketIndex(), true, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}

	// update bucket index
	if err := delVoterBucketIndex(sm, bucket.Owner, act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to delete voter bucket index for voter %s", bucket.Owner.String())
	}
	if err := putVoterBucketIndex(sm, act.VoterAddress(), act.BucketIndex()); err != nil {
		return nil, errors.Wrapf(err, "failed to put candidate bucket index for voter %s", act.VoterAddress().String())
	}

	// update bucket
	bucket.Owner = act.VoterAddress()
	if err := updateBucket(sm, act.BucketIndex(), bucket); err != nil {
		return nil, errors.Wrapf(err, "failed to update bucket for voter %s", bucket.Owner)
	}

	return p.settleAction(ctx, sm, gasFee)
}

func (p *Protocol) handleDepositToStake(ctx context.Context, act *action.DepositToStake, sm protocol.StateManager) (*action.Receipt, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)

	depositor, gasFee, err := fetchCaller(ctx, sm, act.Amount())
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, err := p.fetchBucket(ctx, sm, act.BucketIndex(), false, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}
	if !bucket.AutoStake {
		return nil, errors.New("deposit is only allowed on auto-stake bucket")
	}
	candidate := p.inMemCandidates.GetByOwner(bucket.Candidate)
	if candidate == nil {
		return nil, errors.Wrap(ErrInvalidOwner, "cannot find candidate in candidate center")
	}

	prevWeightedVotes := p.calculateVoteWeight(bucket, p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()))
	// update bucket
	bucket.StakedAmount.Add(bucket.StakedAmount, act.Amount())
	if err := updateBucket(sm, act.BucketIndex(), bucket); err != nil {
		return nil, errors.Wrapf(err, "failed to update bucket for voter %s", bucket.Owner)
	}

	// update candidate
	if err := candidate.SubVote(prevWeightedVotes); err != nil {
		return nil, errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String())
	}
	weightedVotes := p.calculateVoteWeight(bucket, p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()))
	if err := candidate.AddVote(weightedVotes); err != nil {
		return nil, errors.Wrapf(err, "failed to add vote for candidate %s", bucket.Candidate.String())
	}
	if p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()) {
		if err := candidate.AddSelfStake(act.Amount()); err != nil {
			return nil, errors.Wrapf(err, "failed to add self stake for candidate %s", bucket.Candidate.String())
		}
	}
	if err := putCandidate(sm, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of candidate %s", bucket.Candidate.String())
	}

	// update depositor balance
	if err := depositor.SubBalance(act.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of depositor %s", actionCtx.Caller.String())
	}
	// put updated depositor's account state to trie
	if err := accountutil.StoreAccount(sm, actionCtx.Caller.String(), depositor); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actionCtx.Caller.String())
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to settle action")
	}
	if err := p.inMemCandidates.Upsert(candidate); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (p *Protocol) handleRestake(ctx context.Context, act *action.Restake, sm protocol.StateManager) (*action.Receipt, error) {
	_, gasFee, err := fetchCaller(ctx, sm, big.NewInt(0))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	bucket, err := p.fetchBucket(ctx, sm, act.BucketIndex(), true, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch bucket")
	}

	candidate := p.inMemCandidates.GetByOwner(bucket.Candidate)
	if candidate == nil {
		return nil, errors.Wrap(ErrInvalidOwner, "cannot find candidate in candidate center")
	}

	prevWeightedVotes := p.calculateVoteWeight(bucket, p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()))
	// update bucket
	bucket.StakedDuration = time.Duration(act.Duration()) * 24 * time.Hour
	bucket.AutoStake = act.AutoStake()
	if err := updateBucket(sm, act.BucketIndex(), bucket); err != nil {
		return nil, errors.Wrapf(err, "failed to update bucket for voter %s", bucket.Owner)
	}

	// update candidate
	if err := candidate.SubVote(prevWeightedVotes); err != nil {
		return nil, errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String())
	}
	weightedVotes := p.calculateVoteWeight(bucket, p.inMemCandidates.ContainsSelfStakingBucket(act.BucketIndex()))
	if err := candidate.AddVote(weightedVotes); err != nil {
		return nil, errors.Wrapf(err, "failed to add vote for candidate %s", bucket.Candidate.String())
	}
	if err := putCandidate(sm, candidate); err != nil {
		return nil, errors.Wrapf(err, "failed to put state of candidate %s", bucket.Candidate.String())
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, errors.Wrap(err, "failed to settle action")
	}
	if err := p.inMemCandidates.Upsert(candidate); err != nil {
		return nil, err
	}
	return receipt, nil
}

func (p *Protocol) handleCandidateRegister(ctx context.Context, act *action.CandidateRegister, sm protocol.StateManager) (*action.Receipt, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	blkCtx := protocol.MustGetBlockCtx(ctx)

	registrationFee := unit.ConvertIotxToRau(p.config.RegistrationConsts.Fee)

	caller, gasFee, err := fetchCaller(ctx, sm, new(big.Int).Add(act.Amount(), registrationFee))
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch caller")
	}

	owner := actCtx.Caller
	if act.OwnerAddress() != nil {
		owner = act.OwnerAddress()
	}
	bucket := NewVoteBucket(owner, owner, act.Amount(), act.Duration(), blkCtx.BlockTimeStamp, act.AutoStake())
	bucketIdx, err := putBucketAndIndex(sm, bucket)
	if err != nil {
		return nil, errors.Wrap(err, "failed to put bucket")
	}

	c := &Candidate{
		Owner:              owner,
		Operator:           act.OperatorAddress(),
		Reward:             act.RewardAddress(),
		Name:               act.Name(),
		Votes:              p.calculateVoteWeight(bucket, true),
		SelfStakeBucketIdx: bucketIdx,
		SelfStake:          act.Amount(),
	}

	if err := putCandidate(sm, c); err != nil {
		return nil, err
	}

	// update caller balance
	if err := caller.SubBalance(act.Amount()); err != nil {
		return nil, errors.Wrapf(err, "failed to update the balance of staker %s", actCtx.Caller.String())
	}
	// put updated caller's account state to trie
	if err := accountutil.StoreAccount(sm, actCtx.Caller.String(), caller); err != nil {
		return nil, errors.Wrapf(err, "failed to store account %s", actCtx.Caller.String())
	}

	// put registrationFee to reward pool
	if err := p.depositGas(ctx, sm, registrationFee); err != nil {
		return nil, errors.Wrap(err, "failed to deposit gas")
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, err
	}

	if err := p.inMemCandidates.Upsert(c); err != nil {
		return nil, err
	}
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

	if err := putCandidate(sm, c); err != nil {
		return nil, err
	}

	receipt, err := p.settleAction(ctx, sm, gasFee)
	if err != nil {
		return nil, err
	}

	if err := p.inMemCandidates.Upsert(c); err != nil {
		return nil, err
	}
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

func (p *Protocol) fetchBucket(
	ctx context.Context,
	sr protocol.StateReader,
	index uint64,
	checkOwner bool,
	allowSelfStaking bool,
) (*VoteBucket, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)
	bucket, err := getBucket(sr, index)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to fetch bucket by index %d", index)
	}
	if checkOwner && bucket.Owner != actionCtx.Caller {
		return nil, fmt.Errorf("bucket owner does not match action caller, bucket owner %s, action caller %s",
			bucket.Owner.String(), actionCtx.Caller.String())
	}
	if !allowSelfStaking && p.inMemCandidates.ContainsSelfStakingBucket(index) {
		return nil, errors.New("self staking bucket cannot be processed")
	}
	return bucket, nil
}

func putBucketAndIndex(sm protocol.StateManager, bucket *VoteBucket) (uint64, error) {
	index, err := putBucket(sm, bucket)
	if err != nil {
		return 0, errors.Wrap(err, "failed to put bucket")
	}

	if err := putVoterBucketIndex(sm, bucket.Owner, index); err != nil {
		return 0, errors.Wrap(err, "failed to put bucket index")
	}

	if err := putCandBucketIndex(sm, bucket.Candidate, index); err != nil {
		return 0, errors.Wrap(err, "failed to put candidate index")
	}
	return index, nil
}

func fetchCaller(ctx context.Context, sm protocol.StateReader, amount *big.Int) (*state.Account, *big.Int, error) {
	actionCtx := protocol.MustGetActionCtx(ctx)

	caller, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
	if err != nil {
		return nil, nil, errors.Wrapf(err, "failed to load the account of caller %s", actionCtx.Caller.String())
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
