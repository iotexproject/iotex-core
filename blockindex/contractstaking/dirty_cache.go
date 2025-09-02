// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/db/batch"
	"github.com/iotexproject/iotex-core/v2/pkg/util/byteutil"
)

const (
	// bucket related namespace in db
	_StakingBucketInfoNS = "sbi"
	_StakingBucketTypeNS = "sbt"
	_StakingNS           = "sns"
)

type (
	// contractStakingDirty is the dirty data of contract staking
	// main functions:
	// 1. update bucket
	// 2. get up-to-date bucket
	// 3. store delta to merge to clean cache
	contractStakingDirty struct {
		cache stakingCache       // clean cache to get buckets of last block
		batch batch.KVStoreBatch // batch for db to store buckets of current block
		once  sync.Once
	}
)

var (
	_stakingHeightKey           = []byte("shk")
	_stakingTotalBucketCountKey = []byte("stbck")

	errBucketTypeNotExist = errors.New("bucket type does not exist")
)

func newContractStakingDirty(clean stakingCache) *contractStakingDirty {
	return &contractStakingDirty{
		cache: clean,
		batch: batch.NewBatch(),
	}
}

func (dirty *contractStakingDirty) addBucketInfo(id uint64, bi *bucketInfo) {
	data, err := bi.Serialize()
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize bucket info"))
	}
	dirty.batch.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), data, "failed to put bucket info")
	dirty.cache.PutBucketInfo(id, bi)
}

func (dirty *contractStakingDirty) updateBucketInfo(id uint64, bi *bucketInfo) {
	data, err := bi.Serialize()
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize bucket info"))
	}
	dirty.batch.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), data, "failed to put bucket info")
	dirty.cache.PutBucketInfo(id, bi)
}

func (dirty *contractStakingDirty) deleteBucketInfo(id uint64) {
	dirty.batch.Delete(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket info")
	dirty.cache.DeleteBucketInfo(id)
}

func (dirty *contractStakingDirty) putBucketType(bt *BucketType) {
	id, _, ok := dirty.matchBucketType(bt.Amount, bt.Duration)
	if !ok {
		id = dirty.getBucketTypeCount()
		dirty.addBucketType(id, bt)
	}
	dirty.updateBucketType(id, bt)
}

func (dirty *contractStakingDirty) getBucketType(id uint64) (*BucketType, bool) {
	return dirty.cache.BucketType(id)
}

func (dirty *contractStakingDirty) getBucketInfo(id uint64) (*bucketInfo, bool) {
	return dirty.cache.BucketInfo(id)
}

func (dirty *contractStakingDirty) finalize() (batch.KVStoreBatch, stakingCache) {
	b := dirty.finalizeBatch()

	return b, dirty.cache
}

func (dirty *contractStakingDirty) finalizeBatch() batch.KVStoreBatch {
	dirty.once.Do(func() {
		total := dirty.cache.TotalBucketCount()
		dirty.batch.Put(_StakingNS, _stakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(total), "failed to put total bucket count")
	})
	return dirty.batch
}

func (dirty *contractStakingDirty) addBucketType(id uint64, bt *BucketType) {
	data, err := bt.Serialize()
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize bucket type"))
	}
	dirty.batch.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), data, "failed to put bucket type")
	dirty.cache.PutBucketType(id, bt)
}

func (dirty *contractStakingDirty) matchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	return dirty.cache.MatchBucketType(amount, duration)
}

func (dirty *contractStakingDirty) getBucketTypeCount() uint64 {
	return uint64(dirty.cache.BucketTypeCount())
}

func (dirty *contractStakingDirty) updateBucketType(id uint64, bt *BucketType) {
	data, err := bt.Serialize()
	if err != nil {
		panic(errors.Wrap(err, "failed to serialize bucket type"))
	}
	dirty.batch.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), data, "failed to put bucket type")
	dirty.cache.PutBucketType(id, bt)
}
