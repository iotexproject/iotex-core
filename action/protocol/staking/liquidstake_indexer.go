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
	// LiquidStakingIndexer defines the interface of liquid staking reader
	LiquidStakingIndexer interface {
		// CandidateVotes returns the total staked votes of a candidate
		// candidate identified by owner address
		CandidateVotes(ownerAddr address.Address) *big.Int
		// Buckets returns active buckets
		Buckets() ([]*VoteBucket, error)
		// BucketsByIndices returns active buckets by indices
		BucketsByIndices([]uint64) ([]*VoteBucket, error)
		// TotalBucketCount returns the total number of buckets including burned buckets
		TotalBucketCount() uint64
	}

	// TODO (iip-13): remove this empty liquid staking indexer
	emptyLiquidStakingIndexer struct{}
)

var _ LiquidStakingIndexer = (*emptyLiquidStakingIndexer)(nil)

// NewEmptyLiquidStakingIndexer creates an empty liquid staking indexer
func NewEmptyLiquidStakingIndexer() LiquidStakingIndexer {
	return &emptyLiquidStakingIndexer{}
}

func (f *emptyLiquidStakingIndexer) CandidateVotes(ownerAddr address.Address) *big.Int {
	return big.NewInt(0)
}

func (f *emptyLiquidStakingIndexer) Buckets() ([]*VoteBucket, error) {
	return nil, nil
}

func (f *emptyLiquidStakingIndexer) BucketsByIndices([]uint64) ([]*VoteBucket, error) {
	return nil, nil
}

func (f *emptyLiquidStakingIndexer) TotalBucketCount() uint64 {
	return 0
}
