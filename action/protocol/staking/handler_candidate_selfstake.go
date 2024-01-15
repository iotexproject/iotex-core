package staking

import (
	"context"
	"math"
	"math/big"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

const (
	handleCandidateSelfStake = "candidateSelfStake"

	candidateNoSelfStakeBucketIndex = math.MaxUint64
)

func (p *Protocol) handleCandidateSelfStake(ctx context.Context, act *action.CandidateActivate, csm CandidateStateManager,
) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), handleCandidateSelfStake, featureCtx.NewStakingReceiptFormat)

	// caller must be the owner of a candidate
	cand := csm.GetByOwner(actCtx.Caller)
	if cand == nil {
		return log, nil, errCandNotExist
	}

	bucket, rErr := p.fetchBucket(csm, act.BucketID())
	if rErr != nil {
		return log, nil, rErr
	}

	if err := p.validateBucketSelfStake(ctx, csm, NewEndorsementStateManager(csm.SM()), bucket, cand); err != nil {
		return log, nil, err
	}

	// convert previous self-stake bucket to vote bucket
	if cand.SelfStake.Cmp(big.NewInt(0)) > 0 {
		prevBucket, err := p.fetchBucket(csm, cand.SelfStakeBucketIdx)
		if err != nil {
			return log, nil, err
		}
		prevVotes := p.calculateVoteWeight(prevBucket, true)
		postVotes := p.calculateVoteWeight(prevBucket, false)
		if err := cand.SubVote(prevVotes.Sub(prevVotes, postVotes)); err != nil {
			return log, nil, err
		}
		cand.SelfStake = big.NewInt(0)
	}

	// convert vote bucket to self-stake bucket
	cand.SelfStakeBucketIdx = bucket.Index
	cand.SelfStake = big.NewInt(0).SetBytes(bucket.StakedAmount.Bytes())
	prevVotes := p.calculateVoteWeight(bucket, false)
	postVotes := p.calculateVoteWeight(bucket, true)
	if err := cand.AddVote(postVotes.Sub(postVotes, prevVotes)); err != nil {
		return log, nil, err
	}

	if err := csm.Upsert(cand); err != nil {
		return log, nil, csmErrorToHandleError(cand.Owner.String(), err)
	}

	return log, nil, nil
}

func (p *Protocol) validateBucketSelfStake(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, bucket *VoteBucket, cand *Candidate) ReceiptError {
	blkCtx := protocol.MustGetBlockCtx(ctx)
	if err := validateBucketMinAmount(bucket, p.config.RegistrationConsts.MinSelfStake); err != nil {
		return err
	}
	if err := validateBucketStake(bucket, true); err != nil {
		return err
	}
	if err := validateBucketSelfStake(csm, bucket, false); err != nil {
		return err
	}
	if err := validateBucketCandidate(bucket, cand.Owner); err != nil {
		return err
	}
	if validateBucketOwner(bucket, cand.Owner) != nil &&
		validateBucketEndorsement(esm, bucket, true, blkCtx.BlockHeight) != nil {
		return &handleError{
			err:           errors.New("bucket is not a self-owned or endorsed bucket"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}
	return nil
}
