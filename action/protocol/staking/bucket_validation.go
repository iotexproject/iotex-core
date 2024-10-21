package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

func validateBucketOwner(bucket *VoteBucket, owner address.Address) ReceiptError {
	if address.Equal(owner, bucket.Owner) {
		return nil
	}
	return &handleError{
		err:           errors.New("bucket owner does not match"),
		failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
	}
}

func validateBucketMinAmount(bucket *VoteBucket, minAmount *big.Int) ReceiptError {
	if bucket.StakedAmount.Cmp(minAmount) < 0 {
		return &handleError{
			err:           errors.New("bucket amount is unsufficient"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
		}
	}
	return nil
}

func validateBucketStake(bucket *VoteBucket, isStaked bool) ReceiptError {
	if bucket.isUnstaked() == isStaked {
		err := errors.New("bucket is staked")
		if isStaked {
			err = errors.New("bucket is unstaked")
		}
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func validateBucketCandidate(bucket *VoteBucket, candidate address.Address) ReceiptError {
	if !address.Equal(bucket.Candidate, candidate) {
		return &handleError{
			err:           errors.New("bucket is not voted to the candidate"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func validateBucketSelfStake(featureCtx protocol.FeatureCtx, csm CandidateStateManager, bucket *VoteBucket, isSelfStaked bool) ReceiptError {
	selfstake, err := isSelfStakeBucket(featureCtx, csm, bucket)
	if err != nil {
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}
	if selfstake != isSelfStaked {
		err := errors.New("self staking bucket cannot be processed")
		if isSelfStaked {
			err = errors.New("bucket is not self staking")
		}
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func validateBucketWithEndorsement(ctx context.Context, esm *EndorsementStateManager, bucket *VoteBucket, height uint64) ReceiptError {
	featureCtx := protocol.MustGetFeatureCtx(ctx)
	status, err := esm.Status(featureCtx, bucket.Index, height)
	if err != nil {
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}
	if (!featureCtx.EnforceLegacyEndorsement && status != Endorsed) ||
		(featureCtx.EnforceLegacyEndorsement && status != Endorsed && status != UnEndorsing) {
		return &handleError{
			err:           errors.Errorf("bucket is not an endorse bucket"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func validateBucketWithoutEndorsement(ctx context.Context, esm *EndorsementStateManager, bucket *VoteBucket, height uint64) ReceiptError {
	status, err := esm.Status(protocol.MustGetFeatureCtx(ctx), bucket.Index, height)
	if err != nil {
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}
	if status != EndorseExpired {
		return &handleError{
			err:           errors.Errorf("bucket is still endorsed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}

func validateBucketEndorsementWithdrawal(ctx context.Context, esm *EndorsementStateManager, bucket *VoteBucket, height uint64) ReceiptError {
	status, err := esm.Status(protocol.MustGetFeatureCtx(ctx), bucket.Index, height)
	if err != nil {
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}
	if status != Endorsed {
		return &handleError{
			err:           errors.Errorf("bucket is not endorsed"),
			failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
		}
	}
	return nil
}
