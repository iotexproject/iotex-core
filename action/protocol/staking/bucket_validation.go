package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/state"
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

func validateBucketEndorsement(esm *EndorsementStateManager, bucket *VoteBucket, isEndorsed bool, height uint64) ReceiptError {
	endorse, err := esm.Get(bucket.Index)
	switch {
	case err == nil:
		status := endorse.Status(height)
		if isEndorsed && status == EndorseExpired {
			return &handleError{
				err:           errors.New("endorse bucket is expired"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
		if !isEndorsed && status != EndorseExpired {
			return &handleError{
				err:           errors.New("bucket is already endorsed"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
	case errors.Is(err, state.ErrStateNotExist):
		if isEndorsed {
			return &handleError{
				err:           errors.New("bucket is not an endorse bucket"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
	default:
		return &handleError{
			err:           err,
			failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
		}
	}
	return nil
}
