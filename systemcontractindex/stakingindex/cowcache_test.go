package stakingindex

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestCowCache(t *testing.T) {
	r := require.New(t)
	buckets := []*Bucket{
		{Candidate: identityset.Address(1), Owner: identityset.Address(2), StakedAmount: big.NewInt(1000), Timestamped: true, StakedDuration: 3600, CreatedAt: 1622548800, UnlockedAt: 1622552400, UnstakedAt: 1622556000, Muted: false},
		{Candidate: identityset.Address(3), Owner: identityset.Address(4), StakedAmount: big.NewInt(2000), Timestamped: false, StakedDuration: 7200, CreatedAt: 1622548801, UnlockedAt: 1622552401, UnstakedAt: 1622556001, Muted: true},
		{Candidate: identityset.Address(5), Owner: identityset.Address(6), StakedAmount: big.NewInt(3000), Timestamped: true, StakedDuration: 10800, CreatedAt: 1622548802, UnlockedAt: 1622552402, UnstakedAt: 1622556002, Muted: false},
		{Candidate: identityset.Address(7), Owner: identityset.Address(8), StakedAmount: big.NewInt(4000), Timestamped: true, StakedDuration: 10800, CreatedAt: 1622548802, UnlockedAt: 1622552402, UnstakedAt: 1622556002, Muted: false},
	}
	original := newCache("testNS", "testBucketNS")
	original.PutBucket(0, buckets[0])
	// case 1: read cowCache without modification
	cow := newCowCache(original)
	r.Equal(buckets[0], cow.Bucket(0))

	// case 2: modify cowCache but not affect original cache
	cow.PutBucket(1, buckets[1])
	r.Equal(buckets[1], cow.Bucket(1))
	r.Nil(original.Bucket(1))
	cow.DeleteBucket(0)
	r.Nil(cow.Bucket(0))
	r.Equal(buckets[0], original.Bucket(0))

	// case 3: not real copy before modification
	copi := cow.Copy()
	r.Equal(buckets[1], copi.Bucket(1))
	r.Equal(cow.cache, copi.(*cowCache).cache)

	// case 4: copied not affected by original modification
	cow.PutBucket(2, buckets[2])
	r.Equal(buckets[2], cow.Bucket(2))
	r.Nil(copi.Bucket(2))

	// case 5: original not affected by copied modification
	copi.PutBucket(3, buckets[3])
	r.Equal(buckets[3], copi.Bucket(3))
	r.Nil(cow.Bucket(3))
}
