// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/db"
)

const (
	maxBlockNumber uint64 = math.MaxUint64
)

type (
	// ContractIndexer defines the interface of contract staking reader
	ContractIndexer interface {
		blockdao.BlockIndexer

		// CandidateVotes returns the total staked votes of a candidate
		// candidate identified by owner address
		CandidateVotes(ownerAddr address.Address) *big.Int
		// Buckets returns active buckets
		Buckets() ([]*Bucket, error)
		// BucketsByIndices returns active buckets by indices
		BucketsByIndices([]uint64) ([]*Bucket, error)
		// BucketsByCandidate returns active buckets by candidate
		BucketsByCandidate(ownerAddr address.Address) ([]*Bucket, error)
		// TotalBucketCount returns the total number of buckets including burned buckets
		TotalBucketCount() uint64
		// BucketTypes returns the active bucket types
		BucketTypes() ([]*BucketType, error)
	}

	// Indexer is the contract staking indexer
	// Main functions:
	// 		1. handle contract staking contract events when new block comes to generate index data
	// 		2. provide query interface for contract staking index data
	Indexer struct {
		kvstore              db.KVStore            // persistent storage, used to initialize index cache at startup
		cache                *contractStakingCache // in-memory index for clean data, used to query index data
		contractAddress      string                // stake contract address
		contractDeployHeight uint64                // height of the contract deployment
	}
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore, contractAddr string, contractDeployHeight uint64) *Indexer {
	return &Indexer{
		kvstore:              kvStore,
		cache:                newContractStakingCache(contractAddr),
		contractAddress:      contractAddr,
		contractDeployHeight: contractDeployHeight,
	}
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	return s.cache.LoadFromDB(s.kvstore)
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	s.cache = newContractStakingCache(s.contractAddress)
	return nil
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	return s.cache.Height(), nil
}

// StartHeight returns the start height of the indexer
func (s *Indexer) StartHeight() uint64 {
	return s.contractDeployHeight
}

// CandidateVotes returns the candidate votes
func (s *Indexer) CandidateVotes(candidate address.Address) *big.Int {
	return s.cache.CandidateVotes(candidate)
}

// Buckets returns the buckets
func (s *Indexer) Buckets() ([]*Bucket, error) {
	return s.cache.Buckets(), nil
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64) (*Bucket, bool) {
	return s.cache.Bucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	return s.cache.BucketsByIndices(indices)
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	return s.cache.BucketsByCandidate(candidate), nil
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

// PutBlock puts a block into indexer
func (s *Indexer) PutBlock(ctx context.Context, blk *block.Block) error {
	if blk.Height() < s.contractDeployHeight {
		return nil
	}
	// new event handler for this block
	handler := newContractStakingEventHandler(s.cache, blk.Height())

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != s.contractAddress {
				continue
			}
			if err := handler.HandleEvent(ctx, blk, log); err != nil {
				return err
			}
		}
	}

	// commit the result
	return s.commit(handler)
}

// DeleteTipBlock deletes the tip block from indexer
func (s *Indexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

func (s *Indexer) commit(handler *contractStakingEventHandler) error {
	batch, delta := handler.Result()
	if err := s.cache.Merge(delta); err != nil {
		s.reloadCache()
		return err
	}
	if err := s.kvstore.WriteBatch(batch); err != nil {
		s.reloadCache()
		return err
	}
	return nil
}

func (s *Indexer) reloadCache() error {
	s.cache = newContractStakingCache(s.contractAddress)
	return s.cache.LoadFromDB(s.kvstore)
}
