// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package autodeposit

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// IsBucketEligibleForCompound applies IIP-59 §3.6's preconditions 2-4:
//
//  2. The bucket exists in staking state (nil bucket ⇒ ineligible).
//  3. The bucket is a native bucket. LSD / contract-staking buckets are
//     owned by the staking contract, so bucket.Owner never equals the
//     token holder — they are structurally ineligible; parity with today's
//     Hermes behaviour (see IIP-59 §3.6 "LSD parity").
//  4. bucket.AutoStake is true, the bucket is currently active
//     (not unstaked), and bucket.Owner byte-equals voter.
//
// Precondition 1 (non-zero bucket ID from the AutoDeposit contract) is
// handled by Bridge.LookupBucket; this helper takes over once a candidate
// bucket has been fetched from staking state.
//
// The helper is total: nil bucket, nil voter, or any missing precondition
// returns false. It never returns an error so callers can chain it into
// per-voter routing without branching.
func IsBucketEligibleForCompound(bucket *staking.VoteBucket, voter address.Address) bool {
	if bucket == nil || voter == nil {
		return false
	}
	if !bucket.IsNative() {
		return false
	}
	if !bucket.AutoStake {
		return false
	}
	if bucket.IsUnstaked() {
		return false
	}
	return address.Equal(bucket.Owner, voter)
}
