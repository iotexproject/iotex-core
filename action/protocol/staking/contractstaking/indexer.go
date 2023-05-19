// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/db"
)

const (
	// StakingContractAddress  is the address of system staking contract
	// TODO (iip-13): replace with the real system staking contract address
	StakingContractAddress = "io19ys8f4uhwms6lq6ulexr5fwht9gsjes8mvuugd"

	maxBlockNumber uint64 = math.MaxUint64
)

type (
	// Indexer is the contract staking indexer
	// Main functions:
	// 		1. handle contract staking contract events when new block comes to generate index data
	// 		2. provide query interface for contract staking index data
	// Generate index data flow:
	// 		block comes -> new dirty cache -> handle contract events -> update dirty cache -> merge dirty to clean cache
	// Main Object:
	// 		kvstore: persistent storage, used to initialize index cache at startup
	// 		cache: in-memory index for clean data, used to query index data
	//      dirty: the cache to update during event processing, will be merged to clean cache after all events are processed. If errors occur during event processing, dirty cache will be discarded.
	Indexer struct {
		kvstore db.KVStore            // persistent storage
		cache   *contractStakingCache // in-memory index for clean data
	}
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore) *Indexer {
	return &Indexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(),
	}
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	return s.cache.GetHeight(), nil
}

// CandidateVotes returns the candidate votes
func (s *Indexer) CandidateVotes(candidate address.Address) *big.Int {
	return s.cache.GetCandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *Indexer) Buckets() ([]*Bucket, error) {
	return s.cache.GetBuckets()
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64) (*Bucket, error) {
	return s.cache.GetBucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	return s.cache.GetBucketsByIndices(indices)
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	return s.cache.GetBucketsByCandidate(candidate)
}

// TotalBucketCount returns the total bucket count including active and burnt buckets
func (s *Indexer) TotalBucketCount() uint64 {
	return s.cache.GetTotalBucketCount()
}

// BucketTypes returns the active bucket types
func (s *Indexer) BucketTypes() ([]*BucketType, error) {
	btMap := s.cache.GetActiveBucketTypes()
	bts := make([]*BucketType, 0, len(btMap))
	for _, bt := range btMap {
		bts = append(bts, bt)
	}
	return bts, nil
}
