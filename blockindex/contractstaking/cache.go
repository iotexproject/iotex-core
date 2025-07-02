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

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type (
	stakingCache interface {
		BucketInfo(id uint64) (*bucketInfo, bool)
		MustGetBucketInfo(id uint64) *bucketInfo
		MustGetBucketType(id uint64) *BucketType
		MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool)
		BucketType(id uint64) (*BucketType, bool)
		BucketTypeCount() int
		BucketsByCandidate(candidate address.Address) ([]uint64, []*BucketType, []*bucketInfo)
		TotalBucketCount() uint64
		IsDirty() bool
		Base() stakingCache

		PutBucketType(id uint64, bt *BucketType)
		PutBucketInfo(id uint64, bi *bucketInfo)
		DeleteBucketInfo(id uint64)
		Commit()
	}
	contractStakingCache struct {
		bucketInfoMap         map[uint64]*bucketInfo      // map[token]bucketInfo
		candidateBucketMap    map[string]map[uint64]bool  // map[candidate]bucket
		bucketTypeMap         map[uint64]*BucketType      // map[bucketTypeId]BucketType
		propertyBucketTypeMap map[int64]map[uint64]uint64 // map[amount][duration]index
		totalBucketCount      uint64                      // total number of buckets including burned buckets
		height                uint64                      // current block height, it's put in cache for consistency on merge
		mutex                 sync.RWMutex                // a RW mutex for the cache to protect concurrent access
	}
)

var (
	// ErrBucketNotExist is the error when bucket does not exist
	ErrBucketNotExist = errors.New("bucket does not exist")
	// ErrInvalidHeight is the error when height is invalid
	ErrInvalidHeight = errors.New("invalid height")
)

func newContractStakingCache() *contractStakingCache {
	return &contractStakingCache{
		bucketInfoMap:         make(map[uint64]*bucketInfo),
		bucketTypeMap:         make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[uint64]uint64),
		candidateBucketMap:    make(map[string]map[uint64]bool),
	}
}

func (s *contractStakingCache) Buckets() ([]uint64, []*BucketType, []*bucketInfo) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	ids := make([]uint64, 0, len(s.bucketInfoMap))
	ts := make([]*BucketType, 0, len(s.bucketInfoMap))
	infos := make([]*bucketInfo, 0, len(s.bucketInfoMap))
	for id, bi := range s.bucketInfoMap {
		ids = append(ids, id)
		bt := s.mustGetBucketType(bi.TypeIndex)
		ts = append(ts, bt)
		infos = append(infos, bi.Clone())
	}

	return sortByIds(ids, ts, infos)
}

func (s *contractStakingCache) Bucket(id uint64) (*BucketType, *bucketInfo) {
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

func (s *contractStakingCache) BucketsByCandidate(candidate address.Address) ([]uint64, []*BucketType, []*bucketInfo) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	bucketMap := s.candidateBucketMap[candidate.String()]
	ids := make([]uint64, 0, len(bucketMap))
	ts := make([]*BucketType, 0, len(bucketMap))
	infos := make([]*bucketInfo, 0, len(bucketMap))
	for id := range bucketMap {
		info := s.mustGetBucketInfo(id)
		t := s.mustGetBucketType(info.TypeIndex)
		ids = append(ids, id)
		ts = append(ts, t)
		infos = append(infos, info)
	}

	return sortByIds(ids, ts, infos)
}

func (s *contractStakingCache) BucketsByIndices(indices []uint64) ([]*BucketType, []*bucketInfo) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	vbs := make([]*BucketType, 0, len(indices))
	infos := make([]*bucketInfo, 0, len(indices))
	for _, id := range indices {
		bt, info := s.getBucket(id)
		vbs = append(vbs, bt)
		infos = append(infos, info)
	}
	return vbs, infos
}

func (s *contractStakingCache) TotalBucketCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.totalBucketCount
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

func (s *contractStakingCache) MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	id, ok := s.getBucketTypeIndex(amount, duration)
	if !ok {
		return 0, nil, false
	}
	return id, s.mustGetBucketType(id), true
}

