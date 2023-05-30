// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/blockchain/block"
)

type (
	// dummyContractStakingIndexer is a empty contract staking indexer
	dummyContractStakingIndexer struct{}
)

// NewDummyContractStakingIndexer creates an empty contract staking indexer
func NewDummyContractStakingIndexer() ContractIndexer {
	return &dummyContractStakingIndexer{}
}

// Start starts the indexer
func (d *dummyContractStakingIndexer) Start(ctx context.Context) error {
	return nil
}

// Stop stops the indexer
func (d *dummyContractStakingIndexer) Stop(ctx context.Context) error {
	return nil
}

// StartHeight returns the start height of the indexer
func (d *dummyContractStakingIndexer) StartHeight() uint64 {
	return 0
}

// Height returns the height of the indexer
func (d *dummyContractStakingIndexer) Height() (uint64, error) {
	return 0, nil
}

// PutBlock puts a block into the indexer
func (d *dummyContractStakingIndexer) PutBlock(ctx context.Context, blk *block.Block) error {
	return nil
}

// DeleteTipBlock deletes the tip block from the indexer
func (d *dummyContractStakingIndexer) DeleteTipBlock(context.Context, *block.Block) error {
	return nil
}

// CandidateVotes returns the total staked votes of a candidate
func (d *dummyContractStakingIndexer) CandidateVotes(ownerAddr address.Address) *big.Int {
	return big.NewInt(0)
}

// Buckets returns active buckets
func (d *dummyContractStakingIndexer) Buckets() ([]*staking.VoteBucket, error) {
	return nil, nil
}

// BucketsByIndices returns active buckets by indices
func (d *dummyContractStakingIndexer) BucketsByIndices([]uint64) ([]*staking.VoteBucket, error) {
	return nil, nil
}

// TotalBucketCount returns the total number of buckets including burned buckets
func (d *dummyContractStakingIndexer) TotalBucketCount() uint64 {
	return 0
}

// BucketTypes returns the active bucket types
func (d *dummyContractStakingIndexer) BucketTypes() ([]*staking.ContractStakingBucketType, error) {
	return nil, nil
}

// BucketsByCandidate returns active buckets by candidate
func (d *dummyContractStakingIndexer) BucketsByCandidate(ownerAddr address.Address) ([]*staking.VoteBucket, error) {
	return nil, nil
}
