// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// vwModel drives the two derivations of per-(candidate, voter) weight that
// IIP-59 keeps: the incremental view the staking hooks maintain, and the full
// rebuild from buckets that runs at startup. Every mutation updates the bucket
// set and applies the matching delta to the incremental view, exactly as
// addCandidateVotes / subCandidateVotes do in the handlers.
type vwModel struct {
	consts     genesis.VoteWeightCalConsts
	incr       VoterWeightView
	buckets    map[uint64]*VoteBucket
	candidates map[string]*Candidate
	nextIndex  uint64
	clock      time.Time
}

func newVWModel(numCands int) *vwModel {
	m := &vwModel{
		consts:     genesis.TestDefault().Staking.VoteWeightCalConsts,
		incr:       NewVoterWeightView(),
		buckets:    make(map[uint64]*VoteBucket),
		candidates: make(map[string]*Candidate),
		nextIndex:  1,
		clock:      time.Unix(1_700_000_000, 0).UTC(),
	}
	for i := 0; i < numCands; i++ {
		cand := &Candidate{
			Owner:              identityset.Address(i),
			Identifier:         identityset.Address(20 + i),
			Votes:              big.NewInt(0),
			SelfStake:          big.NewInt(0),
			SelfStakeBucketIdx: candidateNoSelfStakeBucketIndex,
		}
		m.candidates[cand.GetIdentifier().String()] = cand
	}
	return m
}

func (m *vwModel) candList() []*Candidate {
	out := make([]*Candidate, 0, len(m.candidates))
	for i := 0; i < len(m.candidates); i++ {
		out = append(out, m.candidates[identityset.Address(20+i).String()])
	}
	return out
}

func (m *vwModel) candFor(b *VoteBucket) *Candidate {
	return m.candidates[b.Candidate.String()]
}

// weight mirrors buildVoterWeightView's self-stake determination so the two
// derivations cannot differ merely because the test computed the flag itself.
func (m *vwModel) weight(b *VoteBucket) *big.Int {
	cand := m.candFor(b)
	if cand == nil {
		return big.NewInt(0)
	}
	isSelfStake := b.ContractAddress == "" && b.Index == cand.SelfStakeBucketIdx
	return CalculateVoteWeight(m.consts, b, isSelfStake)
}

func (m *vwModel) applyDelta(cand address.Address, voter address.Address, delta *big.Int) {
	if delta == nil || delta.Sign() == 0 {
		return
	}
	m.incr.Apply(hash.BytesToHash160(cand.Bytes()), voter, delta)
}

func (m *vwModel) credit(b *VoteBucket) {
	m.applyDelta(b.Candidate, b.Owner, m.weight(b))
}

func (m *vwModel) debit(b *VoteBucket) {
	m.applyDelta(b.Candidate, b.Owner, new(big.Int).Neg(m.weight(b)))
}

func (m *vwModel) bucketList() []*VoteBucket {
	out := make([]*VoteBucket, 0, len(m.buckets))
	for i := uint64(1); i < m.nextIndex; i++ {
		if b, ok := m.buckets[i]; ok {
			out = append(out, b)
		}
	}
	return out
}

// rebuild runs the same code path Protocol.Start uses at every restart.
func (m *vwModel) rebuild() VoterWeightView {
	return buildVoterWeightView(m.bucketList(), m.candFor, m.consts)
}

func (m *vwModel) activeIndices() []uint64 {
	out := make([]uint64, 0, len(m.buckets))
	for i := uint64(1); i < m.nextIndex; i++ {
		if b, ok := m.buckets[i]; ok && !b.isUnstaked() {
			out = append(out, i)
		}
	}
	return out
}

// -------- mutations, each mirroring one handler --------

func (m *vwModel) createStake(rng *rand.Rand) string {
	cands := m.candList()
	cand := cands[rng.Intn(len(cands))]
	b := NewVoteBucket(
		cand.GetIdentifier(),
		identityset.Address(rng.Intn(8)),
		big.NewInt(int64(1+rng.Intn(1000))),
		uint32(rng.Intn(30)),
		m.clock,
		rng.Intn(2) == 0,
	)
	b.Index = m.nextIndex
	m.nextIndex++
	m.buckets[b.Index] = b
	m.credit(b)
	return fmt.Sprintf("createStake idx=%d", b.Index)
}

func (m *vwModel) deposit(rng *rand.Rand) string {
	idx := m.pickActive(rng)
	if idx == 0 {
		return ""
	}
	b := m.buckets[idx]
	m.debit(b)
	b.StakedAmount = new(big.Int).Add(b.StakedAmount, big.NewInt(int64(1+rng.Intn(500))))
	m.credit(b)
	return fmt.Sprintf("deposit idx=%d", idx)
}

// restake may leave the weight unchanged when the duration is re-set to the
// value it already had — the -w/+w pair must still land on the same state.
func (m *vwModel) restake(rng *rand.Rand) string {
	idx := m.pickActive(rng)
	if idx == 0 {
		return ""
	}
	b := m.buckets[idx]
	m.debit(b)
	b.StakedDuration = time.Duration(rng.Intn(30)) * 24 * time.Hour
	m.credit(b)
	return fmt.Sprintf("restake idx=%d", idx)
}

