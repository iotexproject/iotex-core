package blockindex

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/db/batch"
)

type (
	liquidStakingCache struct {
		bucketMap     map[uint64]*BucketInfo // map[token]bucketInfo
		bucketTypes   map[uint64]*BucketType
		bucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
	}
)

func newLiquidStakingCache() *liquidStakingCache {
	return &liquidStakingCache{
		bucketMap:     make(map[uint64]*BucketInfo),
		bucketTypes:   make(map[uint64]*BucketType),
		bucketTypeMap: make(map[int64]map[int64]uint64),
	}
}

func (s *liquidStakingCache) writeBatch(batch batch.KVStoreBatch) error {
	// TODO (iip-13): index write batch
	return nil
}

func (s *liquidStakingCache) putBucketType(id uint64, bt *BucketType) {
	s.bucketTypes[id] = bt
	s.bucketTypeMap[bt.Amount.Int64()][int64(bt.Duration)] = id
}

func (s *liquidStakingCache) putBucketInfo(id uint64, bi *BucketInfo) {
	s.bucketMap[id] = bi
}

func (s *liquidStakingCache) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	m, ok := s.bucketTypeMap[amount.Int64()]
	if !ok {
		return 0, false
	}
	id, ok := m[int64(duration)]
	return id, ok
}

func (s *liquidStakingCache) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.bucketTypes[id]
	return bt, ok
}

func (s *liquidStakingCache) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.bucketMap[id]
	return bi, ok
}
