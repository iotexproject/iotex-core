// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/v2/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/chainservice"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// This file holds the per-voter payout assertions for the IIP-59 drain, plus
// the two harnesses that exercise the parts of the settlement a single
// happy-path run cannot reach: resumability and the era copy-on-write layer.
//
// The whole point of the model below is that it does not call
// computeVoterShares, FrozenVoterWeight, or anything else on the drain's own
// read path. It reconstructs the expected payout from the fixture's stake
// parameters and the frozen pool, using only CalculateVoteWeight -- the vote
// weight rule itself, which is not what these tests are about. An assertion
// written against the drain's own helpers would agree with the drain no matter
// what either of them did.

// iip59PayoutModel is the independent expectation for one settlement over the
// perf-bench fixture.
//
// The fixture is uniform by construction: every voter bucket carries the same
// stake, duration and auto-stake flag, and all buckets owned by voter j target
// delegate j%numDelegates. The scale tier gives some voters multiple native
// buckets and some a contract bucket, so the oracle aggregates bucket weight
// before applying the payout division, exactly once per voter.
type iip59PayoutModel struct {
	tier        perfTier
	pool        []*big.Int // per delegate: VoterAmountFrozen read from the plan
	totalWeight []*big.Int // per delegate: recomputed, then checked against the plan
	voterWeight []*big.Int // per delegate: weight of one uniform voter bucket
	selfShare   []*big.Int // per delegate: what its own self-stake bucket is owed
}

// newIIP59PayoutModel builds the expectation and, on the way, checks the two
// frozen inputs the drain divides by.
//
// The FreezeHeight check is not incidental. Before an era freeze was wired into
// these harnesses every work item carried FreezeHeight 0, which makes a
// delegate unpayable, and every payout assertion that could have caught it was
// missing. Asserting it here is what keeps the harness from going quiet again.
func newIIP59PayoutModel(
	t *testing.T,
	tier perfTier,
	g genesis.Genesis,
	plan []rewarding.TestOnlyDrainDelegateWork,
) *iip59PayoutModel {
	t.Helper()
	r := require.New(t)
	r.Lenf(plan, tier.numDelegates,
		"the settlement plan must name every seeded delegate; got %d for %d", len(plan), tier.numDelegates)

	consts := g.Staking.VoteWeightCalConsts
	// CalculateVoteWeight reads StakedAmount, StakedDuration and AutoStake and
	// nothing else, so the creation time here is immaterial -- it only has to be
	// a valid one.
	ctime := time.Unix(g.Timestamp, 0)

	m := &iip59PayoutModel{
		tier:        tier,
		pool:        make([]*big.Int, tier.numDelegates),
		totalWeight: make([]*big.Int, tier.numDelegates),
		voterWeight: make([]*big.Int, tier.numDelegates),
		selfShare:   make([]*big.Int, tier.numDelegates),
	}
	for i := 0; i < tier.numDelegates; i++ {
		del := staking.TestOnlyPerfBenchDelegateAddress(i)
		work, ok := iip59WorkFor(plan, del)
		r.Truef(ok, "delegate %d (%s) is missing from the settlement plan", i, del.String())
		r.NotZerof(work.FreezeHeight,
			"delegate %d has FreezeHeight 0: the era never froze, so the drain cannot pay anyone", i)
		r.Positivef(work.VoterAmountFrozen.Sign(),
			"delegate %d froze an empty voter pool; the fixture paid it nothing to distribute", i)

		selfWeight := staking.CalculateVoteWeight(consts, staking.NewVoteBucket(
			del, del, iip59PerfDelegateSelfStake(),
			staking.TestOnlyPerfBenchDelegateStakeDurationDays, ctime, true,
		), true)
		// autoStake true, selfStake false -- the seeder plants every voter bucket
		// auto-staked, and only the delegate's own bucket carries the self-stake
		// bonus.
		voterWeight := staking.CalculateVoteWeight(consts, staking.NewVoteBucket(
			del, perfVoterAddress(tier, 0), iip59PerfVoterStake(),
			iip59PerfVoterStakeDurationDays, ctime, true,
		), false)
		bucketCount := 0
		for voter := i; voter < tier.numVoters; voter += tier.numDelegates {
			bucketCount += tier.voterBucketCount(voter)
		}

		total := new(big.Int).Add(selfWeight, new(big.Int).Mul(voterWeight, big.NewInt(int64(bucketCount))))
		r.Equalf(0, total.Cmp(work.TotalWeight),
			"delegate %d: recomputed total weight %s does not match the frozen denominator %s",
			i, total.String(), work.TotalWeight.String())

		m.pool[i] = new(big.Int).Set(work.VoterAmountFrozen)
		m.totalWeight[i] = total
		m.voterWeight[i] = voterWeight
		m.selfShare[i] = new(big.Int).Div(new(big.Int).Mul(m.pool[i], selfWeight), total)
	}
	return m
}

