package contractstaking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

type (
	// Bucket is the structure that holds information about a staking bucket.
	Bucket struct {
		// Candidate is the address of the candidate that this bucket is staking for.
		Candidate address.Address
		// Owner is the address of the owner of this bucket.
		Owner address.Address
		// StakedAmount is the amount of tokens staked in this bucket.
		StakedAmount *big.Int

		// StakedDuration is the duration for which the tokens have been staked.
		StakedDuration uint64 // in seconds if timestamped, in block number if not
		// CreatedAt is the time when the bucket was created.
		CreatedAt uint64 // in unix timestamp if timestamped, in block height if not
		// UnlockedAt is the time when the bucket can be unlocked.
		UnlockedAt uint64 // in unix timestamp if timestamped, in block height if not
		// UnstakedAt is the time when the bucket was unstaked.
		UnstakedAt uint64 // in unix timestamp if timestamped, in block height if not

		// IsTimestampBased indicates whether the bucket is timestamp-based
		IsTimestampBased bool
		// Muted indicates whether the bucket is vote weight muted
		Muted bool
	}
)

func (b *Bucket) toProto() *stakingpb.SystemStakingBucket {
	if b == nil {
		return nil
	}
	return &stakingpb.SystemStakingBucket{
		Owner:      b.Owner.Bytes(),
		Candidate:  b.Candidate.Bytes(),
		Amount:     b.StakedAmount.Bytes(),
		Duration:   b.StakedDuration,
		CreatedAt:  b.CreatedAt,
		UnlockedAt: b.UnlockedAt,
		UnstakedAt: b.UnstakedAt,
		Muted:      b.Muted,
	}
}

// LoadBucketFromProto converts a protobuf representation of a staking bucket to a Bucket struct.
func LoadBucketFromProto(pb *stakingpb.SystemStakingBucket) (*Bucket, error) {
	if pb == nil {
		return nil, nil
	}
	b := &Bucket{}
	owner, err := address.FromBytes(pb.Owner)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert owner bytes to address")
	}
	b.Owner = owner
	cand, err := address.FromBytes(pb.Candidate)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert candidate bytes to address")
	}
	b.Candidate = cand
	b.StakedAmount = new(big.Int).SetBytes(pb.Amount)
	b.StakedDuration = pb.Duration
	b.CreatedAt = pb.CreatedAt
	b.UnlockedAt = pb.UnlockedAt
	b.UnstakedAt = pb.UnstakedAt
	b.Muted = pb.Muted

	return b, nil
}

// Serialize serializes the bucket to a byte slice.
func (b *Bucket) Serialize() ([]byte, error) {
	return proto.Marshal(b.toProto())
}

// Deserialize deserializes the bucket from a byte slice.
func (b *Bucket) Deserialize(data []byte) error {
	m := stakingpb.SystemStakingBucket{}
	if err := proto.Unmarshal(data, &m); err != nil {
		return errors.Wrap(err, "failed to unmarshal bucket data")
	}
	bucket, err := LoadBucketFromProto(&m)
	if err != nil {
		return errors.Wrap(err, "failed to load bucket from proto")
	}
	*b = *bucket
	return nil
}

// Clone creates a deep copy of the Bucket.
func (b *Bucket) Clone() *Bucket {
	return &Bucket{
		Candidate:        b.Candidate,
		Owner:            b.Owner,
		StakedAmount:     new(big.Int).Set(b.StakedAmount),
		StakedDuration:   b.StakedDuration,
		CreatedAt:        b.CreatedAt,
		UnlockedAt:       b.UnlockedAt,
		UnstakedAt:       b.UnstakedAt,
		IsTimestampBased: b.IsTimestampBased,
		Muted:            b.Muted,
	}
}
