// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"
	"sync"
	"time"

	"github.com/iotexproject/iotex-address/address"
)

type (
	// liquidStakingCacheReader is the interface to read liquid staking cache
	// it serves the purpose of preventing modifications to it.
	liquidStakingCacheReader interface {
		getHeight() uint64
		getTotalBucketCount() uint64
		getTotalBucketTypeCount() uint64
		getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool)
		getBucketType(id uint64) (*BucketType, bool)
		getBucketInfo(id uint64) (*BucketInfo, bool)
		mustGetBucketType(id uint64) *BucketType
		mustGetBucketInfo(id uint64) *BucketInfo
		getAllBucketInfo() map[uint64]*BucketInfo
		getCandidateVotes(candidate address.Address) *big.Int
	}

	// liquidStakingCacheManager is the interface to manage liquid staking cache
	// it's used to hide internal data, ensuring thread safety when used within the package
	liquidStakingCacheManager interface {
		liquidStakingCacheReader
		merge(delta *liquidStakingDelta) error
		putHeight(h uint64)
		putTotalBucketCount(cnt uint64)
		putBucketType(id uint64, bt *BucketType)
		putBucketInfo(id uint64, bi *BucketInfo)
		deleteBucketInfo(id uint64)
	}

	liquidStakingCache struct {
		idBucketMap           map[uint64]*BucketInfo     // map[token]BucketInfo
		candidateBucketMap    map[string]map[uint64]bool // map[candidate]bucket
		idBucketTypeMap       map[uint64]*BucketType     // map[token]BucketType
		propertyBucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
		height                uint64
		totalBucketCount      uint64 // total number of buckets including burned buckets
	}

	liquidStakingCacheThreadSafety struct {
		cache liquidStakingCacheManager
		mutex sync.RWMutex
	}
)

func newLiquidStakingCache() *liquidStakingCacheThreadSafety {
	cache := &liquidStakingCache{
		idBucketMap:           make(map[uint64]*BucketInfo),
		idBucketTypeMap:       make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[int64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
	}
	return &liquidStakingCacheThreadSafety{cache: cache}
}

func (s *liquidStakingCache) putHeight(h uint64) {
	s.height = h
}

func (s *liquidStakingCache) getHeight() uint64 {
	return s.height
}

func (s *liquidStakingCache) putBucketType(id uint64, bt *BucketType) {
	amount := bt.Amount.Int64()
	s.idBucketTypeMap[id] = bt
	m, ok := s.propertyBucketTypeMap[amount]
	if !ok {
		s.propertyBucketTypeMap[amount] = make(map[int64]uint64)
		m = s.propertyBucketTypeMap[amount]
	}
	m[int64(bt.Duration)] = id
}

func (s *liquidStakingCache) putBucketInfo(id uint64, bi *BucketInfo) {
	s.idBucketMap[id] = bi
	if _, ok := s.candidateBucketMap[bi.Delegate.String()]; !ok {
		s.candidateBucketMap[bi.Delegate.String()] = make(map[uint64]bool)
	}
	s.candidateBucketMap[bi.Delegate.String()][id] = true
}

func (s *liquidStakingCache) deleteBucketInfo(id uint64) {
	bi, ok := s.idBucketMap[id]
	if !ok {
		return
	}
	delete(s.idBucketMap, id)
	if _, ok := s.candidateBucketMap[bi.Delegate.String()]; !ok {
		return
	}
	delete(s.candidateBucketMap[bi.Delegate.String()], id)
}

func (s *liquidStakingCache) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	m, ok := s.propertyBucketTypeMap[amount.Int64()]
	if !ok {
		return 0, false
	}
	id, ok := m[int64(duration)]
	return id, ok
}

func (s *liquidStakingCache) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.idBucketTypeMap[id]
	return bt, ok
}

func (s *liquidStakingCache) mustGetBucketType(id uint64) *BucketType {
	bt, ok := s.idBucketTypeMap[id]
	if !ok {
		panic("bucket type not found")
	}
	return bt
}

func (s *liquidStakingCache) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.idBucketMap[id]
	return bi, ok
}

