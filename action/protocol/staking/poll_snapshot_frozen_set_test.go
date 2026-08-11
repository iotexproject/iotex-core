// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// These tests pin WHICH candidates FreezePollSnapshot writes.
//
// The rule is: every opted-in candidate in the candidate center, and nothing
// else. It has no relationship to the poll list, which is where this function
// started -- the poll list is filtered by isActiveCandidate and by a
// vote-score threshold, and it is frozen once per reward era while the paid
// set is recomputed every epoch inside that era, so the two drift. A candidate
// that was opted in but off the list got no snapshot at all, and every reader
// reads absence as "not on the rails": 100% delegate / 0% voter, silently, for
// the rest of the era.
//
// Sourcing the set from the opt-in bit instead closes that by construction.
// TestFreezePollSnapshot_UnrankedOptedInCandidateIsFrozen is the regression
// pin for the original defect.

// rawPollSnapshotBytes returns the serialized blob exactly as it sits in the
// trie. Comparing these, rather than the decoded struct, is what makes the
// isolation assertion a byte-identity assertion.
func rawPollSnapshotBytes(t *testing.T, sm protocol.StateManager, candID address.Address) []byte {
	t.Helper()
	snapshot := &CandidatePollSnapshot{}
	_, err := sm.State(snapshot,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidatePollSnapshotKey(candID)))
	require.NoError(t, err)
	raw, err := snapshot.Serialize()
	require.NoError(t, err)
	return raw
}

func freezeCtxAt(height uint64) context.Context {
	return protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: height})
}

// TestFreezePollSnapshot_RecordIsIndependentOfSetSize asserts two things about
// one candidate's frozen record:
//
//   - isolation: adding a second opted-in candidate to the set does not change
//     one byte of the first one's snapshot, and
//   - absolute value: the bytes match a blob assembled by hand from the fields
//     the freezer is specified to write. That second half is what would catch a
//     change that moved every snapshot in the same direction, which the first
//     half alone cannot see.
func TestFreezePollSnapshot_RecordIsIndependentOfSetSize(t *testing.T) {
	r := require.New(t)
	const freezeHeight = uint64(1_234_567)

	// Run 1: a set of one.
	soloCtrl := gomock.NewController(t)
	soloSM := testdb.NewMockStateManager(soloCtrl)
	solo := onchainCandidate(1, "solo", big.NewInt(4_200))
	r.NoError(putOnchainCandidate(newCandidateStateManager(soloSM), solo))
	installCandCenter(t, soloSM, solo)
	r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), soloSM, nil, nil))
	soloBytes := rawPollSnapshotBytes(t, soloSM, solo.GetIdentifier())

	// Run 2: same candidate, same height, but the center now also holds a
	// second opted-in delegate.
	pairCtrl := gomock.NewController(t)
	pairSM := testdb.NewMockStateManager(pairCtrl)
	pairCSM := newCandidateStateManager(pairSM)
	first := onchainCandidate(1, "solo", big.NewInt(4_200))
	second := onchainCandidate(2, "other", big.NewInt(9_900))
	r.NoError(putOnchainCandidate(pairCSM, first))
	r.NoError(putOnchainCandidate(pairCSM, second))
	installCandCenter(t, pairSM, first, second)
	r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), pairSM, nil, nil))
	pairBytes := rawPollSnapshotBytes(t, pairSM, first.GetIdentifier())

	r.Equal(soloBytes, pairBytes,
		"another candidate joining the frozen set must not change one byte of this one's snapshot")

	// Absolute value: what the freezer is specified to write for an opted-in
	// candidate with no DelegateProfile bridge configured.
	expected := &CandidatePollSnapshot{
		BlockCommissionBasisPoints: _fullCommissionBasisPoints,
		EpochCommissionBasisPoints: _fullCommissionBasisPoints,
		Registered:                 false,
		OnchainRewardEnabled:       true,
		TotalWeight:                big.NewInt(4_200),
		FreezeHeight:               freezeHeight,
		SelfStakeBucketIdx:         solo.SelfStakeBucketIdx,
	}
	expectedBytes, err := expected.Serialize()
	r.NoError(err)
	r.Equal(expectedBytes, soloBytes,
		"frozen snapshot drifted from the specified field set")
}

