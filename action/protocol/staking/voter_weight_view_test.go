// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"math/big"
	"math/rand"
	"sort"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func candID(idx int) hash.Hash160 {
	return hash.BytesToHash160(identityset.Address(idx).Bytes())
}

func TestVoterWeightView_ApplyAdd(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)

	v.Apply(cand, voter, big.NewInt(100))

	out := v.VoterWeightsByCandidate(cand)
	r.Len(out, 1)
	r.True(address.Equal(voter, out[0].voter))
	r.Equal(int64(100), out[0].weight.Int64())
}

func TestVoterWeightView_ApplyAggregate(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)

	// Two buckets from the same voter to the same candidate must aggregate.
	v.Apply(cand, voter, big.NewInt(100))
	v.Apply(cand, voter, big.NewInt(50))

	out := v.VoterWeightsByCandidate(cand)
	r.Len(out, 1, "must aggregate, not duplicate")
	r.Equal(int64(150), out[0].weight.Int64())
}

func TestVoterWeightView_ApplyDecrease(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)

	v.Apply(cand, voter, big.NewInt(100))
	v.Apply(cand, voter, big.NewInt(-30))
	r.Equal(int64(70), v.VoterWeightsByCandidate(cand)[0].weight.Int64())

	// Drive to zero — entry must disappear.
	v.Apply(cand, voter, big.NewInt(-70))
	r.Empty(v.VoterWeightsByCandidate(cand))

	// And the candidate itself drops out of the view.
	r.True(v.IsEmpty())
}

func TestVoterWeightView_ApplyDecreaseBelowZeroDoesNotGoNegative(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)

	v.Apply(cand, voter, big.NewInt(50))
	// Withdraw more than present — should drop the entry, not leave -ve weight.
	v.Apply(cand, voter, big.NewInt(-100))
	r.Empty(v.VoterWeightsByCandidate(cand))
}

func TestVoterWeightView_ApplyNoOpOnUnknown(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)

	// Negative against missing candidate: no-op, no panic.
	v.Apply(cand, voter, big.NewInt(-100))
	r.True(v.IsEmpty())

	// Add the candidate, then negative against unknown voter: no-op.
	v.Apply(cand, identityset.Address(3), big.NewInt(100))
	v.Apply(cand, voter, big.NewInt(-100))
	out := v.VoterWeightsByCandidate(cand)
	r.Len(out, 1, "unknown-voter negative must not corrupt the existing voter")
	r.True(address.Equal(identityset.Address(3), out[0].voter))
}

func TestVoterWeightView_VoterWeightsSorted(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)

	// Add voters in non-sorted order.
	voters := []int{5, 2, 8, 1, 7, 3}
	for _, i := range voters {
		v.Apply(cand, identityset.Address(i), big.NewInt(int64(10+i)))
	}

	out := v.VoterWeightsByCandidate(cand)
	r.Len(out, len(voters))

	// Verify lexicographic ordering by address bytes.
	for i := 1; i < len(out); i++ {
		r.True(
			bytes.Compare(out[i-1].voter.Bytes(), out[i].voter.Bytes()) < 0,
			"slice must be sorted ascending by voter address",
		)
	}
}

func TestVoterWeightView_VoterWeightsByCandidateCopySafe(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)
	v.Apply(cand, voter, big.NewInt(100))

	out := v.VoterWeightsByCandidate(cand)
	// Mutating the returned weight must not affect the view.
	out[0].weight.SetInt64(999)

	again := v.VoterWeightsByCandidate(cand)
	r.Equal(int64(100), again[0].weight.Int64(), "caller mutation must not leak into view")
}