// expectedVoterPayout is what seeded voter j must end the settlement with.
func (m *iip59PayoutModel) expectedVoterPayout(j int) *big.Int {
	delegate := j % m.tier.numDelegates
	weight := new(big.Int).Mul(m.voterWeight[delegate], big.NewInt(int64(m.tier.voterBucketCount(j))))
	return new(big.Int).Div(new(big.Int).Mul(m.pool[delegate], weight), m.totalWeight[delegate])
}

// expectedDelegatePayout is the sum of every share drawn from delegate i's
// frozen pool, its own self-stake bucket included.
func (m *iip59PayoutModel) expectedDelegatePayout(i int) *big.Int {
	out := new(big.Int).Set(m.selfShare[i])
	for voter := i; voter < m.tier.numVoters; voter += m.tier.numDelegates {
		out.Add(out, m.expectedVoterPayout(voter))
	}
	return out
}

// iip59WorkFor finds a delegate's work item in a settlement plan.
func iip59WorkFor(
	plan []rewarding.TestOnlyDrainDelegateWork,
	delegate address.Address,
) (rewarding.TestOnlyDrainDelegateWork, bool) {
	want := delegate.Bytes()
	for _, w := range plan {
		if len(w.CandidateID) == len(want) {
			match := true
			for i := range want {
				if w.CandidateID[i] != want[i] {
					match = false
					break
				}
			}
			if match {
				return w, true
			}
		}
	}
	return rewarding.TestOnlyDrainDelegateWork{}, false
}

// assertIIP59PerVoterPayouts is the core cross-check: every seeded voter ended
// the settlement holding exactly floor(pool * weight / totalWeight) for the one
// delegate they staked with, and no delegate paid out more than it froze.
func assertIIP59PerVoterPayouts(t *testing.T, run iip59DrainRun) *iip59PayoutModel {
	t.Helper()
	r := require.New(t)
	m := newIIP59PayoutModel(t, run.tier, run.g, run.plan)

	for j := 0; j < run.tier.numVoters; j++ {
		voter := perfVoterAddress(run.tier, j)
		got, ok := run.balances[voter.String()]
		r.Truef(ok, "voter %d (%s) has no balance sample", j, voter.String())
		want := m.expectedVoterPayout(j)
		r.Equalf(0, want.Cmp(got),
			"voter %d (delegate %d): want payout %s, got %s",
			j, j%run.tier.numDelegates, want.String(), got.String())
	}
	for i := 0; i < run.tier.numDelegates; i++ {
		paid := m.expectedDelegatePayout(i)
		r.LessOrEqualf(paid.Cmp(m.pool[i]), 0,
			"delegate %d paid out %s against a frozen pool of %s",
			i, paid.String(), m.pool[i].String())
	}
	return m
}

func assertIIP59SampledVoterPayouts(t *testing.T, run iip59DrainRun, voters []int) *iip59PayoutModel {
	t.Helper()
	r := require.New(t)
	m := newIIP59PayoutModel(t, run.tier, run.g, run.plan)
	for _, j := range voters {
		voter := perfVoterAddress(run.tier, j)
		got, ok := run.balances[voter.String()]
		r.Truef(ok, "sampled voter %d (%s) has no balance", j, voter.String())
		want := m.expectedVoterPayout(j)
		r.Equalf(0, want.Cmp(got),
			"sampled voter %d (delegate %d, buckets %d): want payout %s, got %s",
			j, j%run.tier.numDelegates, run.tier.voterBucketCount(j), want.String(), got.String())
	}
	for i := 0; i < run.tier.numDelegates; i++ {
		paid := m.expectedDelegatePayout(i)
		r.LessOrEqualf(paid.Cmp(m.pool[i]), 0,
			"delegate %d paid out %s against a frozen pool of %s",
			i, paid.String(), m.pool[i].String())
	}
	return m
}

// iip59DrainWatch tracks the per-delegate running payout totals across a
// settlement. Sampled once per block while the cursor is live, it is the only
// place the "never exceeds VoterAmountFrozen" bound can be observed: the final
// chunk folds each delegate's residual into Distributed, so a post-completion
// read always reports the two as equal.
type iip59DrainWatch struct {
	last map[string]*big.Int
}

