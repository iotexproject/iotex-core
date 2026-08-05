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

// The set FreezePollSnapshot writes is the poll list UNION the live opted-in
// set. The poll list alone is not the set that gets paid: it is filtered by
// isActiveCandidate and by the vote-score threshold, and it is frozen once per
// reward era while the paid set is recomputed every epoch inside that era.
//
// These tests pin the two halves of that: the union actually widens (a
// candidate opted in but absent from the poll list gets a snapshot), and the
// widening is PURELY ADDITIVE (a candidate already in the frozen set gets a
// byte-for-byte identical snapshot).

// rawPollSnapshotBytes returns the serialized blob exactly as it sits in the
// trie. Comparing these, rather than the decoded struct, is what makes the
// additivity assertion a byte-identity assertion.
func rawPollSnapshotBytes(t *testing.T, sm protocol.StateManager, candID address.Address) []byte {
	t.Helper()
	blob := &candidatePollSnapshotBlob{}
	_, err := sm.State(blob,
		protocol.NamespaceOption(_stakingNameSpace),
		protocol.KeyOption(candidatePollSnapshotKey(candID)))
	require.NoError(t, err)
	raw, err := blob.Serialize()
	require.NoError(t, err)
	return raw
}

func freezeCtxAt(height uint64) context.Context {
	return protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: height})
}

// TestFreezePollSnapshot_WideningIsByteIdentical is the non-negotiable
// invariant: for a candidate that was already in the frozen set, the snapshot
// written after the union must be identical to the one written before it, down
// to the serialized bytes.
//
// Both halves are asserted:
//
//   - additivity, by freezing the same poll list twice against a candidate
//     center that does and does not carry the extra opted-in candidate, and
//   - absolute value, against a blob assembled by hand from the fields the
//     freezer is specified to write. That second half is what would catch a
//     change that moved every snapshot in the same direction, which the first
//     half alone cannot see.
func TestFreezePollSnapshot_WideningIsByteIdentical(t *testing.T) {
	r := require.New(t)
	const freezeHeight = uint64(1_234_567)

	// Run 1: the candidate center holds only the poll-list delegate.
	narrowCtrl := gomock.NewController(t)
	narrowSM := testdb.NewMockStateManager(narrowCtrl)
	inPoll := onchainCandidate(1, "in-poll", big.NewInt(4_200))
	r.NoError(putOnchainCandidate(newCandidateStateManager(narrowSM), inPoll))
	installCandCenter(t, narrowSM, inPoll)
	r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), narrowSM, pollListOf(inPoll), nil, nil))
	narrowBytes := rawPollSnapshotBytes(t, narrowSM, inPoll.GetIdentifier())

	// Run 2: same poll list, same delegate, but the center now also holds an
	// opted-in delegate the poll list never mentions — exactly the case the
	// union exists for.
	wideCtrl := gomock.NewController(t)
	wideSM := testdb.NewMockStateManager(wideCtrl)
	wideCSM := newCandidateStateManager(wideSM)
	inPoll2 := onchainCandidate(1, "in-poll", big.NewInt(4_200))
	offPoll := onchainCandidate(2, "off-poll", big.NewInt(9_900))
	r.NoError(putOnchainCandidate(wideCSM, inPoll2))
	r.NoError(putOnchainCandidate(wideCSM, offPoll))
	installCandCenter(t, wideSM, inPoll2, offPoll)
	r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), wideSM, pollListOf(inPoll2), nil, nil))
	wideBytes := rawPollSnapshotBytes(t, wideSM, inPoll2.GetIdentifier())

	r.Equal(narrowBytes, wideBytes,
		"widening the frozen set must not change one byte of an existing candidate's snapshot")

	// Absolute value: what the freezer is specified to write for an opted-in
	// candidate with no DelegateProfile bridge configured.
	expected := &CandidatePollSnapshot{
		BlockCommissionBasisPoints: _fullCommissionBasisPoints,
		EpochCommissionBasisPoints: _fullCommissionBasisPoints,
		Registered:                 false,
		OnchainRewardEnabled:       true,
		TotalWeight:                big.NewInt(4_200),
		FreezeHeight:               freezeHeight,
		SelfStakeBucketIdx:         inPoll.SelfStakeBucketIdx,
	}
	expected.SnapshotHash = eraSnapshotHash(inPoll.GetIdentifier(), expected)
	expectedBytes, err := expected.toBlob().Serialize()
	r.NoError(err)
	r.Equal(expectedBytes, narrowBytes,
		"frozen snapshot drifted from the specified field set")
}

