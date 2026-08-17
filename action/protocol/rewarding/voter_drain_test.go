// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"bytes"
	"context"
	"math/big"
	"sort"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// This file holds the properties of the voter-major address drain that are not
// visible from any single function: what order voters are paid in, that a voter
// is paid once no matter how many bucket kinds they hold, that the weight
// recompute is anchored to the era and not to the block, and that every rau a
// delegate froze is accounted for when the drain ends.

// drainScenario is a planted era plus the cursor that drains it.
type drainScenario struct {
	fixture *iip59DrainFixture
	cursor  *voterRewardDistributionState
	pools   map[string]*big.Int
}

// poolOf returns the frozen voter pool of one delegate.
func (s drainScenario) poolOf(delegate address.Address) *big.Int {
	if p, ok := s.pools[string(delegate.Bytes())]; ok {
		return new(big.Int).Set(p)
	}
	return new(big.Int)
}

// newDrainScenario plants an era and writes the cursor era-boundary setup would have
// written for it.
//
// era-boundary setup is not run here. The poll protocol it needs is not part of the
// newVoterRewardCtx fixture, and running it would tie every assertion below to
// the epoch-reward split. What era-boundary setup produces that matters to the drain is a
// cursor whose TotalWeight and FreezeHeight match the planted era, and that is
// written directly.
//
// TotalWeight is taken from the fixture's own recompute rather than invented,
// so the payout clamp stays out of the way; a fixture that understated it would
// put every voter inside the clamp and quietly change what these tests measure.
func newDrainScenario(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	seed []byte,
	pool int64,
	natives []iip59NativeSeed,
	contracts []iip59ContractSeed,
) drainScenario {
	t.Helper()
	r := require.New(t)

	f := seedIIP59DrainState(t, ctx, sm, iip59FixtureFreezeHeight, natives, contracts)

	r.NoError(p.putState(ctx, sm, _fundKey, &fund{
		totalBalance:     big.NewInt(pool * int64(len(f.delegates)) * 8),
		unclaimedBalance: big.NewInt(pool * int64(len(f.delegates)) * 8),
	}))

	pools := make(map[string]*big.Int, len(f.delegates))
	works := make([]voterRewardDelegateAllocation, 0, len(f.delegates))
	for _, delegate := range f.delegates {
		amount := big.NewInt(pool)
		pools[string(delegate.Bytes())] = amount
		r.NoError(p.creditPendingBlockRewardPool(ctx, sm, delegate.Bytes(), amount))
		r.NoError(p.updateAvailableBalance(ctx, sm, amount))
		works = append(works, voterRewardDelegateAllocation{
			CandidateIdentifier: delegate.Bytes(),
			VoterAmountFrozen:   new(big.Int).Set(amount),
			TotalWeight:         f.totalWeightOf(delegate),
			SelfStakeBucketIdx:  staking.NoSelfStakeBucketIndex,
		})
	}
	cursor := &voterRewardDistributionState{
		voterRewardDistributionPlan: voterRewardDistributionPlan{
			TargetEra:           1,
			FreezeHeight:        iip59FixtureFreezeHeight,
			SettlementSeed:      append([]byte(nil), seed...),
			DelegateAllocations: works,
		},
	}
	r.NoError(p.writeVoterRewardDistributionState(ctx, sm, cursor))
	return drainScenario{fixture: f, cursor: cursor, pools: pools}
}

// drainCollectingVoterPayouts runs the drain to completion and returns the
// voter transfers in the order they were made.
func drainCollectingVoterPayouts(
	t *testing.T,
	ctx context.Context,
	sm protocol.StateManager,
	p *Protocol,
	voters []address.Address,
) []*action.TransactionLog {
	t.Helper()
	r := require.New(t)
	isVoter := make(map[string]bool, len(voters))
	for _, v := range voters {
		isVoter[v.String()] = true
	}
	out := make([]*action.TransactionLog, 0, len(voters))
	for i := 0; ; i++ {
		cursor, err := p.readVoterRewardDistributionState(ctx, sm)
		r.NoError(err)
		if cursor == nil || cursor.completed() {
			return out
		}
		txLogs, _, err := p.GrantVoterRewardChunk(ctx, sm)
		r.NoError(err)
		for _, l := range txLogs {
			if l.Type == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND && isVoter[l.Recipient] {
				out = append(out, l)
			}
		}
		if i > 2000 {
			t.Fatal("drain did not complete in 2000 chunks")
		}
	}
}

