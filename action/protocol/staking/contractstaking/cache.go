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

func (s *contractStakingCache) GetBuckets() ([]*Bucket, error) {
	vbs := []*Bucket{}
	for id, bi := range s.getAllBucketInfo() {
		bt := s.mustGetBucketType(bi.TypeIndex)
		vb, err := assembleBucket(id, bi, bt)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *contractStakingCache) GetBucket(id uint64) (*Bucket, error) {
	return s.getBucket(id)
}

func (s *contractStakingCache) GetBucketsByCandidate(candidate address.Address) ([]*Bucket, error) {
	bucketMap := s.getBucketInfoByCandidate(candidate)
	vbs := make([]*Bucket, 0, len(bucketMap))
	for id := range bucketMap {
		vb, err := s.getBucket(id)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *contractStakingCache) GetBucketsByIndices(indices []uint64) ([]*Bucket, error) {
	vbs := make([]*Bucket, 0, len(indices))
	for _, id := range indices {
		vb, err := s.getBucket(id)
		if err != nil {
			return nil, err
		}
		vbs = append(vbs, vb)
	}
	return vbs, nil
}

func (s *contractStakingCache) GetTotalBucketCount() uint64 {
	return s.totalBucketCount
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

func (s *contractStakingCache) getBucket(id uint64) (*Bucket, error) {
	bi, ok := s.getBucketInfo(id)
	if !ok {
		return nil, errors.Wrapf(ErrBucketNotExist, "id %d", id)
	}
	bt := s.mustGetBucketType(bi.TypeIndex)
	return assembleBucket(id, bi, bt)
}

func (s *contractStakingCache) getTotalBucketTypeCount() uint64 {
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