func newIIP59DrainWatch() *iip59DrainWatch {
	return &iip59DrainWatch{last: make(map[string]*big.Int)}
}

func (w *iip59DrainWatch) observe(t *testing.T, height uint64, plan []rewarding.TestOnlyDrainDelegateWork) {
	t.Helper()
	r := require.New(t)
	for _, work := range plan {
		key := string(work.CandidateID)
		r.LessOrEqualf(work.VoterAmountDistributed.Cmp(work.VoterAmountFrozen), 0,
			"height %d: delegate %x distributed %s of a frozen pool of %s",
			height, work.CandidateID,
			work.VoterAmountDistributed.String(), work.VoterAmountFrozen.String())
		if prev, ok := w.last[key]; ok {
			r.GreaterOrEqualf(work.VoterAmountDistributed.Cmp(prev), 0,
				"height %d: delegate %x distributed total went backwards (%s -> %s)",
				height, work.CandidateID, prev.String(), work.VoterAmountDistributed.String())
		}
		w.last[key] = new(big.Int).Set(work.VoterAmountDistributed)
	}
}

// drainPlan reads the settlement's per-delegate work items at height.
func drainPlan(
	cs *chainservice.ChainService,
	g genesis.Genesis,
	p *rewarding.Protocol,
	height uint64,
) ([]rewarding.TestOnlyDrainDelegateWork, bool, bool, error) {
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	ctx = protocol.WithFeatureCtx(ctx)
	return p.TestOnlyEpochDrainPlan(ctx, cs.StateFactory())
}

// iip59VoterBalances samples the native account balance of every address given,
// keyed by bech32 string.
//
// A voter share is not an unclaimed reward balance: payVoterCombined either
// compounds it into an auto-deposit bucket or credits the recipient's account
// outright (creditPrimaryAccount). There is no claim step, so the account
// balance is where a direct credit lands.
//
// The seeded voter addresses hold nothing at genesis and are never a block
// producer, a delegate or a transaction sender, so for them the balance read
// here is the payout and nothing else. Delegate addresses do carry a genesis
// balance and must be compared as a delta.
func iip59VoterBalances(
	t *testing.T,
	cs *chainservice.ChainService,
	g genesis.Genesis,
	addrs []address.Address,
) map[string]*big.Int {
	t.Helper()
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, g)
	sf := cs.StateFactory()
	out := make(map[string]*big.Int, len(addrs))
	for _, a := range addrs {
		acct, err := accountutil.AccountState(ctx, sf, a)
		require.NoErrorf(t, err, "account state of %s", a.String())
		out[a.String()] = new(big.Int).Set(acct.Balance)
	}
	return out
}

// iip59DrainRun is everything one full settlement lifecycle produced.
type iip59DrainRun struct {
	tier        perfTier
	g           genesis.Genesis
	plan        []rewarding.TestOnlyDrainDelegateWork
	balances    map[string]*big.Int
	drainBlocks int
	startHeight uint64
	endHeight   uint64
}

// runIIP59Drain mints until exactly one settlement lifecycle has completed, then
// samples the plan and every balance the caller could want.
//
// It asserts the fund-conservation invariant and the per-delegate payout bound
// at every block boundary along the way, so a caller only has to make the claim
// specific to its own fixture.
//
// `extra` runs after the standard protocols are registered and before the first
// block, which is where a caller installs its own test protocol.
func runIIP59Drain(
	t *testing.T,
	tier perfTier,
	watch []address.Address,
	extra func(*require.Assertions, *e2etest),
) iip59DrainRun {
	t.Helper()
	r := require.New(t)

	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)
	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)
	if extra != nil {
		extra(r, test)
	}

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto, "rewarding protocol must be registered")

	addrs := append(seededStressAddrs(tier), watch...)
	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	step := 1 * time.Second

	run := iip59DrainRun{tier: tier, g: cfg.Genesis}
	watcher := newIIP59DrainWatch()
	maxBlocks := drainMintCeiling(tier, 1)
	for minted := 0; minted < maxBlocks; minted++ {
		blkTime = blkTime.Add(step)
		_, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())

		height := bc.TipHeight()
		assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, addrs, height)

		_, _, _, _, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at height %d", height)
		plan, completed, planPresent, err := drainPlan(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainPlan at height %d", height)
		if planPresent && !completed {
			watcher.observe(t, height, plan)
		}

		if run.startHeight == 0 {
			if present {
				run.startHeight = height
				run.drainBlocks = 1
			}
			continue
		}
		run.drainBlocks++
		if !present {
			run.endHeight = height
			break
		}
	}
	r.NotZerof(run.startHeight, "drain never began within %d blocks", maxBlocks)
	r.NotZerof(run.endHeight, "drain never completed within %d blocks", maxBlocks)

	plan, completed, planPresent, err := drainPlan(test.cs, cfg.Genesis, rewardProto, run.endHeight)
	r.NoError(err)
	r.True(planPresent, "the completed settlement's plan must survive until the next era boundary")
	r.True(completed, "the settlement must be marked completed once the cursor goes absent")
	run.plan = plan
	run.balances = iip59VoterBalances(t, test.cs, cfg.Genesis, addrs)
	return run
}