func TestVoterWeightView_MultipleCandidates(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	candA, candB := candID(1), candID(2)
	voter := identityset.Address(3)

	// A voter can simultaneously contribute to multiple candidates.
	v.Apply(candA, voter, big.NewInt(100))
	v.Apply(candB, voter, big.NewInt(200))

	r.Equal(int64(100), v.VoterWeightsByCandidate(candA)[0].weight.Int64())
	r.Equal(int64(200), v.VoterWeightsByCandidate(candB)[0].weight.Int64())

	// Withdraw from A — B unaffected.
	v.Apply(candA, voter, big.NewInt(-100))
	r.Empty(v.VoterWeightsByCandidate(candA))
	r.Equal(int64(200), v.VoterWeightsByCandidate(candB)[0].weight.Int64())
}

// findWeight returns the weighted votes for `voter` on `cand`, or nil if
// the voter is not present. Used by tests to look up by address rather than
// by slot (slot order depends on lexicographic address ordering, which is
// not stable across identityset indices).
func findWeight(out []voterWeight, voter address.Address) *big.Int {
	for _, vw := range out {
		if address.Equal(voter, vw.voter) {
			return vw.weight
		}
	}
	return nil
}

func TestVoterWeightView_Clone(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	v.Apply(cand, identityset.Address(2), big.NewInt(100))
	v.Apply(cand, identityset.Address(3), big.NewInt(200))

	clone := v.Clone()
	r.Equal(v.Hash(), clone.Hash())

	// Mutating clone must not affect original.
	clone.Apply(cand, identityset.Address(2), big.NewInt(50))
	r.NotEqual(v.Hash(), clone.Hash())
	r.Equal(int64(100), findWeight(v.VoterWeightsByCandidate(cand), identityset.Address(2)).Int64())
	r.Equal(int64(150), findWeight(clone.VoterWeightsByCandidate(cand), identityset.Address(2)).Int64())
	r.Equal(int64(200), findWeight(v.VoterWeightsByCandidate(cand), identityset.Address(3)).Int64(), "untouched voter must be unchanged")
}

func TestVoterWeightView_HashDeterministic(t *testing.T) {
	r := require.New(t)

	// Two views, same logical state, built in different insertion orders —
	// hashes must match exactly. This is the core determinism property
	// across nodes.
	v1 := NewVoterWeightView()
	v2 := NewVoterWeightView()

	cand := candID(1)
	v1.Apply(cand, identityset.Address(5), big.NewInt(100))
	v1.Apply(cand, identityset.Address(2), big.NewInt(200))
	v1.Apply(cand, identityset.Address(8), big.NewInt(50))

	// Different insertion order.
	v2.Apply(cand, identityset.Address(8), big.NewInt(50))
	v2.Apply(cand, identityset.Address(2), big.NewInt(200))
	v2.Apply(cand, identityset.Address(5), big.NewInt(100))

	r.Equal(v1.Hash(), v2.Hash())
}

func TestVoterWeightView_HashChangesOnUpdate(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	cand := candID(1)
	voter := identityset.Address(2)
	v.Apply(cand, voter, big.NewInt(100))

	h1 := v.Hash()
	v.Apply(cand, voter, big.NewInt(1))
	h2 := v.Hash()
	r.NotEqual(h1, h2, "any weight change must alter the hash")

	// Returning to the same state restores the hash.
	v.Apply(cand, voter, big.NewInt(-1))
	r.Equal(h1, v.Hash())
}

func TestVoterWeightView_HashEmpty(t *testing.T) {
	r := require.New(t)
	v := NewVoterWeightView()
	r.Equal(hash.ZeroHash256, v.Hash())
	r.True(v.IsEmpty())
}

