package blockindex

import (
	"math/big"
	"time"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/pkg/errors"
)

type (
	liquidStakingCache struct {
		bucketMap     map[uint64]*BucketInfo // map[token]BucketInfo
		bucketTypes   map[uint64]*BucketType
		bucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
	}

	liquidStakingData struct {
		dirty      batch.CachedBatch // im-memory dirty data
		dirtyCache *liquidStakingCache
		clean      db.KVStore          // clean data in db
		cleanCache *liquidStakingCache // in-memory index for clean data
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

func newLiquidStakingData(kvStore db.KVStore) (*liquidStakingData, error) {
	data := liquidStakingData{
		dirty:      batch.NewCachedBatch(),
		clean:      kvStore,
		cleanCache: newLiquidStakingCache(),
		dirtyCache: newLiquidStakingCache(),
	}
	return &data, nil
}

func (s *liquidStakingData) loadCache() error {
	ks, vs, err := s.clean.Filter(_liquidStakingBucketInfoNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	}
	for i := range vs {
		var b BucketInfo
		if err := b.deserialize(vs[i]); err != nil {
			return err
		}
		s.cleanCache.putBucketInfo(deserializeUint64(ks[i]), &b)
	}

	ks, vs, err = s.clean.Filter(_liquidStakingBucketTypeNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
	}
	for i := range vs {
		var b BucketType
		if err := b.deserialize(vs[i]); err != nil {
			return err
		}
		s.cleanCache.putBucketType(deserializeUint64(ks[i]), &b)
	}
	return nil
}

func (s *liquidStakingData) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	id, ok := s.dirtyCache.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = s.cleanCache.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (s *liquidStakingData) getBucketTypeCount() uint64 {
	base := len(s.cleanCache.bucketTypes)
	add := 0
	for k, dbt := range s.dirtyCache.bucketTypes {
		_, ok := s.cleanCache.bucketTypes[k]
		if dbt != nil && !ok {
			add++
		} else if dbt == nil && ok {
			add--
		}
	}
	return uint64(base + add)
}

func (s *liquidStakingData) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.dirtyCache.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = s.cleanCache.getBucketType(id)
	return bt, ok
}

func (s *liquidStakingData) putBucketType(id uint64, bt *BucketType) {
	s.dirty.Put(_liquidStakingBucketTypeNS, serializeUint64(id), bt.serialize(), "failed to put bucket type")
	s.dirtyCache.putBucketType(id, bt)
}

func (s *liquidStakingData) putBucketInfo(id uint64, bi *BucketInfo) {
	s.dirty.Put(_liquidStakingBucketInfoNS, serializeUint64(id), bi.serialize(), "failed to put bucket info")
	s.dirtyCache.putBucketInfo(id, bi)
}

func (s *liquidStakingData) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.dirtyCache.getBucketInfo(id)
	if ok {
		return bi, bi != nil
	}
	bi, ok = s.cleanCache.getBucketInfo(id)
	return bi, ok
}

func (s *liquidStakingData) burnBucket(id uint64) {
	s.dirty.Delete(_liquidStakingBucketInfoNS, serializeUint64(id), "failed to delete bucket info")
	s.dirtyCache.putBucketInfo(id, nil)
}

// GetBuckets(height uint64, offset, limit uint32)
// BucketsByVoter(voterAddr string, offset, limit uint32)
// BucketsByCandidate(candidateAddr string, offset, limit uint32)
// BucketByIndices(indecis []uint64)
// BucketCount()
// TotalStakingAmount()

func (s *liquidStakingData) commit() error {
	if err := s.cleanCache.writeBatch(s.dirty); err != nil {
		return err
	}
	if err := s.clean.WriteBatch(s.dirty); err != nil {
		return err
	}
	s.dirty.Lock()
	s.dirty.ClearAndUnlock()
	s.dirtyCache = newLiquidStakingCache()
	return nil
}
