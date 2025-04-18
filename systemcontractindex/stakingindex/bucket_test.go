package stakingindex

import (
	"fmt"
	"math/big"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestBucket_SerializeDeserialize(t *testing.T) {
	r := require.New(t)
	candidate := identityset.Address(1)
	owner := identityset.Address(2)

	original := &Bucket{
		Candidate:      candidate,
		Owner:          owner,
		StakedAmount:   big.NewInt(123456),
		Timestamped:    true,
		StakedDuration: 3600,
		CreatedAt:      111111,
		UnlockedAt:     222222,
		UnstakedAt:     333333,
		Muted:          true,
	}
	data := original.Serialize()

	var deserialized Bucket
	r.NoError(deserialized.Deserialize(data), "Deserialize failed")
	r.Equal(*original, deserialized)
}

func TestBucket_Clone(t *testing.T) {
	r := require.New(t)
	candidate := identityset.Address(1)
	owner := identityset.Address(2)
	original := &Bucket{
		Candidate:      candidate,
		Owner:          owner,
		StakedAmount:   big.NewInt(123456),
		Muted:          true,
		Timestamped:    true,
		StakedDuration: 3600,
		CreatedAt:      111111,
		UnlockedAt:     222222,
		UnstakedAt:     333333,
	}

	clone := original.Clone()
	r.Equal(*original, *clone, "Clone failed")
}

func TestAssembleVoteBucket(t *testing.T) {
	r := require.New(t)
	mockBlocksToDuration := func(height, end uint64) time.Duration {
		return time.Duration(end-height) * time.Second
	}
	candidate := identityset.Address(1)
	owner := identityset.Address(2)
	contract := identityset.Address(3)

	cases := []struct {
		name   string
		bucket *Bucket
	}{
		{"timestamped", &Bucket{
			Candidate:      candidate,
			Owner:          owner,
			StakedAmount:   big.NewInt(1000),
			Timestamped:    true,
			StakedDuration: 3600,
			CreatedAt:      111111,
			UnlockedAt:     222222,
			UnstakedAt:     333333,
			Muted:          false,
		}},
		{"timestamped/muted", &Bucket{
			Candidate:      candidate,
			Owner:          owner,
			StakedAmount:   big.NewInt(1000),
			Timestamped:    true,
			StakedDuration: 3600,
			CreatedAt:      111111,
			UnlockedAt:     222222,
			UnstakedAt:     333333,
			Muted:          true,
		}},
		{"block-based", &Bucket{
			Candidate:      candidate,
			Owner:          owner,
			StakedAmount:   big.NewInt(2000),
			Timestamped:    false,
			StakedDuration: 100,
			CreatedAt:      10,
			UnlockedAt:     20,
			UnstakedAt:     110,
			Muted:          false,
		}},
		{"block-based/muted", &Bucket{
			Candidate:      candidate,
			Owner:          owner,
			StakedAmount:   big.NewInt(2000),
			Timestamped:    false,
			StakedDuration: 100,
			CreatedAt:      10,
			UnlockedAt:     20,
			UnstakedAt:     110,
			Muted:          true,
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			idx := rand.Uint64N(100)
			vb := assembleVoteBucket(idx, c.bucket, contract.String(), mockBlocksToDuration)
			expectCandidate := c.bucket.Candidate
			if c.bucket.Muted {
				expectCandidate, _ = address.FromString(address.ZeroAddress)
			}
			if c.bucket.Timestamped {
				stakeStartTime := time.Unix(int64(c.bucket.CreatedAt), 0)
				unstakeStartTime := time.Unix(0, 0)
				if c.bucket.UnlockedAt != maxStakingNumber {
					stakeStartTime = time.Unix(int64(c.bucket.UnlockedAt), 0)
				}
				if c.bucket.UnstakedAt != maxStakingNumber {
					unstakeStartTime = time.Unix(int64(c.bucket.UnstakedAt), 0)
				}
				r.Equal(VoteBucket{
					Index:            idx,
					StakedAmount:     c.bucket.StakedAmount,
					Timestamped:      true,
					StakedDuration:   time.Duration(c.bucket.StakedDuration) * time.Second,
					CreateTime:       time.Unix(int64(c.bucket.CreatedAt), 0),
					StakeStartTime:   stakeStartTime,
					UnstakeStartTime: unstakeStartTime,
					Candidate:        expectCandidate,
					Owner:            c.bucket.Owner,
					ContractAddress:  contract.String(),
				}, *vb)
			} else {
				stakeStartBlockHeight := c.bucket.CreatedAt
				if c.bucket.UnlockedAt != maxStakingNumber {
					stakeStartBlockHeight = c.bucket.UnlockedAt
				}
				r.Equal(VoteBucket{
					Index:                     idx,
					StakedAmount:              c.bucket.StakedAmount,
					StakedDuration:            mockBlocksToDuration(c.bucket.CreatedAt, c.bucket.CreatedAt+c.bucket.StakedDuration),
					StakedDurationBlockNumber: c.bucket.StakedDuration,
					CreateBlockHeight:         c.bucket.CreatedAt,
					StakeStartBlockHeight:     stakeStartBlockHeight,
					UnstakeStartBlockHeight:   c.bucket.UnstakedAt,
					Candidate:                 expectCandidate,
					Owner:                     c.bucket.Owner,
					ContractAddress:           contract.String(),
				}, *vb)
			}
		})
	}
}

func TestBucketDuration(t *testing.T) {
	r := require.New(t)
	forkHeight := uint64(10000)
	blocksToDuration := func(height, end uint64) time.Duration {
		return time.Duration(end-height) * time.Second * 5
	}
	blocksToDurationPost := func(height, end uint64) time.Duration {
		if height < forkHeight && end < forkHeight {
			return blocksToDuration(height, end)
		} else if height < forkHeight && end >= forkHeight {
			return blocksToDuration(height, forkHeight-1) + time.Duration(end-forkHeight+1)*time.Second*3
		} else {
			return time.Duration(end-height) * time.Second * 3
		}
	}
	cases := []struct {
		bucket         *Bucket
		height         uint64
		expectDuration time.Duration
	}{
		{&Bucket{CreatedAt: forkHeight - 100, StakedDuration: 60}, forkHeight - 1, 300 * time.Second},
		{&Bucket{CreatedAt: forkHeight - 100, StakedDuration: 60}, forkHeight, 300 * time.Second},
		{&Bucket{CreatedAt: forkHeight - 50, StakedDuration: 60}, forkHeight - 1, 300 * time.Second},
		{&Bucket{CreatedAt: forkHeight - 50, StakedDuration: 60}, forkHeight, (49*5 + 11*3) * time.Second},
		{&Bucket{CreatedAt: forkHeight, StakedDuration: 60}, forkHeight - 1, 300 * time.Second},
		{&Bucket{CreatedAt: forkHeight, StakedDuration: 60}, forkHeight, 180 * time.Second},
	}

	for idx, c := range cases {
		t.Run(fmt.Sprintf("case-%d", idx), func(t *testing.T) {
			var bkt *VoteBucket
			if c.height < forkHeight {
				bkt = assembleVoteBucket(0, c.bucket, "", blocksToDuration)
			} else {
				bkt = assembleVoteBucket(0, c.bucket, "", blocksToDurationPost)
			}
			r.Equal(c.expectDuration, bkt.StakedDuration, "StakedDuration mismatch")
		})
	}
}
