package stakingindex

import (
	"errors"
	"sync"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/db"
)

type (
	bucketCache interface {
		Load(kvstore db.KVStore) error
		Copy() bucketCache
		PutBucket(id uint64, bkt *Bucket)
		DeleteBucket(id uint64)
		BucketIdxs() []uint64
		Bucket(id uint64) *Bucket
		Buckets(indices []uint64) []*Bucket
		BucketIdsByCandidate(candidate address.Address) []uint64
		TotalBucketCount() uint64
	}

	cowCache struct {
		cache bucketCache
		dirty bool
		mu    sync.Mutex
	}
)

func newCowCache(cache bucketCache) *cowCache {
	return &cowCache{
		cache: cache,
		dirty: false,
	}
}

func (cow *cowCache) Copy() bucketCache {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	if cow.dirty {
		cow.dirty = false
	}
	return &cowCache{
		cache: cow.cache,
		dirty: false,
	}
}

func (cow *cowCache) Load(kvstore db.KVStore) error {
	return errors.New("not supported in cowCache")
}

func (cow *cowCache) BucketIdsByCandidate(candidate address.Address) []uint64 {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	return cow.cache.BucketIdsByCandidate(candidate)
}

func (cow *cowCache) PutBucket(id uint64, bkt *Bucket) {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	cow.ensureCopied()
	cow.cache.PutBucket(id, bkt)
}

func (cow *cowCache) DeleteBucket(id uint64) {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	cow.ensureCopied()
	cow.cache.DeleteBucket(id)
}

func (cow *cowCache) BucketIdxs() []uint64 {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	return cow.cache.BucketIdxs()
}

func (cow *cowCache) Bucket(id uint64) *Bucket {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	return cow.cache.Bucket(id)
}

func (cow *cowCache) Buckets(indices []uint64) []*Bucket {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	return cow.cache.Buckets(indices)
}

func (cow *cowCache) TotalBucketCount() uint64 {
	cow.mu.Lock()
	defer cow.mu.Unlock()
	return cow.cache.TotalBucketCount()
}

func (cow *cowCache) ensureCopied() {
	if !cow.dirty {
		cow.cache = cow.cache.Copy()
		cow.dirty = true
	}
}
