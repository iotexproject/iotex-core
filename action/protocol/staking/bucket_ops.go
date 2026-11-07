// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/state"
)

// Native bucket operations shared by the legacy staking actions and the
// guarded ops of IIP-65. Each is split into a check, which runs every
// precondition and computes the result on copies without writing any state,
// and an apply, which only writes. A guarded op verifies its signatures
// between the two (IIP-65 §9.2), and the apply carries the authority of the
// write (§10).
//
// From the QSB fork (FeatureCtx.EnableBucketGuard) the legacy handlers use
// them too. Before the fork they keep the *BeforeQSB helpers in handlers.go:
// before ZanzibarGamma a failed receipt kept the writes made before the
// failing check, so their order of checks and writes is part of history.
// The checks here run in the same order as there, so a failing action reports
// the same receipt status.
//
// One difference is deliberate: a failed state read is returned as a plain
// error, which aborts the block, where the *BeforeQSB helpers report
// ErrUnknown. A read error is not a property of the action, and a receipt
// for it would let two nodes settle the same block differently (IIP-65 §9.3).

// bucketChange is the result of an operation's checks: the writes that apply
// it.
type bucketChange interface {
	apply(ctx context.Context, csm CandidateStateManager, auth bucketAuth) error
}

// copyBucket returns a copy of b that can be changed without changing b.
func copyBucket(b *VoteBucket) *VoteBucket {
	c := *b
	c.StakedAmount = new(big.Int).Set(b.StakedAmount)
	return &c
}

