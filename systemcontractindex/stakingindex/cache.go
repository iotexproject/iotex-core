package stakingindex

import (
	"errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

type (
	indexerCache interface {
		PutBucket(id uint64, bkt *Bucket)
		DeleteBucket(id uint64)
		BucketIdxs() []uint64
		Bucket(id uint64) *Bucket
		Buckets(indices []uint64) []*Bucket
		BucketIdsByCandidate(candidate address.Address) []uint64
		TotalBucketCount() uint64
		Base() indexerCache
		Commit() error
		IsDirty() bool
	}
	// base is the in-memory base for staking index
	// it is not thread-safe and should be protected by the caller
	base struct {
		buckets            map[uint64]*Bucket
		bucketsByCandidate map[string]map[uint64]struct{}
		totalBucketCount   uint64
	}

	wrappedCache struct {
		cache              indexerCache
		bucketsByCandidate map[string]map[uint64]bool // buckets by candidate in current block
		updatedBuckets     map[uint64]*Bucket         // updated buckets in current block
		deletedBucketIds   map[uint64]struct{}        // deleted buckets in current block
	}
)

func newCache() *base {
	return &base{
		buckets:            make(map[uint64]*Bucket),
		bucketsByCandidate: make(map[string]map[uint64]struct{}),
	}
}

func (s *base) Load(kvstore db.KVStore, ns, bucketNS string) error {
	// load total bucket count
	var totalBucketCount uint64
	tbc, err := kvstore.Get(ns, stakingTotalBucketCountKey)
	if err != nil {
		if !errors.Is(err, db.ErrNotExist) {
			return err
		}
		totalBucketCount = 0
	} else {
		totalBucketCount = byteutil.BytesToUint64BigEndian(tbc)
	}
	s.totalBucketCount = totalBucketCount

	// load buckets
	ks, vs, err := kvstore.Filter(bucketNS, func(k, v []byte) bool { return true }, nil, nil)
	if err != nil && !errors.Is(err, db.ErrBucketNotExist) {
		return err
	}
	for i := range vs {
		var b Bucket
		if err := b.Deserialize(vs[i]); err != nil {
			return err
		}
		s.PutBucket(byteutil.BytesToUint64BigEndian(ks[i]), &b)
	}
	return nil
}

func (s *base) DeepClone() indexerCache {
	c := newCache()
	for k, v := range s.buckets {
		c.buckets[k] = v.Clone()
	}
	for cand, btks := range s.bucketsByCandidate {
		c.bucketsByCandidate[cand] = make(map[uint64]struct{})
		for btxIdx := range btks {
			c.bucketsByCandidate[cand][btxIdx] = struct{}{}
		}
	}
	c.totalBucketCount = s.totalBucketCount
	return c
}

func (s *base) PutBucket(id uint64, bkt *Bucket) {
	cand := bkt.Candidate.String()
	if s.buckets[id] != nil {
		prevCand := s.buckets[id].Candidate.String()
		if prevCand != cand {
			delete(s.bucketsByCandidate[prevCand], id)
			if len(s.bucketsByCandidate[prevCand]) == 0 {
				delete(s.bucketsByCandidate, prevCand)
			}
		}
	}
	s.buckets[id] = bkt
	if s.bucketsByCandidate[cand] == nil {
		s.bucketsByCandidate[cand] = make(map[uint64]struct{})
	}
	s.bucketsByCandidate[cand][id] = struct{}{}
}

func (s *base) DeleteBucket(id uint64) {
	bkt, ok := s.buckets[id]
	if !ok {
		return
	}
	cand := bkt.Candidate.String()
	delete(s.bucketsByCandidate[cand], id)
	if len(s.bucketsByCandidate[cand]) == 0 {
		delete(s.bucketsByCandidate, cand)
	}
	delete(s.buckets, id)
}

func (s *base) BucketIdxs() []uint64 {
	idxs := make([]uint64, 0, len(s.buckets))
	for id := range s.buckets {
		idxs = append(idxs, id)
	}
	return idxs
}

func (s *base) Bucket(id uint64) *Bucket {
	if bkt, ok := s.buckets[id]; ok {
		return bkt.Clone()
	}
	return nil
}

func (s *base) Buckets(indices []uint64) []*Bucket {
	buckets := make([]*Bucket, 0, len(indices))
	for _, idx := range indices {
		if bkt, ok := s.buckets[idx]; ok {
			buckets = append(buckets, bkt.Clone())
		}
	}
	return buckets
}

func (s *base) BucketIdsByCandidate(candidate address.Address) []uint64 {
	cand := candidate.String()
	buckets := make([]uint64, 0, len(s.bucketsByCandidate[cand]))
	for idx := range s.bucketsByCandidate[cand] {
		buckets = append(buckets, idx)
	}
	return buckets
}

func (s *base) TotalBucketCount() uint64 {
	return s.totalBucketCount
}

func (s *base) Base() indexerCache {
	return s
}

func (s *base) IsDirty() bool {
	return false
}

func (s *base) Commit() error {
	return nil
}

func newWrappedCache(cache indexerCache) *wrappedCache {
	return &wrappedCache{
		cache:              cache,
		bucketsByCandidate: make(map[string]map[uint64]bool),
		updatedBuckets:     make(map[uint64]*Bucket),
		deletedBucketIds:   make(map[uint64]struct{}),
	}
}

func (w *wrappedCache) PutBucket(id uint64, bkt *Bucket) {
	oldBucket, ok := w.updatedBuckets[id]
	if !ok {
		oldBucket = w.cache.Bucket(id)
	}
	if oldBucket != nil {
		oldCand := oldBucket.Candidate.String()
		if w.bucketsByCandidate[oldCand] == nil {
			w.bucketsByCandidate[oldCand] = make(map[uint64]bool)
		}
		w.bucketsByCandidate[oldCand][id] = false
	}
	w.updatedBuckets[id] = bkt
	delete(w.deletedBucketIds, id)
	cand := bkt.Candidate.String()
	if w.bucketsByCandidate[cand] == nil {
		w.bucketsByCandidate[cand] = make(map[uint64]bool)
	}
	w.bucketsByCandidate[cand][id] = true
}

func (w *wrappedCache) DeleteBucket(id uint64) {
	w.deletedBucketIds[id] = struct{}{}
	delete(w.updatedBuckets, id)
	for cand := range w.bucketsByCandidate {
		delete(w.bucketsByCandidate[cand], id)
		if len(w.bucketsByCandidate[cand]) == 0 {
			delete(w.bucketsByCandidate, cand)
		}
	}
}

func (w *wrappedCache) BucketIdxs() []uint64 {
	idxMap := make(map[uint64]struct{})
	// Load from underlying cache
	for _, id := range w.cache.BucketIdxs() {
		if _, deleted := w.deletedBucketIds[id]; !deleted {
			idxMap[id] = struct{}{}
		}
	}
	// Add updatedBuckets
	for id := range w.updatedBuckets {
		if _, deleted := w.deletedBucketIds[id]; !deleted {
			idxMap[id] = struct{}{}
		}
	}
	idxs := make([]uint64, 0, len(idxMap))
	for id := range idxMap {
		idxs = append(idxs, id)
	}
	return idxs
}

func (w *wrappedCache) Bucket(id uint64) *Bucket {
	if _, deleted := w.deletedBucketIds[id]; deleted {
		return nil
	}
	if bkt, ok := w.updatedBuckets[id]; ok {
		return bkt.Clone()
	}
	return w.cache.Bucket(id)
}

func (w *wrappedCache) Buckets(indices []uint64) []*Bucket {
	buckets := make([]*Bucket, 0, len(indices))
	for _, idx := range indices {
		if _, deleted := w.deletedBucketIds[idx]; deleted {
			continue
		}
		if bkt, ok := w.updatedBuckets[idx]; ok {
			buckets = append(buckets, bkt.Clone())
		} else if bkt := w.cache.Bucket(idx); bkt != nil {
			buckets = append(buckets, bkt.Clone())
		}
	}
	return buckets
}

func (w *wrappedCache) BucketIdsByCandidate(candidate address.Address) []uint64 {
	cand := candidate.String()
	ids := make(map[uint64]struct{})
	// Read ids from cache first
	for _, id := range w.cache.BucketIdsByCandidate(candidate) {
		ids[id] = struct{}{}
	}
	// Update ids according to current block changes
	if vals, ok := w.bucketsByCandidate[cand]; ok {
		for id, keep := range vals {
			if keep {
				ids[id] = struct{}{}
			} else {
				delete(ids, id)
			}
		}
	}
	// Remove deleted ids
	for id := range w.deletedBucketIds {
		delete(ids, id)
	}
	result := make([]uint64, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	return result
}

func (w *wrappedCache) Base() indexerCache {
	return w.cache.Base()
}

func (w *wrappedCache) TotalBucketCount() uint64 {
	// TODO: update total bucket count based on current block changes
	return w.cache.TotalBucketCount()
}

func (w *wrappedCache) Commit() error {
	if w.isDirty() {
		for id, bkt := range w.updatedBuckets {
			w.cache.PutBucket(id, bkt)
		}
		for id := range w.deletedBucketIds {
			w.cache.DeleteBucket(id)
		}
		w.updatedBuckets = make(map[uint64]*Bucket)
		w.deletedBucketIds = make(map[uint64]struct{})
		w.bucketsByCandidate = make(map[string]map[uint64]bool)
	}
	return w.cache.Commit()
}

func (w *wrappedCache) isDirty() bool {
	return len(w.updatedBuckets) > 0 || len(w.deletedBucketIds) > 0 || len(w.bucketsByCandidate) > 0
}

func (w *wrappedCache) IsDirty() bool {
	return w.cache.IsDirty() || w.isDirty()
}
