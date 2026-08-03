// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// oracleShares is a deliberately naive third implementation of the voter
// allocation, written to be obviously correct rather than efficient: walk every
// logical position in order, accumulating as it goes, and hand the final
// weighted voter whatever is left. It exists so the distribution path and the
// read-only status path are each checked against something neither of them
// shares — an extraction that changed both in the same wrong way would still
// fail here.
//
// Returns shares indexed by *logical* position.
func oracleShares(
	entries []staking.VoterWeight,
	voterStartIndex uint32,
	pool *big.Int,
	totalWeight *big.Int,
	lastWeightedIndex uint32,
	hasWeightedEntries bool,
) []*big.Int {
	total := uint32(len(entries))
	out := make([]*big.Int, total)
	if total == 0 {
		return out
	}
	start := voterStartIndex % total
	distributed := new(big.Int)
	for logical := uint32(0); logical < total; logical++ {
		share := new(big.Int)
		w := entries[(start+logical)%total].Weight
		if pool.Sign() > 0 && totalWeight.Sign() > 0 && w != nil && w.Sign() > 0 {
			if hasWeightedEntries && logical == lastWeightedIndex {
				share.Sub(pool, distributed)
			} else {
				share.Mul(pool, w)
				share.Div(share, totalWeight)
			}
		}
		out[logical] = share
		distributed.Add(distributed, share)
	}
	return out
}

// allocationCase is one randomized fixture: a frozen snapshot plus the cursor
// work item Phase A would have derived from it.
type allocationCase struct {
	snap *staking.CandidatePollSnapshot
	work *epochDrainDelegateWork
	pool *big.Int
}

func newAllocationCase(rng *rand.Rand) allocationCase {
	n := 1 + rng.Intn(11)
	entries := make([]staking.VoterWeight, n)
	for i := 0; i < n; i++ {
		var w *big.Int
		switch rng.Intn(6) {
		case 0:
			w = new(big.Int) // zero-weight entry: earns nothing, still occupies a slot
		case 1:
			w = nil // malformed entry, tolerated as zero
		default:
			w = big.NewInt(int64(1 + rng.Intn(997)))
		}
		entries[i] = staking.VoterWeight{Voter: identityset.Address(i), Weight: w}
	}
	snap := &staking.CandidatePollSnapshot{Entries: entries}

	voterStartIndex := uint32(rng.Intn(2 * n))
	// Metadata comes from the production derivation, not a test copy: the whole
	// point is that the status path agrees with what Phase A actually froze.
	totalWeight, snapHash, lastWeightedIndex, hasWeighted := voterDistributionMetadata(snap, voterStartIndex)

	pool := big.NewInt(int64(rng.Intn(100_000)))
	if rng.Intn(8) == 0 {
		pool = new(big.Int) // an era where this delegate accrued nothing
	}
	return allocationCase{
		snap: snap,
		pool: pool,
		work: &epochDrainDelegateWork{
			VoterAmountFrozen:  pool,
			TotalWeight:        totalWeight,
			SnapshotHash:       snapHash[:],
			LastWeightedIndex:  lastWeightedIndex,
			HasWeightedEntries: hasWeighted,
			VoterStartIndex:    voterStartIndex,
		},
	}
}

// TestVoterRewardAmountMatchesAllocation pins the read-only status API to the
// allocation rule. shareOf answers "what is voter X owed" for a caller with no
// running total, and must agree rau-for-rau with what the drain pays. The two
// were separate implementations before the allocator was extracted; this is
// what keeps them from drifting apart again.
func TestVoterRewardAmountMatchesAllocation(t *testing.T) {
	r := require.New(t)
	for seed := int64(0); seed < 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		c := newAllocationCase(rng)
		want := oracleShares(
			c.snap.Entries, c.work.VoterStartIndex, c.pool,
			safeBig(c.work.TotalWeight), c.work.LastWeightedIndex, c.work.HasWeightedEntries,
		)
		alloc := voterAllocatorForWork(c.snap, c.work)
		for logical := range want {
			got, err := alloc.shareOf(uint32(logical))
			r.NoError(err)
			r.Zerof(want[logical].Cmp(got),
				"seed %d logical %d: status path says %s, allocation says %s",
				seed, logical, got, want[logical])
		}
	}
}

