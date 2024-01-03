package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action/protocol"
)

type bucketValidator func(*bucketValidation)

type bucketValidation struct {
	owner      address.Address
	minAmount  *big.Int
	staked     *bool
	candidate  address.Address
	selfStaked *bool
	endorsed   *bool
}

func withBucketOwner(owner address.Address) bucketValidator {
	return func(v *bucketValidation) {
		v.owner = owner
	}
}

func withBucketMinAmount(amount *big.Int) bucketValidator {
	return func(v *bucketValidation) {
		v.minAmount = amount
	}
}

func withBucketStake(isStaked bool) bucketValidator {
	return func(v *bucketValidation) {
		v.staked = &isStaked
	}
}

func withBucketCandidate(candidate address.Address) bucketValidator {
	return func(v *bucketValidation) {
		v.candidate = candidate
	}
}

func withBucketSelfStaked(isSelfStaked bool) bucketValidator {
	return func(v *bucketValidation) {
		v.selfStaked = &isSelfStaked
	}
}

func withBucketEndorsed(isEndorsed bool) bucketValidator {
	return func(v *bucketValidation) {
		v.endorsed = &isEndorsed
	}
}

func validateBucket(ctx context.Context, csm CandidateStateManager, esm *EndorsementStateManager, bucket *VoteBucket, validators ...bucketValidator) ReceiptError {
	validation := bucketValidation{}
	for _, v := range validators {
		v(&validation)
	}

	// check bucket owner
	if validation.owner != nil {
		if !address.Equal(validation.owner, bucket.Owner) {
			return &handleError{
				err:           errors.New("bucket owner does not match"),
				failureStatus: iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
			}
		}
	}
	// check bucket amount
	if validation.minAmount != nil {
		if bucket.StakedAmount.Cmp(validation.minAmount) < 0 {
			return &handleError{
				err:           errors.New("bucket amount is unsufficient"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketAmount,
			}
		}
	}
	// check bucket is whether staked
	if validation.staked != nil {
		if bucket.isUnstaked() == *validation.staked {
			err := errors.New("bucket is staked")
			if *validation.staked {
				err = errors.New("bucket is unstaked")
			}
			return &handleError{
				err:           err,
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
	}
	// check bucket is voted to the candidate
	if validation.candidate != nil {
		if !address.Equal(bucket.Candidate, validation.candidate) {
			return &handleError{
				err:           errors.New("bucket is not voted to the candidate"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
	}
	// check bucket is whether self-stake bucket
	if validation.selfStaked != nil {
		if csm.ContainsSelfStakingBucket(bucket.Index) != *validation.selfStaked {
			err := errors.New("self staking bucket cannot be processed")
			if *validation.selfStaked {
				err = errors.New("bucket is not self staking")
			}
			return &handleError{
				err:           err,
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
	}
	// check bucket is whether endorsed
	if validation.endorsed != nil {
		blkCtx := protocol.MustGetBlockCtx(ctx)
		endorse, err := esm.Get(bucket.Index)
		if err != nil {
			return &handleError{
				err:           err,
				failureStatus: iotextypes.ReceiptStatus_ErrUnknown,
			}
		}
		if endorse != nil {
			status := endorse.Status(blkCtx.BlockHeight)
			if *validation.endorsed && status == NotEndorsed {
				return &handleError{
					err:           errors.New("bucket is not endorsed"),
					failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
				}
			}
			if !*validation.endorsed && status != NotEndorsed {
				return &handleError{
					err:           errors.New("bucket is already endorsed"),
					failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
				}
			}
		} else if *validation.endorsed {
			return &handleError{
				err:           errors.New("bucket is not endorsed"),
				failureStatus: iotextypes.ReceiptStatus_ErrInvalidBucketType,
			}
		}
	}

	return nil
}
