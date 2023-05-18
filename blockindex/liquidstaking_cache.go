// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
)

type (
	// contractStakingCacheReader is the interface to read contract staking cache
	// it serves the purpose of preventing modifications to it.
	contractStakingCacheReader interface {
		getHeight() uint64
		getTotalBucketCount() uint64
		getTotalBucketTypeCount() uint64
		getBucketTypeIndex(amount *big.Int, duration uint64) (uint64, bool)
		getBucketType(id uint64) (*ContractStakingBucketType, bool)
		getBucketInfo(id uint64) (*ContractStakingBucketInfo, bool)
		mustGetBucketType(id uint64) *ContractStakingBucketType
		mustGetBucketInfo(id uint64) *ContractStakingBucketInfo
		getAllBucketInfo() map[uint64]*ContractStakingBucketInfo
		getActiveBucketType() map[uint64]*ContractStakingBucketType
		getCandidateVotes(candidate address.Address) *big.Int
		getBucketInfoByCandidate(candidate address.Address) map[uint64]*ContractStakingBucketInfo
	}

	// contractStakingCacheManager is the interface to manage contract staking cache
	// it's used to hide internal data, ensuring thread safety when used within the package
	contractStakingCacheManager interface {
		contractStakingCacheReader
		merge(delta *contractStakingDelta) error
		putHeight(h uint64)
		putTotalBucketCount(cnt uint64)
		putBucketType(id uint64, bt *ContractStakingBucketType)
		putBucketInfo(id uint64, bi *ContractStakingBucketInfo)
		deleteBucketInfo(id uint64)
	}

	contractStakingCache struct {
		idBucketMap           map[uint64]*ContractStakingBucketInfo // map[token]BucketInfo
		candidateBucketMap    map[string]map[uint64]bool            // map[candidate]bucket
		idBucketTypeMap       map[uint64]*ContractStakingBucketType // map[token]BucketType
		propertyBucketTypeMap map[int64]map[uint64]uint64           // map[amount][duration]index
		height                uint64
		totalBucketCount      uint64 // total number of buckets including burned buckets
	}
)

func newContractStakingCache() *contractStakingCache {
	cache := &contractStakingCache{
		idBucketMap:           make(map[uint64]*ContractStakingBucketInfo),
		idBucketTypeMap:       make(map[uint64]*ContractStakingBucketType),
		propertyBucketTypeMap: make(map[int64]map[uint64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
	}
	return cache
}

func (s *contractStakingCache) putHeight(h uint64) {
	s.height = h
}

func (s *contractStakingCache) getHeight() uint64 {
	return s.height
}

func (s *contractStakingCache) putBucketType(id uint64, bt *ContractStakingBucketType) {
	amount := bt.Amount.Int64()
	s.idBucketTypeMap[id] = bt
	m, ok := s.propertyBucketTypeMap[amount]
	if !ok {
		s.propertyBucketTypeMap[amount] = make(map[uint64]uint64)
		m = s.propertyBucketTypeMap[amount]
	}
	m[bt.Duration] = id
}

func (s *contractStakingCache) putBucketInfo(id uint64, bi *ContractStakingBucketInfo) {
	s.idBucketMap[id] = bi
	if _, ok := s.candidateBucketMap[bi.Delegate.String()]; !ok {
		s.candidateBucketMap[bi.Delegate.String()] = make(map[uint64]bool)
	}
	s.candidateBucketMap[bi.Delegate.String()][id] = true
}

func (s *contractStakingCache) deleteBucketInfo(id uint64) {
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

func (s *contractStakingCache) getBucketTypeIndex(amount *big.Int, duration uint64) (uint64, bool) {
	m, ok := s.propertyBucketTypeMap[amount.Int64()]
	if !ok {
		return 0, false
	}
	id, ok := m[duration]
	return id, ok
}

func (s *contractStakingCache) getBucketType(id uint64) (*ContractStakingBucketType, bool) {
	bt, ok := s.idBucketTypeMap[id]
	return bt, ok
}

func (s *contractStakingCache) mustGetBucketType(id uint64) *ContractStakingBucketType {
	bt, ok := s.idBucketTypeMap[id]
	if !ok {
		panic("bucket type not found")
	}
	return bt
}

func (s *contractStakingCache) getBucketInfo(id uint64) (*ContractStakingBucketInfo, bool) {
	bi, ok := s.idBucketMap[id]
	return bi, ok
}

func (s *contractStakingCache) mustGetBucketInfo(id uint64) *ContractStakingBucketInfo {
	bt, ok := s.idBucketMap[id]
	if !ok {
		panic("bucket info not found")
	}
	return bt
}

func (s *contractStakingCache) getCandidateVotes(candidate address.Address) *big.Int {
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
		if bi.UnstakedAt != maxBlockNumber {
			continue
		}
		bt := s.mustGetBucketType(bi.TypeIndex)
		votes.Add(votes, bt.Amount)
	}
	return votes
}

func (s *contractStakingCache) putTotalBucketCount(count uint64) {
	s.totalBucketCount = count
}

func (s *contractStakingCache) getTotalBucketCount() uint64 {
	return s.totalBucketCount
}

func (s *contractStakingCache) getTotalBucketTypeCount() uint64 {
	return uint64(len(s.idBucketTypeMap))
}

func (s *contractStakingCache) getAllBucketInfo() map[uint64]*ContractStakingBucketInfo {
	m := make(map[uint64]*ContractStakingBucketInfo)
	for k, v := range s.idBucketMap {
		m[k] = v
	}
	return m
}

func (s *contractStakingCache) getActiveBucketType() map[uint64]*ContractStakingBucketType {
	m := make(map[uint64]*ContractStakingBucketType)
	for k, v := range s.idBucketTypeMap {
		if v.ActivatedAt != maxBlockNumber {
			m[k] = v
		}
	}
	return m
}

func (s *contractStakingCache) getBucketInfoByCandidate(candidate address.Address) map[uint64]*ContractStakingBucketInfo {
	m := make(map[uint64]*ContractStakingBucketInfo)
	for k, v := range s.candidateBucketMap[candidate.String()] {
		if v {
			m[k] = s.idBucketMap[k]
		}
	}
	return m
}

func (s *contractStakingCache) merge(delta *contractStakingDelta) error {
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
