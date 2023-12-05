package staking

import (
	"context"

	"github.com/pkg/errors"

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
		bucket *VoteBucket
		err    error
		rErr   ReceiptError
		txLogs []*action.TransactionLog

		actCtx     = protocol.MustGetActionCtx(ctx)
		blkCtx     = protocol.MustGetBlockCtx(ctx)
		featureCtx = protocol.MustGetFeatureCtx(ctx)
		log        = newReceiptLog(p.addr.String(), handleCandidateSelfStake, featureCtx.NewStakingReceiptFormat)
	)
	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return log, nil, errCandNotExist
	}

	// check if candidate can self-stake
	if err := p.canCandidateSelfStake(cand); err != nil {
		return log, nil, &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_Failure,
		}
	}

	// bucket check
	if act.IsUsingExistingBucket() {
		bucket, rErr = p.fetchBucket(csm, actCtx.Caller, act.BucketID(), true, false)
		if rErr != nil {
			return log, nil, rErr
		}
	} else {
		bucket = NewVoteBucket(cand.Owner, cand.Owner, act.Amount(), act.Duration(), blkCtx.BlockTimeStamp, act.AutoStake())
		bucket, txLogs, err = p.createSelfStakeBucket(ctx, csm, bucket)
		if err != nil {
			return log, nil, err
		}
	}

	if err = p.bucketCanSelfStake(bucket, cand); err != nil {
		return log, nil, &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_Failure,
		}
	}

	// update candidate bucket
	if err := p.changeSelfBucket(cand, bucket); err != nil {
		return log, nil, &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_Failure,
		}
	}
	if err := csm.Upsert(cand); err != nil {
		return log, nil, csmErrorToHandleError(cand.Owner.String(), err)
	}

	return log, txLogs, nil
}

func (p *Protocol) canCandidateSelfStake(cand *Candidate) error {
	if cand.SelfStakeBucketIdx != 0 {
		return errors.New("candidate has already been self-staked")
	}
	return nil
}

func (p *Protocol) bucketCanSelfStake(bucket *VoteBucket, cand *Candidate) error {
	if bucket.StakedAmount.Cmp(p.config.RegistrationConsts.MinSelfStake) < 0 {
		return errors.New("bucket amount is unsufficient to be self-staked")
	}
	if bucket.isUnstaked() {
		return errors.New("bucket is unstaked")
	}
	if bucket.Candidate != cand.Owner {
		return errors.New("bucket candidate is not the owner")
	}
	return nil
}

func (p *Protocol) changeSelfBucket(cand *Candidate, bucket *VoteBucket) error {
	cand.SelfStakeBucketIdx = bucket.Index
	if err := cand.AddSelfStake(bucket.StakedAmount); err != nil {
		return errors.Wrapf(err, "failed to add self stake for candidate %s", cand.Owner.String())
	}
	if err := cand.AddVote(p.calculateVoteWeight(bucket, true)); err != nil {
		return errors.Wrapf(err, "failed to add vote for candidate %s", cand.Owner.String())
	}
	return nil
}