func (m *vwModel) unstake(rng *rand.Rand) string {
	idx := m.pickActive(rng)
	if idx == 0 {
		return ""
	}
	b := m.buckets[idx]
	// Debit before flipping the flag: the handler subtracts the weight the
	// bucket still has, and the rebuild then skips the bucket entirely.
	m.debit(b)
	b.UnstakeStartTime = b.StakeStartTime.Add(time.Hour)
	if cand := m.candFor(b); cand != nil && cand.SelfStakeBucketIdx == b.Index {
		cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	}
	return fmt.Sprintf("unstake idx=%d", idx)
}

// withdraw removes an already-unstaked bucket. It must apply no delta at all —
// unstake already removed the weight, and a second debit here is the
// double-subtract this test exists to catch.
func (m *vwModel) withdraw(rng *rand.Rand) string {
	for i := uint64(1); i < m.nextIndex; i++ {
		b, ok := m.buckets[i]
		if !ok || !b.isUnstaked() {
			continue
		}
		if rng.Intn(2) == 0 {
			continue
		}
		delete(m.buckets, i)
		return fmt.Sprintf("withdraw idx=%d", i)
	}
	return ""
}

func (m *vwModel) changeCandidate(rng *rand.Rand) string {
	idx := m.pickActive(rng)
	if idx == 0 {
		return ""
	}
	b := m.buckets[idx]
	cands := m.candList()
	next := cands[rng.Intn(len(cands))]
	if address.Equal(next.GetIdentifier(), b.Candidate) {
		return ""
	}
	m.debit(b)
	if cand := m.candFor(b); cand != nil && cand.SelfStakeBucketIdx == b.Index {
		cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	}
	b.Candidate = next.GetIdentifier()
	m.credit(b)
	return fmt.Sprintf("changeCandidate idx=%d", idx)
}

// transferOwnership moves weight between voters without changing the
// candidate's total — the one case that legitimately bypasses the choke point.
func (m *vwModel) transferOwnership(rng *rand.Rand) string {
	idx := m.pickActive(rng)
	if idx == 0 {
		return ""
	}
	b := m.buckets[idx]
	newOwner := identityset.Address(rng.Intn(8))
	if address.Equal(newOwner, b.Owner) {
		return ""
	}
	w := m.weight(b)
	m.applyDelta(b.Candidate, b.Owner, new(big.Int).Neg(w))
	m.applyDelta(b.Candidate, newOwner, w)
	b.Owner = newOwner
	return fmt.Sprintf("transferOwnership idx=%d", idx)
}

// toggleSelfStake exercises the self-stake bonus, whose weight change is driven
// by the candidate record rather than by the bucket.
func (m *vwModel) toggleSelfStake(rng *rand.Rand) string {
	idx := m.pickActive(rng)
	if idx == 0 {
		return ""
	}
	b := m.buckets[idx]
	cand := m.candFor(b)
	if cand == nil {
		return ""
	}
	m.debit(b)
	if cand.SelfStakeBucketIdx == b.Index {
		cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	} else {
		// A candidate has at most one self-stake bucket.
		cand.SelfStakeBucketIdx = b.Index
	}
	m.credit(b)
	return fmt.Sprintf("toggleSelfStake idx=%d", idx)
}

func (m *vwModel) pickActive(rng *rand.Rand) uint64 {
	active := m.activeIndices()
	if len(active) == 0 {
		return 0
	}
	return active[rng.Intn(len(active))]
}

// TestVoterWeightIncrementalMatchesRebuild is the standing guard on the
// assumption IIP-59 rests on: that the incrementally maintained VoterWeightView
// and a from-scratch rebuild of it are the same value.
//
// Those two derivations disagreeing is not a recoverable condition — the digest
// committed into the state root is a hash, so neither derivation can be
// reconstructed from it, and a node that finds a mismatch at startup has no
// safe option but to stop. This test is where that disagreement is supposed to
// be caught: here, at its cause, instead of days later on a validator that
// happens to restart.
func TestVoterWeightIncrementalMatchesRebuild(t *testing.T) {
	r := require.New(t)

	type mutation struct {
		name string
		run  func(*vwModel, *rand.Rand) string
	}
	mutations := []mutation{
		{"createStake", (*vwModel).createStake},
		{"deposit", (*vwModel).deposit},
		{"restake", (*vwModel).restake},
		{"unstake", (*vwModel).unstake},
		{"withdraw", (*vwModel).withdraw},
		{"changeCandidate", (*vwModel).changeCandidate},
		{"transferOwnership", (*vwModel).transferOwnership},
		{"toggleSelfStake", (*vwModel).toggleSelfStake},
	}

	// Anomalies are silent no-ops in production so a bad delta cannot change
	// consensus; in tests they must fail the run at the operation that caused
	// them rather than surfacing later as a hash mismatch.
	prev := voterWeightAnomalyFatal
	voterWeightAnomalyFatal = true
	defer func() { voterWeightAnomalyFatal = prev }()

	for seed := int64(0); seed < 40; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			m := newVWModel(3)
			history := make([]string, 0, 200)

			// Seed a few buckets so the early mutations have something to act on.
			for i := 0; i < 5; i++ {
				history = append(history, m.createStake(rng))
			}

			for step := 0; step < 200; step++ {
				mut := mutations[rng.Intn(len(mutations))]
				if desc := mut.run(m, rng); desc != "" {
					history = append(history, desc)
				}

				r.Equalf(
					m.rebuild().Hash(), m.incr.Hash(),
					"incremental view diverged from rebuild after step %d (%s)\nhistory:\n  %v",
					step, mut.name, history[max(0, len(history)-12):],
				)
			}

			// The per-candidate contents, not just the digest, must line up.
			rebuilt := m.rebuild()
			for _, cand := range m.candList() {
				id := hash.BytesToHash160(cand.GetIdentifier().Bytes())
				r.Equal(
					rebuilt.VoterWeightsByCandidate(id),
					m.incr.VoterWeightsByCandidate(id),
					"per-voter weights differ for candidate %s", cand.GetIdentifier().String(),
				)
			}
		})
	}
}

