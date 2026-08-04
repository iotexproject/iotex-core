// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// These tests are the migrated successors of the old
// TestFreezePollSnapshot_Entries_* suite (poll_snapshot_entries_test.go).
// The freezer no longer materializes a per-voter list; it freezes a single
// scalar, TotalWeight, taken from the candidate record's own Votes
// accumulator at the boundary height H. Everything the old suite asserted
// about the list (presence, per-candidate isolation, degenerate empties,
// determinism, no aliasing of live state) still has to hold for that scalar,
// so each old case has a counterpart below.
//
// The denominator matters: splitDelegateEpochReward divides the voter pool by
// TotalWeight, so if this drifts from what the voter-major drain recomputes
// per voter (staking.FrozenVoterWeight), payouts either over- or under-shoot
// the pool. TestVoterWeightInvariant pins the other side of that equality
// (candidate.Votes == sum of per-voter weights).

// installCandCenter puts the candidates into a candidate center and writes it
// into the view, which is how FreezePollSnapshot reaches candidate.Votes.
// Without a candCenter in the view the freezer has no candidate reader and
// deliberately degrades to TotalWeight=0.
func installCandCenter(t *testing.T, sm protocol.StateManager, cands ...*Candidate) {
	t.Helper()
	center, err := NewCandidateCenter(nil)
	require.NoError(t, err)
	for _, c := range cands {
		require.NoError(t, center.Upsert(c))
	}
	require.NoError(t, sm.WriteView(_protocolID, &viewData{
		candCenter: center,
	}))
}

func pollListOf(cands ...*Candidate) state.CandidateList {
	list := make(state.CandidateList, 0, len(cands))
	for _, c := range cands {
		list = append(list, &state.Candidate{
			Address:       c.Owner.String(),
			Votes:         big.NewInt(1),
			RewardAddress: c.Reward.String(),
		})
	}
	return list
}

func onchainCandidate(idx int, name string, votes *big.Int) *Candidate {
	addr := identityset.Address(idx)
	return &Candidate{
		Owner:    addr,
		Operator: addr,
		Reward:   addr,
		Name:     name,
		Votes:    votes,
		// Distinct per candidate: the candidate center rejects two
		// candidates claiming the same self-stake bucket.
		SelfStakeBucketIdx: uint64(idx),
		SelfStake:          big.NewInt(1),
	}
}

// TestFreezePollSnapshot_TotalWeightIsCandidateVotes is the boundary
// assertion the redesign turns on: the frozen denominator equals the
// candidate's Votes at height H, exactly, with no rounding or re-derivation.
func TestFreezePollSnapshot_TotalWeightIsCandidateVotes(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	votes, ok := new(big.Int).SetString("9223372036854775809000000000000000001", 10)
	r.True(ok)
	cand := onchainCandidate(1, "delegate-a", votes)
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.NotNil(snap.TotalWeight)
	r.Zero(votes.Cmp(snap.TotalWeight),
		"frozen TotalWeight must equal candidate.Votes at the boundary height")
	// Live record must survive the freeze untouched.
	r.Zero(votes.Cmp(csmCandidateVotes(t, sm, cand)))
}

// TestFreezePollSnapshot_TotalWeightMultipleCandidatesIsolated is the migrated
// TestFreezePollSnapshot_Entries_MultipleCandidatesIsolated: no cross-candidate
// leakage, each delegate frozen against its own Votes.
func TestFreezePollSnapshot_TotalWeightMultipleCandidatesIsolated(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	candA := onchainCandidate(1, "A", big.NewInt(300))
	candB := onchainCandidate(2, "B", big.NewInt(700))
	r.NoError(putOnchainCandidate(csm, candA))
	r.NoError(putOnchainCandidate(csm, candB))
	installCandCenter(t, sm, candA, candB)

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(candA, candB), nil, nil))

	snapA, err := PollSnapshotFor(sm, candA.Owner)
	r.NoError(err)
	r.Equal(int64(300), snapA.TotalWeight.Int64())

	snapB, err := PollSnapshotFor(sm, candB.Owner)
	r.NoError(err)
	r.Equal(int64(700), snapB.TotalWeight.Int64())

	r.NotEqual(snapA.SnapshotHash, snapB.SnapshotHash,
		"two delegates in the same era must get distinct snapshot hashes")
}

