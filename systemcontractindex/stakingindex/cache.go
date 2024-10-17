package stakingindex

import (
	"errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/db"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

// cache is the in-memory cache for staking index
// it is not thread-safe and should be protected by the caller
type cache struct {
	buckets            map[uint64]*Bucket
	bucketsByCandidate map[string]map[uint64]struct{}
	totalBucketCount   uint64
	ns, bucketNS       string
}

func newCache(ns, bucketNS string) *cache {
	return &cache{
		buckets:            make(map[uint64]*Bucket),
		bucketsByCandidate: make(map[string]map[uint64]struct{}),
		ns:                 ns,
		bucketNS:           bucketNS,
	}
}

func (s *cache) Load(kvstore db.KVStore) error {
	// load total bucket count
	var totalBucketCount uint64
	tbc, err := kvstore.Get(s.ns, stakingTotalBucketCountKey)
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
	ks, vs, err := kvstore.Filter(s.bucketNS, func(k, v []byte) bool { return true }, nil, nil)
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

func (s *cache) Copy() *cache {
	c := newCache(s.ns, s.bucketNS)
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

func (s *cache) PutBucket(id uint64, bkt *Bucket) {
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
	return
}

func (s *cache) DeleteBucket(id uint64) {
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

func (s *cache) BucketIdxs() []uint64 {
	idxs := make([]uint64, 0, len(s.buckets))
	for id := range s.buckets {
		idxs = append(idxs, id)
	}
	return idxs
}

func (s *cache) Bucket(id uint64) *Bucket {
	if bkt, ok := s.buckets[id]; ok {
		return bkt.Clone()
	}
	return nil
}

func (s *cache) Buckets(indices []uint64) []*Bucket {
	buckets := make([]*Bucket, 0, len(indices))
	for _, idx := range indices {
		if bkt, ok := s.buckets[idx]; ok {
			buckets = append(buckets, bkt.Clone())
		}
	}
	return buckets
}

func (s *cache) BucketIdsByCandidate(candidate address.Address) []uint64 {
	cand := candidate.String()
	buckets := make([]uint64, 0, len(s.bucketsByCandidate[cand]))
	for idx := range s.bucketsByCandidate[cand] {
		buckets = append(buckets, idx)
	}
	return buckets
}

func (s *cache) TotalBucketCount() uint64 {
	return s.totalBucketCount
}
