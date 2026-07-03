// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"
	"sort"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// Basic Apply/Weights round-trip: single delta shows up in Weights.
func TestVoterWeightBase_ApplyAndReadBack(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	v.Apply(cand, voter, big.NewInt(100))
	got := v.Weights(cand)
	require.Len(got, 1)
	require.Equal(voter.String(), got[0].Voter.String())
	require.Zero(big.NewInt(100).Cmp(got[0].Weight))
}

// Same (cand, voter) applied twice aggregates; negative delta subtracts.
func TestVoterWeightBase_ApplyAggregatesAndSubtracts(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	v.Apply(cand, voter, big.NewInt(100))
	v.Apply(cand, voter, big.NewInt(50))
	v.Apply(cand, voter, big.NewInt(-30))

	got := v.Weights(cand)
	require.Len(got, 1)
	require.Zero(big.NewInt(120).Cmp(got[0].Weight))
}

// nil cand, nil voter, nil delta, and zero delta are all no-ops (no panic,
// no entry).
func TestVoterWeightBase_ApplyNoOps(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	cand := identityset.Address(1)
	voter := identityset.Address(2)

	v.Apply(nil, voter, big.NewInt(100))
	v.Apply(cand, nil, big.NewInt(100))
	v.Apply(cand, voter, nil)
	v.Apply(cand, voter, big.NewInt(0))

	require.Nil(v.Weights(cand))
}

// Weights output is sorted ascending by raw voter address bytes; zero-weight
// voters are filtered.
func TestVoterWeightBase_WeightsSortedAndFiltered(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	cand := identityset.Address(1)
	voters := []address.Address{
		identityset.Address(7),
		identityset.Address(3),
		identityset.Address(5),
		identityset.Address(9),
	}
	// Give each a positive weight, then zero out one via a negative delta of
	// equal magnitude so it should drop out of Weights().
	for i, w := range voters {
		v.Apply(cand, w, big.NewInt(int64((i+1)*10)))
	}
	v.Apply(cand, voters[2], big.NewInt(-30)) // voter[2] initial 30 → 0

	got := v.Weights(cand)
	require.Len(got, 3)
	for i := 1; i < len(got); i++ {
		require.Less(
			bytes.Compare(got[i-1].Voter.Bytes(), got[i].Voter.Bytes()),
			0,
			"Weights must be sorted ascending by voter address bytes",
		)
	}
}

// Returned slice is a copy — mutating an entry's Weight must not corrupt the
// view's internal state.
func TestVoterWeightBase_WeightsReturnsFreshCopy(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	cand := identityset.Address(1)
	voter := identityset.Address(2)
	v.Apply(cand, voter, big.NewInt(100))

	got := v.Weights(cand)
	require.Len(got, 1)
	got[0].Weight.SetInt64(9999) // pollute the returned pointer

	// Re-read; view must still reflect the original 100.
	again := v.Weights(cand)
	require.Zero(big.NewInt(100).Cmp(again[0].Weight))
}

// Nil / empty-candidate reads return nil (not a stray zero-length slice with
// backing array capacity).
func TestVoterWeightBase_WeightsNilInputs(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	require.Nil(v.Weights(nil))
	require.Nil(v.Weights(identityset.Address(1)))
}

// Fork produces an independent deep clone: mutating the fork must not touch
// the original, and vice versa.
func TestVoterWeightBase_ForkIsIndependent(t *testing.T) {
	require := require.New(t)
	orig := NewVoterWeightView()
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)

	orig.Apply(cand, voterA, big.NewInt(100))
	orig.Apply(cand, voterB, big.NewInt(200))

	forked := orig.Fork()
	// Mutate fork: shift voterA up, remove voterB.
	forked.Apply(cand, voterA, big.NewInt(50))
	forked.Apply(cand, voterB, big.NewInt(-200))
	forked.Commit()

	// Original still shows 100 + 200.
	origEntries := orig.Weights(cand)
	require.Len(origEntries, 2)
	sum := new(big.Int)
	for _, e := range origEntries {
		sum.Add(sum, e.Weight)
	}
	require.Zero(big.NewInt(300).Cmp(sum))

	// Fork sees the mutated state.
	forkEntries := forked.Weights(cand)
	require.Len(forkEntries, 1)
	require.Zero(big.NewInt(150).Cmp(forkEntries[0].Weight))
}

