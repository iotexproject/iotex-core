// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// errNodeLocal stands in for whatever a state read fails with when the failure
// is the node's and not the chain's: a cold trie node, a closed DB, an evicted
// working set. What matters is only that it is none of the shapes the chain
// agrees on.
var errNodeLocal = errors.New("simulated node-local read failure")

// failingBucketReader fails every lookup. The production SlotBucketReader
// returns an error only for a wiring fault -- registrant and bucket slots that
// read back zero or malformed are reported as (0, false, nil) instead -- so a
// non-nil error from this interface is by construction node-local.
type failingBucketReader struct{}

func (failingBucketReader) LookupBucket(address.Address) (uint64, bool, error) {
	return 0, false, errNodeLocal
}

// TestPayVoterCombinedPropagatesNodeLocalLookupFailure pins the halt half of
// the settle-vs-halt rule on the auto-deposit lookup.
//
// Degrading here is a fork, not a fallback. The lookup decides where the share
// goes: a reader that fails credits the unclaimed balance, a reader that
// succeeds compounds into the bucket and moves candidate votes and the bucket
// pool with it. Whether a given node can serve that read is node-local, so the
// two outcomes land in the same block on different validators and the state
// roots diverge. The share is owed either way -- which is precisely why a fault
// must not be what picks its destination.
func TestPayVoterCombinedPropagatesNodeLocalLookupFailure(t *testing.T) {
	r := require.New(t)
	f := newCompoundFixture(t)
	f.routing.bucketReader = failingBucketReader{}

	shares, in := selfStakeMismatchShares(f.delegate, f.bucket.Index, big.NewInt(777))
	_, err := f.p.payVoterCombined(f.ctx, f.sm, f.routing, in, f.voter, shares, &iip59RouteDurations{})
	r.Error(err, "a node-local lookup failure must not silently pick a payout route")
	r.ErrorIs(err, errNodeLocal)
}

// TestPayVoterCombinedPropagatesNodeLocalBucketReadFailure covers the second
// read on the same path, with the same reasoning: NativeBucket answers "not
// there" with state.ErrStateNotExist or ErrWithdrawnBucket, so anything else is
// the node failing to read state it should have been able to read.
func TestPayVoterCombinedPropagatesNodeLocalBucketReadFailure(t *testing.T) {
	r := require.New(t)
	f := newCompoundFixture(t)

	patches := gomonkey.NewPatches()
	defer patches.Reset()
	patches.ApplyMethodReturn(f.routing.csr, "NativeBucket", nil, errNodeLocal)

	shares, in := selfStakeMismatchShares(f.delegate, f.bucket.Index, big.NewInt(777))
	_, err := f.p.payVoterCombined(f.ctx, f.sm, f.routing, in, f.voter, shares, &iip59RouteDurations{})
	r.Error(err, "a node-local bucket read failure must not silently pick a payout route")
	r.ErrorIs(err, errNodeLocal)
}

// TestPayVoterCombinedDegradesChainDeterminedBucketMiss is the other half. A
// bucket that is absent or withdrawn is a fact every node reads identically off
// committed state, so rerouting the share to a direct credit is a verdict the
// whole network reaches together. This has to keep working, or the two tests
// above turn every ordinary withdrawn bucket into a halted block.
func TestPayVoterCombinedDegradesChainDeterminedBucketMiss(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"absent", state.ErrStateNotExist},
		{"withdrawn", staking.ErrWithdrawnBucket},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			f := newCompoundFixture(t)

			patches := gomonkey.NewPatches()
			defer patches.Reset()
			patches.ApplyMethodReturn(f.routing.csr, "NativeBucket", nil, tc.err)

			amount := big.NewInt(777)
			shares, in := selfStakeMismatchShares(f.delegate, f.bucket.Index, amount)
			payout, err := f.p.payVoterCombined(
				f.ctx, f.sm, f.routing, in, f.voter, shares, &iip59RouteDurations{},
			)
			r.NoError(err, "a chain-determined bucket miss must degrade, not halt")
			r.False(payout.compounded, "the share must have been rerouted to a credit")
			r.Equal(f.voter.String(), payout.recipient.String())
			r.Zero(amount.Cmp(payout.amount), "the share is still owed in full")
		})
	}
}

