package staking

import (
	"context"
	"math"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	handleCandidateActivate = "candidateActivate"

	candidateNoSelfStakeBucketIndex = math.MaxUint64
)

func (p *Protocol) handleCandidateActivate(ctx context.Context, act *action.CandidateActivate, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), handleCandidateActivate, featureCtx.NewStakingReceiptFormat)

	bucket, rErr := p.fetchBucket(csm, act.BucketID())
	if rErr != nil {
		return log, nil, rErr
	}
	// caller must be the owner of a candidate
	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return log, nil, errCandNotExist
	}

	if err := p.validateBucketSelfStake(ctx, csm, NewEndorsementStateManager(csm.SM()), bucket, cand); err != nil {
		return log, nil, err
	}

	log.AddTopics(byteutil.Uint64ToBytesBigEndian(bucket.Index), bucket.Candidate.Bytes())
	// convert previous self-stake bucket to vote bucket
	if cand.SelfStake.Sign() > 0 {
		prevBucket, err := p.fetchBucket(csm, cand.SelfStakeBucketIdx)
		if err != nil {
			return log, nil, err
		}
		if err := cand.SubVote(p.calculateVoteWeight(prevBucket, true)); err != nil {
			return log, nil, err
		}
		if err := cand.AddVote(p.calculateVoteWeight(prevBucket, false)); err != nil {
			return log, nil, err
		}
	}

	// convert vote bucket to self-stake bucket
	cand.SelfStakeBucketIdx = bucket.Index
	cand.SelfStake.SetBytes(bucket.StakedAmount.Bytes())
	if err := cand.SubVote(p.calculateVoteWeight(bucket, false)); err != nil {
		return log, nil, err
	}
	if err := cand.AddVote(p.calculateVoteWeight(bucket, true)); err != nil {
		return log, nil, err
	}

	if err := csm.Upsert(cand); err != nil {
		return log, nil, csmErrorToHandleError(cand.GetIdentifier().String(), err)
	}
	return log, nil, nil
}

func (p *Protocol) validateBucketSelfStake(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, bucket *VoteBucket, cand *Candidate) ReceiptError {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if err := validateBucketMinAmount(bucket, p.config.RegistrationConsts.MinSelfStake); err != nil {
		return err
	}
	if err := validateBucketStake(bucket, true); err != nil {
		return err
	}
	if err := validateBucketSelfStake(featureCtx, csm, bucket, false); err != nil {
		return err
	}
	if err := validateBucketCandidate(bucket, cand.GetIdentifier()); err != nil {
		return err
	}
	if validateBucketOwner(bucket, cand.Owner) != nil &&
		validateBucketWithEndorsement(ctx, esm, bucket, blkCtx.BlockHeight) != nil {
		return &handleError{
			err:           errors.New("bucket is not a self-owned or endorsed bucket"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	return nil
}
