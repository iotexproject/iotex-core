package contractstaking

import (
	"math/big"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

type (
	// BucketType defines the type of contract staking bucket
	BucketType struct {
		Amount      *big.Int
		Duration    uint64
		ActivatedAt uint64
	}
)

func (bt *BucketType) toProto() *stakingpb.BucketType {
	return &stakingpb.BucketType{
		Amount:      bt.Amount.String(),
		Duration:    bt.Duration,
		ActivatedAt: bt.ActivatedAt,
	}
}

// LoadBucketTypeFromProto converts a protobuf representation of a staking bucket type to a BucketType struct.
func LoadBucketTypeFromProto(pb *stakingpb.BucketType) (*BucketType, error) {
	bt := &BucketType{}
	amount, ok := new(big.Int).SetString(pb.Amount, 10)
	if !ok {
		return nil, errors.New("failed to parse amount from string")
	}
	bt.Amount = amount
	bt.Duration = pb.Duration
	bt.ActivatedAt = pb.ActivatedAt
	return bt, nil
}

// Serialize serializes the bucket type
func (bt *BucketType) Serialize() ([]byte, error) {
	return proto.Marshal(bt.toProto())
}

// Deserialize deserializes the bucket type
func (bt *BucketType) Deserialize(b []byte) error {
	m := stakingpb.BucketType{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	loaded, err := LoadBucketTypeFromProto(&m)
	if err != nil {
		return errors.Wrap(err, "failed to load bucket type from proto")
	}
	*bt = *loaded
	return nil
}

// Clone clones the bucket type
func (bt *BucketType) Clone() *BucketType {
	return &BucketType{
		Amount:      big.NewInt(0).Set(bt.Amount),
		Duration:    bt.Duration,
		ActivatedAt: bt.ActivatedAt,
	}
}

// Encode encodes the bucket type into a GenericValue
func (bt *BucketType) Encode() (systemcontracts.GenericValue, error) {
	data, err := bt.Serialize()
	if err != nil {
		return systemcontracts.GenericValue{}, errors.Wrap(err, "failed to serialize bucket type")
	}
	return systemcontracts.GenericValue{PrimaryData: data}, nil
}

// Decode decodes the bucket type from a GenericValue
func (bt *BucketType) Decode(gv systemcontracts.GenericValue) error {
	return bt.Deserialize(gv.PrimaryData)
}
