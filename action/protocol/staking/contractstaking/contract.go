package contractstaking

import (
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
)

// StakingContract represents the staking contract in the system
type StakingContract struct {
	// NumOfBuckets is the number of buckets in the staking contract
	NumOfBuckets uint64
}

func (sc *StakingContract) toProto() *stakingpb.SystemStakingContract {
	if sc == nil {
		return nil
	}
	return &stakingpb.SystemStakingContract{
		NumOfBuckets: sc.NumOfBuckets,
	}
}

// LoadStakingContractFromProto converts a protobuf representation of a staking contract to a StakingContract struct.
func LoadStakingContractFromProto(pb *stakingpb.SystemStakingContract) (*StakingContract, error) {
	if pb == nil {
		return nil, nil
	}
	sc := &StakingContract{
		NumOfBuckets: pb.NumOfBuckets,
	}
	return sc, nil
}

// Serialize serializes the staking contract
func (sc *StakingContract) Serialize() ([]byte, error) {
	return proto.Marshal(sc.toProto())
}

// Deserialize deserializes the staking contract
func (sc *StakingContract) Deserialize(b []byte) error {
	m := stakingpb.SystemStakingContract{}
	if err := proto.Unmarshal(b, &m); err != nil {
		return err
	}
	loaded, err := LoadStakingContractFromProto(&m)
	if err != nil {
		return errors.Wrap(err, "failed to load staking contract from proto")
	}
	*sc = *loaded
	return nil
}
