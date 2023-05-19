// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
)

type (
	contractStakingCache struct {
		idBucketMap           map[uint64]*bucketInfo      // map[token]bucketInfo
		candidateBucketMap    map[string]map[uint64]bool  // map[candidate]bucket
		idBucketTypeMap       map[uint64]*BucketType      // map[token]BucketType
		propertyBucketTypeMap map[int64]map[uint64]uint64 // map[amount][duration]index
		height                uint64
		totalBucketCount      uint64 // total number of buckets including burned buckets
	}
)

var (
	// ErrBucketNotExist is the error when bucket does not exist
	ErrBucketNotExist = errors.New("bucket does not exist")
)

func newContractStakingCache() *contractStakingCache {
	cache := &contractStakingCache{
		idBucketMap:           make(map[uint64]*bucketInfo),
		idBucketTypeMap:       make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[uint64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
	}
	return cache
}

func (s *contractStakingCache) GetHeight() uint64 {
	return s.height
}

func (s *contractStakingCache) GetCandidateVotes(candidate address.Address) *big.Int {
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

func (s *contractStakingCache) GetBuckets() []*Bucket {
	vbs := []*Bucket{}
	for id, bi := range s.getAllBucketInfo() {
		bt := s.mustGetBucketType(bi.TypeIndex)
		vb := assembleBucket(id, bi, bt)
		vbs = append(vbs, vb)
	}
	return vbs
}

func (s *contractStakingCache) GetBucket(id uint64) (*Bucket, bool) {
	return s.getBucket(id)
}

func (s *contractStakingCache) GetBucketInfo(id uint64) (*bucketInfo, bool) {
	return s.getBucketInfo(id)
}

func (s *contractStakingCache) MustGetBucketInfo(id uint64) *bucketInfo {
	return s.mustGetBucketInfo(id)
}

func (s *contractStakingCache) MustGetBucketType(id uint64) *BucketType {
	return s.mustGetBucketType(id)
}

func (s *contractStakingCache) GetBucketType(id uint64) (*BucketType, bool) {
	return s.getBucketType(id)
}

func (s *contractStakingCache) GetBucketsByCandidate(candidate address.Address) []*Bucket {
	bucketMap := s.getBucketInfoByCandidate(candidate)
	vbs := make([]*Bucket, 0, len(bucketMap))
	for id := range bucketMap {
		vb := s.mustGetBucket(id)
		vbs = append(vbs, vb)
	}
	return vbs
}

func (s *contractStakingCache) GetBucketsByIndices(indices []uint64) ([]*Bucket, error) {
	vbs := make([]*Bucket, 0, len(indices))
	for _, id := range indices {
		vb, ok := s.getBucket(id)
		if !ok {
			return nil, errors.Wrapf(ErrBucketNotExist, "id %d", id)
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *contractStakingCache) GetTotalBucketCount() uint64 {
	return s.getTotalBucketCount()
}

func (s *contractStakingCache) GetActiveBucketTypes() map[uint64]*BucketType {
	m := make(map[uint64]*BucketType)
	for k, v := range s.idBucketTypeMap {
		if v.ActivatedAt != maxBlockNumber {
			m[k] = v
		}
	}
	return m
}

func (s *contractStakingCache) PutBucketType(id uint64, bt *BucketType) {
	s.putBucketType(id, bt)
}

func (s *contractStakingCache) PutBucketInfo(id uint64, bi *bucketInfo) {
	s.putBucketInfo(id, bi)
}

func (s *contractStakingCache) DeleteBucketInfo(id uint64) {
	s.deleteBucketInfo(id)
}

func (s *contractStakingCache) PutHeight(h uint64) {
	s.putHeight(h)
}

func (s *contractStakingCache) Merge(delta *contractStakingDelta) error {
	for state, btMap := range delta.BucketTypeDelta() {
		if state == deltaStateAdded || state == deltaStateModified {
			for id, bt := range btMap {
				s.putBucketType(id, bt)
			}
		}
	}
	for state, biMap := range delta.BucketInfoDelta() {
		if state == deltaStateAdded || state == deltaStateModified {
			for id, bi := range biMap {
				s.putBucketInfo(id, bi)
			}
		} else if state == deltaStateRemoved {
			for id := range biMap {
				s.deleteBucketInfo(id)
			}
		}
	}
	s.putHeight(delta.GetHeight())
	s.putTotalBucketCount(s.getTotalBucketCount() + delta.AddedBucketCnt())
	return nil
}

func (s *contractStakingCache) PutTotalBucketCount(count uint64) {
	s.putTotalBucketCount(count)
}

func (s *contractStakingCache) MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	id, ok := s.getBucketTypeIndex(amount, duration)
	if !ok {
		return 0, nil, false
	}
	return id, s.mustGetBucketType(id), true
}

func (s *contractStakingCache) getBucketTypeIndex(amount *big.Int, duration uint64) (uint64, bool) {
	m, ok := s.propertyBucketTypeMap[amount.Int64()]
	if !ok {
		return 0, false
	}
	id, ok := m[duration]
	return id, ok
}

func (s *contractStakingCache) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.idBucketTypeMap[id]
	return bt, ok
}

func (s *contractStakingCache) mustGetBucketType(id uint64) *BucketType {
	bt, ok := s.idBucketTypeMap[id]
	if !ok {
		panic("bucket type not found")
	}
	return bt
}

func (s *contractStakingCache) getBucketInfo(id uint64) (*bucketInfo, bool) {
	bi, ok := s.idBucketMap[id]
	return bi, ok
}

func (s *contractStakingCache) mustGetBucketInfo(id uint64) *bucketInfo {
	bt, ok := s.idBucketMap[id]
	if !ok {
		panic("bucket info not found")
	}
	return bt
}

func (s *contractStakingCache) mustGetBucket(id uint64) *Bucket {
	bi := s.mustGetBucketInfo(id)
	bt := s.mustGetBucketType(bi.TypeIndex)
	return assembleBucket(id, bi, bt)
}

func (s *contractStakingCache) getBucket(id uint64) (*Bucket, bool) {
	bi, ok := s.getBucketInfo(id)
	if !ok {
		return nil, false
	}
	bt := s.mustGetBucketType(bi.TypeIndex)
	return assembleBucket(id, bi, bt), true
}

func (s *contractStakingCache) GetTotalBucketTypeCount() uint64 {
	return uint64(len(s.idBucketTypeMap))
}

func (s *contractStakingCache) getAllBucketInfo() map[uint64]*bucketInfo {
	m := make(map[uint64]*bucketInfo)
	for k, v := range s.idBucketMap {
		m[k] = v
	}
	return m
}

func (s *contractStakingCache) getBucketInfoByCandidate(candidate address.Address) map[uint64]*bucketInfo {
	m := make(map[uint64]*bucketInfo)
	for k, v := range s.candidateBucketMap[candidate.String()] {
		if v {
			m[k] = s.idBucketMap[k]
		}
	}
	return m
}

func (s *contractStakingCache) getTotalBucketCount() uint64 {
	return s.totalBucketCount
}

func (s *contractStakingCache) putBucketType(id uint64, bt *BucketType) {
	amount := bt.Amount.Int64()
	s.idBucketTypeMap[id] = bt
	m, ok := s.propertyBucketTypeMap[amount]
	if !ok {
		s.propertyBucketTypeMap[amount] = make(map[uint64]uint64)
		m = s.propertyBucketTypeMap[amount]
	}
	m[bt.Duration] = id
}

func (s *contractStakingCache) putBucketInfo(id uint64, bi *bucketInfo) {
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

func (s *contractStakingCache) putHeight(h uint64) {
	s.height = h
}

func (s *contractStakingCache) putTotalBucketCount(count uint64) {
	s.totalBucketCount = count
}
