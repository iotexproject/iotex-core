package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	handleCandidateEndorsement       = "candidateEndorsement"
	handleCandidateEndorsementWithOp = "candidateEndorsementWithOp"
)

func (p *Protocol) handleCandidateEndorsement(ctx context.Context, act *action.CandidateEndorsement, csm CandidateStateManager) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	var log *receiptLog
	if featureCtx.EnforceLegacyEndorsement {
		log = newReceiptLog(p.addr.String(), handleCandidateEndorsement, featureCtx.NewStakingReceiptFormat)
	} else {
		log = newReceiptLog(p.addr.String(), handleCandidateEndorsementWithOp, featureCtx.NewStakingReceiptFormat)
	}

	bucket, rErr := p.fetchBucket(csm, act.BucketIndex())
	if rErr != nil {
		return log, nil, rErr
	}
	cand := csm.GetByIdentifier(bucket.Candidate)
	if cand == nil {
		return log, nil, errCandNotExist
	}
	if featureCtx.EnforceLegacyEndorsement {
		log.AddTopics(byteutil.Uint64ToBytesBigEndian(bucket.Index), bucket.Candidate.Bytes(), []byte{byteutil.BoolToByte(act.Op() == action.CandidateEndorsementOpEndorse)})
	} else {
		log.AddTopics(byteutil.Uint64ToBytesBigEndian(bucket.Index), bucket.Candidate.Bytes(), byteutil.Uint32ToBytesBigEndian(uint32(act.Op())))
	}

	esm := NewEndorsementStateManager(csm.SM())
	expireHeight := uint64(0)
	switch act.Op() {
	case action.CandidateEndorsementOpEndorse:
		// handle endorsement
		if err := p.validateEndorsement(ctx, csm, esm, actCtx.Caller, bucket, cand); err != nil {
			return log, nil, err
		}
		expireHeight = uint64(endorsementNotExpireHeight)
	case action.CandidateEndorsementOpIntentToRevoke:
		// handle withdrawal
		if err := p.validateIntentToRevokeEndorsement(ctx, esm, actCtx.Caller, bucket); err != nil {
			return log, nil, err
		}
		// expire immediately if the bucket is not used to activate
		expireHeight = protocol.MustGetBlockCtx(ctx).BlockHeight
		// otherwise, expire after waiting period
		selfStake, err := isSelfStakeBucket(featureCtx, csm, bucket)
		if err != nil {
			return log, nil, err
		}
		if selfStake {
			expireHeight += p.config.EndorsementWithdrawWaitingBlocks
		}
	case action.CandidateEndorsementOpRevoke:
		if err := p.validateRevokeEndorsement(ctx, esm, actCtx.Caller, bucket); err != nil {
			return log, nil, err
		}
		// clear self-stake if the endorse bucket is used
		if cand.SelfStakeBucketIdx == bucket.Index {
			if err := p.clearCandidateSelfStake(bucket, cand); err != nil {
				return log, nil, errors.Wrap(err, "failed to clear candidate self-stake")
			}
			if err := csm.Upsert(cand); err != nil {
				return log, nil, csmErrorToHandleError(actCtx.Caller.String(), err)
			}
		}
		if err := esm.Delete(bucket.Index); err != nil {
			return log, nil, errors.Wrapf(err, "failed to delete endorsement with bucket index %d", bucket.Index)
		}
		return log, nil, nil
	default:
		return log, nil, errors.New("invalid operation")
	}
	// update endorsement state
	if err := esm.Put(bucket.Index, &Endorsement{
		ExpireHeight: expireHeight,
	}); err != nil {
		return log, nil, errors.Wrapf(err, "failed to put endorsement with bucket index %d", bucket.Index)
	}
	return log, nil, nil
}

func (p *Protocol) validateEndorsement(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket, cand *Candidate) ReceiptError {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	if err := validateBucketOwner(bucket, caller); err != nil {
		return err
	}
	if err := validateBucketMinAmount(bucket, p.config.RegistrationConsts.MinSelfStake); err != nil {
		return err
	}
	if err := validateBucketStake(bucket, true); err != nil {
		return err
	}
	if err := validateBucketCandidate(bucket, cand.GetIdentifier()); err != nil {
		return err
	}
	if err := validateBucketSelfStake(featureCtx, csm, bucket, false); err != nil {
		return err
	}
	return validateBucketWithoutEndorsement(ctx, esm, bucket, protocol.MustGetBlockCtx(ctx).BlockHeight)
}

func (p *Protocol) validateIntentToRevokeEndorsement(ctx context.Context, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket) ReceiptError {
	if err := validateBucketOwner(bucket, caller); err != nil {
		return err
	}
	return validateBucketEndorsementWithdrawal(ctx, esm, bucket, protocol.MustGetBlockCtx(ctx).BlockHeight)
}

func (p *Protocol) validateRevokeEndorsement(ctx context.Context, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket) ReceiptError {
	if err := validateBucketOwner(bucket, caller); err != nil {
		return err
	}
	status, err := esm.Status(protocol.MustGetFeatureCtx(ctx), bucket.Index, protocol.MustGetBlockCtx(ctx).BlockHeight)
	if err != nil {
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}
	if status != UnEndorsing {
		return &handleError{
			err:           errors.Errorf("bucket is not ready to revoke endorsement"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func (p *Protocol) clearCandidateSelfStake(bucket *VoteBucket, cand *Candidate) error {
	if cand.SelfStakeBucketIdx != bucket.Index {
		return errors.New("self-stake bucket index mismatch")
	}
	if err := cand.SubVote(p.calculateVoteWeight(bucket, true)); err != nil {
		return errors.Wrapf(err, "failed to subtract vote weight for bucket index %d", bucket.Index)
	}
	if err := cand.AddVote(p.calculateVoteWeight(bucket, false)); err != nil {
		return errors.Wrapf(err, "failed to add vote weight for bucket index %d", bucket.Index)
	}
	cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	cand.SelfStake.SetInt64(0)
	return nil
}
