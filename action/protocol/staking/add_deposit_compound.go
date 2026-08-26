// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// ErrCompoundBucketOwnerMismatch is returned when the bucket's owner does not
// byte-equal the voter passed to AddDepositForCompound. Callers (currently only
// the rewarding protocol's distributeVoterReward) are expected to have already
// filtered via autodeposit.IsBucketEligibleForCompound; hitting this indicates
// a wiring bug, not on-chain data.
var ErrCompoundBucketOwnerMismatch = errors.New(
	"staking: compound bucket owner does not match voter")

// ErrCompoundSelfStakeRoleChanged is returned when the bucket's self-stake role
// is not the one the era froze: it became the candidate's self-stake bucket
// after the boundary, or stopped being it. The share was allocated against the
// frozen role, so compounding into the bucket now would grow a bucket whose
// weight the era never blessed and would move the candidate's votes by a
// different multiplier than the payout assumed.
//
// It is a routing outcome, not a failure: the caller falls back to crediting
// the voter's reward destination, exactly as it does for an ineligible bucket.
var ErrCompoundSelfStakeRoleChanged = errors.New(
	"staking: compound bucket self-stake role changed since the era freeze")

// AddDepositForCompound applies an IIP-59 §3.6 compound deposit into voter's
// registered AutoDeposit bucket. It is the rewarding-side counterpart to
// handleDepositToStake: same in-place bucket + candidate + bucket-pool
// updates, but without the user-action plumbing (no signature check, no gas,
// no caller balance debit, no receipt log — the caller emits a batched
// DelegateVoterRewardsDistributed log instead).
//
// Preconditions the caller MUST have already established:
//
//  1. bucketID came from AutoDeposit.bucket(voter) and is strictly positive
//     (via autodeposit.Bridge.LookupBucket).
//  2. The bucket at bucketID is native, active (not unstaked), has AutoStake
//     set, and its Owner byte-equals voter (via
//     autodeposit.IsBucketEligibleForCompound).
//
// This function re-checks the owner match as a safety net and errors out
// otherwise — the check is cheap and the failure mode of quietly compounding
// into someone else's bucket is unacceptable.
//
// State mutations mirror handleDepositToStake exactly:
//   - bucket.StakedAmount += amount, persisted via csm.updateBucket
//   - candidate weighted-vote recomputed (SubVote(prev) then AddVote(next))
//   - candidate.SelfStake += amount when the bucket is a self-stake bucket
//     (rare here — a voter compounding into their own self-stake bucket —
//     but respected for parity)
//   - bucket pool total grows by amount via DebitBucketPool
//
// No transaction log is returned: the caller wraps the whole per-delegate
// distribution into a single DelegateVoterRewardsDistributed batched log, and the
// rewarding→bucket-pool token movement is captured in that batch's
// transactionLogs slice at the call site.
func (p *Protocol) AddDepositForCompound(
	ctx context.Context,
	sm protocol.StateManager,
	voter address.Address,
	bucketID uint64,
	amount *big.Int,
	era FrozenSelfStake,
) error {
	if voter == nil {
		return errors.New("staking: nil voter address")
	}
	if amount == nil {
		return errors.New("staking: nil compound amount")
	}
	if amount.Sign() <= 0 {
		return errors.New("staking: non-positive compound amount")
	}

	csm, err := NewCandidateStateManagerWithContext(ctx, sm)
	if err != nil {
		return errors.Wrap(err, "staking: build csm for compound")
	}

	bucket, rErr := p.fetchBucket(csm, bucketID)
	if rErr != nil {
		return errors.Wrapf(rErr, "staking: fetch bucket %d for compound", bucketID)
	}
	if !address.Equal(bucket.Owner, voter) {
		return errors.Wrapf(ErrCompoundBucketOwnerMismatch,
			"bucket=%d voter=%s owner=%s",
			bucketID, voter.String(), bucket.Owner.String())
	}

	candidate := csm.GetByIdentifier(bucket.Candidate)
	if candidate == nil {
		return errors.Wrapf(errCandNotExist,
			"staking: candidate %s missing for compound bucket %d",
			bucket.Candidate.String(), bucketID)
	}

	featureCtx := protocol.MustGetFeatureCtx(ctx)
	// The self-stake flag below stays live on purpose. It drives the
	// sub-then-add that keeps candidate.Votes equal to the sum of its buckets'
	// current weights; feeding it the frozen role would subtract a weight the
	// candidate does not actually hold and leave the running total corrupt for
	// every later handler. Only the drain's *share* decisions use frozen state.
	//
	// What the frozen role is used for is the guard right after: if it and the
	// live role disagree, this deposit does not belong in this bucket at all,
	// and bailing out before the first mutation keeps that a routing decision
	// rather than a half-applied one.
	selfStake, err := isSelfStakeBucket(featureCtx, csm, bucket)
	if err != nil {
		return errors.Wrap(err, "staking: self-stake check for compound")
	}
	if era.Known() && era.Covers(bucketID) != csm.ContainsSelfStakingBucket(bucketID) {
		return errors.Wrapf(ErrCompoundSelfStakeRoleChanged,
			"bucket=%d frozenSelfStake=%t liveSelfStake=%t",
			bucketID, era.Covers(bucketID), csm.ContainsSelfStakingBucket(bucketID))
	}

	prevWeighted := p.calculateVoteWeight(bucket, selfStake)
	bucket.StakedAmount = new(big.Int).Add(bucket.StakedAmount, amount)
	if err := csm.updateBucket(bucketID, bucket); err != nil {
		return errors.Wrapf(err, "staking: update compound bucket %d", bucketID)
	}

	if err := candidate.SubVote(prevWeighted); err != nil {
		return errors.Wrapf(err, "staking: subtract vote for candidate %s", bucket.Candidate.String())
	}
	newWeighted := p.calculateVoteWeight(bucket, selfStake)
	if err := candidate.AddVote(newWeighted); err != nil {
		return errors.Wrapf(err, "staking: add vote for candidate %s", bucket.Candidate.String())
	}
	if selfStake {
		if err := candidate.AddSelfStake(amount); err != nil {
			return errors.Wrapf(err, "staking: add self-stake for candidate %s", bucket.Candidate.String())
		}
	}
	if err := csm.Upsert(candidate); err != nil {
		return csmErrorToHandleError(candidate.GetIdentifier().String(), err)
	}

	if err := csm.DebitBucketPool(amount, false); err != nil {
		return errors.Wrap(err, "staking: debit bucket pool for compound")
	}
	return nil
}