// TestIIP59DrainResumeEquivalence settles the same fixture twice: once with the
// per-block voter cap lifted, so the whole era drains in a single chunk, and
// once with a cap tight enough to spread it over many. Every voter must end up
// with the same amount either way.
//
// This is the claim the chunking machinery exists to support. The walk is
// resumable by shard and, inside a shard, by the last address visited; if a
// resume point were ever off by one voter, or if a per-delegate running total
// failed to survive a block boundary, the two runs would disagree here and
// nowhere else -- a single-chunk drain never exercises either mechanism.
func TestIIP59DrainResumeEquivalence(t *testing.T) {
	r := require.New(t)

	base := iip59StressTiers["small"]

	oneShot := base
	oneShot.voterBudgetPerBlock = 0 // 0 means unbounded: one chunk for the whole era
	chunked := base
	// 205 voter keys (200 voters plus 5 self-stake buckets) at 25 per block is
	// about nine chunks, comfortably inside the 20-block era -- a drain still
	// live at the next boundary would be reset by the §10.2 overrun handler and
	// would not be a resumed drain at all.
	chunked.voterBudgetPerBlock = 25

	single := runIIP59Drain(t, oneShot, nil, nil)
	r.Equalf(2, single.drainBlocks,
		"an uncapped drain should be Phase A plus one continuation chunk; took %d blocks", single.drainBlocks)
	many := runIIP59Drain(t, chunked, nil, nil)
	r.Greaterf(many.drainBlocks, 3,
		"the capped drain never chunked: it finished in %d blocks", many.drainBlocks)
	t.Logf("resume equivalence: one-shot span=%d blocks, chunked span=%d blocks",
		single.drainBlocks, many.drainBlocks)

	// Same fixture, same era, same block rewards: the frozen inputs must match
	// before comparing outputs, or a difference in payouts would be ambiguous.
	for i := 0; i < base.numDelegates; i++ {
		del := staking.TestOnlyPerfBenchDelegateAddress(i)
		a, ok := iip59WorkFor(single.plan, del)
		r.True(ok)
		b, ok := iip59WorkFor(many.plan, del)
		r.True(ok)
		r.Equalf(0, a.VoterAmountFrozen.Cmp(b.VoterAmountFrozen),
			"delegate %d froze different pools in the two runs: %s vs %s",
			i, a.VoterAmountFrozen.String(), b.VoterAmountFrozen.String())
		r.Equalf(0, a.TotalWeight.Cmp(b.TotalWeight), "delegate %d froze different total weights", i)
	}

	assertIIP59PerVoterPayouts(t, single)
	assertIIP59PerVoterPayouts(t, many)
	for j := 0; j < base.numVoters; j++ {
		voter := perfVoterAddress(base, j).String()
		r.Equalf(0, single.balances[voter].Cmp(many.balances[voter]),
			"voter %d was paid %s in one chunk but %s across many",
			j, single.balances[voter].String(), many.balances[voter].String())
	}
}

// iip59PostFreezeMutator changes the staking key space after an era has frozen,
// which is the situation the copy-on-write layer exists for.
//
// One block after the freeze it creates a bucket for an address that had none
// at H and deletes every bucket of one that did, both through a candidate state
// manager so the copy-on-write hooks fire exactly as they would for a real
// staking action. Seeding helpers that bypass csm would plant the same state
// without any of the copies, and the era would then see post-boundary state as
// if it had always been there -- which is the bug, not the test.
type iip59PostFreezeMutator struct {
	epochsPerEra uint64
	delegate     address.Address
	newVoter     address.Address
	dropVoter    address.Address

	freezeHeight uint64
	appliedAt    uint64
	dropped      int
}

func (m *iip59PostFreezeMutator) Name() string { return "iip59PostFreezeMutator" }

