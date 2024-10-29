// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"context"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/blockchain/block"
	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
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
		kvstore db.KVStore            // persistent storage, used to initialize index cache at startup
		cache   *contractStakingCache // in-memory index for clean data, used to query index data
		config  Config                // indexer config
	}

	// Config is the config for contract staking indexer
	Config struct {
		ContractAddress      string // stake contract ContractAddress
		ContractDeployHeight uint64 // height of the contract deployment
		// TODO: move calculateVoteWeightFunc out of config
		CalculateVoteWeight calculateVoteWeightFunc // calculate vote weight function
		BlockInterval       time.Duration           // block produce interval
	}

	calculateVoteWeightFunc func(v *Bucket) *big.Int
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore, config Config) (*Indexer, error) {
	if kvStore == nil {
		return nil, errors.New("kv store is nil")
	}
	if _, err := address.FromString(config.ContractAddress); err != nil {
		return nil, errors.Wrapf(err, "invalid contract address %s", config.ContractAddress)
	}
	if config.CalculateVoteWeight == nil {
		return nil, errors.New("calculate vote weight function is nil")
	}
	return &Indexer{
		kvstore: kvStore,
		cache:   newContractStakingCache(config),
		config:  config,
	}, nil
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	return s.loadFromDB()
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	s.cache = newContractStakingCache(s.config)
	return nil
}

// Height returns the tip block height
func (s *Indexer) Height() (uint64, error) {
	h := s.cache.Height()
	if h < s.config.ContractDeployHeight {
		h = s.config.ContractDeployHeight
	}
	return h, nil
}

// ContractAddress returns the contract address
func (s *Indexer) ContractAddress() string {
	return s.config.ContractAddress
}

// CandidateVotes returns the candidate votes
func (s *Indexer) CandidateVotes(ctx context.Context, candidate address.Address, height uint64) (*big.Int, error) {
	if s.isIgnored(height) {
		return big.NewInt(0), nil
	}
	return s.cache.CandidateVotes(ctx, candidate, height)
}

// Buckets returns the buckets
func (s *Indexer) Buckets(height uint64) ([]*Bucket, error) {
	if s.isIgnored(height) {
		return []*Bucket{}, nil
	}
	return s.cache.Buckets(height)
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64, height uint64) (*Bucket, bool, error) {
	if s.isIgnored(height) {
		return nil, false, nil
	}
	return s.cache.Bucket(id, height)
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64, height uint64) ([]*Bucket, error) {
	if s.isIgnored(height) {
		return []*Bucket{}, nil
	}
	return s.cache.BucketsByIndices(indices, height)
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address, height uint64) ([]*Bucket, error) {
	if s.isIgnored(height) {
		return []*Bucket{}, nil
	}
	return s.cache.BucketsByCandidate(candidate, height)
}

// TotalBucketCount returns the total bucket count including active and burnt buckets
func (s *Indexer) TotalBucketCount(height uint64) (uint64, error) {
	if s.isIgnored(height) {
		return 0, nil
	}
	return s.cache.TotalBucketCount(height)
}

// BucketTypes returns the active bucket types
func (s *Indexer) BucketTypes(height uint64) ([]*BucketType, error) {
	if s.isIgnored(height) {
		return []*BucketType{}, nil
	}
	btMap, err := s.cache.ActiveBucketTypes(height)
	if err != nil {
		return nil, err
	}
	bts := make([]*BucketType, 0, len(btMap))
	for _, bt := range btMap {
		bts = append(bts, bt)
	}
	return bts, nil
}

// PutBlock puts a block into indexer
func (s *Indexer) PutBlock(ctx context.Context, blk *block.Block) error {
	expectHeight := s.cache.Height() + 1
	if expectHeight < s.config.ContractDeployHeight {
		expectHeight = s.config.ContractDeployHeight
	}
	if blk.Height() < expectHeight {
		return nil
	}
	if blk.Height() > expectHeight {
		return errors.Errorf("invalid block height %d, expect %d", blk.Height(), expectHeight)
	}
	// new event handler for this block
	handler := newContractStakingEventHandler(s.cache)

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != s.config.ContractAddress {
				continue
			}
			if err := handler.HandleEvent(ctx, blk, log); err != nil {
				return err
			}
		}
	}

	// commit the result
	return s.commit(handler, blk.Height())
}

// DeleteTipBlock deletes the tip block from indexer
func (s *Indexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

func (s *Indexer) commit(handler *contractStakingEventHandler, height uint64) error {
	batch, delta := handler.Result()
	// update cache
	if err := s.cache.Merge(delta, height); err != nil {
		s.reloadCache()
		return err
	}
	// update db
	batch.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(height), "failed to put height")
	if err := s.kvstore.WriteBatch(batch); err != nil {
		s.reloadCache()
		return err
	}
	return nil
}

func (s *Indexer) reloadCache() error {
	s.cache = newContractStakingCache(s.config)
	return s.loadFromDB()
}

func (s *Indexer) loadFromDB() error {
	return s.cache.LoadFromDB(s.kvstore)
}

// isIgnored returns true if before cotractDeployHeight.
// it aims to be compatible with blocks between feature hard-fork and contract deployed
// read interface should return empty result instead of invalid height error if it returns true
func (s *Indexer) isIgnored(height uint64) bool {
	return height < s.config.ContractDeployHeight
}
