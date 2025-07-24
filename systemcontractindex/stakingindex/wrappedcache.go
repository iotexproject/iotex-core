package stakingindex

import (
	"slices"
	"sync"

	"github.com/iotexproject/iotex-address/address"
)

type wrappedCache struct {
	cache              indexerCache
	bucketsByCandidate map[string]map[uint64]bool // buckets by candidate in current block
	updatedBuckets     map[uint64]*Bucket         // updated buckets in current block

	mu              sync.RWMutex
	commitWithClone bool // whether to commit with deep clone
}

func newWrappedCache(cache indexerCache) *wrappedCache {
	return &wrappedCache{
		cache:              cache,
		bucketsByCandidate: make(map[string]map[uint64]bool),
		updatedBuckets:     make(map[uint64]*Bucket),
	}
}

func newWrappedCacheWithCloneInCommit(cache indexerCache) *wrappedCache {
	return &wrappedCache{
		cache:              cache,
		bucketsByCandidate: make(map[string]map[uint64]bool),
		updatedBuckets:     make(map[uint64]*Bucket),
		commitWithClone:    true,
	}
}

func (w *wrappedCache) PutBucket(id uint64, bkt *Bucket) {
	w.mu.Lock()
	defer w.mu.Unlock()
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
	cand := bkt.Candidate.String()
	if w.bucketsByCandidate[cand] == nil {
		w.bucketsByCandidate[cand] = make(map[uint64]bool)
	}
	w.bucketsByCandidate[cand][id] = true
}

func (w *wrappedCache) DeleteBucket(id uint64) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.updatedBuckets[id] = nil
}

func (w *wrappedCache) BucketIdxs() []uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	idxMap := make(map[uint64]struct{})
	// Load from underlying cache
	for _, id := range w.cache.BucketIdxs() {
		if bucket, exist := w.updatedBuckets[id]; !exist || bucket != nil {
			idxMap[id] = struct{}{}
		}
	}
	// Add updatedBuckets
	for id, bucket := range w.updatedBuckets {
		if bucket != nil {
			idxMap[id] = struct{}{}
		}
	}
	idxs := make([]uint64, 0, len(idxMap))
	for id := range idxMap {
		idxs = append(idxs, id)
	}
	slices.Sort(idxs)
	return idxs
}

func (w *wrappedCache) Bucket(id uint64) *Bucket {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if bkt, ok := w.updatedBuckets[id]; ok {
		if bkt == nil {
			return nil
		}
		return bkt.Clone()
	}
	return w.cache.Bucket(id)
}

func (w *wrappedCache) Buckets(indices []uint64) []*Bucket {
	w.mu.RLock()
	defer w.mu.RUnlock()
	buckets := make([]*Bucket, 0, len(indices))
	for _, idx := range indices {
		if bkt, ok := w.updatedBuckets[idx]; ok {
			if bkt == nil {
				buckets = append(buckets, nil)
			} else {
				buckets = append(buckets, bkt.Clone())
			}
		} else if bkt := w.cache.Bucket(idx); bkt != nil {
			buckets = append(buckets, bkt.Clone())
		}
	}
	return buckets
}

func (w *wrappedCache) BucketIdsByCandidate(candidate address.Address) []uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
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
	for id, bucket := range w.updatedBuckets {
		if bucket == nil {
			delete(ids, id)
		}
	}
	result := make([]uint64, 0, len(ids))
	for id := range ids {
		result = append(result, id)
	}
	slices.Sort(result)
	return result
}

func (w *wrappedCache) TotalBucketCount() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	// TODO: update total bucket count based on current block changes
	return w.cache.TotalBucketCount()
}

func (w *wrappedCache) IsDirty() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.cache.IsDirty() || w.isDirty()
}

func (w *wrappedCache) Clone() indexerCache {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cache := w.cache.Clone()
	wc := newWrappedCache(cache)
	for id, bkt := range w.updatedBuckets {
		if bkt != nil {
			wc.updatedBuckets[id] = bkt.Clone()
		} else {
			wc.updatedBuckets[id] = nil
		}
	}
	for cand, btks := range w.bucketsByCandidate {
		wc.bucketsByCandidate[cand] = make(map[uint64]bool)
		for id, keep := range btks {
			wc.bucketsByCandidate[cand][id] = keep
		}
	}
	wc.commitWithClone = w.commitWithClone
	return wc
}

func (w *wrappedCache) Commit() indexerCache {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.isDirty() {
		cache := w.cache
		if w.commitWithClone {
			cache = w.cache.Clone()
		}
		for id, bkt := range w.updatedBuckets {
			if bkt == nil {
				cache.DeleteBucket(id)
			} else {
				cache.PutBucket(id, bkt)
			}
		}
		w.updatedBuckets = make(map[uint64]*Bucket)
		w.bucketsByCandidate = make(map[string]map[uint64]bool)
		w.cache = cache
	}
	return w.cache.Commit()
}

func (w *wrappedCache) isDirty() bool {
	return len(w.updatedBuckets) > 0 || len(w.bucketsByCandidate) > 0
}