func (m *iip59PostFreezeMutator) Handle(
	context.Context, action.Envelope, protocol.StateManager,
) (*action.Receipt, error) {
	return nil, nil
}

func (m *iip59PostFreezeMutator) ReadState(
	context.Context, protocol.StateReader, []byte, ...[]byte,
) ([]byte, uint64, error) {
	return nil, 0, protocol.ErrUnimplemented
}

func (m *iip59PostFreezeMutator) Register(r *protocol.Registry) error {
	return r.Register(m.Name(), m)
}

func (m *iip59PostFreezeMutator) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(m.Name(), m)
}

func (m *iip59PostFreezeMutator) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return nil
	}
	blkCtx := protocol.MustGetBlockCtx(ctx)
	height := blkCtx.BlockHeight
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(height)
	if m.freezeHeight == 0 {
		// Same predicate iip59EraFreezer uses; it runs first at this height and
		// opens the window, so all this has to do is remember where.
		if height == rp.GetEpochHeight(epochNum) && protocol.IsEraBoundary(epochNum, m.epochsPerEra) {
			m.freezeHeight = height
		}
		return nil
	}
	if m.appliedAt != 0 || height != m.freezeHeight+1 {
		return nil
	}
	if _, err := staking.TestOnlyPutVoterBucketThroughCOW(
		ctx, sm, m.delegate, m.newVoter,
		iip59PerfVoterStake(), iip59PerfVoterStakeDurationDays, blkCtx.BlockTimeStamp, true,
	); err != nil {
		return err
	}
	dropped, err := staking.TestOnlyDeleteVoterBucketsThroughCOW(ctx, sm, m.dropVoter)
	if err != nil {
		return err
	}
	m.dropped = dropped
	m.appliedAt = height
	return nil
}

// TestIIP59DrainPaysTheFrozenEraNotTheLiveOne is the copy-on-write layer's
// reason for existing, stated as an outcome rather than as a mechanism.
//
// A voter who acquires their first bucket after the era froze is owed nothing
// by that era, even though the shard walk lands on them: their voter-index key
// exists live, and only the tombstone the copy-on-write layer wrote keeps the
// weight recompute from seeing the new bucket.
//
// A voter who withdraws their last bucket after the freeze is still owed the
// share the era froze, even though the walk can no longer find them in live
// state: their index key is gone and only the era's copy still names them.
//
// The second half is the one that fails silently. Nothing errors if such a
// voter is dropped -- their share simply becomes residual and is swept away at
// completion, so the fund invariant still balances and only the voter notices.
func TestIIP59DrainPaysTheFrozenEraNotTheLiveOne(t *testing.T) {
	r := require.New(t)

	tier := iip59StressTiers["small"]
	// Voter 0 belongs to delegate 0 (round-robin), so both halves of the test
	// draw on the same frozen pool and the same expected share.
	dropVoter := perfVoterAddress(tier, 0)
	newVoter := identityset.Address(30)
	mutator := &iip59PostFreezeMutator{
		epochsPerEra: tier.epochsPerEra,
		delegate:     staking.TestOnlyPerfBenchDelegateAddress(0),
		newVoter:     newVoter,
		dropVoter:    dropVoter,
	}

	run := runIIP59Drain(t, tier, []address.Address{newVoter}, func(r *require.Assertions, test *e2etest) {
		r.NoError(mutator.ForceRegister(test.cs.Registry()))
	})
	r.NotZerof(mutator.appliedAt, "the post-freeze mutation never ran")
	r.Positivef(mutator.dropped, "the dropped voter had no buckets to delete; the fixture changed under the test")
	t.Logf("post-freeze mutation applied at height=%d (freeze=%d), dropped %d bucket(s)",
		mutator.appliedAt, mutator.freezeHeight, mutator.dropped)

	m := assertIIP59PerVoterPayouts(t, run)

	// The dropped voter is covered by the loop above -- it asserts every seeded
	// voter's exact share, and voter 0 has no live bucket left by then. Restate
	// it here so a failure names the reason rather than an index.
	r.Equalf(0, m.expectedVoterPayout(0).Cmp(run.balances[dropVoter.String()]),
		"a voter whose bucket was withdrawn after the freeze must still be paid the frozen share: want %s, got %s",
		m.expectedVoterPayout(0).String(), run.balances[dropVoter.String()].String())
	r.Equalf(0, run.balances[newVoter.String()].Sign(),
		"a voter whose bucket was created after the freeze must be paid nothing, got %s",
		run.balances[newVoter.String()].String())
}
