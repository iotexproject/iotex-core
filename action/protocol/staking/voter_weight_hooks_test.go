// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// applyVoterWeightDelta is the single funnel every mutating staking handler
// calls to keep the IIP-59 VoterWeightView in sync. These tests pin the
// no-op semantics for arguments that upstream call sites are allowed to hand
// in (nil, zero delta, missing view) without a pre-check — regressions here
// silently drop bucket-weight updates and are only caught much later at the
// view-hash verification on restart.

func TestApplyVoterWeightDelta_NilCSMIsNoop(t *testing.T) {
	require.NotPanics(t, func() {
		applyVoterWeightDelta(nil, identityset.Address(1), identityset.Address(2), big.NewInt(100))
	})
}

func TestApplyVoterWeightDelta_NilCandIdentifierIsNoop(t *testing.T) {
	csm, view := newCSMWithVoterWeightView(t)
	applyVoterWeightDelta(csm, nil, identityset.Address(2), big.NewInt(100))
	require.Equal(t, hash.ZeroHash256, view.Hash(), "view must stay untouched when candIdentifier is nil")
}

func TestApplyVoterWeightDelta_NilVoterIsNoop(t *testing.T) {
	csm, view := newCSMWithVoterWeightView(t)
	applyVoterWeightDelta(csm, identityset.Address(1), nil, big.NewInt(100))
	require.Equal(t, hash.ZeroHash256, view.Hash())
}

func TestApplyVoterWeightDelta_NilDeltaIsNoop(t *testing.T) {
	csm, view := newCSMWithVoterWeightView(t)
	applyVoterWeightDelta(csm, identityset.Address(1), identityset.Address(2), nil)
	require.Equal(t, hash.ZeroHash256, view.Hash())
}

func TestApplyVoterWeightDelta_ZeroDeltaIsNoop(t *testing.T) {
	csm, view := newCSMWithVoterWeightView(t)
	applyVoterWeightDelta(csm, identityset.Address(1), identityset.Address(2), big.NewInt(0))
	require.Equal(t, hash.ZeroHash256, view.Hash())
}

func TestApplyVoterWeightDelta_ViewNotInstalledIsNoop(t *testing.T) {
	// A csm whose viewData has voterWeights=nil (e.g. tests that skip the
	// Protocol.Start bootstrap) must not panic — the fork gate is enforced
	// by the presence/absence of voterWeights, not by callers.
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	require.NoError(t, sm.WriteView(_protocolID, &viewData{}))
	csm := newCandidateStateManager(sm)
	require.NotPanics(t, func() {
		applyVoterWeightDelta(csm, identityset.Address(1), identityset.Address(2), big.NewInt(100))
	})
}

func TestApplyVoterWeightDelta_PositiveDeltaFlows(t *testing.T) {
	r := require.New(t)
	csm, view := newCSMWithVoterWeightView(t)
	cand := identityset.Address(1)
	voter := identityset.Address(5)

	applyVoterWeightDelta(csm, cand, voter, big.NewInt(1_000))

	out := view.VoterWeightsByCandidate(hash.BytesToHash160(cand.Bytes()))
	r.Len(out, 1)
	r.Equal(int64(1_000), out[0].weight.Int64())
}

func TestApplyVoterWeightDelta_NegativeDeltaFlows(t *testing.T) {
	r := require.New(t)
	csm, view := newCSMWithVoterWeightView(t)
	cand := identityset.Address(1)
	voter := identityset.Address(5)

	applyVoterWeightDelta(csm, cand, voter, big.NewInt(1_000))
	applyVoterWeightDelta(csm, cand, voter, big.NewInt(-300))

	out := view.VoterWeightsByCandidate(hash.BytesToHash160(cand.Bytes()))
	r.Len(out, 1)
	r.Equal(int64(700), out[0].weight.Int64())
}

func TestApplyVoterWeightDelta_OverWithdrawClamps(t *testing.T) {
	// Withdrawing more than the voter currently holds must not push the
	// entry negative. This mirrors the handler-side invariant that bucket
	// state never allows over-withdrawal in production, but the view stays
	// safe if a hook fires with a stale weight.
	r := require.New(t)
	csm, view := newCSMWithVoterWeightView(t)
	cand := identityset.Address(1)
	voter := identityset.Address(5)

	applyVoterWeightDelta(csm, cand, voter, big.NewInt(100))
	applyVoterWeightDelta(csm, cand, voter, big.NewInt(-1_000))

	r.Empty(view.VoterWeightsByCandidate(hash.BytesToHash160(cand.Bytes())))
}

func TestApplyVoterWeightDelta_UnknownVoterWithdrawIsNoop(t *testing.T) {
	// Negative delta against a candidate the view has never seen: the view
	// simply ignores it (see voterWeightBase.Apply). Regression guard —
	// this used to accidentally seed the map with a zero entry.
	r := require.New(t)
	csm, view := newCSMWithVoterWeightView(t)
	applyVoterWeightDelta(csm, identityset.Address(1), identityset.Address(2), big.NewInt(-50))
	r.Equal(hash.ZeroHash256, view.Hash())
}

// newCSMWithVoterWeightView returns a mock-backed csm whose viewData carries
// a fresh VoterWeightView. Returns the underlying view so tests can assert on
// its post-condition without going through the DirtyView plumbing.
func newCSMWithVoterWeightView(t *testing.T) (CandidateStateManager, VoterWeightView) {
	t.Helper()
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)
	view := NewVoterWeightView()
	require.NoError(t, sm.WriteView(_protocolID, &viewData{voterWeights: view}))
	return newCandidateStateManager(sm), view
}