// TestFreezePollSnapshot_OptedInCandidateAbsentFromPollList is the defect this
// change exists for. Before the union, this candidate got no snapshot at all,
// and every reader read that absence as "no frozen era": the commission split
// silently fell back to 100% delegate / 0% voter for the rest of the era.
func TestFreezePollSnapshot_OptedInCandidateAbsentFromPollList(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)
	const freezeHeight = uint64(777_000)

	inPoll := onchainCandidate(1, "in-poll", big.NewInt(100))
	offPoll := onchainCandidate(2, "off-poll", big.NewInt(555))
	r.NoError(putOnchainCandidate(csm, inPoll))
	r.NoError(putOnchainCandidate(csm, offPoll))
	installCandCenter(t, sm, inPoll, offPoll)

	// The poll list carries only the first delegate.
	r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), sm, pollListOf(inPoll), nil, nil))

	snap, err := PollSnapshotFor(sm, offPoll.GetIdentifier())
	r.NoError(err, "an opted-in candidate absent from the poll list must still be frozen")
	r.True(snap.OnchainRewardEnabled)
	r.Equal(freezeHeight, snap.FreezeHeight)
	r.Equal(offPoll.SelfStakeBucketIdx, snap.SelfStakeBucketIdx)
	r.Zero(big.NewInt(555).Cmp(snap.TotalWeight),
		"the widened candidate's denominator must come from the candidate center, "+
			"the same source the poll-list candidates use")
	r.Equal(_fullCommissionBasisPoints, snap.EpochCommissionBasisPoints)
}

// TestFreezePollSnapshot_OptedOutCandidateNotWidenedIn keeps the union from
// becoming "freeze everything": a candidate that never opted in is not paid
// through the protocol-native path, so freezing it would write a placeholder
// per candidate per era for no reader.
func TestFreezePollSnapshot_OptedOutCandidateNotWidenedIn(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	csm := newCandidateStateManager(sm)

	inPoll := onchainCandidate(1, "in-poll", big.NewInt(100))
	r.NoError(putOnchainCandidate(csm, inPoll))
	// Not opted in: no explicit opt-in bit and a reward address that is not a
	// configured Hermes vault.
	optedOut := onchainCandidate(2, "opted-out", big.NewInt(500))
	optedOut.VoterRewardOnchainOptIn = false
	r.NoError(csm.putCandidate(optedOut))
	installCandCenter(t, sm, inPoll, optedOut)

	r.NoError(FreezePollSnapshot(freezeCtxAt(4_242), sm, pollListOf(inPoll), nil, nil))

	_, err := PollSnapshotFor(sm, optedOut.GetIdentifier())
	r.ErrorIs(err, state.ErrStateNotExist)
}

// TestFreezePollSnapshot_WideningIsDeterministic guards the ordering rule: the
// candidate center enumerates from a Go map, and the order it hands back
// reaches both PutState and the DelegateProfile bridge call. Iterating it
// unsorted would make the freeze non-deterministic across nodes.
func TestFreezePollSnapshot_WideningIsDeterministic(t *testing.T) {
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
		// Only the first is in the poll list; the other five come in through
		// the union.
		r.NoError(FreezePollSnapshot(freezeCtxAt(freezeHeight), sm, pollListOf(cands[0]), nil, nil))

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
