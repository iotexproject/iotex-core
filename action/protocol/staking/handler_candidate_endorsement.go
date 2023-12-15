package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
)

const (
	handleCandidateEndorsement = "candidateEndorsement"
)

func (p *Protocol) handleCandidateEndorsement(ctx context.Context, act *action.CandidateEndorsement, csm CandidateStateManager) (*receiptLog, []*action.TransactionLog, error) {
	var (
		bucket *VoteBucket
		err    error
		rErr   ReceiptError
		txLogs []*action.TransactionLog
		cand   *Candidate

		actCtx     = protocol.MustGetActionCtx(ctx)
		featureCtx = protocol.MustGetFeatureCtx(ctx)
		log        = newReceiptLog(p.addr.String(), handleCandidateEndorsement, featureCtx.NewStakingReceiptFormat)
	)
	esm := NewEndorsementStateManager(csm.SM())
	bucket, rErr = p.fetchBucket(csm, act.BucketIndex())
	if rErr != nil {
		return log, nil, rErr
	}
	cand = csm.GetByOwner(bucket.Candidate)
	if cand == nil {
		return log, nil, errCandNotExist
	}

	if act.Endorse() {
		err = p.endorseCandidate(ctx, csm, esm, actCtx.Caller, bucket, cand)
	} else {
		err = p.unEndorseCandidate(ctx, csm, esm, actCtx.Caller, bucket, cand)
	}
	if err != nil {
		return log, nil, err
	}

	return log, txLogs, nil
}

func (p *Protocol) endorseCandidate(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket, cand *Candidate) error {
	if err := p.validateEndorse(ctx, csm, esm, caller, bucket, cand); err != nil {
		return err
	}

	if err := esm.Put(bucket.Index, &Endorsement{
		ExpireHeight: endorsementNotExpireHeight,
	}); err != nil {
		return csmErrorToHandleError(caller.String(), err)
	}
	return nil
}

func (p *Protocol) unEndorseCandidate(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket, cand *Candidate) error {
	blkCtx := protocol.MustGetBlockCtx(ctx)

	if err := p.validateUnEndorse(ctx, csm, esm, caller, bucket); err != nil {
		return err
	}
	if err := esm.Put(bucket.Index, &Endorsement{
		ExpireHeight: blkCtx.BlockHeight + p.config.UnEndorseWaitingBlocks,
	}); err != nil {
		return csmErrorToHandleError(caller.String(), err)
	}
	return nil
}

func (p *Protocol) validateEndorse(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket, cand *Candidate) ReceiptError {
	validators := []bucketValidator{
		withBucketOwner(caller),
		withBucketMinAmount(p.config.RegistrationConsts.MinSelfStake),
		withBucketStake(true),
		withBucketCandidate(cand.Owner),
		withBucketSelfStaked(false),
		withBucketEndorsed(false),
	}
	return validateBucket(ctx, csm, esm, bucket, validators...)
}

func (p *Protocol) validateUnEndorse(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket) ReceiptError {
	if validateBucket(ctx, csm, esm, bucket, withBucketOwner(caller)) != nil &&
		validateBucket(ctx, csm, esm, bucket, withBucketOwner(caller)) != nil {
		return &handleError{
			err:           errors.New("bucket owner or candidate does not match"),
			failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		}
	}

	return validateBucket(ctx, csm, esm, bucket, withBucketEndorsed(true))
}