func (s *liquidStakingCache) mustGetBucketInfo(id uint64) *BucketInfo {
	bt, ok := s.idBucketMap[id]
	if !ok {
		panic("bucket info not found")
	}
	return bt
}

func (s *liquidStakingCache) getCandidateVotes(candidate address.Address) *big.Int {
	votes := big.NewInt(0)
	m, ok := s.candidateBucketMap[candidate.String()]
	if !ok {
		return votes
	}
	for id, existed := range m {
		if !existed {
			continue
		}
		bi, ok := s.idBucketMap[id]
		if !ok {
			continue
		}
		if !bi.UnstakedAt.IsZero() {
			continue
		}
		bt := s.mustGetBucketType(bi.TypeIndex)
		votes.Add(votes, bt.Amount)
	}
	return votes
}

func (s *liquidStakingCache) putTotalBucketCount(count uint64) {
	s.totalBucketCount = count
}

func (s *liquidStakingCache) getTotalBucketCount() uint64 {
	return s.totalBucketCount
}

func (s *liquidStakingCache) getTotalBucketTypeCount() uint64 {
	return uint64(len(s.idBucketTypeMap))
}

func (s *liquidStakingCache) getAllBucketInfo() map[uint64]*BucketInfo {
	m := make(map[uint64]*BucketInfo)
	for k, v := range s.idBucketMap {
		m[k] = v
	}
	return m
}

func (s *liquidStakingCache) merge(delta *liquidStakingDelta) error {
	for id, state := range delta.bucketTypeDeltaState {
		if state == deltaStateAdded || state == deltaStateModified {
			s.putBucketType(id, delta.mustGetBucketType(id))
		}
	}
	for id, state := range delta.bucketInfoDeltaState {
		if state == deltaStateAdded || state == deltaStateModified {
			s.putBucketInfo(id, delta.mustGetBucketInfo(id))
		} else if state == deltaStateRemoved {
			s.deleteBucketInfo(id)
		}
	}
	s.putHeight(delta.getHeight())
	s.putTotalBucketCount(s.getTotalBucketCount() + delta.addedBucketCnt())
	return nil
}

func (s *liquidStakingCacheThreadSafety) putHeight(h uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.cache.putHeight(h)
}

func (s *liquidStakingCacheThreadSafety) getHeight() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getHeight()
}

func (s *liquidStakingCacheThreadSafety) putBucketType(id uint64, bt *BucketType) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cache.putBucketType(id, bt)
}

func (s *liquidStakingCacheThreadSafety) putBucketInfo(id uint64, bi *BucketInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cache.putBucketInfo(id, bi)
}

func (s *liquidStakingCacheThreadSafety) deleteBucketInfo(id uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cache.deleteBucketInfo(id)
}

func (s *liquidStakingCacheThreadSafety) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getBucketTypeIndex(amount, duration)
}

func (s *liquidStakingCacheThreadSafety) getBucketType(id uint64) (*BucketType, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getBucketType(id)
}

func (s *liquidStakingCacheThreadSafety) mustGetBucketType(id uint64) *BucketType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.mustGetBucketType(id)
}

func (s *liquidStakingCacheThreadSafety) getBucketInfo(id uint64) (*BucketInfo, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getBucketInfo(id)
}

func (s *liquidStakingCacheThreadSafety) mustGetBucketInfo(id uint64) *BucketInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.mustGetBucketInfo(id)
}

func (s *liquidStakingCacheThreadSafety) getCandidateVotes(candidate address.Address) *big.Int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getCandidateVotes(candidate)
}

func (s *liquidStakingCacheThreadSafety) putTotalBucketCount(count uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.cache.putTotalBucketCount(count)
}

func (s *liquidStakingCacheThreadSafety) getTotalBucketCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getTotalBucketCount()
}

func (s *liquidStakingCacheThreadSafety) getTotalBucketTypeCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getTotalBucketTypeCount()
}

func (s *liquidStakingCacheThreadSafety) getAllBucketInfo() map[uint64]*BucketInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.cache.getAllBucketInfo()
}

func (s *liquidStakingCacheThreadSafety) merge(delta *liquidStakingDelta) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.cache.merge(delta)
}
