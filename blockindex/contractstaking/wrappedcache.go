// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package contractstaking

import (
	"math/big"
	"sort"
	"sync"

	"github.com/iotexproject/iotex-address/address"
)

type (
	wrappedCache struct {
		totalBucketCount      uint64
		updatedBucketInfos    map[uint64]*bucketInfo
		updatedBucketTypes    map[uint64]*BucketType
		updatedCandidates     map[string]map[uint64]bool
		propertyBucketTypeMap map[uint64]map[uint64]uint64

		mu   sync.RWMutex
		base stakingCache
	}
)

func newWrappedCache(base stakingCache) *wrappedCache {
	if base == nil {
		panic("base staking cache cannot be nil")
	}
	totalBucketCount := base.TotalBucketCount()

	return &wrappedCache{
		totalBucketCount:      totalBucketCount,
		updatedBucketInfos:    make(map[uint64]*bucketInfo),
		updatedBucketTypes:    make(map[uint64]*BucketType),
		updatedCandidates:     make(map[string]map[uint64]bool),
		propertyBucketTypeMap: make(map[uint64]map[uint64]uint64),
		mu:                    sync.RWMutex{},
		base:                  base,
	}
}

func (wc *wrappedCache) BucketInfo(id uint64) (*bucketInfo, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	info, ok := wc.updatedBucketInfos[id]
	if !ok {
		return wc.base.BucketInfo(id)
	}
	if info == nil {
		return nil, false
	}
	return info.Clone(), true
}

func (wc *wrappedCache) MustGetBucketInfo(id uint64) *bucketInfo {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	info, ok := wc.updatedBucketInfos[id]
	if !ok {
		return wc.base.MustGetBucketInfo(id)
	}
	if info == nil {
		panic("must get bucket info from wrapped cache")
	}

	return info
}

func (wc *wrappedCache) MustGetBucketType(id uint64) *BucketType {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.mustGetBucketType(id)
}

func (wc *wrappedCache) mustGetBucketType(id uint64) *BucketType {
	bt, ok := wc.updatedBucketTypes[id]
	if !ok {
		return wc.base.MustGetBucketType(id)
	}
	if bt == nil {
		panic("must get bucket type from wrapped cache")
	}

	return bt
}

func (wc *wrappedCache) BucketType(id uint64) (*BucketType, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.bucketType(id)
}

func (wc *wrappedCache) bucketType(id uint64) (*BucketType, bool) {
	bt, ok := wc.updatedBucketTypes[id]
	if !ok {
		return wc.base.BucketType(id)
	}
	return bt, ok
}

func (wc *wrappedCache) BucketsByCandidate(candidate address.Address) ([]uint64, []*BucketType, []*bucketInfo) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	ids, _, infos := wc.base.BucketsByCandidate(candidate)
	reverseMap := make(map[uint64]int, len(ids))
	for i, id := range ids {
		reverseMap[id] = i
	}
	for id := range wc.updatedCandidates[candidate.String()] {
		info, ok := wc.updatedBucketInfos[id]
		if !ok {
			// TODO: should not be false, double check
			panic("bucket should exist in updated bucket info")
		}
		if info == nil || info.Delegate.String() != candidate.String() {
			delete(reverseMap, id)
		} else {
			if _, ok := reverseMap[id]; !ok {
				reverseMap[id] = len(infos)
				infos = append(infos, info.Clone())
			} else {
				infos[reverseMap[id]] = info.Clone()
			}
		}
	}
	retIDs := make([]uint64, 0, len(reverseMap))
	for id := range reverseMap {
		retIDs = append(retIDs, id)
	}
	retInfos := make([]*bucketInfo, 0, len(retIDs))
	retTypes := make([]*BucketType, 0, len(retIDs))
	sort.Slice(retIDs, func(i, j int) bool { return retIDs[i] < retIDs[j] })
	for _, id := range retIDs {
		info, ok := wc.updatedBucketInfos[id]
		if !ok {
			info = infos[reverseMap[id]]
			retInfos = append(retInfos, info)
			retTypes = append(retTypes, wc.mustGetBucketType(info.TypeIndex))
		} else if info != nil {
			retInfos = append(retInfos, info.Clone())
			retTypes = append(retTypes, wc.mustGetBucketType(info.TypeIndex))
		} else {
			panic("bucket info should not be nil in updated bucket infos")
		}
	}
	return retIDs, retTypes, retInfos
}

