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
	// contractStakingDirty is the dirty data of contract staking
	// main functions:
	// 1. update bucket
	// 2. get up-to-date bucket
	// 3. store delta to merge to clean cache
	contractStakingDirty struct {
		clean      contractStakingCacheReader // clean cache to get buckets of last block
		delta      *contractStakingDelta      // delta for cache to store buckets of current block
		batch      batch.KVStoreBatch         // batch for db to store buckets of current block
		tokenOwner map[uint64]address.Address
		once       sync.Once
	}
)

func newContractStakingDirty(clean contractStakingCacheReader) *contractStakingDirty {
	return &contractStakingDirty{
		clean:      clean,
		delta:      newContractStakingDelta(),
		batch:      batch.NewBatch(),
		tokenOwner: make(map[uint64]address.Address),
	}
}

func (dirty *contractStakingDirty) putHeight(h uint64) {
	dirty.batch.Put(_StakingNS, _stakingHeightKey, byteutil.Uint64ToBytesBigEndian(h), "failed to put height")
	dirty.delta.putHeight(h)
}

func (dirty *contractStakingDirty) addBucketType(id uint64, bt *ContractStakingBucketType) error {
	dirty.batch.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.serialize(), "failed to put bucket type")
	return dirty.delta.addBucketType(id, bt)
}

func (dirty *contractStakingDirty) updateBucketType(id uint64, bt *ContractStakingBucketType) error {
	dirty.batch.Put(_StakingBucketTypeNS, byteutil.Uint64ToBytesBigEndian(id), bt.serialize(), "failed to put bucket type")
	return dirty.delta.updateBucketType(id, bt)
}

func (dirty *contractStakingDirty) addBucketInfo(id uint64, bi *ContractStakingBucketInfo) error {
	dirty.batch.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.serialize(), "failed to put bucket info")
	return dirty.delta.addBucketInfo(id, bi)
}

func (dirty *contractStakingDirty) updateBucketInfo(id uint64, bi *ContractStakingBucketInfo) error {
	dirty.batch.Put(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), bi.serialize(), "failed to put bucket info")
	return dirty.delta.updateBucketInfo(id, bi)
}

func (dirty *contractStakingDirty) burnBucket(id uint64) error {
	dirty.batch.Delete(_StakingBucketInfoNS, byteutil.Uint64ToBytesBigEndian(id), "failed to delete bucket info")
	return dirty.delta.deleteBucketInfo(id)
}

func (dirty *contractStakingDirty) getBucketTypeIndex(amount *big.Int, duration uint64) (uint64, bool) {
	id, ok := dirty.delta.getBucketTypeIndex(amount, duration)
	if ok {
		return id, true
	}
	id, ok = dirty.clean.getBucketTypeIndex(amount, duration)
	return id, ok
}

func (dirty *contractStakingDirty) getBucketTypeCount() uint64 {
	return dirty.clean.getTotalBucketTypeCount() + dirty.delta.addedBucketTypeCnt()
}

func (dirty *contractStakingDirty) getBucketType(id uint64) (*ContractStakingBucketType, bool) {
	bt, ok := dirty.delta.getBucketType(id)
	if ok {
		return bt, true
	}
	bt, ok = dirty.clean.getBucketType(id)
	return bt, ok
}

func (dirty *contractStakingDirty) getBucketInfo(id uint64) (*ContractStakingBucketInfo, bool) {
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

func (dirty *contractStakingDirty) finalizeBatch() batch.KVStoreBatch {
	dirty.once.Do(func() {
		total := dirty.clean.getTotalBucketCount() + dirty.delta.addedBucketCnt()
		dirty.batch.Put(_StakingNS, _stakingTotalBucketCountKey, byteutil.Uint64ToBytesBigEndian(total), "failed to put total bucket count")
	})
	return dirty.batch
}

func (dirty *contractStakingDirty) finalize() (batch.KVStoreBatch, *contractStakingDelta) {
	return dirty.finalizeBatch(), dirty.delta
}

func (dirty *contractStakingDirty) handleTransferEvent(event eventParam) error {
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

func (dirty *contractStakingDirty) handleBucketTypeActivatedEvent(event eventParam, height uint64) error {
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

func (dirty *contractStakingDirty) handleBucketTypeDeactivatedEvent(event eventParam, height uint64) error {
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

func (dirty *contractStakingDirty) handleStakedEvent(event eventParam, height uint64) error {
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
	owner, ok := dirty.tokenOwner[tokenIDParam.Uint64()]
	if !ok {
		return errors.Errorf("no owner for token id %d", tokenIDParam.Uint64())
	}
	bucket := ContractStakingBucketInfo{
		TypeIndex:  btIdx,
		Delegate:   delegateParam,
		Owner:      owner,
		CreatedAt:  height,
		UnlockedAt: maxBlockNumber,
		UnstakedAt: maxBlockNumber,
	}
	return dirty.addBucketInfo(tokenIDParam.Uint64(), &bucket)
}

func (dirty *contractStakingDirty) handleLockedEvent(event eventParam) error {
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

func (dirty *contractStakingDirty) handleUnlockedEvent(event eventParam, height uint64) error {
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

func (dirty *contractStakingDirty) handleUnstakedEvent(event eventParam, height uint64) error {
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

func (dirty *contractStakingDirty) handleMergedEvent(event eventParam) error {
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

func (dirty *contractStakingDirty) handleDurationExtendedEvent(event eventParam) error {
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

func (dirty *contractStakingDirty) handleAmountIncreasedEvent(event eventParam) error {
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

func (dirty *contractStakingDirty) handleDelegateChangedEvent(event eventParam) error {
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

func (dirty *contractStakingDirty) handleWithdrawalEvent(event eventParam) error {
	tokenIDParam, err := event.indexedFieldUint256("tokenId")
	if err != nil {
		return err
	}

	return dirty.burnBucket(tokenIDParam.Uint64())
}
