// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	_ "embed"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
)

var (
	// StakingContractJSONABI is the abi json of staking contract
	//go:embed contract_staking_abi.json
	StakingContractJSONABI string
	// StakingContractABI is the abi of staking contract
	StakingContractABI abi.ABI
)

type (

	// ContractStakingIndexer defines the interface of contract staking reader
	ContractStakingIndexer interface {
		// Buckets returns active buckets
		Buckets(height uint64) ([]*VoteBucket, error)
		// BucketsByIndices returns active buckets by indices
		BucketsByIndices([]uint64, uint64) ([]*VoteBucket, error)
		// BucketsByCandidate returns active buckets by candidate
		BucketsByCandidate(ownerAddr address.Address, height uint64) ([]*VoteBucket, error)
		// TotalBucketCount returns the total number of buckets including burned buckets
		TotalBucketCount(height uint64) (uint64, error)
		// ContractAddress returns the contract address
		ContractAddress() string
	}
	// ContractStakingIndexerWithBucketType defines the interface of contract staking reader with bucket type
	ContractStakingIndexerWithBucketType interface {
		ContractStakingIndexer
		// BucketTypes returns the active bucket types
		BucketTypes(height uint64) ([]*ContractStakingBucketType, error)
	}
)

func init() {
	var err error
	StakingContractABI, err = abi.JSON(strings.NewReader(StakingContractJSONABI))
	if err != nil {
		panic(err)
	}
}
