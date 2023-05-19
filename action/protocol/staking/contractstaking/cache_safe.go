// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"sync"

	"github.com/iotexproject/iotex-address/address"
)

type (
	contractStakingCacheSafe struct {
		inner *contractStakingCache
		mutex sync.RWMutex
	}
)

func newContractStakingCacheSafe() *contractStakingCacheSafe {
	return &contractStakingCacheSafe{
		inner: newContractStakingCache(),
	}
}

func (s *contractStakingCacheSafe) Unsafe() *contractStakingCache {
	return s.inner
}

func (s *contractStakingCacheSafe) GetHeight() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetHeight()
}

func (s *contractStakingCacheSafe) GetCandidateVotes(candidate address.Address) *big.Int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetCandidateVotes(candidate)
}

func (s *contractStakingCacheSafe) GetBuckets() []*Bucket {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetBuckets()
}

func (s *contractStakingCacheSafe) GetBucket(id uint64) (*Bucket, bool) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetBucket(id)
}

func (s *contractStakingCacheSafe) GetBucketsByIndices(indices []uint64) ([]*Bucket, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetBucketsByIndices(indices)
}

func (s *contractStakingCacheSafe) GetBucketsByCandidate(candidate address.Address) []*Bucket {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetBucketsByCandidate(candidate)
}

func (s *contractStakingCacheSafe) GetTotalBucketCount() uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetTotalBucketCount()
}

func (s *contractStakingCacheSafe) GetActiveBucketTypes() map[uint64]*BucketType {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.inner.GetActiveBucketTypes()
}

func (s *contractStakingCacheSafe) Merge(delta *contractStakingDelta) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.inner.Merge(delta)
}
