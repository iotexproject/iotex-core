package staking

import (
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil/testdb"
)

func TestValidateBucket(t *testing.T) {
	r := require.New(t)
	initState := func() (CandidateStateManager, *EndorsementStateManager) {
		ctrl := gomock.NewController(t)
		sm := testdb.NewMockStateManager(ctrl)
		v, _, err := CreateBaseView(sm, false)
		r.NoError(err)
		sm.WriteView(_protocolID, v)
		csm, err := NewCandidateStateManager(sm, false)
		r.NoError(err)
		esm := NewEndorsementStateManager(sm)
		return csm, esm
	}
	t.Run("validate bucket owner", func(t *testing.T) {
		csm, _ := initState()
		owner1 := identityset.Address(1)
		owner2 := identityset.Address(2)
		bkt := NewVoteBucket(owner1, owner1, big.NewInt(0), 1, time.Now(), false)
		_, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		r.Nil(validateBucketOwner(bkt, owner1))
		r.ErrorContains(validateBucketOwner(bkt, owner2), "bucket owner does not match")
	})
	t.Run("validate bucket min amount", func(t *testing.T) {
		csm, _ := initState()
		owner := identityset.Address(1)
		bkt := NewVoteBucket(owner, owner, big.NewInt(10000), 1, time.Now(), false)
		_, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		r.Nil(validateBucketMinAmount(bkt, big.NewInt(1000)))
		r.ErrorContains(validateBucketMinAmount(bkt, big.NewInt(100000)), "bucket amount is unsufficient")
	})
	t.Run("validate bucket staked", func(t *testing.T) {
		csm, _ := initState()
		owner := identityset.Address(1)
		// staked bucket
		bkt := NewVoteBucket(owner, owner, big.NewInt(10000), 1, time.Now(), false)
		_, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		r.Nil(validateBucketStake(bkt, true))
		r.ErrorContains(validateBucketStake(bkt, false), "bucket is staked")
		// unstaked bucket
		bkt.UnstakeStartTime = bkt.StakeStartTime.Add(1 * time.Hour)
		_, err = csm.putBucketAndIndex(bkt)
		r.NoError(err)
		r.Nil(validateBucketStake(bkt, false))
		r.ErrorContains(validateBucketStake(bkt, true), "bucket is unstaked")
	})
	t.Run("validate bucket candidate", func(t *testing.T) {
		csm, _ := initState()
		owner := identityset.Address(1)
		candidate1 := identityset.Address(2)
		candidate2 := identityset.Address(3)
		bkt := NewVoteBucket(candidate1, owner, big.NewInt(10000), 1, time.Now(), false)
		_, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		r.Nil(validateBucketCandidate(bkt, candidate1))
		r.ErrorContains(validateBucketCandidate(bkt, candidate2), "bucket is not voted to the candidate")
	})
	t.Run("validate bucket self staked", func(t *testing.T) {
		csm, _ := initState()
		owner := identityset.Address(1)
		candidate := identityset.Address(2)
		// not selfstaked bucket
		bkt := NewVoteBucket(candidate, owner, big.NewInt(10000), 1, time.Now(), false)
		bktIdx, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		r.Nil(validateBucketSelfStake(csm, bkt, false))
		r.ErrorContains(validateBucketSelfStake(csm, bkt, true), "bucket is not self staking")
		// selfstaked bucket
		r.NoError(csm.Upsert(&Candidate{
			Owner:              candidate,
			Operator:           candidate,
			Reward:             candidate,
			Name:               "test",
			Votes:              big.NewInt(10000),
			SelfStakeBucketIdx: bktIdx,
			SelfStake:          big.NewInt(10000),
		}))
		r.Nil(validateBucketSelfStake(csm, bkt, true))
		r.ErrorContains(validateBucketSelfStake(csm, bkt, false), "self staking bucket cannot be processed")
	})
	t.Run("validate bucket endorsed", func(t *testing.T) {
		csm, esm := initState()
		owner := identityset.Address(1)
		candidate := identityset.Address(2)
		bkt := NewVoteBucket(candidate, owner, big.NewInt(10000), 1, time.Now(), false)
		bktIdx, err := csm.putBucketAndIndex(bkt)
		r.NoError(err)
		blkHeight := uint64(10)
		// not endorsed bucket
		r.Nil(validateBucketEndorsement(esm, bkt, false, blkHeight))
		r.ErrorContains(validateBucketEndorsement(esm, bkt, true, blkHeight), "bucket is not endorsed")
		// endorsed bucket
		r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: endorsementNotExpireHeight}))
		r.Nil(validateBucketEndorsement(esm, bkt, true, blkHeight))
		r.ErrorContains(validateBucketEndorsement(esm, bkt, false, blkHeight), "bucket is already endorsed")
		// unendorsing bucket
		r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: blkHeight + 1}))
		r.Nil(validateBucketEndorsement(esm, bkt, true, blkHeight))
		r.ErrorContains(validateBucketEndorsement(esm, bkt, false, blkHeight), "bucket is already endorsed")
		// endorse expired bucket
		r.NoError(esm.Put(bktIdx, &Endorsement{ExpireHeight: blkHeight}))
		r.Nil(validateBucketEndorsement(esm, bkt, false, blkHeight))
		r.ErrorContains(validateBucketEndorsement(esm, bkt, true, blkHeight), "bucket is not endorsed")
	})
}