// Commit prunes zero-weight entries and drops candidate maps that become
// empty. Guards against unbounded growth as voters churn.
func TestVoterWeightBase_CommitPrunesZeros(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)

	base.Apply(cand, voterA, big.NewInt(100))
	base.Apply(cand, voterB, big.NewInt(50))
	// Zero out voterB.
	base.Apply(cand, voterB, big.NewInt(-50))

	base.Commit()

	inner, ok := base.cands[string(cand.Bytes())]
	require.True(ok)
	require.Len(inner, 1)
	_, present := inner[string(voterB.Bytes())]
	require.False(present, "zero-weight voter must be pruned by Commit")

	// Zero out the remaining voter → candidate bucket must be pruned entirely.
	base.Apply(cand, voterA, big.NewInt(-100))
	base.Commit()
	_, present = base.cands[string(cand.Bytes())]
	require.False(present, "empty candidate map must be pruned by Commit")
}

// Base's IsDirty always reports false; dirtiness is a wrapper concept.
func TestVoterWeightBase_IsDirtyAlwaysFalse(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	require.False(base.IsDirty())
	base.Apply(identityset.Address(1), identityset.Address(2), big.NewInt(1))
	require.False(base.IsDirty(), "base view mutates in place; only wrappers are dirty")
}

// Wrap produces an overlay: Apply on the wrapper does not touch the base's
// internal state, but Weights() through the wrapper sees the merged result.
func TestVoterWeightWrapper_OverlaySemantics(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)
	base.Apply(cand, voterA, big.NewInt(100))

	wrap := base.Wrap()
	wrap.Apply(cand, voterA, big.NewInt(25))
	wrap.Apply(cand, voterB, big.NewInt(50))

	// Base still shows 100 for voterA and no voterB entry.
	baseEntries := base.Weights(cand)
	require.Len(baseEntries, 1)
	require.Zero(big.NewInt(100).Cmp(baseEntries[0].Weight))

	// Wrapper shows the merged view: voterA=125, voterB=50.
	wrapEntries := wrap.Weights(cand)
	require.Len(wrapEntries, 2)
	byAddr := map[string]*big.Int{}
	for _, e := range wrapEntries {
		byAddr[e.Voter.String()] = e.Weight
	}
	require.Zero(big.NewInt(125).Cmp(byAddr[voterA.String()]))
	require.Zero(big.NewInt(50).Cmp(byAddr[voterB.String()]))
}

// Snapshot/Revert parity: wrapping the view, applying changes, then throwing
// the wrapper away must leave the base bit-for-bit identical.
func TestVoterWeightWrapper_SnapshotRevertParity(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)
	base.Apply(cand, voterA, big.NewInt(100))
	base.Apply(cand, voterB, big.NewInt(200))
	baselineA := base.Weights(cand)

	wrap := base.Wrap()
	// Simulate a mid-handler failure: several deltas applied, then discarded.
	wrap.Apply(cand, voterA, big.NewInt(-100)) // would drop voterA
	wrap.Apply(cand, voterB, big.NewInt(5000))
	wrap.Apply(cand, identityset.Address2(99), big.NewInt(1))
	// Drop wrap by not calling Commit — GC handles it.
	_ = wrap

	// Base must be exactly as before.
	afterA := base.Weights(cand)
	require.Equal(len(baselineA), len(afterA))
	for i := range baselineA {
		require.Equal(baselineA[i].Voter.String(), afterA[i].Voter.String())
		require.Zero(baselineA[i].Weight.Cmp(afterA[i].Weight))
	}
}