func (wc *wrappedCache) TotalBucketCount() uint64 {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.totalBucketCount
}

func (wc *wrappedCache) PutBucketType(id uint64, bt *BucketType) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if bt == nil {
		panic("bucket type cannot be nil")
	}
	oldBt, existed := wc.bucketType(id)
	if existed {
		if oldBt.Amount.Cmp(bt.Amount) != 0 || oldBt.Duration != bt.Duration {
			panic("bucket type amount or duration cannot be changed")
		}
	}
	oldId, _, ok := wc.matchBucketType(bt.Amount, bt.Duration)
	if ok && oldId != id {
		panic("bucket type with same amount and duration already exists")
	}
	if _, ok := wc.propertyBucketTypeMap[bt.Amount.Uint64()]; !ok {
		wc.propertyBucketTypeMap[bt.Amount.Uint64()] = make(map[uint64]uint64)
	} else {
		oldID, ok := wc.propertyBucketTypeMap[bt.Amount.Uint64()][bt.Duration]
		if ok && oldID != id {
			panic("bucket type with same amount and duration already exists")
		}
	}
	wc.updatedBucketTypes[id] = bt
	wc.propertyBucketTypeMap[bt.Amount.Uint64()][bt.Duration] = id
}

func (wc *wrappedCache) PutBucketInfo(id uint64, bi *bucketInfo) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if id > wc.totalBucketCount {
		wc.totalBucketCount = id
	}
	if _, ok := wc.updatedBucketInfos[id]; !ok {
		if oldInfo, ok := wc.base.BucketInfo(id); ok {
			if _, ok := wc.updatedCandidates[oldInfo.Delegate.String()]; !ok {
				wc.updatedCandidates[oldInfo.Delegate.String()] = make(map[uint64]bool)
			}
			wc.updatedCandidates[oldInfo.Delegate.String()][id] = true
		}
	}
	wc.updatedBucketInfos[id] = bi
	if _, ok := wc.updatedCandidates[bi.Delegate.String()]; !ok {
		wc.updatedCandidates[bi.Delegate.String()] = make(map[uint64]bool)
	}
	wc.updatedCandidates[bi.Delegate.String()][id] = true
}

func (wc *wrappedCache) Base() stakingCache {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.base
}

func (wc *wrappedCache) Commit() {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	for id, bt := range wc.updatedBucketTypes {
		wc.base.PutBucketType(id, bt)
	}
	for id, bi := range wc.updatedBucketInfos {
		if bi == nil {
			wc.base.DeleteBucketInfo(id)
		} else {
			wc.base.PutBucketInfo(id, bi)
		}
	}
}

func (wc *wrappedCache) IsDirty() bool {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return len(wc.updatedBucketInfos) > 0 || len(wc.updatedBucketTypes) > 0
}

func (wc *wrappedCache) DeleteBucketInfo(id uint64) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if _, ok := wc.updatedBucketInfos[id]; !ok {
		oldInfo, ok := wc.base.BucketInfo(id)
		if ok {
			wc.updatedCandidates[oldInfo.Delegate.String()][id] = true
		}
	}
	wc.updatedBucketInfos[id] = nil
}

func (wc *wrappedCache) MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	return wc.matchBucketType(amount, duration)
}

func (wc *wrappedCache) matchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	amountUint64 := amount.Uint64()
	if amountMap, ok := wc.propertyBucketTypeMap[amountUint64]; ok {
		if id, ok := amountMap[duration]; ok {
			if bt, ok := wc.updatedBucketTypes[id]; ok {
				if bt != nil {
					return id, bt, true
				}
				return 0, nil, false
			}
		}
	}

	return wc.base.MatchBucketType(amount, duration)
}

func (wc *wrappedCache) BucketTypeCount() int {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	total := wc.base.BucketTypeCount()
	for id := range wc.updatedBucketTypes {
		if _, exists := wc.base.BucketType(id); !exists {
			total += 1
		}
	}
	return total
}