// TestGrantEpochRewardSurvivesAnEraThatNeverFroze is the activation gap.
//
// The freeze rides PutPollResult, which executes roughly one and a half epochs
// before the era-boundary block that builds the cursor. Both sides gate on the
// same epoch arithmetic, but they evaluate it at different heights, so an
// activation height landing between the two leaves the boundary running with no
// window ever opened for its era.
//
// Nothing is owed in that state: a pending pool is credited only for a delegate
// carrying a snapshot from this era's freeze, and no freeze means no snapshot.
// So the boundary has to skip the cursor and let the rest of the epoch grant --
// commissions, foundation bonus, sentinel, none of which are IIP-59's --
// complete.
func TestGrantEpochRewardSurvivesAnEraThatNeverFroze(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		// Deliberately no openEraWindowForTest: that is the whole point. The
		// fork turned on after the block that would have opened the window.
		window, err := staking.LoadEraCOWWindow(sm)
		r.NoError(err)
		r.False(window.Open(), "fixture must start with no window, or it proves nothing")

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.NoError(err, "an era that never froze must not cost the epoch its rewards")

		cursor, err := p.readVoterRewardDistributionState(ctx, sm)
		r.NoError(err)
		r.Nil(cursor, "no window means no era to settle, so no cursor")

		// The half that has nothing to do with IIP-59 has to have happened.
		r.Error(p.assertNoRewardYet(ctx, sm, _epochRewardHistoryKeyPrefix, 1),
			"the epoch sentinel must be written even when the era cannot settle")
	}, nil, false, 0)
}

// TestEpochRewardNodeLocalErrorFailsTheBlock pins the receipt-root half.
//
// Handle turned every GrantEpochReward error into a Failure receipt. That is
// safe only for verdicts every node reaches from committed state. A node-local
// fault settled as a Failure receipt lets one validator commit "no epoch
// rewards at all" while the rest commit the full grant -- same block, two
// receipt roots. Post-fork the block has to fail instead.
func TestEpochRewardNodeLocalErrorFailsTheBlock(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyPrivateMethod(p, "loadEpochDistributionInputs", func(
			_ *Protocol, _ context.Context, _ protocol.StateManager, _ uint64,
		) (*admin, map[string]interface{}, map[string]uint64,
			[]*state.Candidate, []*state.Candidate, []address.Address, []*big.Int, error) {
			return nil, nil, nil, nil, nil, nil, nil, errNodeLocal
		})

		testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))
		_, err := p.Handle(ctx, epochRewardEnvelope(t), sm)
		r.Error(err, "a node-local epoch-grant failure must fail the block, not settle a receipt")
		r.ErrorIs(err, errNodeLocal)
	}, nil, false, 0)
}

// TestEpochRewardSettleableErrorStillSettles keeps the verdicts every node
// derives identically on the receipt path. Being dispatched for an epoch that
// already carries its sentinel is committed state, so a Failure receipt is an
// answer the whole network agrees on and the block still commits.
func TestEpochRewardSettleableErrorStillSettles(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		// Write the sentinel first so the grant below is a replay.
		r.NoError(p.updateRewardHistory(ctx, sm, _epochRewardHistoryKeyPrefix, 1))

		testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))
		receipt, err := p.Handle(ctx, epochRewardEnvelope(t), sm)
		r.NoError(err, "a replayed epoch grant is a chain-determined verdict")
		r.NotNil(receipt)
		r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
	}, nil, false, 0)
}

// TestEpochRewardPreForkStillSettlesEveryError is the replay guard. Mainnet has
// already committed blocks whose epoch grant failed and settled a Failure
// receipt. Propagating those during a historical replay would reject a block
// the chain accepted, so the new behaviour has to begin at the fork, not before.
func TestEpochRewardPreForkStillSettlesEveryError(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		r.True(protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution,
			"fixture must be pre-fork, or it proves nothing")

		patches := gomonkey.NewPatches()
		defer patches.Reset()
		patches.ApplyPrivateMethod(p, "loadEpochDistributionInputs", func(
			_ *Protocol, _ context.Context, _ protocol.StateManager, _ uint64,
		) (*admin, map[string]interface{}, map[string]uint64,
			[]*state.Candidate, []*state.Candidate, []address.Address, []*big.Int, error) {
			return nil, nil, nil, nil, nil, nil, nil, errNodeLocal
		})

		testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))
		receipt, err := p.Handle(ctx, epochRewardEnvelope(t), sm)
		r.NoError(err, "pre-fork behaviour must stay byte-identical to what mainnet committed")
		r.NotNil(receipt)
		r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
	}, nil, false, 0)
}