// payoutOrder reduces a transfer sequence to recipient strings.
func payoutOrder(logs []*action.TransactionLog) []string {
	out := make([]string, len(logs))
	for i, l := range logs {
		out[i] = l.Recipient
	}
	return out
}

// TestVoterDrainAddressOrderIsDeterministic is required test #3.
//
// The payout sequence must be a function of the settlement seed and the voter
// addresses, and of nothing else. In particular it must not depend on the order
// the buckets were written in, because that order is attacker-influenced (a
// voter chooses when to stake) and because two nodes replaying the same era
// from different starting states must produce the same sequence.
func TestVoterDrainAddressOrderIsDeterministic(t *testing.T) {
	r := require.New(t)
	seed := []byte{0x9e, 0x21, 0x77, 0x04, 0x31, 0xbc, 0x5a, 0xd0}
	delegate := identityset.Address(4)
	const rau = int64(1_000_000_000_000_000_000)

	// Voters are spread deliberately across the address space, and their prefixes are
	// unrelated to the order they are planted in.
	prefixes := []byte{0xf1, 0x03, 0xa7, 0x40, 0x11, 0xcc}
	voters := make([]address.Address, len(prefixes))
	for i, prefix := range prefixes {
		voters[i] = voterWithPrefix(prefix, i)
	}

	run := func(order []int) []string {
		ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
		p.cfg.VoterBudgetPerBlock = 1
		seeds := make([]iip59NativeSeed, 0, len(order))
		for _, i := range order {
			seeds = append(seeds, iip59NativeSeed{
				delegate: delegate, voter: voters[i], amount: int64(i+1) * rau,
			})
		}
		newDrainScenario(t, ctx, sm, p, seed, 1_000_000, seeds, nil)
		return payoutOrder(drainCollectingVoterPayouts(t, ctx, sm, p, voters))
	}

	forward := []int{0, 1, 2, 3, 4, 5}
	reverse := []int{5, 4, 3, 2, 1, 0}
	first := run(forward)
	second := run(reverse)
	r.Len(first, len(voters), "every voter must be paid exactly once")
	r.Equal(first, second,
		"payout order must depend on the seed and the addresses, not on insertion order")

	// And it is the circular order the seed picked, not plain ascending order.
	start := settlementStartVoter(seed)
	ordered := append([]address.Address(nil), voters...)
	sort.Slice(ordered, func(i, j int) bool {
		iTail := bytes.Compare(ordered[i].Bytes(), start) >= 0
		jTail := bytes.Compare(ordered[j].Bytes(), start) >= 0
		if iTail != jTail {
			return iTail
		}
		return bytes.Compare(ordered[i].Bytes(), ordered[j].Bytes()) < 0
	})
	want := make([]string, len(ordered))
	for i, voter := range ordered {
		want[i] = voter.String()
	}
	r.Equal(want, first, "the walk must start at the seed-derived address and wrap")
}

func TestVoterDrainStartAddressIsVisitedOnce(t *testing.T) {
	r := require.New(t)
	delegate := identityset.Address(4)
	before := voterWithPrefix(0x20, 0)
	start := voterWithPrefix(0x80, 0)
	after := voterWithPrefix(0xc0, 0)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	p.cfg.VoterBudgetPerBlock = 1
	newDrainScenario(t, ctx, sm, p, start.Bytes(), 1_000_000,
		[]iip59NativeSeed{
			{delegate: delegate, voter: before, amount: 1_000_000_000_000_000_000},
			{delegate: delegate, voter: start, amount: 1_000_000_000_000_000_000},
			{delegate: delegate, voter: after, amount: 1_000_000_000_000_000_000},
		}, nil,
	)
	logs := drainCollectingVoterPayouts(t, ctx, sm, p, []address.Address{before, start, after})
	r.Equal([]string{start.String(), after.String(), before.String()}, payoutOrder(logs))
}

func TestPostFreezeVoterDoesNotConsumeVoterBudget(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	p.cfg.VoterBudgetPerBlock = 1
	delegate := identityset.Address(4)
	newcomer := voterWithPrefix(0x10, 0)
	owed := voterWithPrefix(0xf0, 0)
	newDrainScenario(t, ctx, sm, p, make([]byte, 20), 1_000_000,
		[]iip59NativeSeed{{
			delegate: delegate, voter: owed, amount: 1_000_000_000_000_000_000,
		}}, nil,
	)

	// The newcomer sorts before the frozen voter but first stakes only after
	// the era window opened. Its COW tombstone must keep it out of the one-voter
	// budget so the owed voter is still paid in this block.
	_, err := staking.TestOnlyPutVoterBucketThroughCOW(
		ctx, sm, delegate, newcomer, big.NewInt(1_000_000_000_000_000_000),
		91, time.Unix(1_700_000_000, 0), true,
	)
	r.NoError(err)
	txLogs, _, err := p.GrantVoterRewardChunk(ctx, sm)
	r.NoError(err)

	paid := make([]string, 0, 1)
	for _, log := range txLogs {
		if log.Type == iotextypes.TransactionLogType_CLAIM_FROM_REWARDING_FUND {
			paid = append(paid, log.Recipient)
		}
	}
	r.Equal([]string{owed.String()}, paid)
}

