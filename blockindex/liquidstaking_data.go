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
		idBucketMap           map[uint64]*BucketInfo     // map[token]BucketInfo
		candidateBucketMap    map[string]map[uint64]bool // map[candidate]bucket
		idBucketTypeMap       map[uint64]*BucketType
		propertyBucketTypeMap map[int64]map[int64]uint64 // map[amount][duration]index
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
		idBucketMap:           make(map[uint64]*BucketInfo),
		idBucketTypeMap:       make(map[uint64]*BucketType),
		propertyBucketTypeMap: make(map[int64]map[int64]uint64),
	}
}

func (s *liquidStakingCache) writeBatch(b batch.KVStoreBatch) error {
	for i := 0; i < b.Size(); i++ {
		write, err := b.Entry(i)
		if err != nil {
			return err
		}
		switch write.Namespace() {
		case _liquidStakingBucketInfoNS:
			if write.WriteType() == batch.Put {
				var bi BucketInfo
				if err = bi.deserialize(write.Value()); err != nil {
					return err
				}
				id := deserializeUint64(write.Key())
				s.putBucketInfo(id, &bi)
			} else if write.WriteType() == batch.Delete {
				id := deserializeUint64(write.Key())
				s.deleteBucketInfo(id)
			}
		case _liquidStakingBucketTypeNS:
			if write.WriteType() == batch.Put {
				var bt BucketType
				if err = bt.deserialize(write.Value()); err != nil {
					return err
				}
				id := deserializeUint64(write.Key())
				s.putBucketType(id, &bt)
			}
		}
	}
	return nil
}

func (s *liquidStakingCache) putBucketType(id uint64, bt *BucketType) {
	s.idBucketTypeMap[id] = bt
	s.propertyBucketTypeMap[bt.Amount.Int64()][int64(bt.Duration)] = id
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
	s.idBucketTypeMap[id] = nil
	if _, ok := s.candidateBucketMap[bi.Delegate]; !ok {
		return
	}
	s.candidateBucketMap[bi.Delegate][id] = false
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

func (s *liquidStakingCache) getCandidateVotes(name string) *big.Int {
	votes := big.NewInt(0)
	m, ok := s.candidateBucketMap[name]
	if !ok {
		return votes
	}
	for k, v := range m {
		if v {
			bi, ok := s.idBucketMap[k]
			if !ok {
				continue
			}
			bt := s.mustGetBucketType(bi.TypeIndex)
			votes.Add(votes, bt.Amount)
		}
	}
	return votes
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

func (s *liquidStakingIndexer) loadCache() error {
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

func (s *liquidStakingIndexer) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	id, ok := s.dirtyCache.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = s.cleanCache.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (s *liquidStakingIndexer) getBucketTypeCount() uint64 {
	base := len(s.cleanCache.idBucketTypeMap)
	add := 0
	for k, dbt := range s.dirtyCache.idBucketTypeMap {
		_, ok := s.cleanCache.idBucketTypeMap[k]
		if dbt != nil && !ok {
			add++
		} else if dbt == nil && ok {
			add--
		}
	}
	return uint64(base + add)
}

func (s *liquidStakingIndexer) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.dirtyCache.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = s.cleanCache.getBucketType(id)
	return bt, ok
}

func (s *liquidStakingIndexer) putBucketType(id uint64, bt *BucketType) {
	s.dirty.Put(_liquidStakingBucketTypeNS, serializeUint64(id), bt.serialize(), "failed to put bucket type")
	s.dirtyCache.putBucketType(id, bt)
}

func (s *liquidStakingIndexer) putBucketInfo(id uint64, bi *BucketInfo) {
	s.dirty.Put(_liquidStakingBucketInfoNS, serializeUint64(id), bi.serialize(), "failed to put bucket info")
	s.dirtyCache.putBucketInfo(id, bi)
}

func (s *liquidStakingIndexer) getBucketInfo(id uint64) (*BucketInfo, bool) {
	bi, ok := s.dirtyCache.getBucketInfo(id)
	if ok {
		return bi, bi != nil
	}
	bi, ok = s.cleanCache.getBucketInfo(id)
	return bi, ok
}

func (s *liquidStakingIndexer) burnBucket(id uint64) {
	s.dirty.Delete(_liquidStakingBucketInfoNS, serializeUint64(id), "failed to delete bucket info")
	s.dirtyCache.putBucketInfo(id, nil)
}

// GetBuckets(height uint64, offset, limit uint32)
// BucketsByVoter(voterAddr string, offset, limit uint32)
// BucketsByCandidate(candidateAddr string, offset, limit uint32)
// BucketByIndices(indecis []uint64)
// BucketCount()
// TotalStakingAmount()

func (s *liquidStakingIndexer) commit() error {
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