// withFixOff returns patches that make every MustGetFeatureCtx in the call
// under test report the pre-correction era: IIP-59 active,
// FixEpochSettlementFaultHandling not yet.
//
// The two flags are wired to the same height today, so no genesis can separate
// them and the pre-correction branches would otherwise be unreachable from a
// test. They stop moving together as soon as the named heights are assigned,
// and until then this is the only way to hold replay behaviour down.
func withFixOff(t *testing.T, ctx context.Context) *gomonkey.Patches {
	t.Helper()
	fc, ok := protocol.GetFeatureCtx(ctx)
	require.True(t, ok, "fixture must already carry a feature context")
	require.False(t, fc.NoVoterRewardDistribution, "fixture must be post-IIP-59")
	fc.FixEpochSettlementFaultHandling = false
	return gomonkey.NewPatches().ApplyFuncReturn(protocol.MustGetFeatureCtx, fc)
}

// TestPayVoterCombinedDegradesLookupFailureBeforeTheFix pins what a chain that
// activated IIP-59 ahead of the correction already committed: the fault picked
// the destination, and the voter was credited. Replaying those blocks has to
// reproduce it, so the old branch cannot simply be deleted.
func TestPayVoterCombinedDegradesLookupFailureBeforeTheFix(t *testing.T) {
	r := require.New(t)
	f := newCompoundFixture(t)
	f.routing.bucketReader = failingBucketReader{}

	patches := withFixOff(t, f.ctx)
	defer patches.Reset()

	amount := big.NewInt(777)
	shares, in := selfStakeMismatchShares(f.delegate, f.bucket.Index, amount)
	payout, err := f.p.payVoterCombined(
		f.ctx, f.sm, f.routing, in, f.voter, shares, &iip59RouteDurations{},
	)
	r.NoError(err, "before the fix, a lookup fault degraded to a credit")
	r.False(payout.compounded)
	r.Equal(f.voter.String(), payout.recipient.String())
	r.Zero(amount.Cmp(payout.amount))
}

// TestGrantEpochRewardFailsOnAClosedWindowBeforeTheFix is the same guarantee
// for the era boundary: the grant used to fail outright, taking the epoch's
// commissions and foundation bonus with it, and that is what a replaying node
// has to reproduce.
func TestGrantEpochRewardFailsOnAClosedWindowBeforeTheFix(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)

		_, err := p.Deposit(ctx, sm, big.NewInt(500), iotextypes.TransactionLogType_DEPOSIT_TO_REWARDING_FUND)
		r.NoError(err)

		patches := withFixOff(t, ctx)
		defer patches.Reset()
		sp := &staking.Protocol{}
		r.NoError(sp.Register(protocol.MustGetRegistry(ctx)))
		patches.ApplyMethodReturn(sp, "SlashCandidateByOperator", nil)
		patches.ApplyMethodReturn(sp, "SlashCandidateByID", nil)

		_, _, err = p.GrantEpochReward(ctx, sm)
		r.ErrorContains(err, "era copy-on-write window is closed at era boundary")
	}, nil, false, 0)
}

// TestEpochRewardSettlesEveryErrorBeforeTheFix covers the receipt half. With
// IIP-59 live but the correction not yet active, a node-local fault still
// settles a Failure receipt -- which is the divergence the fix removes, and
// also exactly what such a chain has on disk.
func TestEpochRewardSettlesEveryErrorBeforeTheFix(t *testing.T) {
	testProtocol(t, func(t *testing.T, ctx context.Context, sm protocol.StateManager, p *Protocol) {
		r := require.New(t)
		ctx = enableIIP59(t, ctx)
		testdb.AllowRevert(sm.(*mock_chainmanager.MockStateManager))

		patches := withFixOff(t, ctx)
		defer patches.Reset()
		patches.ApplyPrivateMethod(p, "loadEpochDistributionInputs", func(
			_ *Protocol, _ context.Context, _ protocol.StateManager, _ uint64,
		) (*admin, map[string]interface{}, map[string]uint64,
			[]*state.Candidate, []*state.Candidate, []address.Address, []*big.Int, error) {
			return nil, nil, nil, nil, nil, nil, nil, errNodeLocal
		})

		receipt, err := p.Handle(ctx, epochRewardEnvelope(t), sm)
		r.NoError(err, "before the fix, every epoch-grant error settled a receipt")
		r.NotNil(receipt)
		r.Equal(uint64(iotextypes.ReceiptStatus_Failure), receipt.Status)
	}, nil, false, 0)
}

// epochRewardEnvelope builds the system action Handle dispatches on.
func epochRewardEnvelope(t *testing.T) action.Envelope {
	t.Helper()
	return (&action.EnvelopeBuilder{}).
		SetGasPrice(big.NewInt(0)).
		SetAction(action.NewGrantReward(action.EpochReward, 1)).
		Build()
}
