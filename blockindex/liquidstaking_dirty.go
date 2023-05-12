// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"
	"sync"
	"time"

	"github.com/iotexproject/iotex-core/db/batch"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

type (
	// liquidStakingDirty is the dirty data of liquid staking
	// main functions:
	// 1. update bucket
	// 2. get up-to-date bucket
	// 3. store delta to merge to clean cache
	liquidStakingDirty struct {
		clean      *liquidStakingCache // clean cache to get buckets of last block
		delta      *liquidStakingDelta // delta for cache to store buckets of current block
		batch      batch.KVStoreBatch  // batch for db to store buckets of current block
		tokenOwner map[uint64]string
		once       sync.Once
	}
)

func newLiquidStakingDirty(clean *liquidStakingCache) *liquidStakingDirty {
	return &liquidStakingDirty{
		clean:      clean,
		delta:      newLiquidStakingDelta(),
		batch:      batch.NewBatch(),
		tokenOwner: make(map[uint64]string),
	}
}

func (s *liquidStakingCache) merge(delta *liquidStakingDelta) error {
	for id, state := range delta.bucketTypeDeltaState {
		if state == deltaStateAdded || state == deltaStateModified {
			s.putBucketType(id, delta.mustGetBucketType(id))
		}
	}
	for id, state := range delta.bucketInfoDeltaState {
		if state == deltaStateAdded || state == deltaStateModified {
			s.putBucketInfo(id, delta.mustGetBucketInfo(id))
		} else if state == deltaStateRemoved {
			s.deleteBucketInfo(id)
		}
	}
	s.putHeight(delta.getHeight())
	s.putTotalBucketCount(s.getTotalBucketCount() + delta.addedBucketCnt())
	return nil
}

func (s *liquidStakingDirty) putHeight(h uint64) {
	s.batch.Put(_liquidStakingNS, _liquidStakingHeightKey, byteutil.Uint64ToBytesBigEndian(h), "failed to put height")
	s.delta.putHeight(h)
}

func (s *liquidStakingDirty) addBucketType(id uint64, bt *BucketType) error {
	s.batch.Put(_liquidStakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.serialize(), "failed to put bucket type")
	return s.delta.addBucketType(id, bt)
}

func (s *liquidStakingDirty) updateBucketType(id uint64, bt *BucketType) error {
	s.batch.Put(_liquidStakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.serialize(), "failed to put bucket type")
	return s.delta.updateBucketType(id, bt)
}

func (s *liquidStakingDirty) addBucketInfo(id uint64, bi *BucketInfo) error {
	s.batch.Put(_liquidStakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.serialize(), "failed to put bucket info")
	return s.delta.addBucketInfo(id, bi)
}

func (s *liquidStakingDirty) updateBucketInfo(id uint64, bi *BucketInfo) error {
	s.batch.Put(_liquidStakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.serialize(), "failed to put bucket info")
	return s.delta.updateBucketInfo(id, bi)
}

func (s *liquidStakingDirty) burnBucket(id uint64) error {
	s.batch.Delete(_liquidStakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket info")
	return s.delta.deleteBucketInfo(id)
}

func (s *liquidStakingDirty) getBucketTypeIndex(amount *big.Int, duration time.Duration) (uint64, bool) {
	id, ok := s.delta.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = s.clean.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (s *liquidStakingDirty) getBucketTypeCount() uint64 {
	base := len(s.clean.idBucketTypeMap)
	add := 0
	for k, dbt := range s.delta.idBucketTypeMap {
		_, ok := s.clean.idBucketTypeMap[k]
		if dbt != nil && !ok {
			add++
		} else if dbt == nil && ok {
			add--
		}
	}
	return uint64(base + add)
}

func (s *liquidStakingDirty) getBucketType(id uint64) (*BucketType, bool) {
	bt, ok := s.delta.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = s.clean.getBucketType(id)
	return bt, ok
}

func (s *liquidStakingDirty) getBucketInfo(id uint64) (*BucketInfo, bool) {
	if s.delta.isBucketDeleted(id) {
		return nil, false
	}
	bi, ok := s.delta.getBucketInfo(id)
	if ok {
		return bi, true
	}
	bi, ok = s.clean.getBucketInfo(id)
	return bi, ok
}

func (s *liquidStakingDirty) finalizeBatch() batch.KVStoreBatch {
	s.once.Do(func() {
		total := s.clean.getTotalBucketCount() + s.delta.addedBucketCnt()
		s.batch.Put(_liquidStakingNS, _liquidStakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(total), "failed to put total bucket count")
	})
	return s.batch
}

func (s *liquidStakingDirty) finalize() (batch.KVStoreBatch, *liquidStakingDelta) {
	return s.finalizeBatch(), s.delta
}