// TestFreezePollSnapshot_TotalWeightZeroVotes is the migrated
// TestFreezePollSnapshot_Entries_CandidateWithNoVoters: a delegate on the poll
// list with nothing staked to it freezes at zero, which rewarding reads as
// "no payable voter set this era".
func TestFreezePollSnapshot_TotalWeightZeroVotes(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := onchainCandidate(1, "empty", big.NewInt(0))
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.NotNil(snap.TotalWeight, "zero must round-trip as a zero big.Int, never nil")
	r.Zero(snap.TotalWeight.Sign())
}

// TestFreezePollSnapshot_TotalWeightViewMissingDegrades is the migrated
// TestFreezePollSnapshot_Entries_ViewMissingDegrades: no view installed at
// all (pre-fork fixtures, tests that predate Protocol.Start). The boundary
// must still write a record, degraded to zero weight, on every validator
// identically.
func TestFreezePollSnapshot_TotalWeightViewMissingDegrades(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := onchainCandidate(1, "pre-fork", big.NewInt(555))
	r.NoError(putOnchainCandidate(csm, cand))
	// deliberately no installCandCenter

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.NotNil(snap.TotalWeight)
	r.Zero(snap.TotalWeight.Sign())
	r.False(snap.Registered)
	r.Equal(uint64(candidateNoSelfStakeBucketIndex), snap.SelfStakeBucketIdx)
}

// TestFreezePollSnapshot_SnapshotHashDeterministic is the migrated
// TestFreezePollSnapshot_Entries_DeterministicOrder. The old suite guarded
// map-iteration nondeterminism in the entry list; the hash is now the only
// derived value the boundary produces, and two freezes of identical state
// must yield identical bytes or validators fork.
func TestFreezePollSnapshot_SnapshotHashDeterministic(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	cand := onchainCandidate(1, "det", big.NewInt(1_234_567))
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))
	first, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.NotEqual(hash.ZeroHash256, first.SnapshotHash)

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))
	second, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Equal(first.SnapshotHash, second.SnapshotHash)
	r.Zero(first.TotalWeight.Cmp(second.TotalWeight))

	// The hash must actually commit to TotalWeight, otherwise it is useless
	// as a per-delegate-per-era identifier for off-chain log assembly.
	cand.Votes = big.NewInt(999)
	installCandCenter(t, sm, cand)
	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))
	third, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.NotEqual(first.SnapshotHash, third.SnapshotHash)
}

// TestFreezePollSnapshot_VotesCloneIsolation is the migrated
// TestFreezePollSnapshot_Entries_WeightCloneIsolation: the freezer must copy
// Votes, not alias it. Votes keeps moving for the rest of the era as buckets
// are created and unstaked; an aliased frozen denominator would silently
// change the divisor mid-era.
func TestFreezePollSnapshot_VotesCloneIsolation(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	votes := big.NewInt(1_000)
	cand := onchainCandidate(1, "clone", votes)
	r.NoError(putOnchainCandidate(csm, cand))
	installCandCenter(t, sm, cand)

	r.NoError(FreezePollSnapshot(context.Background(), sm, pollListOf(cand), nil, nil))

	// Mutate the same *big.Int the candidate record carries, in place —
	// this is what a post-freeze stake change looks like to an aliasing bug.
	votes.SetInt64(1)

	snap, err := PollSnapshotFor(sm, cand.Owner)
	r.NoError(err)
	r.Equal(int64(1_000), snap.TotalWeight.Int64(),
		"post-freeze Votes movement must not affect the frozen denominator")
}

// csmCandidateVotes re-reads the persisted candidate record's Votes.
func csmCandidateVotes(t *testing.T, sm protocol.StateManager, cand *Candidate) *big.Int {
	t.Helper()
	var got Candidate
	_, err := sm.State(&got,
		protocol.NamespaceOption(_candidateNameSpace),
		protocol.KeyOption(cand.GetIdentifier().Bytes()))
	require.NoError(t, err)
	return got.Votes
}