// checkBucketNotEndorsed is validateBucketWithoutEndorsement with a failed
// read of the endorsement returned as a storage error.
func checkBucketNotEndorsed(featureCtx protocol.FeatureCtx, sr protocol.StateReader, bucket *VoteBucket, height uint64) error {
	status, err := NewEndorsementStateReader(sr).Status(featureCtx, bucket.Index, height)
	if err != nil {
		return errors.Wrapf(err, "failed to read endorsement of bucket %d", bucket.Index)
	}
	if status != EndorseExpired {
		return &handleError{
			err:           errors.Errorf("bucket is still endorsed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

// checkSelfStakeBucket is isSelfStakeBucket with its read errors returned as
// storage errors.
func checkSelfStakeBucket(featureCtx protocol.FeatureCtx, csm CandidateStateManager, bucket *VoteBucket) (bool, error) {
	selfStake, err := isSelfStakeBucket(featureCtx, csm, bucket)
	if err != nil {
		return false, errors.Wrapf(err, "failed to read the self-stake status of bucket %d", bucket.Index)
	}
	return selfStake, nil
}

type unstakeChange struct {
	index     uint64
	bucket    *VoteBucket
	candidate *Candidate
}

// checkUnstake checks an unstake of bucket and computes its result.
func (p *Protocol) checkUnstake(ctx context.Context, csm CandidateStateManager, index uint64, bucket *VoteBucket) (*unstakeChange, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	candidate := csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return nil, errCandNotExist
	}
	if featureCtx.CannotUnstakeAgain && bucket.isUnstaked() {
		return nil, &handleError{
			err:           errors.New("unstake an already unstaked bucket again not allowed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	if bucket.AutoStake {
		return nil, &handleError{
			err:           errors.New("AutoStake should be disabled first in order to unstake"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	if blkCtx.BlockTimeStamp.Before(bucket.StakeStartTime.Add(bucket.StakedDuration)) {
		return nil, &handleError{
			err:           errors.New("bucket is not ready to be unstaked"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity,
		}
	}
	if !featureCtx.DisableDelegateEndorsement {
		if err := checkBucketNotEndorsed(featureCtx, csm.SR(), bucket, blkCtx.BlockHeight); err != nil {
			return nil, err
		}
	}
	selfStake, err := checkSelfStakeBucket(featureCtx, csm, bucket)
	if err != nil {
		return nil, err
	}
	if selfStake && !featureCtx.NoCandidateExitQueue {
		return nil, &handleError{
			err:           ErrExitNotReady,
			failureStatus: iotextypes.ReceiptStatus_ErrUnstakeBeforeMaturity,
		}
	}
	next := copyBucket(bucket)
	next.UnstakeStartTime = blkCtx.BlockTimeStamp.UTC()
	if err := candidate.SubVote(p.calculateVoteWeight(next, selfStake)); err != nil {
		return nil, &handleError{
			err:           errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String()),
			failureStatus: iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		}
	}
	// clear candidate's self stake if the bucket is self staking
	if selfStake {
		candidate.SelfStake = big.NewInt(0)
		if !featureCtx.UnstakedButNotClearSelfStakeAmount {
			candidate.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
		}
	}
	return &unstakeChange{index: index, bucket: next, candidate: candidate}, nil
}

func (c *unstakeChange) apply(_ context.Context, csm CandidateStateManager, auth bucketAuth) error {
	if err := csm.updateBucket(c.index, c.bucket, auth); err != nil {
		return keepReceiptError(err, "failed to update bucket for voter %s", c.bucket.Owner.String())
	}
	if err := csm.Upsert(c.candidate); err != nil {
		return csmErrorToHandleError(c.candidate.GetIdentifier().String(), err)
	}
	return nil
}

type withdrawChange struct {
	index         uint64
	bucket        *VoteBucket
	recipient     address.Address
	recipientAcct *state.Account
}

// checkWithdraw checks a withdrawal of bucket that credits recipient, whose
// loaded account is recipientAcct.
func (p *Protocol) checkWithdraw(ctx context.Context, csm CandidateStateManager, index uint64, bucket *VoteBucket, recipient address.Address, recipientAcct *state.Account) (*withdrawChange, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	cannotWithdraw := bucket.UnstakeStartTime.Unix() == 0
	if featureCtx.CannotUnstakeAgain {
		cannotWithdraw = !bucket.isUnstaked()
	}
	if cannotWithdraw {
		return nil, &handleError{
			err:           errors.New("bucket has not been unstaked"),
			failureStatus: iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake,
		}
	}
	withdrawWaitTime := p.config.WithdrawWaitingPeriod
	if !featureCtx.NewStakingReceiptFormat {
		withdrawWaitTime = _withdrawWaitingTime
	}
	if blkCtx.BlockTimeStamp.Before(bucket.UnstakeStartTime.Add(withdrawWaitTime)) {
		return nil, &handleError{
			err:           errors.New("stake is not ready to withdraw"),
			failureStatus: iotextypes.ReceiptStatus_ErrWithdrawBeforeMaturity,
		}
	}
	if err := csm.checkCreditBucketPool(bucket.StakedAmount, true); err != nil {
		return nil, &handleError{
			err:           errors.Wrapf(err, "failed to update staking bucket pool %s", err.Error()),
			failureStatus: iotextypes.ReceiptStatus_ErrWriteAccount,
		}
	}
	return &withdrawChange{index: index, bucket: bucket, recipient: recipient, recipientAcct: recipientAcct}, nil
}

func (c *withdrawChange) apply(ctx context.Context, csm CandidateStateManager, auth bucketAuth) error {
	if err := csm.delBucketAndIndex(c.bucket.Owner, c.bucket.Candidate, c.index, auth); err != nil {
		return keepReceiptError(err, "failed to delete bucket for candidate %s", c.bucket.Candidate.String())
	}
	if err := deleteBucketGuardWithBucket(ctx, csm.SM(), c.index); err != nil {
		return err
	}
	if err := csm.CreditBucketPool(c.bucket.StakedAmount, true); err != nil {
		return errors.Wrap(err, "failed to update staking bucket pool after its check")
	}
	if err := c.recipientAcct.AddBalance(c.bucket.StakedAmount); err != nil {
		return errors.Wrapf(err, "failed to add balance %s", c.bucket.StakedAmount)
	}
	if err := accountutil.StoreAccount(csm.SM(), c.recipient, c.recipientAcct); err != nil {
		return errors.Wrapf(err, "failed to store account %s", c.recipient.String())
	}
	return nil
}

type changeCandidateChange struct {
	index      uint64
	prior      *VoteBucket
	bucket     *VoteBucket
	prevCand   *Candidate
	targetCand *Candidate
}

// checkChangeCandidate checks moving bucket's votes to candidate.
func (p *Protocol) checkChangeCandidate(ctx context.Context, csm CandidateStateManager, index uint64, bucket *VoteBucket, candidate *Candidate) (*changeCandidateChange, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	prevCandidate := csm.GetByIdentifier(bucket.Candidate)
	if prevCandidate == nil {
		return nil, errCandNotExist
	}
	if featureCtx.CannotUnstakeAgain && bucket.isUnstaked() {
		return nil, &handleError{
			err:           errors.New("change candidate for an unstaked bucket not allowed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	if featureCtx.CannotTranferToSelf && address.Equal(prevCandidate.GetIdentifier(), candidate.GetIdentifier()) {
		return nil, &handleError{
			err:           errors.New("change to same candidate"),
			failureStatus: iotextypes.ReceiptStatus_ErrCandidateAlreadyExist,
		}
	}
	next := copyBucket(bucket)
	next.Candidate = candidate.GetIdentifier()
	// IIP-59: this pair moves the voter's weight from the old candidate to the
	// new one.
	weightedVotes := p.calculateVoteWeight(next, false)
	if err := prevCandidate.SubVote(weightedVotes); err != nil {
		return nil, &handleError{
			err:           errors.Wrapf(err, "failed to subtract vote for previous candidate %s", prevCandidate.GetIdentifier().String()),
			failureStatus: iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		}
	}
	// if the bucket equals to the previous candidate's self-stake bucket, it must be expired endorse bucket
	// so we need to clear the self-stake of the previous candidate
	if !featureCtx.DisableDelegateEndorsement && prevCandidate.SelfStakeBucketIdx == bucket.Index {
		prevCandidate.SelfStake.SetInt64(0)
		prevCandidate.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	}
	if err := candidate.AddVote(weightedVotes); err != nil {
		return nil, &handleError{
			err:           errors.Wrapf(err, "failed to add vote for candidate %s", candidate.GetIdentifier().String()),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
		}
	}
	return &changeCandidateChange{index: index, prior: bucket, bucket: next, prevCand: prevCandidate, targetCand: candidate}, nil
}

func (c *changeCandidateChange) apply(_ context.Context, csm CandidateStateManager, auth bucketAuth) error {
	if err := csm.updateBucket(c.index, c.bucket, auth); err != nil {
		return keepReceiptError(err, "failed to update bucket for voter %s", c.bucket.Owner.String())
	}
	if err := csm.delCandBucketIndex(c.prior.Candidate, c.index); err != nil {
		return errors.Wrapf(err, "failed to delete candidate bucket index for candidate %s", c.prior.Candidate.String())
	}
	if err := csm.putCandBucketIndex(c.bucket.Candidate, c.index); err != nil {
		return errors.Wrapf(err, "failed to put candidate bucket index for candidate %s", c.bucket.Candidate.String())
	}
	if err := csm.Upsert(c.prevCand); err != nil {
		return csmErrorToHandleError(c.prevCand.GetIdentifier().String(), err)
	}
	if err := csm.Upsert(c.targetCand); err != nil {
		return csmErrorToHandleError(c.targetCand.GetIdentifier().String(), err)
	}
	return nil
}

type transferChange struct {
	index  uint64
	prior  *VoteBucket
	bucket *VoteBucket
}

// checkTransfer checks transferring bucket to newOwner.
func (p *Protocol) checkTransfer(ctx context.Context, _ CandidateStateManager, index uint64, bucket *VoteBucket, newOwner address.Address) (*transferChange, error) {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if featureCtx.CannotTranferToSelf && address.Equal(newOwner, bucket.Owner) {
		return nil, &handleError{
			err:           errors.New("transfer to same owner"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	next := copyBucket(bucket)
	next.Owner = newOwner
	return &transferChange{index: index, prior: bucket, bucket: next}, nil
}

func (c *transferChange) apply(_ context.Context, csm CandidateStateManager, auth bucketAuth) error {
	if err := csm.updateBucket(c.index, c.bucket, auth); err != nil {
		return keepReceiptError(err, "failed to update bucket for voter %s", c.bucket.Owner.String())
	}
	if err := csm.delVoterBucketIndex(c.prior.Owner, c.index); err != nil {
		return errors.Wrapf(err, "failed to delete voter bucket index for voter %s", c.prior.Owner.String())
	}
	if err := csm.putVoterBucketIndex(c.bucket.Owner, c.index); err != nil {
		return errors.Wrapf(err, "failed to put candidate bucket index for voter %s", c.bucket.Owner.String())
	}
	// A transfer keeps the bucket's candidate, weight and duration, so the
	// candidate's votes are unchanged.
	return nil
}

type restakeChange struct {
	index     uint64
	bucket    *VoteBucket
	candidate *Candidate
}

// checkRestake checks restaking bucket for duration days with autoStake.
func (p *Protocol) checkRestake(ctx context.Context, csm CandidateStateManager, index uint64, bucket *VoteBucket, duration uint32, autoStake bool) (*restakeChange, error) {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	candidate := csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return nil, errCandNotExist
	}
	if featureCtx.CannotUnstakeAgain && bucket.isUnstaked() {
		return nil, &handleError{
			err:           errors.New("restake an unstaked bucket not allowed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	selfStake, err := checkSelfStakeBucket(featureCtx, csm, bucket)
	if err != nil {
		return nil, err
	}
	prevWeightedVotes := p.calculateVoteWeight(bucket, selfStake)
	actDuration := time.Duration(duration) * 24 * time.Hour
	if bucket.StakedDuration.Hours() > actDuration.Hours() {
		// in case of reducing the duration
		if bucket.AutoStake {
			return nil, &handleError{
				err:           errors.New("AutoStake should be disabled first in order to reduce duration"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		} else if blkCtx.BlockTimeStamp.Before(bucket.StakeStartTime.Add(bucket.StakedDuration)) {
			return nil, &handleError{
				err:           errors.New("bucket is not ready to be able to reduce duration"),
				failureStatus: iotextypes.ReceiptStatus_ErrReduceDurationBeforeMaturity,
			}
		}
	}
	next := copyBucket(bucket)
	next.StakedDuration = actDuration
	next.StakeStartTime = blkCtx.BlockTimeStamp.UTC()
	next.AutoStake = autoStake
	if err := candidate.SubVote(prevWeightedVotes); err != nil {
		return nil, &handleError{
			err:           errors.Wrapf(err, "failed to subtract vote for candidate %s", bucket.Candidate.String()),
			failureStatus: iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		}
	}
	if err := candidate.AddVote(p.calculateVoteWeight(next, selfStake)); err != nil {
		return nil, &handleError{
			err:           errors.Wrapf(err, "failed to add vote for candidate %s", candidate.GetIdentifier().String()),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
		}
	}
	return &restakeChange{index: index, bucket: next, candidate: candidate}, nil
}

func (c *restakeChange) apply(_ context.Context, csm CandidateStateManager, auth bucketAuth) error {
	if err := csm.updateBucket(c.index, c.bucket, auth); err != nil {
		return keepReceiptError(err, "failed to update bucket for voter %s", c.bucket.Owner.String())
	}
	if err := csm.Upsert(c.candidate); err != nil {
		return csmErrorToHandleError(c.candidate.GetIdentifier().String(), err)
	}
	return nil
}
