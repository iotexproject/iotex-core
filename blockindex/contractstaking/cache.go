// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"sync"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	contractStakingCache struct {
		bucketInfoMap         map[uint64]*bucketInfo      // map[token]bucketInfo
		candidateBucketMap    map[string]map[uint64]bool  // map[candidate]bucket
		bucketTypeMap         map[uint64]*BucketType      // map[bucketTypeId]BucketType
		propertyBucketTypeMap map[int64]map[uint64]uint64 // map[amount][duration]index
		height                uint64
		totalBucketCount      uint64       // total number of buckets including burned buckets
		contractAddress       string       // contract address for the bucket
		mutex                 sync.RWMutex // a RW mutex for the cache to protect concurrent access
	}
)

var (
	// ErrBucketNotExist is the error when bucket does not exist
	ErrBucketNotExist = errors.New("bucket does not exist")
)

func newContractStakingCache(contractAddr string) *contractStakingCache {
	return &contractStakingCache{
		bucketInfoMap:         make(map[uint64]*bucketInfo),
		bucketTypeMap:         make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[uint64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
		contractAddress:       contractAddr,
	}
}

func (s *contractStakingCache) Height() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.height
}

func (s *contractStakingCache) CandidateVotes(candidate address.Address) *big.Int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	votes := big.NewInt(0)
	m, ok := s.candidateBucketMap[candidate.String()]
	if !ok {
		return votes
	}
	for id, existed := range m {
		if !existed {
			continue
		}
		bi := s.mustGetBucketInfo(id)
		// only count the bucket that is not unstaked
		if bi.UnstakedAt != maxBlockNumber {
			continue
		}
		bt := s.mustGetBucketType(bi.TypeIndex)
		votes.Add(votes, bt.Amount)
	}
	return votes
}

func (s *contractStakingCache) Buckets() []*Bucket {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := []*Bucket{}
	for id, bi := range s.getAllBucketInfo() {
		bt := s.mustGetBucketType(bi.TypeIndex)
		vb := assembleBucket(id, bi, bt, s.contractAddress)
		vbs = append(vbs, vb)
	}
	return vbs
}

func (s *contractStakingCache) Bucket(id uint64) (*Bucket, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getBucket(id)
}

func (s *contractStakingCache) BucketInfo(id uint64) (*bucketInfo, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getBucketInfo(id)
}

func (s *contractStakingCache) MustGetBucketInfo(id uint64) *bucketInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.mustGetBucketInfo(id)
}

func (s *contractStakingCache) MustGetBucketType(id uint64) *BucketType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.mustGetBucketType(id)
}

func (s *contractStakingCache) BucketType(id uint64) (*BucketType, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getBucketType(id)
}

func (s *contractStakingCache) BucketsByCandidate(candidate address.Address) []*Bucket {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	bucketMap := s.getBucketInfoByCandidate(candidate)
	vbs := make([]*Bucket, 0, len(bucketMap))
	for id := range bucketMap {
		vb := s.mustGetBucket(id)
		vbs = append(vbs, vb)
	}
	return vbs
}

func (s *contractStakingCache) BucketsByIndices(indices []uint64) ([]*Bucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := make([]*Bucket, 0, len(indices))
	for _, id := range indices {
		vb, ok := s.getBucket(id)
		if ok {
			vbs = append(vbs, vb)
		}
	}
	return vbs, nil
}

func (s *contractStakingCache) TotalBucketCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.getTotalBucketCount()
}

func (s *contractStakingCache) ActiveBucketTypes() map[uint64]*BucketType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	m := make(map[uint64]*BucketType)
	for k, v := range s.bucketTypeMap {
		if v.ActivatedAt != maxBlockNumber {
			m[k] = v.Clone()
		}
	}
	return m
}

func (s *contractStakingCache) PutBucketType(id uint64, bt *BucketType) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.putBucketType(id, bt)
}

func (s *contractStakingCache) PutBucketInfo(id uint64, bi *bucketInfo) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.putBucketInfo(id, bi)
}

func (s *contractStakingCache) DeleteBucketInfo(id uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.deleteBucketInfo(id)
}

func (s *contractStakingCache) PutHeight(h uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.putHeight(h)
}

func (s *contractStakingCache) Merge(delta *contractStakingDelta) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.merge(delta)
}

func (s *contractStakingCache) PutTotalBucketCount(count uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.putTotalBucketCount(count)
}

func (s *contractStakingCache) MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	id, ok := s.getBucketTypeIndex(amount, duration)
	if !ok {
		return 0, nil, false
	}
	return id, s.mustGetBucketType(id), true
}