// TestVoterDrainMergesNativeAndContractStreams is required test #4.
//
// A voter holding both a native bucket and a liquid-staking bucket appears in
// two independent key streams. The ordered walk merges them, so the voter must be
// visited once and paid once -- but for the sum of both, not for whichever
// stream happened to be scanned first.
func TestVoterDrainMergesNativeAndContractStreams(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	nativeDelegate := identityset.Address(4)
	contractDelegate := identityset.Address(5)
	contract := identityset.Address(20)
	both := identityset.Address(8)
	nativeOnly := identityset.Address(9)
	const rau = int64(1_000_000_000_000_000_000)
	const pool = int64(1_000_000)

	s := newDrainScenario(t, ctx, sm, p, []byte{1, 2, 3, 4}, pool,
		[]iip59NativeSeed{
			{delegate: nativeDelegate, voter: both, amount: 3 * rau},
			{delegate: nativeDelegate, voter: nativeOnly, amount: rau},
		},
		[]iip59ContractSeed{{
			delegate: contractDelegate, voter: both, amount: 7 * rau,
			contract: contract, bucketID: 1,
			timestamped: true, duration: 30 * 24 * 3600, createdAt: 1,
		}},
	)
	r.Contains(s.fixture.delegates, nativeDelegate)
	r.Contains(s.fixture.delegates, contractDelegate)
	r.True(s.fixture.weightOf(contractDelegate, both).Sign() > 0,
		"the contract bucket must carry weight, else the merge is untested")

	txLogs := drainCollectingVoterPayouts(t, ctx, sm, p, []address.Address{both, nativeOnly})
	paidTo := map[string]int{}
	for _, l := range txLogs {
		paidTo[l.Recipient]++
	}
	r.Equal(1, paidTo[both.String()],
		"a voter present in both streams must be visited and paid exactly once")
	r.Equal(1, paidTo[nativeOnly.String()])

	want := new(big.Int).Add(
		s.fixture.expectedShare(nativeDelegate, both, s.poolOf(nativeDelegate)),
		s.fixture.expectedShare(contractDelegate, both, s.poolOf(contractDelegate)),
	)
	r.True(want.Sign() > 0)
	balances := accountBalances(t, sm, []address.Address{both, nativeOnly})
	r.Zero(balances[both.String()].Cmp(want),
		"the single transfer must carry both streams' weight (got %s want %s)",
		balances[both.String()], want)
	r.Zero(balances[nativeOnly.String()].Cmp(
		s.fixture.expectedShare(nativeDelegate, nativeOnly, s.poolOf(nativeDelegate))))
}

