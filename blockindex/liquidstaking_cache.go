// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"
	"time"
)

type (
	liquidStakingCache struct {
		idBucketMap           map[uint64]*BucketInfo     // map[token]BucketInfo
		candidateBucketMap    map[string]map[uint64]bool // map[candidate]bucket
		idBucketTypeMap       map[uint64]*BucketType     // map[token]BucketType
		propertyBucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
		height                uint64
		totalBucketCount      uint64 // total number of buckets including burned buckets
	}
)

func newLiquidStakingCache() *liquidStakingCache {
	return &liquidStakingCache{
		idBucketMap:           make(map[uint64]*BucketInfo),
		idBucketTypeMap:       make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[int64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
	}
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
	if _, ok := s.candidateBucketMap[bi.Delegate]; !ok {
		s.candidateBucketMap[bi.Delegate] = make(map[uint64]bool)
	}
	s.candidateBucketMap[bi.Delegate][id] = true
}

func (s *liquidStakingCache) deleteBucketInfo(id uint64) {
	bi, ok := s.idBucketMap[id]
	if !ok {
		return
	}
	delete(s.idBucketMap, id)
	if _, ok := s.candidateBucketMap[bi.Delegate]; !ok {
		return
	}
	delete(s.candidateBucketMap[bi.Delegate], id)
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

func (s *liquidStakingCache) getCandidateVotes(ownerAddr string) *big.Int {
	votes := big.NewInt(0)
	m, ok := s.candidateBucketMap[ownerAddr]
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
