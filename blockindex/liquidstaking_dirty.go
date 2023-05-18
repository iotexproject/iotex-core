// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"
	"sync"

	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"

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
		clean      liquidStakingCacheReader // clean cache to get buckets of last block
		delta      *liquidStakingDelta      // delta for cache to store buckets of current block
		batch      batch.KVStoreBatch       // batch for db to store buckets of current block
		tokenOwner map[uint64]address.Address
		once       sync.Once
	}
)

func newLiquidStakingDirty(clean liquidStakingCacheReader) *liquidStakingDirty {
	return &liquidStakingDirty{
		clean:      clean,
		delta:      newLiquidStakingDelta(),
		batch:      batch.NewBatch(),
		tokenOwner: make(map[uint64]address.Address),
	}
}

func (dirty *liquidStakingDirty) putHeight(h uint64) {
	dirty.batch.Put(_liquidStakingNS, _liquidStakingHeightKey, byteutil.Uint64ToBytesBigEndian(h), "failed to put height")
	dirty.delta.putHeight(h)
}

func (dirty *liquidStakingDirty) addBucketType(id uint64, bt *ContractStakingBucketType) error {
	dirty.batch.Put(_liquidStakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.serialize(), "failed to put bucket type")
	return dirty.delta.addBucketType(id, bt)
}

func (dirty *liquidStakingDirty) updateBucketType(id uint64, bt *ContractStakingBucketType) error {
	dirty.batch.Put(_liquidStakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.serialize(), "failed to put bucket type")
	return dirty.delta.updateBucketType(id, bt)
}

func (dirty *liquidStakingDirty) addBucketInfo(id uint64, bi *ContractStakingBucketInfo) error {
	dirty.batch.Put(_liquidStakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.serialize(), "failed to put bucket info")
	return dirty.delta.addBucketInfo(id, bi)
}

func (dirty *liquidStakingDirty) updateBucketInfo(id uint64, bi *ContractStakingBucketInfo) error {
	dirty.batch.Put(_liquidStakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.serialize(), "failed to put bucket info")
	return dirty.delta.updateBucketInfo(id, bi)
}

func (dirty *liquidStakingDirty) burnBucket(id uint64) error {
	dirty.batch.Delete(_liquidStakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket info")
	return dirty.delta.deleteBucketInfo(id)
}

func (dirty *liquidStakingDirty) getBucketTypeIndex(amount *big.Int, duration uint64) (uint64, bool) {
	id, ok := dirty.delta.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = dirty.clean.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (dirty *liquidStakingDirty) getBucketTypeCount() uint64 {
	return dirty.clean.getTotalBucketTypeCount() + dirty.delta.addedBucketTypeCnt()
}

func (dirty *liquidStakingDirty) getBucketType(id uint64) (*ContractStakingBucketType, bool) {
	bt, ok := dirty.delta.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = dirty.clean.getBucketType(id)
	return bt, ok
}

func (dirty *liquidStakingDirty) getBucketInfo(id uint64) (*ContractStakingBucketInfo, bool) {
	if dirty.delta.isBucketDeleted(id) {
		return nil, false
	}
	bi, ok := dirty.delta.getBucketInfo(id)
	if ok {
		return bi, true
	}
	bi, ok = dirty.clean.getBucketInfo(id)
	return bi, ok
}

func (dirty *liquidStakingDirty) finalizeBatch() batch.KVStoreBatch {
	dirty.once.Do(func() {
		total := dirty.clean.getTotalBucketCount() + dirty.delta.addedBucketCnt()
		dirty.batch.Put(_liquidStakingNS, _liquidStakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(total), "failed to put total bucket count")
	})
	return dirty.batch
}

func (dirty *liquidStakingDirty) finalize() (batch.KVStoreBatch, *liquidStakingDelta) {
	return dirty.finalizeBatch(), dirty.delta
}

func (dirty *liquidStakingDirty) handleTransferEvent(event eventParam) error {
	to, err := event.indexedFieldAddress("to")
	if err != nil {
		return err
	}
	tokenID, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	dirty.tokenOwner[tokenID.Uint64()] = to
	return nil
}

func (dirty *liquidStakingDirty) handleBucketTypeActivatedEvent(event eventParam, height uint64) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	bt := ContractStakingBucketType{
		Amount:      amountParam,
		Duration:    durationParam.Uint64(),
		ActivatedAt: height,
	}
	id, ok := dirty.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		id = dirty.getBucketTypeCount()
		err = dirty.addBucketType(id, &bt)
	} else {
		err = dirty.updateBucketType(id, &bt)
	}

	return err
}