// TestVoterWeightView_IncrementalMatchesBatch is the foundational
// determinism property: applying N (cand, voter, delta) operations in any
// order produces the same view hash as the equivalent batch — voters can
// stake/unstake in any sequence and arrive at the same state.
func TestVoterWeightView_IncrementalMatchesBatch(t *testing.T) {
	r := require.New(t)
	rng := rand.New(rand.NewSource(42))

	// Build a baseline: 5 candidates, 20 voters, random (cand, voter)
	// final-state weights. Apply in random insertion order on view1; in a
	// different random order on view2.
	type entry struct {
		cand   hash.Hash160
		voter  address.Address
		weight int64
	}
	entries := make([]entry, 0, 60)
	for c := 0; c < 5; c++ {
		for vi := 0; vi < 12; vi++ {
			entries = append(entries, entry{
				cand:   candID(c),
				voter:  identityset.Address(vi),
				weight: int64(1 + rng.Intn(10000)),
			})
		}
	}

	v1 := NewVoterWeightView()
	for _, e := range entries {
		v1.Apply(e.cand, e.voter, big.NewInt(e.weight))
	}

	// Shuffle and replay against v2.
	shuffled := make([]entry, len(entries))
	copy(shuffled, entries)
	rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

	v2 := NewVoterWeightView()
	for _, e := range shuffled {
		v2.Apply(e.cand, e.voter, big.NewInt(e.weight))
	}

	r.Equal(v1.Hash(), v2.Hash())
}

// TestVoterWeightView_IncrementalMatchesRebuild stresses the "restart"
// path: a long sequence of bucket-lifecycle-realistic adds and
// withdrawals (incremental) must match a one-shot rebuild from the final
// per-(cand, voter) weights.
//
// Realistic means: a voter cannot withdraw more weight than it currently
// holds on a candidate (the staking handlers enforce this via bucket
// state). If we allowed over-withdrawal, the incremental view silently
// absorbs the excess (entry drops to 0 and is removed) while a net-sum
// rebuild keeps the negative carry — producing two different end states
// for the same op stream. This is a property of the protocol, not a bug.
func TestVoterWeightView_IncrementalMatchesRebuild(t *testing.T) {
	r := require.New(t)
	rng := rand.New(rand.NewSource(7))

	const (
		nCands  = 4
		nVoters = 8
		nOps    = 200
	)

	type runningKey struct {
		cand  int
		voter int
	}
	running := make(map[runningKey]int64)

	type op struct {
		cand  hash.Hash160
		voter address.Address
		delta int64
	}
	ops := make([]op, 0, nOps)
	for i := 0; i < nOps; i++ {
		c := rng.Intn(nCands)
		v := rng.Intn(nVoters)
		k := runningKey{c, v}
		var delta int64
		if running[k] == 0 || rng.Intn(2) == 0 {
			// add positive weight
			delta = int64(1 + rng.Intn(1000))
		} else {
			// partial or full withdrawal (clamped to running balance)
			maxWithdraw := running[k]
			delta = -(1 + rng.Int63n(maxWithdraw))
		}
		running[k] += delta
		ops = append(ops, op{
			cand:  candID(c),
			voter: identityset.Address(v),
			delta: delta,
		})
	}

	// Replay incrementally.
	v := NewVoterWeightView()
	for _, o := range ops {
		v.Apply(o.cand, o.voter, big.NewInt(o.delta))
	}

	// Rebuild from final net sums (positive only — the running map already
	// stays non-negative because of the clamping above).
	rebuild := NewVoterWeightView()
	keys := make([]runningKey, 0, len(running))
	for k := range running {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].cand != keys[j].cand {
			return keys[i].cand < keys[j].cand
		}
		return keys[i].voter < keys[j].voter
	})
	for _, k := range keys {
		if running[k] <= 0 {
			continue
		}
		rebuild.Apply(candID(k.cand), identityset.Address(k.voter), big.NewInt(running[k]))
	}

	r.Equal(v.Hash(), rebuild.Hash())
}

func BenchmarkVoterWeightView_Apply(b *testing.B) {
	v := NewVoterWeightView()
	cand := candID(1)
	// identityset has a fixed-size key list — stay within range.
	const nVoters = 30
	voters := make([]address.Address, nVoters)
	for i := range voters {
		voters[i] = identityset.Address(i)
	}
	weights := []*big.Int{
		big.NewInt(123),
		big.NewInt(50),
		big.NewInt(7777),
		big.NewInt(100),
	}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		v.Apply(cand, voters[n%nVoters], weights[n%len(weights)])
	}
}