// TestVoterAllocatorIndexRoundTrip pins physicalIndex and logicalIndex as
// inverses. The status query finds a voter by binary-searching the snapshot
// (a physical index) and must convert to the payout order to compare against
// the cursor's progress; an off-by-one there reports the wrong voter's amount.
func TestVoterAllocatorIndexRoundTrip(t *testing.T) {
	r := require.New(t)
	for seed := int64(0); seed < 100; seed++ {
		rng := rand.New(rand.NewSource(seed))
		c := newAllocationCase(rng)
		alloc := voterAllocatorForWork(c.snap, c.work)
		for i := uint32(0); i < alloc.count(); i++ {
			r.Equal(i, alloc.logicalIndex(alloc.physicalIndex(i)), "seed %d logical %d", seed, i)
			r.Equal(i, alloc.physicalIndex(alloc.logicalIndex(i)), "seed %d physical %d", seed, i)
		}
	}
}

// TestVoterAllocationIsExact is the fund-conservation property IIP-59 states
// as "every rau in a delegate's voter pool is paid out": the shares must sum to
// the pool exactly whenever at least one voter carries weight, with the final
// weighted voter absorbing the integer-division remainder.
func TestVoterAllocationIsExact(t *testing.T) {
	r := require.New(t)
	for seed := int64(0); seed < 300; seed++ {
		rng := rand.New(rand.NewSource(seed))
		c := newAllocationCase(rng)
		shares := oracleShares(
			c.snap.Entries, c.work.VoterStartIndex, c.pool,
			safeBig(c.work.TotalWeight), c.work.LastWeightedIndex, c.work.HasWeightedEntries,
		)
		sum := new(big.Int)
		for _, s := range shares {
			r.GreaterOrEqualf(s.Sign(), 0, "seed %d: negative share %s", seed, s)
			sum.Add(sum, s)
		}
		if c.work.HasWeightedEntries && c.pool.Sign() > 0 {
			r.Zerof(sum.Cmp(c.pool), "seed %d: shares sum to %s, pool is %s", seed, sum, c.pool)
		} else {
			r.Zerof(sum.Sign(), "seed %d: nothing should be paid, got %s", seed, sum)
		}
	}
}

// TestVoterAllocationIsChunkInvariant states the property the chunked drain
// rests on: where the block boundaries fall must not change any voter's amount.
// The running accumulator the distribution path carries across chunks
// (VoterAmountDistributed) must reproduce the single-pass result for every
// chunk size.
func TestVoterAllocationIsChunkInvariant(t *testing.T) {
	r := require.New(t)
	for seed := int64(0); seed < 200; seed++ {
		rng := rand.New(rand.NewSource(seed))
		c := newAllocationCase(rng)
		want := oracleShares(
			c.snap.Entries, c.work.VoterStartIndex, c.pool,
			safeBig(c.work.TotalWeight), c.work.LastWeightedIndex, c.work.HasWeightedEntries,
		)
		alloc := voterAllocatorForWork(c.snap, c.work)
		total := alloc.count()
		for _, chunk := range []uint32{1, 2, 3, 5, 100} {
			t.Run(fmt.Sprintf("seed_%d/chunk_%d", seed, chunk), func(t *testing.T) {
				var got []*big.Int
				distributed := new(big.Int)
				for start := uint32(0); start < total; start += chunk {
					end := start + chunk
					if end > total {
						end = total
					}
					for logical := start; logical < end; logical++ {
						s, err := alloc.shareAt(logical, distributed)
						r.NoError(err)
						got = append(got, s)
						distributed.Add(distributed, s)
					}
				}
				for i := range want {
					r.Zerof(want[i].Cmp(got[i]),
						"seed %d chunk %d logical %d: %s != %s", seed, chunk, i, got[i], want[i])
				}
			})
		}
	}
}
