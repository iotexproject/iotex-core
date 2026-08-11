// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// This file is the P5 replacement for the allocator property tests. The old
// suite checked a per-candidate allocator that walked a frozen entry list and
// handed the integer-division remainder to a designated last voter. Both the
// allocator and the entry list are gone: shares are now recomputed per voter
// from the era's frozen buckets, and the remainder stays in the pending pool.
//
// What survives is the claim those tests existed to protect -- that the
// read-only status path and the paying path cannot drift -- and it now holds by
// construction, because both call computeVoterShares. What replaces them is the
// property that construction cannot give for free: that the sum of what the
// drain pays a delegate's voters never exceeds what the era froze for them.

// clampFixture is the planted state plus the numbers the clamp must produce.
type clampFixture struct {
	fixture     *iip59DrainFixture
	delegate    address.Address
	voters      []address.Address
	pool        *big.Int
	understated *big.Int
}

// naiveShare is floor(pool * weight / understatedTotal): what the drain would
// pay this voter if there were no clamp.
func (c clampFixture) naiveShare(voter address.Address) *big.Int {
	share := new(big.Int).Mul(c.pool, c.fixture.weightOf(c.delegate, voter))
	return share.Div(share, c.understated)
}

// TestVoterShareClampNeverOverpaysDelegatePool is the clamp's reason to exist:
// when the recomputed weights sum to more than the frozen TotalWeight, the
// drain must stop at the frozen pool rather than pay out money the era never
// set aside.
func TestVoterShareClampNeverOverpaysDelegatePool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	c := newClampFixture(t, ctx, sm, p, 3)

	// Precondition: without the clamp this fixture over-pays. If the fixture
	// ever stops doing that, everything below passes for the wrong reason.
	naiveTotal := new(big.Int)
	for _, v := range c.voters {
		naiveTotal.Add(naiveTotal, c.naiveShare(v))
	}
	r.True(naiveTotal.Cmp(c.pool) > 0,
		"fixture must force an over-payment: naive total %s vs pool %s", naiveTotal, c.pool)

	chunks := drainVoterRewardsToCompletion(t, ctx, sm, p)
	r.Positive(chunks, "the drain must actually have run")

	balances := accountBalances(t, sm, c.voters)
	paid := new(big.Int)
	truncated, zeroed := 0, 0
	for _, v := range c.voters {
		got := balances[v.String()]
		r.True(got.Sign() >= 0, "no voter may be paid a negative amount")
		r.True(got.Cmp(c.naiveShare(v)) <= 0,
			"the clamp may only reduce a share, never raise it")
		if got.Cmp(c.naiveShare(v)) < 0 {
			truncated++
		}
		if got.Sign() == 0 {
			zeroed++
		}
		paid.Add(paid, got)
	}

	r.Zero(paid.Cmp(c.pool),
		"the drain must pay out exactly the frozen pool, not %s", paid)
	r.Positive(truncated, "at least one voter must have been truncated by the clamp")
	r.Positive(zeroed, "the pool must run out before the last voters in walk order")

	// The pool key is drawn down by exactly what was paid and never below zero.
	left, err := p.readPendingBlockRewardPool(ctx, sm, c.delegate.Bytes())
	r.NoError(err)
	r.Zero(left.Sign(), "pool must be exhausted, and never negative")

	// The fund invariant still holds: an over-payment would have shown up as a
	// totalBalance that no longer accounts for what was moved.
	r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, c.voters))
}

