// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// TestVoterDrainPayoutsUnchangedByScalarSnapshot is the refactor-safety net for
// removing CandidatePollSnapshot.Entries.
//
// Before the removal the frozen denominator was the sum of the materialized
// per-voter weight list; now it is the candidate's Votes accumulator, captured
// as a scalar at the boundary. Those are the same number by construction (the
// invariant TestVoterWeightInvariant pins), but "the same number" is a claim
// about the design, not a fact about the code — so this test pins the money.
//
// The expected values below are golden. Provenance, stated exactly: they were
// captured from this fixture after the removal, not replayed against a
// pre-removal build — the P5 drain fixture this sits on was never committed at
// the revision that still had Entries, so a literal before/after run was not
// available. What makes them *the same* amounts is asserted here rather than
// assumed: the last loop rebuilds the pre-removal denominator (the sum over the
// per-voter frozen weights, which is what the deleted list added up to) and
// requires the persisted scalar to equal it, delegate by delegate. Every other
// input to a payout — the per-voter weight, which the drain recomputes with
// staking.FrozenVoterWeight, and the pool — was untouched by the removal.
//
// If any future change moves a single rau, this goes red and names the voter.
// Do not regenerate these numbers to make a failure go away without first
// establishing why the payout changed.
func TestVoterDrainPayoutsUnchangedByScalarSnapshot(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	delegates := []address.Address{identityset.Address(4), identityset.Address(5)}
	voters := []address.Address{
		identityset.Address(8), identityset.Address(9),
		identityset.Address(10), identityset.Address(11),
	}
	const rau = int64(1_000_000_000_000_000_000)
	// Deliberately indivisible by the weights so the floor division and the
	// residual sweep both participate — a refactor that changed rounding
	// direction would show up here and nowhere else.
	const pool = int64(999_997)

	seeds := make([]iip59NativeSeed, 0, len(delegates)*len(voters))
	for di, delegate := range delegates {
		for vi, voter := range voters {
			seeds = append(seeds, iip59NativeSeed{
				delegate: delegate, voter: voter,
				amount: int64(di*3+vi+1) * rau,
			})
		}
	}
	s := newDrainScenario(t, ctx, sm, p, []byte{0x5c, 0x11}, pool, seeds, nil)

	before := accountBalances(t, sm, delegates)
	drainCollectingVoterPayouts(t, ctx, sm, p, voters)

	done, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(done)
	r.True(done.Completed)

	after := accountBalances(t, sm, delegates)
	balances := accountBalances(t, sm, voters)

	// Golden per-voter totals, summed across both delegates.
	wantVoter := map[string]int64{
		identityset.Address(8).String():  281_816,
		identityset.Address(9).String():  427_271,
		identityset.Address(10).String(): 572_725,
		identityset.Address(11).String(): 718_178,
	}
	gotVoter := make(map[string]int64, len(voters))
	for _, voter := range voters {
		gotVoter[voter.String()] = balances[voter.String()].Int64()
	}
	r.Equal(wantVoter, gotVoter, "per-voter payouts drifted")

	// Golden per-delegate residual sweeps. The residual is what floor division
	// left behind, so it is the most sensitive witness to a denominator change:
	// a denominator that grew by even one unit shifts rau out of the voters and
	// into this number.
	wantSweep := map[string]int64{
		identityset.Address(4).String(): 2,
		identityset.Address(5).String(): 2,
	}
	gotSweep := make(map[string]int64, len(delegates))
	for _, delegate := range delegates {
		gotSweep[delegate.String()] = new(big.Int).Sub(
			after[delegate.String()], before[delegate.String()],
		).Int64()
	}
	r.Equal(wantSweep, gotSweep, "per-delegate residual sweeps drifted")

	// And the whole era still conserves: nothing was created or destroyed by
	// changing where the denominator comes from.
	grandTotal := new(big.Int)
	for _, work := range done.Delegates {
		grandTotal.Add(grandTotal, work.VoterAmountFrozen)
	}
	r.Equal(int64(len(delegates))*pool, grandTotal.Int64())

	moved := new(big.Int)
	for _, voter := range voters {
		moved.Add(moved, balances[voter.String()])
	}
	for _, delegate := range delegates {
		moved.Add(moved, new(big.Int).Sub(after[delegate.String()], before[delegate.String()]))
	}
	r.Zero(moved.Cmp(grandTotal))

	// Independently: recompute every share with the *pre-refactor* definition
	// of the denominator — the sum over the per-voter frozen weights, i.e. what
	// the deleted Entries list added up to — and require the drain's actual
	// payouts to match it exactly.
	for _, delegate := range delegates {
		oldDenominator := new(big.Int)
		for _, voter := range voters {
			oldDenominator.Add(oldDenominator, s.fixture.weightOf(delegate, voter))
		}
		r.True(oldDenominator.Sign() > 0, "fixture must carry non-zero weight")
		r.Zero(oldDenominator.Cmp(s.fixture.totalWeightOf(delegate)),
			"the fixture's running total must equal the sum over its per-voter weights")
		// The load-bearing one: the scalar the boundary actually persisted has
		// to equal what the deleted per-voter list would have added up to.
		persisted, sErr := staking.PollSnapshotFor(sm, delegate)
		r.NoError(sErr)
		r.Zero(oldDenominator.Cmp(persisted.TotalWeight),
			"delegate %s: frozen scalar TotalWeight != sum of the pre-refactor entry list",
			delegate.String())
		for _, voter := range voters {
			want := new(big.Int).Mul(big.NewInt(pool), s.fixture.weightOf(delegate, voter))
			want.Div(want, oldDenominator)
			r.Zero(want.Cmp(s.fixture.expectedShare(delegate, voter, s.poolOf(delegate))),
				"delegate %s voter %s share drifted from the pre-refactor formula",
				delegate.String(), voter.String())
		}
	}
}
