// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
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

	contractStakingDirtyWithHeight struct {
		*contractStakingDirty
		height uint64
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

func newContractStakingDirtyWithHeight(dirty *contractStakingDirty, height uint64) *contractStakingDirtyWithHeight {
	return &contractStakingDirtyWithHeight{
		contractStakingDirty: dirty,
		height:               height,
	}
}

func (dirty *contractStakingDirty) PutBucketType(contractAddr address.Address, bt *BucketType) error {
	dirty.putBucketType(bt)
	return nil
}

func (dirty *contractStakingDirty) DeductBucket(contractAddr address.Address, id uint64) (*contractstaking.Bucket, error) {
	bi, ok := dirty.cache.BucketInfo(id)
	if !ok {
		return nil, errors.Wrapf(contractstaking.ErrBucketNotExist, "bucket info %d not found", id)
	}
	bt, ok := dirty.cache.BucketType(bi.TypeIndex)
	if !ok {
		return nil, errors.New("bucket type not found")
	}
	return &contractstaking.Bucket{
		StakedAmount:   bt.Amount,
		StakedDuration: bt.Duration,
		CreatedAt:      bi.CreatedAt,
		UnlockedAt:     bi.UnlockedAt,
		UnstakedAt:     bi.UnstakedAt,
		Candidate:      bi.Delegate,
		Owner:          bi.Owner,
	}, nil
}

func (dirty *contractStakingDirty) DeleteBucket(contractAddr address.Address, id uint64) error {
	dirty.deleteBucketInfo(id)
	return nil
}

func (dirty *contractStakingDirty) PutBucket(contractAddr address.Address, id uint64, bkt *contractstaking.Bucket) error {
	bi, err := dirty.convertToBucketInfo(bkt)
	if err != nil {
		return err
	}
	dirty.addBucketInfo(id, bi)
	return nil
}

func (dirty *contractStakingDirty) convertToBucketInfo(bucket *contractstaking.Bucket) (*bucketInfo, error) {
	if bucket == nil {
		return nil, nil
	}
	tid, old := dirty.matchBucketType(bucket.StakedAmount, bucket.StakedDuration)
	if old == nil {
		return nil, errBucketTypeNotExist
	}
	return &bucketInfo{
		TypeIndex:  tid,
		CreatedAt:  bucket.CreatedAt,
		UnlockedAt: bucket.UnlockedAt,
		UnstakedAt: bucket.UnstakedAt,
		Delegate:   bucket.Candidate,
		Owner:      bucket.Owner,
	}, nil
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
	id, old := dirty.matchBucketType(bt.Amount, bt.Duration)
	if old == nil {
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

func (dirty *contractStakingDirty) Finalize() (batch.KVStoreBatch, stakingCache) {
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

func (dirty *contractStakingDirty) matchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType) {
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

func (dirty *contractStakingDirtyWithHeight) Finalize(height uint64) {
	dirty.height = height
}

func (dirty *contractStakingDirtyWithHeight) ContractStakingBuckets() (uint64, map[uint64]*contractstaking.Bucket, error) {
	ids, typs, infos := dirty.contractStakingDirty.cache.Buckets()
	res := make(map[uint64]*contractstaking.Bucket)
	for i, id := range ids {
		res[id] = assembleContractBucket(infos[i], typs[i])
	}
	return dirty.height, res, nil
}
