// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/contractstaking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// The single-block backfill trades the old job's per-block amortization for
// doing everything in the activation block, so its cost has to fit the 2.5s
// block budget. This benchmark measures the part of that cost that is ours: the
// scan, the decode, the per-owner grouping, and the writes.
//
// It does NOT measure trie or database work -- it runs against the in-memory
// mock state manager, so every State/PutState is a map operation. Treat the
// number as a lower bound on the real block, useful for catching an accidental
// quadratic in this code, not as a verdict on the budget.
//
// The structural argument for the budget is separate and does not depend on
// this: protocol.go already runs contractsStake.Migrate at XinguBlockHeight,
// which writes every contract bucket into state in one block. The backfill
// reads the same buckets and writes one key per distinct owner, which is
// strictly fewer writes against the same backing store.
//
//	go test ./action/protocol/staking/ -run '^$' -bench BackfillOwnerIndex -benchtime 1x

// benchBackfillOwner spreads buckets over owners the way mainnet does: a long
// tail of one-bucket owners plus a few holding many. bucketsPerOwner sets the
// average.
func benchBackfillOwner(i, bucketsPerOwner int) address.Address {
	var b [20]byte
	binary.BigEndian.PutUint64(b[12:], uint64(i/bucketsPerOwner)+1)
	addr, err := address.FromBytes(b[:])
	if err != nil {
		panic(err)
	}
	return addr
}

func benchmarkBackfillOwnerIndex(b *testing.B, totalBuckets, bucketsPerOwner int) {
	b.Helper()
	for n := 0; n < b.N; n++ {
		b.StopTimer()
		sm := testdb.NewMockStateManager(gomock.NewController(b))
		cs := contractstaking.NewContractStakingStateManager(sm)
		g := genesis.TestDefault()
		// Three contracts, split the way mainnet splits: V1 holds most of the
		// buckets, V2 and V3 the remainder.
		shares := []float64{0.7, 0.2, 0.1}
		next := 0
		for ci, contractStr := range []string{
			g.SystemStakingContractAddress,
			g.SystemStakingContractV2Address,
			g.SystemStakingContractV3Address,
		} {
			contract, err := address.FromString(contractStr)
			if err != nil {
				b.Fatal(err)
			}
			count := int(float64(totalBuckets) * shares[ci])
			for k := 0; k < count; k++ {
				// Ids are spread out, not dense: mainnet's id space has large
				// holes from burnt buckets, and the scan must not care.
				id := uint64(k) * 3
				bkt := &contractstaking.Bucket{
					Candidate:      identityset.Address(30),
					Owner:          benchBackfillOwner(next, bucketsPerOwner),
					StakedAmount:   big.NewInt(1_000),
					StakedDuration: 86400,
					CreatedAt:      1,
				}
				// No feature context: the gate is shut, so this plants the
				// bucket without the owner index, which is the pre-activation
				// state the backfill exists to repair.
				if err := cs.UpsertBucket(context.Background(), contract, id, bkt); err != nil {
					b.Fatal(err)
				}
				next++
			}
		}
		ctx := forkGateCtx(backfillActivationHeight, true)
		b.StartTimer()
		if err := backfillOwnerIndex(ctx, sm); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBackfillOwnerIndex(b *testing.B) {
	// 10k is the stated ceiling across all three contracts; 5k is the current
	// mainnet-scale estimate. bucketsPerOwner=1 is the worst case for the write
	// side (one owner-index key per bucket), =8 for the batching path.
	for _, tc := range []struct {
		buckets, perOwner int
	}{
		{5_000, 1}, {5_000, 8}, {10_000, 1}, {10_000, 8},
	} {
		b.Run(fmt.Sprintf("buckets=%d/perOwner=%d", tc.buckets, tc.perOwner), func(b *testing.B) {
			benchmarkBackfillOwnerIndex(b, tc.buckets, tc.perOwner)
		})
	}
}