// TestVoterDrainEvalHeightIsTheFreezeHeight is required test #5.
//
// A contract bucket that is not timestamp-based stores its stake duration as a
// block count, and the block count is converted to wall-clock time at the height
// the weight is evaluated at -- a hardfork that changes the block interval makes
// the same span worth a different duration before and after it. The drain runs
// many blocks after the era boundary and may span many blocks, so if the current
// block height reaches the recompute the same frozen bucket is worth a different
// amount in different chunks and a voter's payout depends on which block
// happened to pay them.
//
// Two voters are needed, not one: with a single voter the share is the whole
// pool no matter what the weight is, so a wrong evaluation height cancels out.
// Their buckets carry equal stake and very different durations, so a change of
// evaluation height moves the ratio between them and therefore the split.
func TestVoterDrainEvalHeightIsTheFreezeHeight(t *testing.T) {
	r := require.New(t)
	delegate := identityset.Address(4)
	contract := identityset.Address(20)
	long, short := identityset.Address(8), identityset.Address(9)
	voters := []address.Address{long, short}
	const rau = int64(1_000_000_000_000_000_000)
	const pool = int64(1_000_000)

	seeds := []iip59ContractSeed{
		{
			delegate: delegate, voter: long, amount: 5 * rau,
			contract: contract, bucketID: 1,
			timestamped: false, duration: 100_000, createdAt: 1,
		},
		{
			delegate: delegate, voter: short, amount: 5 * rau,
			contract: contract, bucketID: 2,
			timestamped: false, duration: 4_000, createdAt: 1,
		},
	}

	// Precondition: the recompute must disagree with itself across heights, and
	// disagree by a different factor for each bucket, or the split is unchanged
	// and this test cannot detect a leak of the block height into it.
	func() {
		ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
		newDrainScenario(t, ctx, sm, p, []byte{7}, pool, nil, seeds)
		window, err := staking.LoadEraCOWWindow(sm)
		r.NoError(err)
		sp := staking.FindProtocol(protocol.MustGetRegistry(ctx))
		weightAt := func(voter address.Address, h uint64) *big.Int {
			w, err := staking.FrozenVoterWeight(
				sm, window, sp, delegate, voter, staking.NoSelfStakeBucketIndex, h)
			r.NoError(err)
			return w
		}
		ratio := func(h uint64) *big.Rat {
			return new(big.Rat).SetFrac(weightAt(long, h), weightAt(short, h))
		}
		r.NotZero(ratio(iip59FixtureFreezeHeight).Cmp(ratio(900_000)),
			"fixture must make the split height-sensitive: %s at freeze vs %s at block",
			ratio(iip59FixtureFreezeHeight).FloatString(6), ratio(900_000).FloatString(6))
	}()

	payAt := func(blockHeight uint64) map[string]*big.Int {
		ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
		newDrainScenario(t, ctx, sm, p, []byte{7}, pool, nil, seeds)
		blk := protocol.MustGetBlockCtx(ctx)
		blk.BlockHeight = blockHeight
		ctx = protocol.WithBlockCtx(ctx, blk)
		ctx = protocol.WithFeatureCtx(ctx)
		ctx = protocol.WithFeatureWithHeightCtx(ctx)
		drainCollectingVoterPayouts(t, ctx, sm, p, voters)
		return accountBalances(t, sm, voters)
	}

	// testBlockIntervalSwitchHeight sits between these two, so a drain that used
	// its own block height would convert the durations differently in each run.
	early := payAt(100)
	late := payAt(900_000)
	for _, voter := range voters {
		r.True(early[voter.String()].Sign() > 0, "voter %s must actually have been paid", voter)
		r.Zero(early[voter.String()].Cmp(late[voter.String()]),
			"voter %s: a frozen bucket must be worth the same in every chunk of the drain (%s at height 100, %s at height 900000)",
			voter, early[voter.String()], late[voter.String()])
	}
}

// TestVoterDrainConservesEveryDelegatePool is required test #6.
//
// Floor division and the payout clamp both leave money behind. The residual
// stays in the delegate's pending pool so a later era can distribute it. The
// accounting claim is: for every delegate, what the voters received plus what
// remains pending equals exactly what the era froze -- no rau is created or
// lost.
func TestVoterDrainConservesEveryDelegatePool(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	delegates := []address.Address{identityset.Address(4), identityset.Address(5)}
	voters := []address.Address{
		identityset.Address(8), identityset.Address(9),
		identityset.Address(10), identityset.Address(11),
	}
	const rau = int64(1_000_000_000_000_000_000)
	// A pool that does not divide evenly by the weights, so a residual exists.
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

	done, err := p.readVoterRewardDistributionState(ctx, sm)
	r.NoError(err)
	r.NotNil(done)
	r.True(done.completed())

	after := accountBalances(t, sm, delegates)
	balances := accountBalances(t, sm, voters)

	grandTotal := new(big.Int)
	residualTotal := new(big.Int)
	for i, work := range done.DelegateAllocations {
		delegate, err := address.FromBytes(work.CandidateIdentifier)
		r.NoError(err)

		// Payouts owed by this delegate, recomputed independently of the drain.
		owed := new(big.Int)
		for _, voter := range voters {
			owed.Add(owed, s.fixture.expectedShare(delegate, voter, s.poolOf(delegate)))
		}
		r.Zero(done.distributedAt(i).Cmp(owed),
			"delegate %s: distributed %s != voter payouts %s",
			delegate, done.distributedAt(i), owed)

		left, err := p.readPendingBlockRewardPool(ctx, sm, work.CandidateIdentifier)
		r.NoError(err)
		wantResidual := new(big.Int).Sub(work.VoterAmountFrozen, owed)
		r.True(wantResidual.Sign() > 0, "this fixture must leave a residual pending")
		r.Zero(left.Cmp(wantResidual),
			"delegate %s: pending residual %s != frozen %s - payouts %s",
			delegate, left, work.VoterAmountFrozen, owed)
		residualTotal.Add(residualTotal, left)
		r.Zero(after[delegate.String()].Cmp(before[delegate.String()]),
			"delegate %s must not receive a residual sweep", delegate)
		grandTotal.Add(grandTotal, work.VoterAmountFrozen)
	}

	moved := new(big.Int)
	for _, voter := range voters {
		moved.Add(moved, balances[voter.String()])
	}
	r.Zero(new(big.Int).Add(moved, residualTotal).Cmp(grandTotal),
		"voter payouts (%s) plus pending residuals (%s) must equal total frozen (%s)",
		moved, residualTotal, grandTotal)
	r.NoError(p.TestOnlyAssertFundInvariant(ctx, sm, append(append([]address.Address(nil), voters...), delegates...)))
}

