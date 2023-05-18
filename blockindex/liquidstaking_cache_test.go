// Copyright (c) 2023 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package blockindex

import (
	"math/big"
	"sync"
	"testing"
)

func TestLiquidStakingCacheThreadSafety(t *testing.T) {
	cache := newLiquidStakingCache()

	wait := sync.WaitGroup{}
	wait.Add(2)
	go func() {
		for i := 0; i < 1000; i++ {
			cache.putBucketType(uint64(i), &ContractStakingBucketType{
				Amount:      big.NewInt(int64(i)),
				Duration:    1000,
				ActivatedAt: 10,
			})
		}
		wait.Done()
	}()

	go func() {
		for i := 0; i < 1000; i++ {
			cache.getBucketType(uint64(i))
		}
		wait.Done()
	}()

	wait.Wait()
	// no panic means thread safety
}
