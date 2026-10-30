// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

import (
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// newActiveNativeBucket returns a bucket that passes every eligibility
// precondition — the "happy path" bucket for the tests below to then
// mutate one field at a time.
func newActiveNativeBucket(owner, candidate address.Address) *staking.VoteBucket {
	return staking.NewVoteBucket(
		candidate,
		owner,
		big.NewInt(1_000_000),
		30,
		time.Now().UTC(),
		true, // AutoStake
	)
}

func TestIsBucketEligibleForCompound_NilBucket(t *testing.T) {
	r := require.New(t)
	voter := identityset.Address(1)
	r.False(IsBucketEligibleForCompound(nil, voter))
}

func TestIsBucketEligibleForCompound_NilVoter(t *testing.T) {
	// Guard against caller-side nil address slipping through: callers of
	// LookupBucket already reject nil, but the eligibility helper must
	// stand on its own since PR 3' calls it independently.
	r := require.New(t)
	owner := identityset.Address(1)
	candidate := identityset.Address(2)
	bucket := newActiveNativeBucket(owner, candidate)
	r.False(IsBucketEligibleForCompound(bucket, nil))
}

func TestIsBucketEligibleForCompound_WrongOwner(t *testing.T) {
	r := require.New(t)
	owner := identityset.Address(1)
	stranger := identityset.Address(3)
	candidate := identityset.Address(2)
	bucket := newActiveNativeBucket(owner, candidate)
	r.False(IsBucketEligibleForCompound(bucket, stranger))
}

func TestIsBucketEligibleForCompound_AutoStakeFalse(t *testing.T) {
	// Even with the correct owner, a non-auto-stake bucket must not be
	// compounded — parity with Hermes' current filter.
	r := require.New(t)
	owner := identityset.Address(1)
	candidate := identityset.Address(2)
	bucket := newActiveNativeBucket(owner, candidate)
	bucket.AutoStake = false
	r.False(IsBucketEligibleForCompound(bucket, owner))
}

func TestIsBucketEligibleForCompound_Unstaked(t *testing.T) {
	// An unstaked bucket can no longer accept deposits — compound routing
	// must exclude it and let the share flow to unclaimedBalance.
	r := require.New(t)
	owner := identityset.Address(1)
	candidate := identityset.Address(2)
	bucket := newActiveNativeBucket(owner, candidate)
	bucket.UnstakeStartTime = bucket.StakeStartTime.Add(time.Hour)
	r.True(bucket.IsUnstaked(), "sanity: fixture must actually be unstaked")
	r.False(IsBucketEligibleForCompound(bucket, owner))
}

func TestIsBucketEligibleForCompound_ContractStaking(t *testing.T) {
	// Contract-staking (LSD) buckets are owned by the staking contract,
	// not the token holder — bucket.Owner would never equal the voter in
	// production. Assert the ineligibility even if a synthetic bucket
	// were to somehow satisfy the Owner check.
	r := require.New(t)
	owner := identityset.Address(1)
	candidate := identityset.Address(2)
	bucket := newActiveNativeBucket(owner, candidate)
	bucket.ContractAddress = "io1abcdefghijklmnopqrstuvwxyz1234567890abc"
	r.False(bucket.IsNative(), "sanity: fixture must be non-native")
	r.False(IsBucketEligibleForCompound(bucket, owner))
}

func TestIsBucketEligibleForCompound_HappyPath(t *testing.T) {
	r := require.New(t)
	owner := identityset.Address(1)
	candidate := identityset.Address(2)
	bucket := newActiveNativeBucket(owner, candidate)
	r.True(IsBucketEligibleForCompound(bucket, owner))
}
