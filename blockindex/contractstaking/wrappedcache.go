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

		mu    sync.RWMutex
		cache stakingCache
	}
)

func newWrappedCache(cache stakingCache) *wrappedCache {
	return &wrappedCache{
		updatedBucketInfos:    make(map[uint64]*bucketInfo),
		updatedBucketTypes:    make(map[uint64]*BucketType),
		updatedCandidates:     make(map[string]map[uint64]bool),
		propertyBucketTypeMap: make(map[uint64]map[uint64]uint64),
		mu:                    sync.RWMutex{},
		cache:                 cache,
	}
}

func (wc *wrappedCache) BucketInfo(id uint64) (*bucketInfo, bool) {
	info, ok := wc.updatedBucketInfos[id]
	if !ok {
		return wc.cache.BucketInfo(id)
	}
	if info == nil {
		return nil, false
	}
	return info.Clone(), true
}

func (wc *wrappedCache) MustGetBucketInfo(id uint64) *bucketInfo {
	info, ok := wc.updatedBucketInfos[id]
	if !ok {
		return wc.cache.MustGetBucketInfo(id)
	}
	if info == nil {
		panic("must get bucket info from wrapped cache")
	}

	return info
}

func (wc *wrappedCache) MustGetBucketType(id uint64) *BucketType {
	return wc.mustGetBucketType(id)
}

func (wc *wrappedCache) mustGetBucketType(id uint64) *BucketType {
	bt, ok := wc.updatedBucketTypes[id]
	if !ok {
		return wc.cache.MustGetBucketType(id)
	}
	if bt == nil {
		panic("must get bucket type from wrapped cache")
	}

	return bt
}

func (wc *wrappedCache) BucketType(id uint64) (*BucketType, bool) {
	bt, ok := wc.updatedBucketTypes[id]
	if !ok {
		return wc.cache.BucketType(id)
	}
	return bt, ok
}

func (wc *wrappedCache) BucketsByCandidate(candidate address.Address) ([]uint64, []*BucketType, []*bucketInfo) {
	ids, types, infos := wc.cache.BucketsByCandidate(candidate)
	bucketMap := make(map[uint64]*bucketInfo, len(ids))
	for i, id := range ids {
		bucketMap[id] = infos[i]
	}
	for id := range wc.updatedCandidates[candidate.String()] {
		info, ok := wc.updatedBucketInfos[id]
		if !ok {
			// TODO: should not be false, double check
			panic("bucket should exist in updated bucket info")
		}
		if info == nil || info.Delegate.String() != candidate.String() {
			delete(bucketMap, id)
		} else {
			bucketMap[id] = info.Clone()
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		info, ok := wc.updatedBucketInfos[id]
		if !ok {
			ids = append(ids, id)
			info = bucketMap[id]
			types = append(types, wc.cache.MustGetBucketType(info.TypeIndex))
		} else if info != nil {
			ids = append(ids, id)
			infos = append(infos, info.Clone())
			types = append(types, wc.mustGetBucketType(info.TypeIndex))
		}
	}

	return ids, types, infos
}

func (wc *wrappedCache) TotalBucketCount() uint64 {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	total := wc.cache.TotalBucketCount()
	return max(total, wc.totalBucketCount)
}

func (wc *wrappedCache) PutBucketType(id uint64, bt *BucketType) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	wc.updatedBucketTypes[id] = bt
	if bt != nil {
		if _, ok := wc.propertyBucketTypeMap[bt.Amount.Uint64()]; !ok {
			wc.propertyBucketTypeMap[bt.Amount.Uint64()] = make(map[uint64]uint64)
		}
		wc.propertyBucketTypeMap[bt.Amount.Uint64()][bt.Duration] = id
	}
}

func (wc *wrappedCache) PutBucketInfo(id uint64, bi *bucketInfo) {
	wc.mu.Lock()
	defer wc.mu.Unlock()
	if id >= wc.totalBucketCount {
		wc.totalBucketCount = id + 1
	}
	if _, ok := wc.updatedBucketInfos[id]; !ok {
		if oldInfo, ok := wc.cache.BucketInfo(id); ok {
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
	return wc.cache
}

func (wc *wrappedCache) Commit() {
	wc.mu.Lock()
	defer wc.mu.Unlock()

	for id, bt := range wc.updatedBucketTypes {
		wc.cache.PutBucketType(id, bt)
	}

	for id, bi := range wc.updatedBucketInfos {
		if bi == nil {
			wc.cache.DeleteBucketInfo(id)
		} else {
			wc.cache.PutBucketInfo(id, bi)
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
		oldInfo, ok := wc.cache.BucketInfo(id)
		if ok {
			wc.updatedCandidates[oldInfo.Delegate.String()][id] = true
		}
	}
	wc.updatedBucketInfos[id] = nil
}

func (wc *wrappedCache) MatchBucketType(amount *big.Int, duration uint64) (uint64, *BucketType, bool) {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	if !amount.IsUint64() {
		panic("amount must be uint64")
	}
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

	return wc.cache.MatchBucketType(amount, duration)
}

func (wc *wrappedCache) BucketTypeCount() int {
	wc.mu.RLock()
	defer wc.mu.RUnlock()
	total := wc.cache.BucketTypeCount()
	for id := range wc.updatedBucketTypes {
		if _, exists := wc.cache.BucketType(id); !exists {
			total += 1
		}
	}
	return total
}
