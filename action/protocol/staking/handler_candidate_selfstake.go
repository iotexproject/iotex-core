package staking

import (
	"context"
	"math/big"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

const (
	handleCandidateSelfStake = "candidateSelfStake"
)

func (p *Protocol) handleCandidateSelfStake(ctx context.Context, act *action.CandidateSelfStake, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	var (
		bucket     *VoteBucket
		prevBucket *VoteBucket
		err        error
		rErr       ReceiptError
		txLogs     []*action.TransactionLog
		cand       *Candidate
		bucketCand *Candidate

		actCtx     = protocol.MustGetActionCtx(ctx)
		featureCtx = protocol.MustGetFeatureCtx(ctx)
		log        = newReceiptLog(p.addr.String(), handleCandidateSelfStake, featureCtx.NewStakingReceiptFormat)
	)
	// caller must be the owner of a candidate
	cand = csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return log, nil, errCandNotExist
	}
	if cand.SelfStakeBucketIdx != 0 {
		prevBucket, err = p.fetchBucket(csm, cand.SelfStakeBucketIdx)
		if err != nil {
			return log, nil, err
		}
	}

	bucket, rErr = p.fetchBucket(csm, act.BucketID())
	if rErr != nil {
		return log, nil, rErr
	}
	if bucketCand = csm.GetByOwner(bucket.Candidate); bucketCand == nil {
		return log, nil, &handleError{
			err:           errors.New("bucket candidate does not exist"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}

	if err = p.validateBucketSelfStake(csm, bucket, cand); err != nil {
		return log, nil, err
	}

	// unbind previous bucket
	if prevBucket != nil {
		cand.SubVote(p.calculateVoteWeight(prevBucket, true))
		cand.AddVote(p.calculateVoteWeight(prevBucket, false))
		cand.SelfStakeBucketIdx = 0
		cand.SelfStake = big.NewInt(0)
	}
	// change bucket candidate
	if !address.Equal(bucket.Candidate, cand.Owner) {
		if err = p.changeBucketCandidate(csm, bucket, bucketCand, cand); err != nil {
			return log, nil, err
		}
	}
	// bind new bucket
	cand.SelfStakeBucketIdx = bucket.Index
	cand.SelfStake = big.NewInt(bucket.StakedAmount.Int64())
	cand.SubVote(p.calculateVoteWeight(bucket, false))
	cand.AddVote(p.calculateVoteWeight(bucket, true))

	if err = csm.Upsert(cand); err != nil {
		return log, nil, csmErrorToHandleError(cand.Owner.String(), err)
	}

	return log, txLogs, nil
}

func (p *Protocol) validateBucketSelfStake(csm CandidateStateManager, bucket *VoteBucket, cand *Candidate) ReceiptError {
	// check bucket amount
	if bucket.StakedAmount.Cmp(p.config.RegistrationConsts.MinSelfStake) < 0 {
		return &handleError{
			err:           errors.New("bucket amount is unsufficient to be self-staked"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
		}
	}
	// check bucket has not been unstaked
	if bucket.isUnstaked() {
		return &handleError{
			err:           errors.New("bucket is unstaked"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	// check bucket owner is the candidate owner
	if !address.Equal(bucket.Owner, cand.Owner) {
		return &handleError{
			err:           errors.New("bucket owner is not the same as candidate owner"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	// check bucket is not self-stake bucket
	if csm.ContainsSelfStakingBucket(bucket.Index) {
		return &handleError{
			err:           errors.New("self staking bucket cannot be processed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func (p *Protocol) changeBucketCandidate(csm CandidateStateManager, bucket *VoteBucket, prevCand, cand *Candidate) error {
	// update bucket index
	if err := csm.delCandBucketIndex(bucket.Candidate, bucket.Index); err != nil {
		return err
	}
	if err := csm.putCandBucketIndex(cand.Owner, bucket.Index); err != nil {
		return err
	}
	// update bucket candidate
	bucket.Candidate = cand.Owner
	if err := csm.updateBucket(bucket.Index, bucket); err != nil {
		return err
	}
	// update previous candidate
	weightedVotes := p.calculateVoteWeight(bucket, false)
	if err := prevCand.SubVote(weightedVotes); err != nil {
		return &handleError{
			err:           errors.Wrapf(err, "failed to subtract vote for previous candidate %s", prevCand.Owner.String()),
			failureStatus: iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		}
	}
	if err := csm.Upsert(prevCand); err != nil {
		return csmErrorToHandleError(prevCand.Owner.String(), err)
	}

	// update current candidate
	if err := cand.AddVote(weightedVotes); err != nil {
		return &handleError{
			err:           errors.Wrapf(err, "failed to add vote for candidate %s", cand.Owner.String()),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
		}
	}
	if err := csm.Upsert(cand); err != nil {
		return csmErrorToHandleError(cand.Owner.String(), err)
	}

	return nil
}