func (s *contractStakingCache) LoadFromDB(kvstore db.KVStore) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// load total bucket count
	var totalBucketCount uint64
	tbc, err := kvstore.Get(_StakingNS, _stakingTotalBucketCountKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		totalBucketCount = 0
	} else {
		totalBucketCount = byteutil.BytesToUint64BigEndian(tbc)
	}
	s.totalBucketCount = totalBucketCount

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

func (s *contractStakingCache) Clone() *contractStakingCache {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	c := &contractStakingCache{
		totalBucketCount: s.totalBucketCount,
	}
	c.bucketInfoMap = make(map[uint64]*bucketInfo, len(s.bucketInfoMap))
	for k, v := range s.bucketInfoMap {
		c.bucketInfoMap[k] = v.Clone()
	}
	c.candidateBucketMap = make(map[string]map[uint64]bool, len(s.candidateBucketMap))
	for k, v := range s.candidateBucketMap {
		c.candidateBucketMap[k] = make(map[uint64]bool, len(v))
		for k1, v1 := range v {
			c.candidateBucketMap[k][k1] = v1
		}
	}
	c.bucketTypeMap = make(map[uint64]*BucketType, len(s.bucketTypeMap))
	for k, v := range s.bucketTypeMap {
		c.bucketTypeMap[k] = v.Clone()
	}
	c.propertyBucketTypeMap = make(map[int64]map[uint64]uint64, len(s.propertyBucketTypeMap))
	for k, v := range s.propertyBucketTypeMap {
		c.propertyBucketTypeMap[k] = make(map[uint64]uint64, len(v))
		for k1, v1 := range v {
			c.propertyBucketTypeMap[k][k1] = v1
		}
	}
	return c
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
	return bi.Clone(), ok
}

func (s *contractStakingCache) mustGetBucketInfo(id uint64) *bucketInfo {
	bt, ok := s.getBucketInfo(id)
	if !ok {
		panic("bucket info not found")
	}
	return bt
}

func (s *contractStakingCache) getBucket(id uint64) (*BucketType, *bucketInfo) {
	bi, ok := s.getBucketInfo(id)
	if !ok {
		return nil, nil
	}
	return s.mustGetBucketType(bi.TypeIndex), bi
}

func (s *contractStakingCache) putBucketType(id uint64, bt *BucketType) {
	// remove old bucket map
	if oldBt, existed := s.bucketTypeMap[id]; existed {
		if oldBt.Amount.Cmp(bt.Amount) != 0 || oldBt.Duration != bt.Duration {
			panic("bucket type amount or duration cannot be changed")
		}
	}
	// add new bucket map
	amount := bt.Amount.Int64()
	m, ok := s.propertyBucketTypeMap[amount]
	if !ok {
		s.propertyBucketTypeMap[amount] = make(map[uint64]uint64)
		m = s.propertyBucketTypeMap[amount]
	} else {
		oldId, ok := m[bt.Duration]
		if ok && oldId != id {
			panic("bucket type with same amount and duration already exists")
		}
	}
	s.bucketTypeMap[id] = bt
	m[bt.Duration] = id
}

func (s *contractStakingCache) putBucketInfo(id uint64, bi *bucketInfo) {
	oldBi := s.bucketInfoMap[id]
	s.bucketInfoMap[id] = bi
	if id > s.totalBucketCount {
		s.totalBucketCount = id
	}
	// update candidate bucket map
	newDelegate := bi.Delegate.String()
	if _, ok := s.candidateBucketMap[newDelegate]; !ok {
		s.candidateBucketMap[newDelegate] = make(map[uint64]bool)
	}
	s.candidateBucketMap[newDelegate][id] = true
	// delete old candidate bucket map
	if oldBi == nil {
		return
	}
	oldDelegate := oldBi.Delegate.String()
	if oldDelegate == newDelegate {
		return
	}
	delete(s.candidateBucketMap[oldDelegate], id)
	if len(s.candidateBucketMap[oldDelegate]) == 0 {
		delete(s.candidateBucketMap, oldDelegate)
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

func (s *contractStakingCache) SetHeight(height uint64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.height = height
}

func (s *contractStakingCache) BucketTypeCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return len(s.bucketTypeMap)
}

func (s *contractStakingCache) Base() stakingCache {
	return s
}

func (s *contractStakingCache) IsDirty() bool {
	return false
}

func (s *contractStakingCache) Commit() {
}
