package staking

import (
	"context"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	handleCandidateEndorsement = "candidateEndorsement"
)

func (p *Protocol) handleCandidateEndorsement(ctx context.Context, act *action.CandidateEndorsement, csm CandidateStateManager) (*receiptLog, []*action.TransactionLog, error) {
	actCtx := protocol.MustGetActionCtx(ctx)
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	log := newReceiptLog(p.addr.String(), handleCandidateEndorsement, featureCtx.NewStakingReceiptFormat)

	bucket, rErr := p.fetchBucket(csm, act.BucketIndex())
	if rErr != nil {
		return log, nil, rErr
	}
	cand := csm.GetByOwner(bucket.Candidate)
	if cand == nil {
		return log, nil, errCandNotExist
	}
	log.AddTopics(byteutil.Uint64ToBytesBigEndian(bucket.Index), bucket.Candidate.Bytes(), []byte{byteutil.BoolToByte(act.Endorse())})

	esm := NewEndorsementStateManager(csm.SM())
	// handle endorsement
	if act.Endorse() {
		if err := p.validateEndorsement(ctx, csm, esm, actCtx.Caller, bucket, cand); err != nil {
			return log, nil, err
		}
		// new endorsement with not expire height
		if err := esm.Put(bucket.Index, &Endorsement{
			ExpireHeight: endorsementNotExpireHeight,
		}); err != nil {
			return log, nil, errors.Wrapf(err, "failed to put endorsement with bucket index %d", bucket.Index)
		}
		return log, nil, nil
	}

	// handle withdrawal
	if err := p.validateEndorsementWithdrawal(ctx, esm, actCtx.Caller, bucket); err != nil {
		return log, nil, err
	}
	// expire immediately if the bucket is not self-staked
	// otherwise, expire after withdraw waiting period
	expireHeight := protocol.MustGetBlockCtx(ctx).BlockHeight
	if csm.ContainsSelfStakingBucket(bucket.Index) {
		expireHeight += p.config.EndorsementWithdrawWaitingBlocks
	}
	if err := esm.Put(bucket.Index, &Endorsement{
		ExpireHeight: expireHeight,
	}); err != nil {
		return log, nil, errors.Wrapf(err, "failed to put endorsement with bucket index %d", bucket.Index)
	}
	return log, nil, nil
}

func (p *Protocol) validateEndorsement(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket, cand *Candidate) ReceiptError {
	if err := validateBucketOwner(bucket, caller); err != nil {
		return err
	}
	if err := validateBucketMinAmount(bucket, p.config.RegistrationConsts.MinSelfStake); err != nil {
		return err
	}
	if err := validateBucketStake(bucket, true); err != nil {
		return err
	}
	if err := validateBucketCandidate(bucket, cand.Owner); err != nil {
		return err
	}
	if err := validateBucketSelfStake(csm, bucket, false); err != nil {
		return err
	}
	return validateBucketEndorsement(esm, bucket, false, protocol.MustGetBlockCtx(ctx).BlockHeight)
}

func (p *Protocol) validateEndorsementWithdrawal(ctx context.Context, esm *EndorsementStateManager, caller address.Address, bucket *VoteBucket) ReceiptError {
	if err := validateBucketOwner(bucket, caller); err != nil {
		return err
	}
	return validateBucketEndorsement(esm, bucket, true, protocol.MustGetBlockCtx(ctx).BlockHeight)
}
