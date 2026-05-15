package staking

import (
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// isSelfStakeBucket returns true if the bucket is self-stake bucket and not expired
func isSelfStakeBucket(featureCtx protocol.FeatureCtx, csc CandidiateStateCommon, bucket *VoteBucket) (bool, error) {
	// bucket index should be settled in one of candidates
	selfStake := csc.ContainsSelfStakingBucket(bucket.Index)
	if featureCtx.DisableDelegateEndorsement || !selfStake {
		return selfStake, nil
	}

	// bucket should not be unstaked if it is self-owned
	if isSelfOwnedBucket(csc, bucket) {
		return !bucket.isUnstaked(), nil
	}
	// otherwise bucket should be an endorse bucket which is not expired
	esm := NewEndorsementStateReader(csc.SR())
	height, err := esm.Height()
	if err != nil {
		return false, err
	}
	status, err := esm.Status(featureCtx, bucket.Index, height)
	if err != nil {
		return false, err
	}
	return status != EndorseExpired, nil
}

func isSelfOwnedBucket(csc CandidiateStateCommon, bucket *VoteBucket) bool {
	cand := csc.GetByIdentifier(bucket.Candidate)
	if cand == nil {
		return false
	}
	return address.Equal(bucket.Owner, cand.Owner)
}
