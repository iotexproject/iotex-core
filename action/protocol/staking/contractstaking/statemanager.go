package contractstaking

import (
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
)

// ContractStakingStateManager wraps a state manager to provide staking contract-specific writes.
type ContractStakingStateManager struct {
	ContractStakingStateReader
	sm protocol.StateManager
}

// NewContractStakingStateManager creates a new ContractStakingStateManager
func NewContractStakingStateManager(sm protocol.StateManager, opts ...protocol.StateOption) *ContractStakingStateManager {
	return &ContractStakingStateManager{
		ContractStakingStateReader: *NewStateReader(sm, opts...),
		sm:                         sm,
	}
}

// UpsertBucketType inserts or updates a bucket type for a given contract and bucket ID.
func (cs *ContractStakingStateManager) UpsertBucketType(contractAddr address.Address, bucketID uint64, bucketType *BucketType) error {
	_, err := cs.sm.PutState(
		bucketType,
		cs.makeOpts(
			bucketTypeNamespaceOption(contractAddr),
			bucketIDKeyOption(bucketID),
		)...,
	)

	return err
}

// DeleteBucket removes a bucket for a given contract and bucket ID.
func (cs *ContractStakingStateManager) DeleteBucket(contractAddr address.Address, bucketID uint64) error {
	_, err := cs.sm.DelState(
		cs.makeOpts(
			bucketTypeNamespaceOption(contractAddr),
			bucketIDKeyOption(bucketID),
			protocol.ObjectOption(&Bucket{}),
		)...,
	)

	return err
}

// UpsertBucket inserts or updates a bucket for a given contract and bid.
func (cs *ContractStakingStateManager) UpsertBucket(contractAddr address.Address, bid uint64, bucket *Bucket) error {
	_, err := cs.sm.PutState(
		bucket,
		cs.makeOpts(
			contractNamespaceOption(contractAddr),
			bucketIDKeyOption(bid),
		)...,
	)

	return err
}

// UpdateNumOfBuckets updates the number of buckets.
func (cs *ContractStakingStateManager) UpdateNumOfBuckets(contractAddr address.Address, numOfBuckets uint64) error {
	_, err := cs.sm.PutState(
		&StakingContract{
			NumOfBuckets: uint64(numOfBuckets),
		},
		cs.makeOpts(
			metaNamespaceOption(),
			contractKeyOption(contractAddr),
		)...,
	)

	return err
}
