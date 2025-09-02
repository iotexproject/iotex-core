package stakingindex

import (
	"context"
	"errors"
	"sync"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
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
		Clone() indexerCache
		Commit(context.Context, address.Address, bool, protocol.StateManager) (indexerCache, error)
		IsDirty() bool
	}
	// base is the in-memory base for staking index
	// it is not thread-safe and should be protected by the caller
	base struct {
		buckets            map[uint64]*Bucket
		bucketsByCandidate map[string]map[uint64]struct{}
		totalBucketCount   uint64
		mu                 sync.RWMutex
		delta              map[uint64]*Bucket
	}
)

func newCache() *base {
	return &base{
		buckets:            make(map[uint64]*Bucket),
		bucketsByCandidate: make(map[string]map[uint64]struct{}),
		delta:              make(map[uint64]*Bucket),
	}
}

func (s *base) Load(kvstore db.KVStore, ns, bucketNS string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *base) Clone() indexerCache {
	s.mu.RLock()
	defer s.mu.RUnlock()
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

	for k, v := range s.delta {
		if v == nil {
			c.delta[k] = nil
		} else {
			c.delta[k] = v.Clone()
		}
	}

	return c
}

func (s *base) PutBucket(id uint64, bkt *Bucket) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

	s.delta[id] = bkt
}

func (s *base) DeleteBucket(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
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

	s.delta[id] = nil
}

func (s *base) BucketIdxs() []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	idxs := make([]uint64, 0, len(s.buckets))
	for id := range s.buckets {
		idxs = append(idxs, id)
	}
	return idxs
}

func (s *base) Bucket(id uint64) *Bucket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if bkt, ok := s.buckets[id]; ok {
		return bkt.Clone()
	}
	return nil
}

func (s *base) Buckets(indices []uint64) []*Bucket {
	s.mu.RLock()
	defer s.mu.RUnlock()
	buckets := make([]*Bucket, 0, len(indices))
	for _, idx := range indices {
		if bkt, ok := s.buckets[idx]; ok {
			buckets = append(buckets, bkt.Clone())
		}
	}
	return buckets
}

func (s *base) BucketIdsByCandidate(candidate address.Address) []uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cand := candidate.String()
	buckets := make([]uint64, 0, len(s.bucketsByCandidate[cand]))
	for idx := range s.bucketsByCandidate[cand] {
		buckets = append(buckets, idx)
	}
	return buckets
}

func (s *base) TotalBucketCount() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.totalBucketCount
}

func (s *base) IsDirty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return false
}

func (s *base) Commit(ctx context.Context, ca address.Address, timestamp bool, sm protocol.StateManager) (indexerCache, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sm == nil {
		s.delta = make(map[uint64]*Bucket)
		return s, nil
	}
	cssm := contractstaking.NewContractStakingStateManager(sm)
	for id, bkt := range s.delta {
		if bkt == nil {
			if err := cssm.DeleteBucket(ca, id); err != nil {
				return nil, err
			}
		} else {
			bkt.IsTimestampBased = timestamp
			if err := cssm.UpsertBucket(ca, id, bkt); err != nil {
				return nil, err
			}
		}
	}
	s.delta = make(map[uint64]*Bucket)
	return s, nil
}