// TestVoterWeightPersistRoundTrip is the invariant that lets the startup halt
// go away: the per-block incremental writes accumulate to exactly the state a
// fresh process loads back.
//
// Each step writes only the (candidate, voter) pairs that step touched, the way
// a real block does. If that bookkeeping ever misses a pair — or writes one it
// should have deleted — the loaded view drifts from the live one, and the
// digest that used to catch it at restart is gone. This is what catches it now,
// and it catches it at the step that caused it.
func TestVoterWeightPersistRoundTrip(t *testing.T) {
	r := require.New(t)

	mutations := []func(*vwModel, *rand.Rand) string{
		(*vwModel).createStake,
		(*vwModel).deposit,
		(*vwModel).restake,
		(*vwModel).unstake,
		(*vwModel).withdraw,
		(*vwModel).changeCandidate,
		(*vwModel).transferOwnership,
		(*vwModel).toggleSelfStake,
	}

	prev := voterWeightAnomalyFatal
	voterWeightAnomalyFatal = true
	defer func() { voterWeightAnomalyFatal = prev }()

	for seed := int64(0); seed < 10; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("seed_%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			m := newVWModel(3)
			store := newFakeVoterWeightStore()
			store.markSeedComplete()
			base := m.incr.(*voterWeightBase)

			commit := func() {
				r.NoError(base.persist(store))
				base.dirty = false
				base.touched = nil
			}

			for i := 0; i < 5; i++ {
				m.createStake(rng)
			}
			commit()

			for step := 0; step < 120; step++ {
				mutations[rng.Intn(len(mutations))](m, rng)
				commit()

				loaded, err := loadVoterWeightView(store.reader(t), nil, nil, m.consts)
				r.NoErrorf(err, "load failed after step %d", step)
				r.Equalf(m.incr.Hash(), loaded.Hash(),
					"persisted state diverged from the live view after step %d", step)
			}
		})
	}
}

// TestVoterWeightZeroCrossing pins the asymmetry the two derivations are most
// likely to differ on: the rebuild skips a bucket whose weight is zero and one
// that is unstaked, while the incremental path reaches the same state by
// subtracting down to zero and removing the entry. Both must land on an
// identical map, not merely an equal total.
func TestVoterWeightZeroCrossing(t *testing.T) {
	r := require.New(t)
	m := newVWModel(1)
	cand := m.candList()[0]
	candKey := hash.BytesToHash160(cand.GetIdentifier().Bytes())

	b := NewVoteBucket(cand.GetIdentifier(), identityset.Address(3), big.NewInt(100), 7, m.clock, false)
	b.Index = m.nextIndex
	m.nextIndex++
	m.buckets[b.Index] = b
	m.credit(b)
	r.Equal(m.rebuild().Hash(), m.incr.Hash())
	r.Len(m.incr.VoterWeightsByCandidate(candKey), 1)

	// Drive the voter's only weight to exactly zero.
	m.debit(b)
	b.UnstakeStartTime = b.StakeStartTime.Add(time.Hour)

	r.Empty(m.incr.VoterWeightsByCandidate(candKey), "voter entry must be removed, not left at zero")
	r.Equal(m.rebuild().Hash(), m.incr.Hash())
	r.Equal(hash.ZeroHash256, m.incr.Hash(), "an empty view must hash to zero on both paths")

	// Re-staking the same voter must reproduce the original state exactly.
	b2 := NewVoteBucket(cand.GetIdentifier(), identityset.Address(3), big.NewInt(100), 7, m.clock, false)
	b2.Index = m.nextIndex
	m.nextIndex++
	m.buckets[b2.Index] = b2
	m.credit(b2)
	r.Equal(m.rebuild().Hash(), m.incr.Hash())
}