// Commit folds the wrapper's change into the base and resets the wrapper's
// dirty state. After Commit the wrapper reports IsDirty()==false.
func TestVoterWeightWrapper_CommitFoldsIntoBase(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	voterB := identityset.Address(3)
	base.Apply(cand, voterA, big.NewInt(100))

	wrap := base.Wrap()
	wrap.Apply(cand, voterA, big.NewInt(25))
	wrap.Apply(cand, voterB, big.NewInt(50))
	require.True(wrap.IsDirty())

	committed := wrap.Commit()
	require.False(wrap.IsDirty(), "wrapper must be clean after Commit")

	// Base now shows the merged state; committed is the base (return value
	// contract for wrapper.Commit).
	require.Same(base, committed)
	baseEntries := base.Weights(cand)
	require.Len(baseEntries, 2)
	byAddr := map[string]*big.Int{}
	for _, e := range baseEntries {
		byAddr[e.Voter.String()] = e.Weight
	}
	require.Zero(big.NewInt(125).Cmp(byAddr[voterA.String()]))
	require.Zero(big.NewInt(50).Cmp(byAddr[voterB.String()]))
}

// Nested Wrap on a wrapper stacks a second overlay. Discarding the outer
// wrapper leaves the inner overlay intact; this matches the two-level
// snapshot pattern csm uses when running an action inside a receipt handler.
func TestVoterWeightWrapper_NestedWrap(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	cand := identityset.Address(1)
	voterA := identityset.Address(2)
	base.Apply(cand, voterA, big.NewInt(100))

	inner := base.Wrap()
	inner.Apply(cand, voterA, big.NewInt(25)) // inner change: +25

	outer := inner.Wrap()
	outer.Apply(cand, voterA, big.NewInt(500)) // outer change: +500 (throwaway)

	// Outer view sees 100+25+500.
	outEntries := outer.Weights(cand)
	require.Len(outEntries, 1)
	require.Zero(big.NewInt(625).Cmp(outEntries[0].Weight))

	// Drop outer; inner must still see 100+25.
	innerEntries := inner.Weights(cand)
	require.Len(innerEntries, 1)
	require.Zero(big.NewInt(125).Cmp(innerEntries[0].Weight))
}

// Fork on a wrapper deep-clones both base and change: mutating one path
// (via the fork) leaves the original wrapper's view unchanged.
func TestVoterWeightWrapper_ForkIsIndependent(t *testing.T) {
	require := require.New(t)
	base := newVoterWeightBase()
	cand := identityset.Address(1)
	voter := identityset.Address(2)
	base.Apply(cand, voter, big.NewInt(100))

	wrap := base.Wrap()
	wrap.Apply(cand, voter, big.NewInt(50)) // change: +50 → view 150

	forked := wrap.Fork()
	forked.Apply(cand, voter, big.NewInt(1000))
	forked.Commit()

	// Original wrap still views 150; base still holds 100.
	origEntries := wrap.Weights(cand)
	require.Len(origEntries, 1)
	require.Zero(big.NewInt(150).Cmp(origEntries[0].Weight))

	baseEntries := base.Weights(cand)
	require.Len(baseEntries, 1)
	require.Zero(big.NewInt(100).Cmp(baseEntries[0].Weight))
}

// Deterministic incremental updates: same sequence of deltas → same output;
// permuted equivalent deltas (that yield the same per-voter sum) → same
// output. This is the invariant the reward snapshot's byte-equality skip
// relies on.
func TestVoterWeightBase_DeterministicIncrementalUpdates(t *testing.T) {
	require := require.New(t)
	cand := identityset.Address(1)
	voters := make([]address.Address, 8)
	for i := range voters {
		voters[i] = identityset.Address(i + 10)
	}

	// Reference: apply each voter's total weight once, in some order.
	ref := newVoterWeightBase()
	for i, v := range voters {
		ref.Apply(cand, v, big.NewInt(int64((i+1)*1000)))
	}
	refOut := ref.Weights(cand)

	// Variant: apply many small deltas in a permuted order that sums to the
	// same per-voter total.
	perm := []int{5, 0, 3, 7, 1, 4, 6, 2}
	got := newVoterWeightBase()
	for _, i := range perm {
		total := int64((i + 1) * 1000)
		// Split into 3 pieces so the sequence is genuinely different.
		got.Apply(cand, voters[i], big.NewInt(total/3))
		got.Apply(cand, voters[i], big.NewInt(total/3))
		got.Apply(cand, voters[i], big.NewInt(total-2*(total/3)))
	}
	gotOut := got.Weights(cand)

	require.Equal(len(refOut), len(gotOut))
	for i := range refOut {
		require.Equal(refOut[i].Voter.String(), gotOut[i].Voter.String())
		require.Zero(refOut[i].Weight.Cmp(gotOut[i].Weight))
	}
}

