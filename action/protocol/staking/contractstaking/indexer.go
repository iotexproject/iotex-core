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
	maxBlockNumber uint64 = math.MaxUint64
)

type (
	// Indexer is the contract staking indexer
	// Main functions:
	// 		1. handle contract staking contract events when new block comes to generate index data
	// 		2. provide query interface for contract staking index data
	Indexer struct {
		kvstore         db.KVStore            // persistent storage, used to initialize index cache at startup
		cache           *contractStakingCache // in-memory index for clean data, used to query index data
		contractAddress string                // stake contract address
	}
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore, contractAddr string) *Indexer {
	return &Indexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(contractAddr),
	}
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	return s.cache.Height(), nil
}

// CandidateVotes returns the candidate votes
func (s *Indexer) CandidateVotes(candidate address.Address) *big.Int {
	return s.cache.CandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *Indexer) Buckets() ([]*Bucket, error) {
	return s.cache.Buckets()
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64) (*Bucket, error) {
	return s.cache.Bucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	return s.cache.BucketsByIndices(indices)
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	return s.cache.BucketsByCandidate(candidate)
}

// TotalBucketCount returns the total bucket count including active and burnt buckets
func (s *Indexer) TotalBucketCount() uint64 {
	return s.cache.TotalBucketCount()
}

// BucketTypes returns the active bucket types
func (s *Indexer) BucketTypes() ([]*BucketType, error) {
	btMap := s.cache.ActiveBucketTypes()
	bts := make([]*BucketType, 0, len(btMap))
	for _, bt := range btMap {
		bts = append(bts, bt)
	}
	return bts, nil
}
