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
		clean *contractStakingCache // clean cache to get buckets of last block
		delta *contractStakingDelta // delta for cache to store buckets of current block
		batch batch.KVStoreBatch    // batch for db to store buckets of current block
		once  sync.Once
	}
)

var (
	_stakingHeightKey           = []byte("shk")
	_stakingTotalBucketCountKey = []byte("stbck")

	errBucketTypeNotExist = errors.New("bucket type does not exist")
)

func newContractStakingDirty(clean *contractStakingCache) *contractStakingDirty {
	return &contractStakingDirty{
		clean: clean,
		delta: newContractStakingDelta(),
		batch: batch.NewBatch(),
	}
}

func (dirty *contractStakingDirty) addBucketInfo(id uint64, bi *bucketInfo) error {
	dirty.batch.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.Serialize(), "failed to put bucket info")
	return dirty.delta.AddBucketInfo(id, bi)
}

func (dirty *contractStakingDirty) updateBucketInfo(id uint64, bi *bucketInfo) error {
	dirty.batch.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.Serialize(), "failed to put bucket info")
	return dirty.delta.UpdateBucketInfo(id, bi)
}

func (dirty *contractStakingDirty) deleteBucketInfo(id uint64) error {
	dirty.batch.Delete(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket info")
	return dirty.delta.DeleteBucketInfo(id)
}

func (dirty *contractStakingDirty) putBucketType(bt *BucketType) error {
	id, _, ok := dirty.matchBucketType(bt.Amount, bt.Duration)
	if !ok {
		id = dirty.getBucketTypeCount()
		if err := dirty.addBucketType(id, bt); err != nil {
			return err
		}
	}
	return dirty.updateBucketType(id, bt)
}

func (dirty *contractStakingDirty) getBucketType(id uint64) (*BucketType, bool) {
	bt, state := dirty.delta.GetBucketType(id)
	switch state {
	case deltaStateAdded, deltaStateModified:
		return bt, true
	default:
		return dirty.clean.BucketType(id)
	}
}

func (dirty *contractStakingDirty) getBucketInfo(id uint64) (*bucketInfo, bool) {
	bi, state := dirty.delta.GetBucketInfo(id)
	switch state {
	case deltaStateAdded, deltaStateModified:
		return bi, true
	case deltaStateRemoved:
		return nil, false
	default:
		return dirty.clean.BucketInfo(id)
	}
}

func (dirty *contractStakingDirty) finalize() (batch.KVStoreBatch, *contractStakingDelta) {
	return dirty.finalizeBatch(), dirty.delta
}

func (dirty *contractStakingDirty) finalizeBatch() batch.KVStoreBatch {
	dirty.once.Do(func() {
		tbc, _ := dirty.clean.TotalBucketCount(0)
		total := tbc + dirty.delta.AddedBucketCnt()
		dirty.batch.Put(_StakingNS, _stakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(total), "failed to put total bucket count")
	})
	return dirty.batch
}

func (dirty *contractStakingDirty) addBucketType(id uint64, bt *BucketType) error {
	dirty.batch.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.Serialize(), "failed to put bucket type")
	return dirty.delta.AddBucketType(id, bt)
}

func (dirty *contractStakingDirty) matchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	id, bt, ok := dirty.delta.MatchBucketType(amount, duration)
	if ok {
		return id, bt, true
	}
	return dirty.clean.MatchBucketType(amount, duration)
}

func (dirty *contractStakingDirty) getBucketTypeCount() uint64 {
	btc, _ := dirty.clean.BucketTypeCount(0)
	return uint64(btc) + dirty.delta.AddedBucketTypeCnt()
}

func (dirty *contractStakingDirty) updateBucketType(id uint64, bt *BucketType) error {
	dirty.batch.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.Serialize(), "failed to put bucket type")
	return dirty.delta.UpdateBucketType(id, bt)
}