// Sort determinism check: shuffle insertion order; Weights() must yield the
// same byte-ordered slice every time.
func TestVoterWeightBase_SortIsDeterministic(t *testing.T) {
	require := require.New(t)
	cand := identityset.Address(1)
	voters := make([]address.Address, 12)
	for i := range voters {
		voters[i] = identityset.Address(i + 3)
	}

	insertOrders := [][]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
		{11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
		{5, 0, 11, 3, 7, 2, 8, 1, 4, 6, 9, 10},
	}

	var reference []VoterWeight
	for run, order := range insertOrders {
		v := newVoterWeightBase()
		for _, i := range order {
			v.Apply(cand, voters[i], big.NewInt(int64(i+1)*7))
		}
		got := v.Weights(cand)
		require.Len(got, len(voters))
		if run == 0 {
			reference = got
			// Sanity: reference is ascending.
			require.True(sort.SliceIsSorted(got, func(i, j int) bool {
				return bytes.Compare(got[i].Voter.Bytes(), got[j].Voter.Bytes()) < 0
			}))
			continue
		}
		require.Len(got, len(reference))
		for i := range reference {
			require.Equal(reference[i].Voter.String(), got[i].Voter.String(),
				"sort must not depend on insertion order (run %d, index %d)", run, i)
			require.Zero(reference[i].Weight.Cmp(got[i].Weight),
				"weight must not depend on insertion order (run %d, index %d)", run, i)
		}
	}
}

// Multi-candidate isolation: Apply on candidate A must not leak into
// candidate B's weight map.
func TestVoterWeightBase_CandidateIsolation(t *testing.T) {
	require := require.New(t)
	v := NewVoterWeightView()
	candA := identityset.Address(1)
	candB := identityset.Address(2)
	voter := identityset.Address(3)

	v.Apply(candA, voter, big.NewInt(100))
	v.Apply(candB, voter, big.NewInt(200))

	got := v.Weights(candA)
	require.Len(got, 1)
	require.Zero(big.NewInt(100).Cmp(got[0].Weight))

	got = v.Weights(candB)
	require.Len(got, 1)
	require.Zero(big.NewInt(200).Cmp(got[0].Weight))
}

// Context sink round-trip: WithVoterDeltaSink then VoterDeltaSinkFrom yields
// the same sink. Nil sink is a no-op. Bare context yields nil.
func TestVoterDeltaSink_ContextPlumbing(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	require.Nil(VoterDeltaSinkFrom(ctx))

	// Nil sink: ctx must be unchanged (WithVoterDeltaSink returns the input
	// ctx untouched so downstream consumers still see nil).
	nilCtx := WithVoterDeltaSink(ctx, nil)
	require.Nil(VoterDeltaSinkFrom(nilCtx))

	// Non-nil sink: recoverable via VoterDeltaSinkFrom.
	sink := newVoterWeightBase()
	sinkCtx := WithVoterDeltaSink(ctx, sink)
	require.Equal(VoterDeltaSink(sink), VoterDeltaSinkFrom(sinkCtx))
}

// End-to-end sink integration: mutating a view through the sink interface
// (as contract-staking event handling does) yields the same state as calling
// Apply directly.
func TestVoterDeltaSink_ApplyEquivalence(t *testing.T) {
	require := require.New(t)
	direct := NewVoterWeightView()
	viaSink := NewVoterWeightView()

	cand := identityset.Address(1)
	voter := identityset.Address(2)

	direct.Apply(cand, voter, big.NewInt(100))
	direct.Apply(cand, voter, big.NewInt(-25))

	sink := viaSink.(VoterDeltaSink)
	sink.Apply(cand, voter, big.NewInt(100))
	sink.Apply(cand, voter, big.NewInt(-25))

	d := direct.Weights(cand)
	s := viaSink.Weights(cand)
	require.Equal(len(d), len(s))
	require.Zero(d[0].Weight.Cmp(s[0].Weight))
}
