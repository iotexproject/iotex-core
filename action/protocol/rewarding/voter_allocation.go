// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
)

// rotatedIndex maps a logical payout position to its index in the frozen entry
// list. The rotation is what keeps a settlement that repeatedly runs long from
// always serving the head of the list first; it changes the order of service,
// never who is included or what they are owed.
func rotatedIndex(start, logical, total uint32) uint32 {
	if total == 0 {
		return 0
	}
	return (start + logical) % total
}

// voterDistributionMetadata derives the three inputs the allocator cannot
// compute for itself — the weight total, which payout position absorbs the
// dust, and whether any position has weight at all. Phase A calls it once
// against the freshly frozen snapshot and stores the results in the cursor;
// every later chunk reads them back. Continuation chunks must NOT recompute
// them: the snapshot keeps mutating underneath a settlement that spans blocks,
// and a recomputed total would shift every remaining voter's share, which is
// exactly what TestVoterAllocationIsChunkInvariant forbids. That is why this
// is a free function producing allocator inputs rather than a method deriving
// them lazily.
//
// staking.CandidatePollSnapshot.refreshDerivedMetadata computes fields by the
// same names, but in *physical* index space over every entry; this one works in
// *logical* (post-rotation) space and ignores non-positive weights. The two
// TotalWeight values agree — rotation and zero-weight entries change neither
// the sum nor HasWeightedEntries — but the two LastWeightedIndex values do not
// once voterStartIndex is non-zero, and only the logical one names the dust
// absorber. voterRewardStatus's staleness check compares the first two fields
// and deliberately omits LastWeightedIndex for this reason.
func voterDistributionMetadata(
	snap *staking.CandidatePollSnapshot,
	voterStartIndex uint32,
) (*big.Int, hash.Hash256, uint32, bool) {
	totalWeight := new(big.Int)
	var lastWeightedIndex uint32
	var hasWeightedEntries bool
	if snap == nil || len(snap.Entries) == 0 {
		return totalWeight, hash.ZeroHash256, lastWeightedIndex, hasWeightedEntries
	}
	totalVoters := uint32(len(snap.Entries))
	voterStartIndex %= totalVoters
	for logicalIndex := uint32(0); logicalIndex < totalVoters; logicalIndex++ {
		entry := snap.Entries[rotatedIndex(voterStartIndex, logicalIndex, totalVoters)]
		if entry.Weight == nil || entry.Weight.Sign() <= 0 {
			continue
		}
		totalWeight.Add(totalWeight, entry.Weight)
		lastWeightedIndex = logicalIndex
		hasWeightedEntries = true
	}
	return totalWeight, snapshotHashFull(snap), lastWeightedIndex, hasWeightedEntries
}

// voterAllocator is the single implementation of the per-voter share rule.
//
// Two callers need it: the drain, which pays, and the read-only status query,
// which reports what a voter is owed. They ask in different shapes — the drain
// walks forward carrying a running total across block boundaries, the status
// query jumps straight to one voter — so the rule used to be written twice.
// Keeping one implementation behind both is what makes
// TestVoterRewardAmountMatchesAllocation a statement about the code rather than
// a coincidence.
type voterAllocator struct {
	entries            []staking.VoterWeight
	start              uint32
	pool               *big.Int
	totalWeight        *big.Int
	lastWeightedIndex  uint32
	hasWeightedEntries bool
}

func newVoterAllocator(
	snap *staking.CandidatePollSnapshot,
	pool *big.Int,
	totalWeight *big.Int,
	voterStartIndex uint32,
	lastWeightedIndex uint32,
	hasWeightedEntries bool,
) *voterAllocator {
	a := &voterAllocator{
		pool:               safeBig(pool),
		totalWeight:        safeBig(totalWeight),
		lastWeightedIndex:  lastWeightedIndex,
		hasWeightedEntries: hasWeightedEntries,
	}
	if snap != nil {
		a.entries = snap.Entries
	}
	if total := a.count(); total > 0 {
		a.start = voterStartIndex % total
	}
	return a
}

// voterAllocatorForWork builds the allocator from a frozen cursor work item,
// which is where every input lives once a settlement has started.
func voterAllocatorForWork(snap *staking.CandidatePollSnapshot, work *epochDrainDelegateWork) *voterAllocator {
	if work == nil {
		return newVoterAllocator(snap, nil, nil, 0, 0, false)
	}
	return newVoterAllocator(
		snap,
		work.VoterAmountFrozen,
		work.TotalWeight,
		work.VoterStartIndex,
		work.LastWeightedIndex,
		work.HasWeightedEntries,
	)
}

func (a *voterAllocator) count() uint32 {
	return uint32(len(a.entries))
}

// physicalIndex maps a logical payout position to its snapshot index.
func (a *voterAllocator) physicalIndex(logical uint32) uint32 {
	return rotatedIndex(a.start, logical, a.count())
}

// logicalIndex is the inverse of physicalIndex: where in the payout order a
// given snapshot entry falls.
func (a *voterAllocator) logicalIndex(physical uint32) uint32 {
	total := a.count()
	if total == 0 {
		return 0
	}
	return (physical + total - a.start) % total
}

func (a *voterAllocator) entryAt(logical uint32) staking.VoterWeight {
	return a.entries[a.physicalIndex(logical)]
}

// isDustVoter reports whether this position absorbs the integer-division
// remainder. Exactly one position does, so sum(shares) equals the pool exactly.
func (a *voterAllocator) isDustVoter(logical uint32) bool {
	return a.hasWeightedEntries && logical == a.lastWeightedIndex
}

// shareAt returns what the voter at logical position is owed, given the amount
// already allocated to earlier positions. Callers walking forward pass their
// running total; callers landing on one voter pass allocatedBefore(logical).
func (a *voterAllocator) shareAt(logical uint32, allocatedBefore *big.Int) (*big.Int, error) {
	share := new(big.Int)
	if a.count() == 0 || a.pool.Sign() <= 0 || a.totalWeight.Sign() <= 0 {
		return share, nil
	}
	weight := a.entryAt(logical).Weight
	if weight == nil || weight.Sign() <= 0 {
		return share, nil
	}
	if a.isDustVoter(logical) {
		share.Sub(a.pool, safeBig(allocatedBefore))
		if share.Sign() < 0 {
			return nil, errors.New("rewarding: distributed voter amount exceeds frozen pool")
		}
		return share, nil
	}
	share.Mul(a.pool, weight)
	return share.Div(share, a.totalWeight), nil
}

// allocatedBefore sums the shares owed to every position ahead of logical. It
// is O(logical) and only the dust voter's share depends on it, so callers
// without a running total should go through shareOf rather than paying for it
// on every lookup.
func (a *voterAllocator) allocatedBefore(logical uint32) *big.Int {
	sum := new(big.Int)
	if a.count() == 0 || a.pool.Sign() <= 0 || a.totalWeight.Sign() <= 0 {
		return sum
	}
	for i := uint32(0); i < logical; i++ {
		weight := a.entryAt(i).Weight
		if weight == nil || weight.Sign() <= 0 {
			continue
		}
		share := new(big.Int).Mul(a.pool, weight)
		sum.Add(sum, share.Div(share, a.totalWeight))
	}
	return sum
}

// shareOf answers "what is this one voter owed" for callers with no running
// total, reconstructing the prefix sum only when the answer depends on it.
func (a *voterAllocator) shareOf(logical uint32) (*big.Int, error) {
	var before *big.Int
	if a.isDustVoter(logical) {
		before = a.allocatedBefore(logical)
	}
	return a.shareAt(logical, before)
}