// TestComputeVoterSharesRecordsTheClamp checks the clamp is observable rather
// than silent. A share reduced to fit the pool is flagged, because "the drain
// paid less than the formula" is a fact an operator investigating a shortfall
// has to be able to establish.
func TestComputeVoterSharesRecordsTheClamp(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	c := newClampFixture(t, ctx, sm, p, 3)

	cursor, err := p.readEpochDrainCursor(ctx, sm)
	r.NoError(err)
	r.NotNil(cursor)
	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	in := voterShareInputs{
		window:       window,
		staking:      staking.FindProtocol(protocol.MustGetRegistry(ctx)),
		delegates:    cursor.Delegates,
		byCandidate:  delegateWorkIndex(cursor.Delegates),
		freezeHeight: cursor.FreezeHeight,
		distributed:  cursor.Distributed,
	}

	// Walk the voters in address order, accumulating into the
	// same aliased vector the drain uses.
	order := append([]address.Address(nil), c.voters...)
	sortAddrs(order)

	running := new(big.Int)
	sawClamp := false
	for _, voter := range order {
		shares, err := computeVoterShares(sm, in, voter)
		r.NoError(err)
		for _, s := range shares.shares {
			r.True(s.share.Sign() > 0, "a zero share must not be recorded at all")
			if s.clamped {
				sawClamp = true
			}
			cursor.Distributed[s.delegateIndex] = new(big.Int).Add(
				cursor.Distributed[s.delegateIndex], s.share,
			)
			running.Add(running, s.share)
		}
		r.True(running.Cmp(c.pool) <= 0,
			"the running total must never exceed the pool, reached %s", running)
	}
	r.True(sawClamp, "the clamp must have fired and been recorded")
	r.Zero(running.Cmp(c.pool), "a clamped drain exhausts the pool exactly")
}

// TestComputeVoterSharesRefusesWorkItemWithoutFreezeHeight pins the refusal
// that keeps the current block height from leaking into the weight recompute.
// A work item with no freeze height has no defensible evaluation height, and
// substituting one silently would make a non-timestamp contract bucket worth a
// different amount in every chunk of the same drain.
func TestComputeVoterSharesRefusesWorkItemWithoutFreezeHeight(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	delegate, voter := identityset.Address(4), identityset.Address(8)
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, []iip59NativeSeed{
		{delegate: delegate, voter: voter, amount: 1_000_000_000_000_000_000},
	}, nil)
	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)

	work := epochDrainDelegateWork{
		CandidateIdentifier: delegate.Bytes(),
		VoterAmountFrozen:   big.NewInt(100),
		TotalWeight:         f.totalWeightOf(delegate),
		// FreezeHeight deliberately left at zero.
		SelfStakeBucketIdx: staking.NoSelfStakeBucketIndex,
	}
	_ = p
	_, err = computeVoterShares(sm, voterShareInputs{
		window:      window,
		staking:     staking.FindProtocol(protocol.MustGetRegistry(ctx)),
		delegates:   []epochDrainDelegateWork{work},
		byCandidate: delegateWorkIndex([]epochDrainDelegateWork{work}),
		distributed: []*big.Int{new(big.Int)},
	}, voter)
	r.Error(err)
	r.Contains(err.Error(), "no freeze height")
}

// newClampFixture plants the shared clamp fixture: four voters on one delegate,
// a funded pool, and a cursor whose TotalWeight is understated by divisor.
func newClampFixture(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	divisor int64,
) clampFixture {
	t.Helper()
	r := require.New(t)
	delegate := identityset.Address(4)
	voters := []address.Address{
		identityset.Address(8), identityset.Address(9),
		identityset.Address(10), identityset.Address(11),
	}
	const rau = int64(1_000_000_000_000_000_000)
	seeds := make([]iip59NativeSeed, len(voters))
	for i, v := range voters {
		seeds[i] = iip59NativeSeed{delegate: delegate, voter: v, amount: int64(i+1) * rau}
	}
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, seeds, nil)

	pool := big.NewInt(1_000)
	understated := new(big.Int).Div(f.totalWeightOf(delegate), big.NewInt(divisor))
	r.True(understated.Sign() > 0)

	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance: big.NewInt(100_000), unclaimedBalance: big.NewInt(100_000),
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, delegate.Bytes(), pool))
	r.NoError(p.updateAvailableBalance(ctx, sm, pool))
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:    1,
			FreezeHeight: iip59FixtureFreezeHeight,
			Delegates: []epochDrainDelegateWork{{
				CandidateIdentifier: delegate.Bytes(),
				VoterAmountFrozen:   pool,
				TotalWeight:         understated,
				SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
			}},
		},
	}))
	return clampFixture{
		fixture: f, delegate: delegate, voters: voters,
		pool: pool, understated: understated,
	}
}

