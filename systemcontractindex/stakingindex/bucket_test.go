package stakingindex

import (
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