func (dirty *liquidStakingDirty) handleBucketTypeDeactivatedEvent(event eventParam, height uint64) error {
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	id, ok := dirty.getBucketTypeIndex(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bt, ok := dirty.getBucketType(id)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", id)
	}
	bt.ActivatedAt = maxBlockNumber
	return dirty.updateBucketType(id, bt)
}

func (dirty *liquidStakingDirty) handleStakedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldAddress("delegate")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	btIdx, ok := dirty.getBucketTypeIndex(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	bucket := ContractStakingBucketInfo{
		TypeIndex:  btIdx,
		Delegate:   delegateParam,
		Owner:      dirty.tokenOwner[tokenIDParam.Uint64()],
		CreatedAt:  height,
		UnlockedAt: maxBlockNumber,
		UnstakedAt: maxBlockNumber,
	}
	return dirty.addBucketInfo(tokenIDParam.Uint64(), &bucket)
}

func (dirty *liquidStakingDirty) handleLockedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := dirty.getBucketTypeIndex(bt.Amount, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %v, duration %d", bt.Amount, durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	b.UnlockedAt = maxBlockNumber
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (dirty *liquidStakingDirty) handleUnlockedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnlockedAt = height
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (dirty *liquidStakingDirty) handleUnstakedEvent(event eventParam, height uint64) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.UnstakedAt = height
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (dirty *liquidStakingDirty) handleMergedEvent(event eventParam) error {
	tokenIDsParam, err := event.fieldUint256Slice("tokenIds")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	// merge to the first bucket
	btIdx, ok := dirty.getBucketTypeIndex(amountParam, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), durationParam.Uint64())
	}
	b, ok := dirty.getBucketInfo(tokenIDsParam[0].Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDsParam[0].Uint64())
	}
	b.TypeIndex = btIdx
	b.UnlockedAt = maxBlockNumber
	for i := 1; i < len(tokenIDsParam); i++ {
		if err = dirty.burnBucket(tokenIDsParam[i].Uint64()); err != nil {
			return err
		}
	}
	return dirty.updateBucketInfo(tokenIDsParam[0].Uint64(), b)
}

func (dirty *liquidStakingDirty) handleDurationExtendedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	durationParam, err := event.fieldUint256("duration")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := dirty.getBucketTypeIndex(bt.Amount, durationParam.Uint64())
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", bt.Amount.Int64(), durationParam.Uint64())
	}
	b.TypeIndex = newBtIdx
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (dirty *liquidStakingDirty) handleAmountIncreasedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	amountParam, err := event.fieldUint256("amount")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	bt, ok := dirty.getBucketType(b.TypeIndex)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "id %d", b.TypeIndex)
	}
	newBtIdx, ok := dirty.getBucketTypeIndex(amountParam, bt.Duration)
	if !ok {
		return errors.Wrapf(errBucketTypeNotExist, "amount %d, duration %d", amountParam.Int64(), bt.Duration)
	}
	b.TypeIndex = newBtIdx
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (dirty *liquidStakingDirty) handleDelegateChangedEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}
	delegateParam, err := event.fieldAddress("newDelegate")
	if err != nil {
		return err
	}

	b, ok := dirty.getBucketInfo(tokenIDParam.Uint64())
	if !ok {
		return errors.Wrapf(ErrBucketInfoNotExist, "token id %d", tokenIDParam.Uint64())
	}
	b.Delegate = delegateParam
	return dirty.updateBucketInfo(tokenIDParam.Uint64(), b)
}

func (dirty *liquidStakingDirty) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	return dirty.burnBucket(tokenIDParam.Uint64())
}
