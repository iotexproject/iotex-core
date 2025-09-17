package contractstaking

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/stakingpb"
	"github.com/iotexproject/iotex-core/v2/state/factory/erigonstore"
	"github.com/iotexproject/iotex-core/v2/systemcontracts"
)

// StakingContract represents the staking contract in the system
type StakingContract struct {
	// NumOfBuckets is the number of buckets in the staking contract
	NumOfBuckets uint64
	// Height is the height of the staking contract
	Height uint64
}

var _ erigonstore.ContractStorageProxy = (*StakingContract)(nil)

func (sc *StakingContract) toProto() *stakingpb.SystemStakingContract {
	if sc == nil {
		return nil
	}
	return &stakingpb.SystemStakingContract{
		NumOfBuckets: sc.NumOfBuckets,
		Height:       sc.Height,
	}
}

// LoadStakingContractFromProto converts a protobuf representation of a staking contract to a StakingContract struct.
func LoadStakingContractFromProto(pb *stakingpb.SystemStakingContract) (*StakingContract, error) {
	if pb == nil {
		return nil, nil
	}
	sc := &StakingContract{
		NumOfBuckets: pb.NumOfBuckets,
		Height:       pb.Height,
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

// ContractStorageAddress returns the address of the contract storage
func (sc *StakingContract) ContractStorageAddress(ns string) (address.Address, error) {
	return systemcontracts.SystemContracts[systemcontracts.StakingContractIndex].Address, nil
}

// New creates a new instance of the staking contract
func (sc *StakingContract) New(data []byte) (any, error) {
	c := &StakingContract{}
	if err := c.Deserialize(data); err != nil {
		return nil, err
	}
	return c, nil
}

// ContractStorageProxy returns the contract storage proxy
func (sc *StakingContract) ContractStorageProxy() erigonstore.ContractStorage {
	return erigonstore.NewContractStorageNamespacedWrapper(sc)
}