// TestFreezePollSnapshot_UnrankedOptedInCandidateIsFrozen is the defect this
// design exists for, kept as a regression pin now that the poll list is gone
// from the signature: a candidate's presence in the frozen set must depend on
// nothing but its opt-in bit -- not on vote rank, not on activity, not on
// having been in whatever list happened to ride the boundary block.
func TestFreezePollSnapshot_UnrankedOptedInCandidateIsFrozen(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	const freezeHeight = uint64(777_000)

	ranked := onchainCandidate(1, "ranked", big.NewInt(100))
	// Everything the old poll list would have filtered on: far fewer votes,
	// and no self-stake, which is what isActiveCandidate rejects.
	unranked := onchainCandidate(2, "unranked", big.NewInt(555))
	unranked.SelfStake = big.NewInt(0)
	unranked.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
	r.NoError(putOnchainCandidate(csm, ranked))
	r.NoError(putOnchainCandidate(csm, unranked))
	installCandCenter(t, sm, ranked, unranked)

	r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), sm, nil, nil))

	snap, err := PollSnapshotFor(sm, unranked.GetIdentifier())
	r.NoError(err, "an opted-in candidate must be frozen regardless of rank or activity")
	r.True(snap.OnchainRewardEnabled)
	r.Equal(freezeHeight, snap.FreezeHeight)
	r.Equal(unranked.SelfStakeBucketIdx, snap.SelfStakeBucketIdx)
	r.Zero(big.NewInt(555).Cmp(snap.TotalWeight),
		"the denominator must come from the candidate center, the same source every member uses")
	r.Equal(_fullCommissionBasisPoints, snap.EpochCommissionBasisPoints)
}

// TestFreezePollSnapshot_OptedOutCandidateNotFrozen is the other side of the
// membership rule. It also fixes the shape of what rewarding sees for an
// opted-out delegate: nothing at all, rather than the zeroed placeholder this
// used to write for opted-out poll members. The two are consensus-equivalent
// -- voter_reward.go maps both to onchainRewardEnabled=false -- so the
// placeholder was a state write per candidate per era for no reader.
func TestFreezePollSnapshot_OptedOutCandidateNotFrozen(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	optedIn := onchainCandidate(1, "opted-in", big.NewInt(100))
	r.NoError(putOnchainCandidate(csm, optedIn))
	// Not opted in: the persisted opt-in bit is false.
	optedOut := onchainCandidate(2, "opted-out", big.NewInt(500))
	optedOut.VoterRewardOnchainOptIn = false
	r.NoError(csm.putCandidate(optedOut))
	installCandCenter(t, sm, optedIn, optedOut)

	r.NoError(FreezePollSnapshot(freezeCtxAt(4_242), sm, nil, nil))

	_, err := PollSnapshotFor(sm, optedOut.GetIdentifier())
	r.ErrorIs(err, state.ErrStateNotExist)

	_, err = PollSnapshotFor(sm, optedIn.GetIdentifier())
	r.NoError(err)
}

// TestFreezePollSnapshot_FrozenSetOrderIsDeterministic guards the ordering
// rule: the candidate center enumerates from a Go map, and the order it hands
// back reaches both PutState and the DelegateProfile bridge call. Iterating it
// unsorted would make the freeze non-deterministic across nodes.
func TestFreezePollSnapshot_FrozenSetOrderIsDeterministic(t *testing.T) {
	r := require.New(t)
	const freezeHeight = uint64(31_337)

	var reference [][]byte
	for run := 0; run < 8; run++ {
		ctrl := gomock.NewController(t)
		sm := testdb.NewMockStateManager(ctrl)
		csm := newCandidateStateManager(sm)
		cands := make([]*Candidate, 0, 6)
		for i := 1; i <= 6; i++ {
			c := onchainCandidate(i, string(rune('a'+i)), big.NewInt(int64(i*11)))
			r.NoError(putOnchainCandidate(csm, c))
			cands = append(cands, c)
		}
		installCandCenter(t, sm, cands...)
		r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), sm, nil, nil))

		got := make([][]byte, 0, len(cands))
		for _, c := range cands {
			got = append(got, rawPollSnapshotBytes(t, sm, c.GetIdentifier()))
		}
		if reference == nil {
			reference = got
			continue
		}
		r.Equal(reference, got, "freeze must be identical across runs")
	}
}