// TestVoterDrainRefusesASupersededEraWindow covers the one window transition the
// drain cannot survive: the next era's freeze landing while this era still has
// voters to pay.
//
// The freeze rides PutPollResult, which fires around the midpoint of the epoch
// before the boundary epoch — roughly 1.5 epochs before the boundary block where
// era-boundary setup hands an overrunning cursor to rollOverIncompleteVoterRewardDistribution. eracow.Begin
// does not refuse to supersede an open window, so for that entire stretch
// LoadEraCOWWindow answers at the new freeze height while the cursor still carries
// the old one. Nothing errors: the reads simply answer for the wrong era, at
// which point the drain pays real money on buckets that grew after its own
// boundary, and on buckets that did not exist at it. See
// TestSupersededWindowSilentlyAnswersForTheWrongEra in the staking package for
// that mechanism in isolation.
//
// So the drain must notice the swap itself. Stopping costs nothing: era-boundary setup of
// the incoming era rolls every delegate's residue into an era that can freeze it
// properly.
func TestVoterDrainRefusesASupersededEraWindow(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	delegate := identityset.Address(4)
	const rau = int64(1_000_000_000_000_000_000)

	// One voter per block, so the drain is guaranteed to be mid-flight when the
	// next era freezes.
	p.cfg.VoterBudgetPerBlock = 1
	voters := []address.Address{
		voterWithPrefix(0x11, 0), voterWithPrefix(0x22, 1), voterWithPrefix(0x33, 2),
	}
	seeds := make([]iip59NativeSeed, 0, len(voters))
	for i, voter := range voters {
		seeds = append(seeds, iip59NativeSeed{
			delegate: delegate, voter: voter, amount: int64(i+1) * rau,
		})
	}
	newDrainScenario(t, ctx, sm, p, []byte{0x71, 0x0d}, 1_000_000, seeds, nil)

	// A chunk under the era's own window is fine.
	_, _, err := p.GrantVoterRewardChunk(ctx, sm)
	r.NoError(err)
	before, err := p.readVoterRewardDistributionState(ctx, sm)
	r.NoError(err)
	r.NotNil(before)
	r.False(before.completed(), "this fixture must leave the distribution mid-flight")

	// The next era boundary freezes. The drain is still owed voters.
	r.NoError(staking.TestOnlyBeginEraCOWWindow(ctx, sm, iip59FixtureFreezeHeight+2_000))
	window, err := staking.LoadEraCOWWindow(sm)
	r.NoError(err)
	r.True(window.Open(), "the old Open() gate on its own still passes")
	r.NotEqual(before.FreezeHeight, window.FreezeHeight)

	_, _, err = p.GrantVoterRewardChunk(ctx, sm)
	r.Error(err, "the drain must not pay through a window it was not frozen against")
	r.ErrorContains(err, "outlived its copy-on-write window")
	// Both heights are committed state, so every node reaches this verdict on
	// the same block: it settles as a Failure receipt instead of failing the
	// block for the whole network.
	r.True(voterChunkErrorIsSettleable(err),
		"a halted block here would stop the chain for 1.5 epochs, not just the drain")

	// Refusing must be inert: no partial payout, no cursor movement, so the
	// residue era-boundary setup rolls forward is still whole.
	after, err := p.readVoterRewardDistributionState(ctx, sm)
	r.NoError(err)
	r.NotNil(after)
	r.Equal(before.ScanPhase, after.ScanPhase)
	r.Equal(before.ResumeVoter, after.ResumeVoter)
	r.False(after.completed())
	for i := range after.DelegateAllocations {
		r.Zero(before.distributedAt(i).Cmp(after.distributedAt(i)),
			"delegate %d paid out while the window was superseded", i)
	}
}
