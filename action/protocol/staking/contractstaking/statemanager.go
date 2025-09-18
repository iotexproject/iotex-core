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
func NewContractStakingStateManager(sm protocol.StateManager) *ContractStakingStateManager {
	return &ContractStakingStateManager{
		ContractStakingStateReader: ContractStakingStateReader{sr: sm},
		sm:                         sm,
	}
}

// UpsertBucketType inserts or updates a bucket type for a given contract and bucket ID.
func (cs *ContractStakingStateManager) UpsertBucketType(contractAddr address.Address, bucketID uint64, bucketType *BucketType) error {
	_, err := cs.sm.PutState(
		bucketType,
		bucketTypeNamespaceOption(contractAddr),
		bucketIDKeyOption(bucketID),
	)

	return err
}

// DeleteBucket removes a bucket for a given contract and bucket ID.
func (cs *ContractStakingStateManager) DeleteBucket(contractAddr address.Address, bucketID uint64) error {
	_, err := cs.sm.DelState(
		contractNamespaceOption(contractAddr),
		bucketIDKeyOption(bucketID),
	)

	return err
}

// UpsertBucket inserts or updates a bucket for a given contract and bid.
func (cs *ContractStakingStateManager) UpsertBucket(contractAddr address.Address, bid uint64, bucket *Bucket) error {
	_, err := cs.sm.PutState(
		bucket,
		contractNamespaceOption(contractAddr),
		bucketIDKeyOption(bid),
	)

	return err
}

// UpdateNumOfBuckets updates the number of buckets.
func (cs *ContractStakingStateManager) UpdateNumOfBuckets(contractAddr address.Address, numOfBuckets uint64) error {
	_, err := cs.sm.PutState(
		&StakingContract{
			NumOfBuckets: uint64(numOfBuckets),
		},
		metaNamespaceOption(),
		contractKeyOption(contractAddr),
	)

	return err
}
