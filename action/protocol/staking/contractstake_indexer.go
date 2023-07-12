// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
)

type (

	// ContractStakingIndexer defines the interface of contract staking reader
	ContractStakingIndexer interface {
		// CandidateVotes returns the total staked votes of a candidate
		// candidate identified by owner address
		CandidateVotes(ownerAddr address.Address) *big.Int
		// Buckets returns active buckets
		Buckets() ([]*VoteBucket, error)
		// BucketsByIndices returns active buckets by indices
		BucketsByIndices([]uint64) ([]*VoteBucket, error)
		// BucketsByCandidate returns active buckets by candidate
		BucketsByCandidate(ownerAddr address.Address) ([]*VoteBucket, error)
		// TotalBucketCount returns the total number of buckets including burned buckets
		TotalBucketCount() uint64
		// BucketTypes returns the active bucket types
		BucketTypes() ([]*ContractStakingBucketType, error)
	}
)