func (s *contractStakingCache) BucketTypeCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return uint64(len(s.bucketTypeMap))
}

func (s *contractStakingCache) LoadFromDB(kvstore db.KVStore) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// load height
	var height uint64
	h, err := kvstore.Get(_StakingNS, _stakingHeightKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		height = 0
	} else {
		height = byteutil.BytesToUint64BigEndian(h)

	}
	s.putHeight(height)

	// load total bucket count
	var totalBucketCount uint64
	tbc, err := kvstore.Get(_StakingNS, _stakingTotalBucketCountKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	} else {
		totalBucketCount = byteutil.BytesToUint64BigEndian(tbc)
	}
	s.putTotalBucketCount(totalBucketCount)

	// load bucket info
	ks, vs, err := kvstore.Filter(_StakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b bucketInfo
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		s.putBucketInfo(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}

	// load bucket type
	ks, vs, err = kvstore.Filter(_StakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b BucketType
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		s.putBucketType(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}
	return nil
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
	bt, ok := s.bucketTypeMap[id]
	if !ok {
		return nil, false
	}
	return bt.Clone(), ok
}

func (s *contractStakingCache) mustGetBucketType(id uint64) *BucketType {
	bt, ok := s.getBucketType(id)
	if !ok {
		panic("bucket type not found")
	}
	return bt
}

func (s *contractStakingCache) getBucketInfo(id uint64) (*bucketInfo, bool) {
	bi, ok := s.bucketInfoMap[id]
	if !ok {
		return nil, false
	}
	return bi.clone(), ok
}

func (s *contractStakingCache) mustGetBucketInfo(id uint64) *bucketInfo {
	bt, ok := s.getBucketInfo(id)
	if !ok {
		panic("bucket info not found")
	}
	return bt
}

func (s *contractStakingCache) mustGetBucket(id uint64) *Bucket {
	bi := s.mustGetBucketInfo(id)
	bt := s.mustGetBucketType(bi.TypeIndex)
	return assembleBucket(id, bi, bt, s.contractAddress)
}

func (s *contractStakingCache) getBucket(id uint64) (*Bucket, bool) {
	bi, ok := s.getBucketInfo(id)
	if !ok {
		return nil, false
	}
	bt := s.mustGetBucketType(bi.TypeIndex)
	return assembleBucket(id, bi, bt, s.contractAddress), true
}

func (s *contractStakingCache) getAllBucketInfo() map[uint64]*bucketInfo {
	m := make(map[uint64]*bucketInfo)
	for k, v := range s.bucketInfoMap {
		m[k] = v.clone()
	}
	return m
}

func (s *contractStakingCache) getBucketInfoByCandidate(candidate address.Address) map[uint64]*bucketInfo {
	m := make(map[uint64]*bucketInfo)
	for k, v := range s.candidateBucketMap[candidate.String()] {
		if v {
			m[k] = s.bucketInfoMap[k].clone()
		}
	}
	return m
}

func (s *contractStakingCache) getTotalBucketCount() uint64 {
	return s.totalBucketCount
}

func (s *contractStakingCache) putBucketType(id uint64, bt *BucketType) {
	amount := bt.Amount.Int64()
	s.bucketTypeMap[id] = bt
	m, ok := s.propertyBucketTypeMap[amount]
	if !ok {
		s.propertyBucketTypeMap[amount] = make(map[uint64]uint64)
		m = s.propertyBucketTypeMap[amount]
	}
	m[bt.Duration] = id
}

func (s *contractStakingCache) putBucketInfo(id uint64, bi *bucketInfo) {
	oldBi := s.bucketInfoMap[id]
	s.bucketInfoMap[id] = bi
	// update candidate bucket map
	newDelegate := bi.Delegate.String()
	if _, ok := s.candidateBucketMap[newDelegate]; !ok {
		s.candidateBucketMap[newDelegate] = make(map[uint64]bool)
	}
	s.candidateBucketMap[newDelegate][id] = true
	// delete old candidate bucket map
	if oldBi != nil {
		oldDelegate := oldBi.Delegate.String()
		if oldDelegate != newDelegate {
			delete(s.candidateBucketMap[oldDelegate], id)
		}
	}
}

func (s *contractStakingCache) deleteBucketInfo(id uint64) {
	bi, ok := s.bucketInfoMap[id]
	if !ok {
		return
	}
	delete(s.bucketInfoMap, id)
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

func (s *contractStakingCache) merge(delta *contractStakingDelta) error {
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
	return nil
}
