// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// BenchmarkVoterWeightSeedFlush measures one block's worth of the IIP-59
// activation flush, so VoterWeightSeedBatchSize can be set from a number rather
// than inherited from VoterBudgetPerBlock.
//
// One "op" is one block: list the next batch in key order, mark those pairs, and
// write them. That is exactly what CreatePreStates + viewData.Commit do.
//
// **Fidelity caveat**, same as BenchmarkFreezeSnapshotNativeEnumeration: the
// writes go to testdb.NewMockStateManager, an in-memory map. Production writes
// go through the state trie, which is slower by an implementation-dependent
// factor. These numbers are a LOWER BOUND on per-block cost and an upper bound
// on the batch size they justify. Treat them as a red-light signal, and leave
// headroom when picking the value.
func BenchmarkVoterWeightSeedFlush(b *testing.B) {
	for _, tier := range []struct {
		name       string
		candidates int
		perCand    int
	}{
		// Roughly today's mainnet shape: ~52 delegates, ~7.5k voter pairs.
		{name: "mainnet_52d_7500p", candidates: 52, perCand: 145},
		// The ceiling the drain was sized against: one whale plus a long tail.
		{name: "ceiling_30000p", candidates: 60, perCand: 500},
	} {
		for _, batch := range []int{500, 1000, 2000, 5000} {
			b.Run(fmt.Sprintf("%s/batch_%d", tier.name, batch), func(b *testing.B) {
				total := tier.candidates * tier.perCand
				blocks := (total + batch - 1) / batch

				b.ResetTimer()
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					view := benchSeedView(tier.candidates, tier.perCand)
					ctrl := gomock.NewController(b)
					sm := testdb.NewMockStateManager(ctrl)
					base := view.(*voterWeightBase)
					var cursor voterWeightSeedCursor
					b.StartTimer()

					// Time the whole flush, then report per-block below: a
					// single block is too fast to time reliably on its own.
					for !cursor.Done {
						pairs := view.SeedPairsAfter(cursor.position(), batch)
						for _, ref := range pairs {
							view.MarkForRewrite(ref.cand, ref.voter)
						}
						if len(pairs) > 0 {
							last := pairs[len(pairs)-1]
							cursor.LastCand, cursor.LastVoter = last.cand, last.voter
							cursor.Started = true
						}
						if len(pairs) < batch {
							cursor.Done = true
						}
						if err := base.persist(sm); err != nil {
							b.Fatalf("persist: %v", err)
						}
						base.dirty = false
						base.touched = nil
					}
				}
				b.StopTimer()

				nsPerFlush := float64(b.Elapsed().Nanoseconds()) / float64(b.N)
				msPerBlock := nsPerFlush / 1e6 / float64(blocks)
				b.ReportMetric(msPerBlock, "ms/block")
				b.ReportMetric(float64(blocks), "blocks")
				b.Logf("tier=%s batch=%d pairs=%d blocks=%d per-block=%.2fms total=%.0fms budget=2500ms",
					tier.name, batch, total, blocks, msPerBlock, nsPerFlush/1e6)
			})
		}
	}
}

// benchSeedView builds a view with the requested shape. Voter addresses are
// synthesized rather than drawn from identityset, which only has 35 entries.
func benchSeedView(candidates, perCand int) VoterWeightView {
	v := NewVoterWeightView()
	for c := 0; c < candidates; c++ {
		cand := hash.BytesToHash160(benchAddress(1_000_000 + c).Bytes())
		for i := 0; i < perCand; i++ {
			v.Apply(cand, benchAddress(c*perCand+i), big.NewInt(int64(1_000+i)))
		}
	}
	return v
}
