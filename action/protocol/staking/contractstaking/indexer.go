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
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
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
		kvstore db.KVStore                // persistent storage
		cache   *contractStakingCacheSafe // in-memory index for clean data
	}
)

// NewContractStakingIndexer creates a new contract staking indexer
func NewContractStakingIndexer(kvStore db.KVStore) *Indexer {
	return &Indexer{
		kvstore: kvStore,
		cache:   newContractStakingCacheSafe(),
	}
}

// Start starts the indexer
func (s *Indexer) Start(ctx context.Context) error {
	if err := s.kvstore.Start(ctx); err != nil {
		return err
	}
	return s.loadCache()
}

// Stop stops the indexer
func (s *Indexer) Stop(ctx context.Context) error {
	if err := s.kvstore.Stop(ctx); err != nil {
		return err
	}
	s.cache = newContractStakingCacheSafe()
	return nil
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
	return s.cache.GetBuckets(), nil
}

// Bucket returns the bucket
func (s *Indexer) Bucket(id uint64) (*Bucket, bool) {
	return s.cache.GetBucket(id)
}

// BucketsByIndices returns the buckets by indices
func (s *Indexer) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	return s.cache.GetBucketsByIndices(indices)
}

// BucketsByCandidate returns the buckets by candidate
func (s *Indexer) BucketsByCandidate(candidate address.Address) []*Bucket {
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

// PutBlock puts a block into indexer
func (s *Indexer) PutBlock(ctx context.Context, blk *block.Block) error {
	// new dirty cache for this block
	// it's not necessary to use thread safe cache here, because only one thread will call this function
	// and no update to cache will happen before dirty merge to clean
	dirty := newContractStakingDirty(s.cache.Unsafe())
	dirty.PutHeight(blk.Height())
	handler := newContractStakingEventHandler(dirty)

	// handle events of block
	for _, receipt := range blk.Receipts {
		if receipt.Status != uint64(iotextypes.ReceiptStatus_Success) {
			continue
		}
		for _, log := range receipt.Logs() {
			if log.Address != StakingContractAddress {
				continue
			}
			if err := handler.HandleEvent(ctx, blk, log); err != nil {
				return err
			}
		}
	}

	// commit dirty cache
	return s.commit(dirty)
}

// DeleteTipBlock deletes the tip block from indexer
func (s *Indexer) DeleteTipBlock(context.Context, *block.Block) error {
	return errors.New("not implemented")
}

func (s *Indexer) commit(dirty *contractStakingDirty) error {
	batch, delta := dirty.Finalize()
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
	s.cache = newContractStakingCacheSafe()
	return s.loadCache()
}

func (s *Indexer) loadCache() error {
	delta := newContractStakingDelta()
	// load height
	var height uint64
	h, err := s.kvstore.Get(_StakingNS, _stakingHeightKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		height = 0
	} else {
		height = byteutil.BytesToUint64BigEndian(h)

	}
	delta.PutHeight(height)

	// load total bucket count
	var totalBucketCount uint64
	tbc, err := s.kvstore.Get(_StakingNS, _stakingTotalBucketCountKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	} else {
		totalBucketCount = byteutil.BytesToUint64BigEndian(tbc)
	}
	delta.PutTotalBucketCount(totalBucketCount)

	// load bucket info
	ks, vs, err := s.kvstore.Filter(_StakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b bucketInfo
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		delta.addBucketInfo(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}

	// load bucket type
	ks, vs, err = s.kvstore.Filter(_StakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b BucketType
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		delta.AddBucketType(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}
	return s.cache.Merge(delta)
}