// TestLapsedSelfStakeBonusCannotOverpayDelegatePool is the clamp test for the
// one over-payment condition that is not artificial: the self-stake bonus skew
// left in candidate.Votes by an endorsement that lapsed passively.
//
// TestVoterShareClampNeverOverpaysDelegatePool understates TotalWeight by an
// invented divisor. That proves the clamp arithmetic but says nothing about
// whether the condition it guards can arise. This test constructs the condition
// out of the actual disagreement between the two self-stake predicates, at the
// real magnitude it has -- one bucket's 1.06x bonus, not a factor of three.
//
// The state it builds is what a candidate looks like after an endorsement
// lapsed and a bucket was then mutated:
//
//   - the endorser's bucket is still named by Candidate.SelfStakeBucketIdx,
//     because only an explicit revoke clears that field. The frozen snapshot
//     therefore carries that index, and FrozenVoterWeight -- which decides the
//     bonus statelessly, with bkt.Index == selfStakeBucketIdx -- pays the bonus.
//   - candidate.Votes does not contain the bonus, because the mutator that last
//     touched it used the refined isSelfStakeBucket predicate, which had already
//     started answering "no". Whatever the mutation was, it subtracted a
//     non-bonus weight and added a non-bonus weight, leaving Votes permanently
//     short by the bonus.
//
// So the frozen denominator is smaller than the sum of the numerators the drain
// recomputes, and the naive shares sum to more than the pool. TestOnly seeding
// reproduces the end state directly rather than replaying the handler sequence,
// because the sequence needs pre-Upernavik heights (see
// staking.TestLapsedEndorsementDivergesSelfStakePredicates: post-Upernavik an
// endorsement no longer lapses on its own, so this skew can only be inherited
// from history, never created) and the drain only ever sees the end state.
//
// The assertion is that the delegate's pool is still an exact ceiling: the drain
// pays out the frozen amount and not one rau more, no matter that its own
// numerators asked for more.
func TestLapsedSelfStakeBonusCannotOverpayDelegatePool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)

	delegate := identityset.Address(4)
	endorser := identityset.Address(7)
	ordinary := []address.Address{identityset.Address(8), identityset.Address(9)}

	const rau = int64(1_000_000_000_000_000_000)

	// The endorser's bucket has to be auto-staked for at least 91 days or
	// CalculateVoteWeight never applies the self-stake multiplier at all, and
	// the whole fixture would be silently degenerate. It is also the dominant
	// stake, so the bonus it carries is large enough to push the naive shares
	// past the pool on its own.
	selfStakeIdx, err := staking.TestOnlySeedNativeVoterBucket(
		sm, delegate, endorser, new(big.Int).Mul(big.NewInt(10_000_000), big.NewInt(rau)),
		91, time.Unix(0, 0).UTC(), true,
	)
	r.NoError(err)
	r.NotEqual(staking.NoSelfStakeBucketIndex, selfStakeIdx)

	// The two ordinary voters are seeded through the shared fixture, which also
	// opens the era window. The endorser's bucket is planted first, so the
	// window freezes it too.
	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, []iip59NativeSeed{
		{delegate: delegate, voter: ordinary[0], amount: rau},
		{delegate: delegate, voter: ordinary[1], amount: 2 * rau},
	}, nil)
	window, err := staking.EraCOWWindow(sm)
	r.NoError(err)
	stakingProto := staking.FindProtocol(protocol.MustGetRegistry(ctx))
	r.NotNil(stakingProto)

	frozenWeight := func(voter address.Address, selfStake uint64) *big.Int {
		w, wErr := staking.FrozenVoterWeight(
			sm, window, stakingProto, delegate, voter, selfStake, iip59FixtureFreezeHeight,
		)
		r.NoError(wErr)
		return w
	}

	// The endorser's two possible weights: what the drain recomputes (bonus,
	// because the frozen snapshot still names the bucket) and what the
	// accumulator holds (no bonus, because the refined predicate had already
	// stopped agreeing).
	endorserNumerator := frozenWeight(endorser, selfStakeIdx)
	endorserAccumulated := frozenWeight(endorser, staking.NoSelfStakeBucketIndex)
	skew := new(big.Int).Sub(endorserNumerator, endorserAccumulated)
	r.Positive(skew.Sign(),
		"fixture is degenerate: the self-stake bonus did not apply, so there is no skew to clamp")

	// The frozen TotalWeight is candidate.Votes: every voter at its accumulated
	// weight, the endorser without the bonus.
	totalWeight := new(big.Int).Add(f.totalWeightOf(delegate), endorserAccumulated)

	voters := append([]address.Address{endorser}, ordinary...)
	pool := big.NewInt(1_000)
	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance: big.NewInt(100_000), unclaimedBalance: big.NewInt(100_000),
	}))
	r.NoError(p.creditPendingBlockRewardPool(ctx, sm, delegate.Bytes(), pool))
	r.NoError(p.updateAvailableBalance(ctx, sm, pool))
	r.NoError(p.writeEpochDrainCursor(ctx, sm, &epochDrainCursor{
		epochDrainPlan: epochDrainPlan{
			TargetEra:    1,
			FreezeHeight: iip59FixtureFreezeHeight,
			Delegates: []epochDrainDelegateWork{{
				CandidateIdentifier: delegate.Bytes(),
				VoterAmountFrozen:   pool,
				TotalWeight:         totalWeight,
				// The index the lapsed endorsement left behind. This is the entire
				// cause of the over-payment condition.
				SelfStakeBucketIdx: selfStakeIdx,
			}},
		},
	}))

	// Precondition (a): the numerators the drain will recompute really do sum to
	// more than the denominator it will divide by, and by exactly the bonus.
	numeratorSum := new(big.Int)
	naiveTotal := new(big.Int)
	naive := make(map[string]*big.Int, len(voters))
	for _, v := range voters {
		// Same call the drain makes, with the same lapsed index: the endorser
		// picks up the bonus here.
		w := frozenWeight(v, selfStakeIdx)
		numeratorSum.Add(numeratorSum, w)
		share := new(big.Int).Mul(pool, w)
		share.Div(share, totalWeight)
		naive[v.String()] = share
		naiveTotal.Add(naiveTotal, share)
	}
	r.Positive(numeratorSum.Cmp(totalWeight),
		"Sigma FrozenVoterWeight must exceed the frozen TotalWeight or there is nothing to clamp")
	r.Zero(new(big.Int).Sub(numeratorSum, totalWeight).Cmp(skew),
		"the overshoot must be exactly the self-stake bonus, not some other drift")

	// Precondition (b): unclamped, that overshoot is an over-payment.
	r.Positive(naiveTotal.Cmp(pool),
		"without the clamp this fixture pays %s out of a %s pool", naiveTotal, pool)

	chunks := drainVoterRewardsToCompletion(t, ctx, sm, p)
	r.Positive(chunks, "the drain must actually have run")

	balances := accountBalances(t, sm, voters)
	paid := new(big.Int)
	truncated := 0
	for _, v := range voters {
		got := balances[v.String()]
		r.True(got.Sign() >= 0, "no voter may be paid a negative amount")
		r.True(got.Cmp(naive[v.String()]) <= 0,
			"the clamp may only reduce voter %s's share, never raise it", v.String())
		if got.Cmp(naive[v.String()]) < 0 {
			truncated++
		}
		paid.Add(paid, got)
	}
	r.Zero(paid.Cmp(pool),
		"the delegate's frozen pool is an exact ceiling; the drain paid %s of %s", paid, pool)
	r.Positive(truncated, "the bonus skew must have forced at least one truncation")

	left, err := p.readPendingBlockRewardPool(ctx, sm, delegate.Bytes())
	r.NoError(err)
	r.Zero(left.Sign(), "pool must be exhausted, and never negative")
	r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, voters))
}
