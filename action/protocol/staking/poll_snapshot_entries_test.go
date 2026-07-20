// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"bytes"
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// The freezer's Entries population is the whole point of PR 5.5a: without
// it, splitDelegateEpochReward falls into the "no voters known" branch every
// epoch on every delegate and 100% of the epoch reward routes to delegate
// commission. These tests wire an in-memory VoterWeightView, populate it
// directly, then run FreezePollSnapshot and assert the per-candidate blob
// carries the expected per-voter aggregate.

func TestFreezePollSnapshot_Entries_NativeVoters(t *testing.T) {
	// One candidate, three voters — Entries must come out sorted by voter
	// bytes (VoterWeightView invariant) and each weight must round-trip
	// through the blob without loss.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner:                   identityset.Address(1),
		Operator:                identityset.Address(1),
		Reward:                  identityset.Address(1),
		Name:                    "delegate-a",
		Votes:                   big.NewInt(1),
		SelfStake:               big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	r.NoError(csm.putCandidate(cand))

	view := NewVoterWeightView()
	candHash := hash.BytesToHash160(cand.GetIdentifier().Bytes())
	weightByVoter := map[int]*big.Int{
		4: big.NewInt(3_000),
		2: big.NewInt(5_000),
		9: big.NewInt(1_234),
	}
	for idx, w := range weightByVoter {
		view.Apply(candHash, identityset.Address(idx), w)
	}
	r.NoError(sm.WriteView(_protocolID, &viewData{voterWeights: view}))

	list := state.CandidateList{
		&state.Candidate{
			Address:       cand.Owner.String(),
			Votes:         big.NewInt(1),
			RewardAddress: cand.Reward.String(),
		},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Len(snap.Entries, 3)
	assertEntriesSortedByVoterBytes(t, snap.Entries)
	for _, e := range snap.Entries {
		var matched *big.Int
		for idx, w := range weightByVoter {
			if address.Equal(identityset.Address(idx), e.Voter) {
				matched = w
				break
			}
		}
		r.NotNil(matched, "voter %s must appear in seed set", e.Voter.String())
		r.Zero(matched.Cmp(e.Weight), "voter %s weight mismatch", e.Voter.String())
	}
}

func TestFreezePollSnapshot_Entries_MultipleCandidatesIsolated(t *testing.T) {
	// Two candidates with disjoint voter sets — freezer must not leak
	// voters across candidates and each candidate's list must be
	// independently sorted.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	candA := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(1), Reward: identityset.Address(1),
		Name: "A", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	candB := &Candidate{
		Owner: identityset.Address(2), Operator: identityset.Address(2), Reward: identityset.Address(2),
		Name: "B", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(csm.putCandidate(candA))
	r.NoError(csm.putCandidate(candB))

	view := NewVoterWeightView()
	view.Apply(hash.BytesToHash160(candA.GetIdentifier().Bytes()), identityset.Address(10), big.NewInt(100))
	view.Apply(hash.BytesToHash160(candA.GetIdentifier().Bytes()), identityset.Address(11), big.NewInt(200))
	view.Apply(hash.BytesToHash160(candB.GetIdentifier().Bytes()), identityset.Address(12), big.NewInt(300))
	r.NoError(sm.WriteView(_protocolID, &viewData{voterWeights: view}))

	list := state.CandidateList{
		&state.Candidate{Address: candA.Owner.String(), Votes: big.NewInt(1), RewardAddress: candA.Reward.String()},
		&state.Candidate{Address: candB.Owner.String(), Votes: big.NewInt(1), RewardAddress: candB.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))

	snapA, err := PollSnapshotFor(sm, candA.Owner)
	r.NoError(err)
	r.Len(snapA.Entries, 2)
	assertEntriesSortedByVoterBytes(t, snapA.Entries)
	for _, e := range snapA.Entries {
		r.False(address.Equal(identityset.Address(12), e.Voter),
			"candA snapshot must not contain candB's voter")
	}

	snapB, err := PollSnapshotFor(sm, candB.Owner)
	r.NoError(err)
	r.Len(snapB.Entries, 1)
	r.True(address.Equal(identityset.Address(12), snapB.Entries[0].Voter))
	r.Zero(big.NewInt(300).Cmp(snapB.Entries[0].Weight))
}

func TestFreezePollSnapshot_Entries_CandidateWithNoVoters(t *testing.T) {
	// The delegate is on the poll list but the view has no per-voter data
	// for them (fresh delegate, all buckets unstaked, etc.) — snapshot
	// must be written with Entries left nil so rewarding's degenerate
	// branch still fires.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(1), Reward: identityset.Address(1),
		Name: "empty", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(csm.putCandidate(cand))

	r.NoError(sm.WriteView(_protocolID, &viewData{voterWeights: NewVoterWeightView()}))

	list := state.CandidateList{
		&state.Candidate{Address: cand.Owner.String(), Votes: big.NewInt(1), RewardAddress: cand.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Empty(snap.Entries)
}

func TestFreezePollSnapshot_Entries_ViewMissingDegrades(t *testing.T) {
	// No viewData installed at all: voterWeightsFromSM returns nil, the
	// freezer degrades gracefully to empty Entries, and the block still
	// gets snapshots (Registered=false, opt-in captured). Regression
	// guard for tests that pre-date Protocol.Start.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(1), Reward: identityset.Address(1),
		Name: "pre-fork", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
		VoterRewardOnchainOptIn: true,
	}
	r.NoError(csm.putCandidate(cand))

	list := state.CandidateList{
		&state.Candidate{Address: cand.Owner.String(), Votes: big.NewInt(1), RewardAddress: cand.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Empty(snap.Entries)
	r.True(snap.VoterRewardOnchainOptIn, "opt-in still captured even without view")
}

func TestFreezePollSnapshot_Entries_DeterministicOrder(t *testing.T) {
	// Freezing the same view twice must produce byte-identical Entries
	// order. Downstream distribution iterates Entries directly and emits
	// receipt logs in that order, so any nondeterminism here forks the
	// chain immediately.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(1), Reward: identityset.Address(1),
		Name: "det", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(csm.putCandidate(cand))

	view := NewVoterWeightView()
	candHash := hash.BytesToHash160(cand.GetIdentifier().Bytes())
	// Insert in scrambled order — a naive iteration over a Go map would
	// return them in a different order across runs.
	for _, idx := range []int{7, 3, 12, 2, 9, 15, 4} {
		view.Apply(candHash, identityset.Address(idx), big.NewInt(int64(1000+idx)))
	}
	r.NoError(sm.WriteView(_protocolID, &viewData{voterWeights: view}))

	list := state.CandidateList{
		&state.Candidate{Address: cand.Owner.String(), Votes: big.NewInt(1), RewardAddress: cand.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))

	first, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	assertEntriesSortedByVoterBytes(t, first.Entries)

	// Re-freeze; a re-run against the same view must produce identical
	// order + weights.
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))
	second, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Equal(len(first.Entries), len(second.Entries))
	for i := range first.Entries {
		r.True(address.Equal(first.Entries[i].Voter, second.Entries[i].Voter),
			"index %d voter must match across freezes", i)
		r.Zero(first.Entries[i].Weight.Cmp(second.Entries[i].Weight))
	}
}

func TestFreezePollSnapshot_Entries_WeightCloneIsolation(t *testing.T) {
	// The freezer must copy each *big.Int weight into the snapshot rather
	// than share the pointer — otherwise a later view.Apply that mutates
	// the aggregate would retroactively edit the frozen blob's in-memory
	// representation before it's re-read from state.
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := &Candidate{
		Owner: identityset.Address(1), Operator: identityset.Address(1), Reward: identityset.Address(1),
		Name: "clone", Votes: big.NewInt(1), SelfStake: big.NewInt(1),
	}
	r.NoError(csm.putCandidate(cand))

	view := NewVoterWeightView()
	candHash := hash.BytesToHash160(cand.GetIdentifier().Bytes())
	voter := identityset.Address(5)
	view.Apply(candHash, voter, big.NewInt(1_000))
	r.NoError(sm.WriteView(_protocolID, &viewData{voterWeights: view}))

	list := state.CandidateList{
		&state.Candidate{Address: cand.Owner.String(), Votes: big.NewInt(1), RewardAddress: cand.Reward.String()},
	}
	r.NoError(FreezePollSnapshot(context.Background(), sm, list, nil, nil))

	// Mutate the live view AFTER the freeze.
	view.Apply(candHash, voter, big.NewInt(-500))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Len(snap.Entries, 1)
	r.Equal(int64(1_000), snap.Entries[0].Weight.Int64(),
		"post-freeze Apply must not affect the persisted blob")
}

// assertEntriesSortedByVoterBytes fails the test if the entries are not in
// strict ascending voter-bytes order — the invariant VoterWeightView promises
// to the freezer.
func assertEntriesSortedByVoterBytes(t *testing.T, entries []VoterWeight) {
	t.Helper()
	for i := 1; i < len(entries); i++ {
		if bytes.Compare(entries[i-1].Voter.Bytes(), entries[i].Voter.Bytes()) >= 0 {
			t.Fatalf("entries not sorted by voter bytes at index %d (prev=%s cur=%s)",
				i, entries[i-1].Voter.String(), entries[i].Voter.String())
		}
	}
}
