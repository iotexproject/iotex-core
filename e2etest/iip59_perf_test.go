// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/actpool"
	"github.com/iotexproject/iotex-core/v2/blockchain"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/chainservice"
	"github.com/iotexproject/iotex-core/v2/config"
	"github.com/iotexproject/iotex-core/v2/pkg/unit"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil"
)

// perfTier parameterizes the IIP-59 e2e drain bench. Each tier picks
// scale (delegates × voters), era cadence (epochs per era), and the
// per-block voter budget. The budget is picked deliberately low so the drain spans multiple
// blocks — otherwise the bench collapses to a single-block run and
// stops measuring what we care about (chunking behaviour).
type perfTier struct {
	numDelegates        int
	numVoters           int
	numNativeBuckets    int
	numContractBuckets  int
	shardVoterAddresses bool
	sampleVoterPayouts  bool
	epochsPerEra        uint64
	voterBudgetPerBlock uint64
}

var iip59PerfTiers = map[string]perfTier{
	"small":   {numDelegates: 3, numVoters: 100, epochsPerEra: 2, voterBudgetPerBlock: 50},
	"medium":  {numDelegates: 10, numVoters: 1_000, epochsPerEra: 4, voterBudgetPerBlock: 250},
	"mainnet": {numDelegates: 24, numVoters: 27_020, epochsPerEra: 24, voterBudgetPerBlock: 4_504},
	"scale": {
		numDelegates:        24,
		numVoters:           30_000,
		numNativeBuckets:    50_000,
		numContractBuckets:  10_000,
		shardVoterAddresses: true,
		sampleVoterPayouts:  true,
		epochsPerEra:        2,
		voterBudgetPerBlock: 4_504,
	},
}

const iip59PerfTierEnv = "IIP59_PERF_TIER"

// The stake parameters every seeded bucket gets. They are package-level rather
// than inline in installIIP59PerfHooks because the payout assertions recompute
// vote weights from them: if the two ever disagreed, the assertions would be
// checking the drain against a fixture that does not exist.
const iip59PerfVoterStakeDurationDays uint32 = 30

func iip59PerfDelegateSelfStake() *big.Int { return unit.ConvertIotxToRau(1_200_000) }
func iip59PerfVoterStake() *big.Int        { return unit.ConvertIotxToRau(1_000) }

func (tier perfTier) nativeBucketCount() int {
	if tier.numNativeBuckets == 0 {
		return tier.numVoters
	}
	return tier.numNativeBuckets
}

func (tier perfTier) voterBucketCount(voter int) int {
	count := 0
	if voter >= 0 && voter < tier.numVoters {
		if native := tier.nativeBucketCount(); voter < native {
			count += (native-1-voter)/tier.numVoters + 1
		}
		if voter < tier.numContractBuckets {
			count += (tier.numContractBuckets-1-voter)/tier.numVoters + 1
		}
	}
	return count
}

func perfVoterAddress(tier perfTier, voter int) address.Address {
	if tier.shardVoterAddresses {
		return staking.TestOnlyPerfBenchShardedVoterAddress(voter)
	}
	return staking.TestOnlyPerfBenchVoterAddress(voter)
}

type iip59PerfMetric struct {
	wall       time.Duration
	totalAlloc uint64
	mallocs    uint64
	heapAlloc  uint64
	peakRSS    uint64
}

func mintOneMeasured(
	bc blockchain.Blockchain,
	ap actpool.ActPool,
	blkTime time.Time,
) (iip59PerfMetric, error) {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	wall, err := mintOne(bc, ap, blkTime)
	runtime.ReadMemStats(&after)
	return iip59PerfMetric{
		wall:       wall,
		totalAlloc: after.TotalAlloc - before.TotalAlloc,
		mallocs:    after.Mallocs - before.Mallocs,
		heapAlloc:  after.HeapAlloc,
		peakRSS:    iip59PeakRSSBytes(),
	}, err
}

func logIIP59PerfMetric(t *testing.T, phase string, height uint64, metric iip59PerfMetric) {
	t.Helper()
	t.Logf("iip59 phase=%s height=%d wall=%v alloc=%.2fMiB mallocs=%d heap=%.2fMiB peak_rss=%.2fMiB",
		phase, height, metric.wall, float64(metric.totalAlloc)/(1<<20), metric.mallocs,
		float64(metric.heapAlloc)/(1<<20), float64(metric.peakRSS)/(1<<20))
}

func sampledIIP59PerfVoters(tier perfTier) []int {
	if tier.numVoters == 0 {
		return nil
	}
	selected := make(map[int]struct{}, 520)
	add := func(voter int) {
		if voter >= 0 && voter < tier.numVoters {
			selected[voter] = struct{}{}
		}
	}
	add(0)
	add(tier.numVoters - 1)
	// First and last voter in every shard pin both shard transitions and the
	// full key range within each shard.
	for shard := 0; shard < 256; shard++ {
		add(shard)
		last := tier.numVoters - 1
		last -= (last - shard) & 255
		add(last)
	}
	// Boundaries where the fixture changes from multiple to one native bucket,
	// and from owning to not owning a contract bucket.
	for _, boundary := range []int{
		tier.nativeBucketCount() % tier.numVoters,
		tier.numContractBuckets % tier.numVoters,
	} {
		add(boundary - 1)
		add(boundary)
	}
	out := make([]int, 0, len(selected))
	for voter := range selected {
		out = append(out, voter)
	}
	sort.Ints(out)
	return out
}

func sampledIIP59PerfVoterAddrs(tier perfTier) []address.Address {
	voters := sampledIIP59PerfVoters(tier)
	out := make([]address.Address, 0, len(voters))
	for _, voter := range voters {
		out = append(out, perfVoterAddress(tier, voter))
	}
	return out
}

// TestIIP59EpochGrantPerf spins up a real itx.Server, plants a
// tier-sized delegate/voter population directly in the genesis state,
// enables the IIP-59 fork from height 1, and mints blocks until the
// chunked era-boundary drain completes. It reports per-block wall-clock
// samples for the drain window and asserts the drain spans multiple
// blocks (so the chunking mechanism is actually exercised, not
// short-circuited).
//
// Default tier is small (3 delegates / 100 voters) so CI can run this
// in seconds. Set IIP59_PERF_TIER=medium or =mainnet to bench larger
// scales; mainnet is skipped under go test -short.
func TestIIP59EpochGrantPerf(t *testing.T) {
	r := require.New(t)

	tierName := os.Getenv(iip59PerfTierEnv)
	if tierName == "" {
		tierName = "small"
	}
	tier, ok := iip59PerfTiers[tierName]
	if !ok {
		t.Fatalf("unknown %s=%q; want small|medium|mainnet|scale", iip59PerfTierEnv, tierName)
	}
	if (tierName == "mainnet" || tierName == "scale") && testing.Short() {
		t.Skipf("skipping %s tier under -short", tierName)
	}

	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	// The injection seams travel with this one server rather than through
	// package-level state, so there is nothing to reset afterwards.
	test := newE2ETest(t, cfg, iip59PerfBuildOptions(t, tier, cfg.Genesis)...)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)
	if tier.numContractBuckets > 0 {
		registerIIP59PreActivationContractSeeder(r, test, iip59PerfSeederSpec(t, tier, cfg.Genesis), 1)
	}

	bc := test.cs.Blockchain()
	ap := test.cs.ActionPool()
	rp := rolldpos.FindProtocol(test.cs.Registry())
	r.NotNil(rp, "rolldpos protocol must be registered")
	rewardProto := rewarding.FindProtocol(test.cs.Registry())
	r.NotNil(rewardProto, "rewarding protocol must be registered")

	// Baseline: mint one throwaway block so the fork gate flips before
	// any measurements. IIP-59 turns on at ToBeEnabledBlockHeight=1.
	blkTime := time.Unix(cfg.Genesis.Timestamp, 0)
	step := 1 * time.Second

	// Phase 1: mint through the first full epoch of the era so voter
	// buckets accrue block-reward pool credits, then keep going until
	// we cross an era boundary. Once the target-era epoch closes, the
	// drain cursor is installed and continuation grants begin.
	var (
		drainStartHeight uint64
		drainStartEpoch  uint64
		chunkTimes       []time.Duration
		chunkMetrics     []iip59PerfMetric
		activationMetric *iip59PerfMetric
		freezeMetric     *iip59PerfMetric
	)
	// Guard: derive the ceiling from the active Roll-DPoS epoch calculator.
	// This avoids duplicating NumSubEpochs assumptions here: for example, the
	// mainnet tier uses 24 delegates × 2 sub-epochs × 24 epochs = 1152 blocks
	// before its first drain can begin. Leave one extra epoch after the
	// estimated chunks so epoch-boundary blocks skipped by the drain cannot
	// make a healthy run hit the guard.
	blocksPerEpoch := rp.GetEpochHeight(2) - rp.GetEpochHeight(1)
	eraEndHeight := rp.GetEpochHeight(tier.epochsPerEra+1) - 1
	drainEntries := uint64(tier.numVoters + tier.numDelegates)
	estimatedDrainBlocks := (drainEntries + tier.voterBudgetPerBlock - 1) / tier.voterBudgetPerBlock
	maxBlocks := int(eraEndHeight + estimatedDrainBlocks + blocksPerEpoch)
	t.Logf("perf guard: era_end=%d blocks_per_epoch=%d estimated_drain_blocks=%d max_blocks=%d",
		eraEndHeight, blocksPerEpoch, estimatedDrainBlocks, maxBlocks)

	minted := 0
	watcher := newIIP59DrainWatch()
	for minted < maxBlocks {
		blkTime = blkTime.Add(step)
		nextHeight := bc.TipHeight() + 1
		nextEpoch := rp.GetEpochNum(nextHeight)
		isFreezeBlock := nextHeight == rp.GetEpochHeight(nextEpoch) &&
			protocol.IsEraBoundary(nextEpoch, tier.epochsPerEra)
		metric, err := mintOneMeasured(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())
		minted++

		height := bc.TipHeight()
		if height == cfg.Genesis.ToBeEnabledBlockHeight {
			m := metric
			activationMetric = &m
			logIIP59PerfMetric(t, "activation-backfill", height, metric)
		}
		if isFreezeBlock && freezeMetric == nil {
			m := metric
			freezeMetric = &m
			logIIP59PerfMetric(t, "era-freeze", height, metric)
		}
		delegateIndex, voterIndex, totalDelegates, tEra, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at height %d", height)
		// Sampled here rather than at the end because completion folds each
		// delegate's residual into its distributed total, after which the bound
		// this checks is trivially tight.
		if plan, completed, planPresent, planErr := drainPlan(test.cs, cfg.Genesis, rewardProto, height); planPresent && !completed {
			r.NoErrorf(planErr, "drainPlan at height %d", height)
			watcher.observe(t, height, plan)
		} else {
			r.NoErrorf(planErr, "drainPlan at height %d", height)
		}

		if drainStartHeight == 0 && present {
			drainStartHeight = height
			drainStartEpoch = rp.GetEpochNum(height)
			t.Logf("drain begins at height=%d epoch=%d targetEra=%d", height, drainStartEpoch, tEra)
		}
		if drainStartHeight != 0 {
			chunkTimes = append(chunkTimes, metric.wall)
			chunkMetrics = append(chunkMetrics, metric)
			t.Logf("h=%d epoch=%d present=%v idx=%d/%d voterIdx=%d era=%d wall=%v",
				height, rp.GetEpochNum(height), present, delegateIndex, totalDelegates, voterIndex, tEra, metric.wall)
			if !present {
				break
			}
		}
	}
	r.NotZerof(drainStartHeight, "drain never began after %d blocks", minted)
	r.Truef(len(chunkTimes) >= 2,
		"drain must span ≥ 2 blocks to exercise chunking; got %d (tier=%s, voterBudget=%d, voters=%d)",
		len(chunkTimes), tierName, tier.voterBudgetPerBlock, tier.numVoters)

	// Confirm the drain completed cleanly: the era boundary the cursor
	// was targeting has flipped back to "no drain in progress."
	tipHeight := bc.TipHeight()
	_, _, _, _, stillDraining, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.Falsef(stillDraining, "drain still in progress at tip height %d", tipHeight)
	r.NotNil(activationMetric, "activation block was not measured")
	r.NotNil(freezeMetric, "era freeze block was not measured")

	// Latency is only worth reporting for a settlement that was correct. The
	// bench mints a real era freeze and a real drain, so the same run can carry
	// the per-voter payout assertions -- and until it did, nothing here would
	// have noticed the harness paying nobody at all.
	plan, completed, planPresent, err := drainPlan(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.True(planPresent, "the completed settlement's plan must be readable at tip")
	r.True(completed, "the settlement must be marked completed once the cursor goes absent")
	assertStressInvariant(t, test.cs, cfg.Genesis, rewardProto, seededStressAddrs(tier), tipHeight)
	checkAddrs := seededStressAddrs(tier)
	if tier.sampleVoterPayouts {
		checkAddrs = sampledIIP59PerfVoterAddrs(tier)
	}
	run := iip59DrainRun{
		tier:     tier,
		g:        cfg.Genesis,
		plan:     plan,
		balances: iip59VoterBalances(t, test.cs, cfg.Genesis, checkAddrs),
	}
	if tier.sampleVoterPayouts {
		assertIIP59SampledVoterPayouts(t, run, sampledIIP59PerfVoters(tier))
	} else {
		assertIIP59PerVoterPayouts(t, run)
	}

	total := time.Duration(0)
	for _, d := range chunkTimes {
		total += d
	}
	sorted := make([]time.Duration, len(chunkTimes))
	copy(sorted, chunkTimes)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	p50 := sorted[len(sorted)/2]
	p95 := sorted[(len(sorted)*95+99)/100-1]
	maxD := sorted[len(sorted)-1]
	var drainAlloc, drainMallocs, maxDrainHeap, maxDrainRSS uint64
	for _, metric := range chunkMetrics {
		drainAlloc += metric.totalAlloc
		drainMallocs += metric.mallocs
		if metric.heapAlloc > maxDrainHeap {
			maxDrainHeap = metric.heapAlloc
		}
		if metric.peakRSS > maxDrainRSS {
			maxDrainRSS = metric.peakRSS
		}
	}

	t.Logf("iip59 perf: tier=%s delegates=%d voters=%d native_buckets=%d contract_buckets=%d era_epochs=%d voter_budget=%d drain_blocks=%d p50=%v p95=%v max=%v total=%v drain_alloc=%.2fMiB drain_mallocs=%d max_drain_heap=%.2fMiB peak_rss=%.2fMiB",
		tierName, tier.numDelegates, tier.numVoters, tier.nativeBucketCount(), tier.numContractBuckets,
		tier.epochsPerEra, tier.voterBudgetPerBlock, len(chunkTimes), p50, p95, maxD, total,
		float64(drainAlloc)/(1<<20), drainMallocs, float64(maxDrainHeap)/(1<<20), float64(maxDrainRSS)/(1<<20))
}

// newIIP59PerfCfg derives a config from initCfg(), then overrides the
// knobs the bench needs: fork on at height 1, tier-sized delegate set,
// era cadence, chunk size, and the AutoDeposit contract address (whose
// reads are hooked out, so only the address has to parse).
func newIIP59PerfCfg(r *require.Assertions, tier perfTier) config.Config {
	cfg := config.Default
	cfg.Genesis = genesis.TestDefault()
	cfg = deepcopy.Copy(cfg).(config.Config)
	initDBPaths(r, &cfg)

	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Chain.TrieDBPatchFile = ""
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.InitBalanceMap[identityset.Address(1).String()] = "100000000000000000000000000"

	cfg.Genesis.TsunamiBlockHeight = 1
	cfg.Genesis.UpernavikBlockHeight = 2
	// Turn IIP-59 on from height 1 so the very first block already runs
	// through the new voter-reward gate. NoVoterRewardDistribution is
	// bound to !g.IsToBeEnabled(height) — see action/protocol/context.go.
	cfg.Genesis.ToBeEnabledBlockHeight = 1
	if tier.numContractBuckets > 0 {
		cfg.Genesis.XinguBlockHeight = 1
		cfg.Genesis.XinguBetaBlockHeight = 1
		cfg.Genesis.ToBeEnabledBlockHeight = 2
	}

	// Tier: shrink NumDelegates to match the seeded set; keep
	// sub-epochs small so era boundaries arrive quickly.
	cfg.Genesis.NumDelegates = uint64(tier.numDelegates)
	cfg.Genesis.NumCandidateDelegates = uint64(tier.numDelegates)
	cfg.Genesis.NumSubEpochs = 2
	cfg.Genesis.DardanellesNumSubEpochs = 2
	cfg.Genesis.WakeNumSubEpochs = 2

	cfg.Genesis.Rewarding.EpochsPerRewardEra = tier.epochsPerEra
	cfg.Genesis.Rewarding.VoterBudgetPerBlock = tier.voterBudgetPerBlock

	// Populate genesis Delegates with the perf-bench addresses so the
	// LifeLong poll protocol returns exactly the delegates the seeder
	// plants — otherwise GrantEpochReward would try to pay rewards to
	// identityset addresses that have no CandidatePollSnapshot.
	cfg.Genesis.Delegates = cfg.Genesis.Delegates[:0]
	votesPerDelegate := unit.ConvertIotxToRau(1200000).String()
	for i := 0; i < tier.numDelegates; i++ {
		addr := staking.TestOnlyPerfBenchDelegateAddress(i).String()
		cfg.Genesis.Delegates = append(cfg.Genesis.Delegates, genesis.Delegate{
			OperatorAddrStr: addr,
			RewardAddrStr:   addr,
			VotesStr:        votesPerDelegate,
		})
		cfg.Genesis.InitBalanceMap[addr] = unit.ConvertIotxToRau(2000000).String()
	}

	// Point AutoDeposit at a real bech32 address. The reader stub installed
	// below intercepts every call, so the bytecode never actually needs to
	// exist — the address only has to parse.
	//
	// DelegateProfileContractAddress stays empty on purpose: this harness
	// never reaches the contract read (see installIIP59PerfHooks), and an
	// empty address keeps the bridge nil, so if it ever did the snapshot
	// would record zero rates instead of calling into nonexistent bytecode.
	cfg.Genesis.Blockchain.AutoDepositContractAddress = identityset.Address(25).String()

	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	return cfg
}

// iip59PerfBuildOptions returns the two injection seams this bench needs,
// scoped to the one server it is passed to:
//   - a staking genesis seeder that plants the perf-bench candidates + voter
//     buckets directly in genesis, bypassing the action pool.
//   - a rewarding option swapping the AutoDeposit ContractReader for one that
//     always reports "unregistered", which routes every voter share through the
//     credit path (no compound dependency on planted bucket IDs).
//
// There is deliberately no DelegateProfile seam. The commission rates below are
// planted straight into each snapshot by the seeder, and this harness never
// reaches freezeIIP59PollSnapshot's contract read at all: it runs on
// poll.NewLifeLongDelegatesProtocol, whose single setCandidates call happens at
// genesis, where the fork gate (height 0 < ToBeEnabledBlockHeight) and the era
// gate (IsEraBoundary(1, epochsPerEra) is false for every tier) each block it
// independently. DelegateProfileContractAddress is likewise left unset, so the
// bridge stays nil and the path degrades to zero rates rather than an EVM call
// against nonexistent bytecode if that ever changes.
func iip59PerfBuildOptions(t *testing.T, tier perfTier, g genesis.Genesis) []chainservice.BuildOption {
	spec := iip59PerfSeederSpec(t, tier, g)
	adReader := newMockAutoDepositReader(t)
	return []chainservice.BuildOption{
		chainservice.WithStakingOptions(staking.WithGenesisStateSeeder(
			func(ctx context.Context, csm staking.CandidateStateManager) error {
				_, err := staking.TestOnlySeedPerfBenchState(ctx, csm, spec)
				return err
			},
		)),
		chainservice.WithRewardingOptions(rewarding.WithAutoDepositBucketReader(
			func(autodeposit.SlotReader) autodeposit.BucketReader { return adReader },
		)),
	}
}

func iip59PerfSeederSpec(t *testing.T, tier perfTier, g genesis.Genesis) staking.TestOnlyPerfBenchSpec {
	contractAddrs := make([]address.Address, 0, 3)
	if tier.numContractBuckets > 0 {
		for _, raw := range []string{
			g.SystemStakingContractAddress,
			g.SystemStakingContractV2Address,
			g.SystemStakingContractV3Address,
		} {
			contract, err := address.FromString(raw)
			require.NoError(t, err)
			contractAddrs = append(contractAddrs, contract)
		}
	}
	return staking.TestOnlyPerfBenchSpec{
		NumDelegates:               tier.numDelegates,
		NumVoters:                  tier.numVoters,
		NumNativeBuckets:           tier.nativeBucketCount(),
		NumContractBuckets:         tier.numContractBuckets,
		ContractStakingAddresses:   contractAddrs,
		ShardVoterAddresses:        tier.shardVoterAddresses,
		DeferContractBucketSeeding: tier.numContractBuckets > 0,
		DelegateSelfStake:          iip59PerfDelegateSelfStake(),
		VoterStake:                 iip59PerfVoterStake(),
		VoterStakedDurationDays:    iip59PerfVoterStakeDurationDays,
		VoteWeightCalConsts:        g.Staking.VoteWeightCalConsts,
		// 9000 bp commission (10 % voter take) on both the block-side and
		// epoch-side streams. This is the only source of the bench's rates.
		BlockCommissionBasisPoints: 9000,
		EpochCommissionBasisPoints: 9000,
	}
}

type iip59PreActivationContractSeeder struct {
	height uint64
	spec   staking.TestOnlyPerfBenchSpec
}

func (s *iip59PreActivationContractSeeder) Name() string {
	return "iip59PreActivationContractSeeder"
}

func (s *iip59PreActivationContractSeeder) Handle(
	context.Context, action.Envelope, protocol.StateManager,
) (*action.Receipt, error) {
	return nil, nil
}

func (s *iip59PreActivationContractSeeder) ReadState(
	context.Context, protocol.StateReader, []byte, ...[]byte,
) ([]byte, uint64, error) {
	return nil, 0, protocol.ErrUnimplemented
}

func (s *iip59PreActivationContractSeeder) Register(r *protocol.Registry) error {
	return r.Register(s.Name(), s)
}

func (s *iip59PreActivationContractSeeder) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(s.Name(), s)
}

func (s *iip59PreActivationContractSeeder) CreatePreStates(
	ctx context.Context,
	sm protocol.StateManager,
) error {
	if protocol.MustGetBlockCtx(ctx).BlockHeight != s.height {
		return nil
	}
	return staking.TestOnlySeedPerfBenchContractBuckets(ctx, sm, s.spec)
}

func registerIIP59PreActivationContractSeeder(
	r *require.Assertions,
	test *e2etest,
	spec staking.TestOnlyPerfBenchSpec,
	height uint64,
) {
	r.NoError((&iip59PreActivationContractSeeder{height: height, spec: spec}).ForceRegister(test.cs.Registry()))
}

// iip59EraFreezer supplies the era-boundary freeze that these harnesses cannot
// get from their poll protocol.
//
// On a real chain the freeze happens inside PutPollResult: it opens the
// copy-on-write window that pins every staking bucket at height H and stamps H
// into each delegate's poll snapshot. Both harnesses run on
// poll.NewLifeLongDelegatesProtocol, which calls setCandidates once at genesis
// and never again, so neither ever happens -- the window stays shut and every
// snapshot carries FreezeHeight 0.
//
// Before the drain became voter-major that was invisible: it paid from the
// frozen entry list in the snapshot, which the genesis seeder plants directly.
// The shard walk instead recomputes each voter's weight from the era's frozen
// buckets, so a closed window is now a hard error and a zero freeze height
// makes the delegate unpayable. This protocol reinstates the missing half of
// the boundary at the first block of every era-boundary epoch, which is where
// PutPollResult would have run.
type iip59EraFreezer struct {
	numDelegates int
	epochsPerEra uint64
}

func (f *iip59EraFreezer) Name() string { return "iip59EraFreezer" }

func (f *iip59EraFreezer) Handle(
	context.Context, action.Envelope, protocol.StateManager,
) (*action.Receipt, error) {
	return nil, nil
}

func (f *iip59EraFreezer) ReadState(
	context.Context, protocol.StateReader, []byte, ...[]byte,
) ([]byte, uint64, error) {
	return nil, 0, protocol.ErrUnimplemented
}

func (f *iip59EraFreezer) Register(r *protocol.Registry) error {
	return r.Register(f.Name(), f)
}

func (f *iip59EraFreezer) ForceRegister(r *protocol.Registry) error {
	return r.ForceRegister(f.Name(), f)
}

func (f *iip59EraFreezer) CreatePreStates(ctx context.Context, sm protocol.StateManager) error {
	if protocol.MustGetFeatureCtx(ctx).NoVoterRewardDistribution {
		return nil
	}
	height := protocol.MustGetBlockCtx(ctx).BlockHeight
	rp := rolldpos.MustGetProtocol(protocol.MustGetRegistry(ctx))
	epochNum := rp.GetEpochNum(height)
	if height != rp.GetEpochHeight(epochNum) || !protocol.IsEraBoundary(epochNum, f.epochsPerEra) {
		return nil
	}
	if err := staking.TestOnlyBeginEraCOWWindow(ctx, sm, height); err != nil {
		return err
	}
	for i := 0; i < f.numDelegates; i++ {
		id := staking.TestOnlyPerfBenchDelegateAddress(i)
		snap, err := staking.PollSnapshotFor(sm, id)
		if err != nil {
			return err
		}
		// The window and the snapshot must agree on H: the copy-on-write layer
		// resolves frozen values by the window's height, while the drain
		// evaluates weights at the snapshot's.
		snap.FreezeHeight = height
		if err := staking.TestOnlyPutPollSnapshotFor(sm, id, snap); err != nil {
			return err
		}
	}
	return nil
}

// registerIIP59EraFreezer installs the freezer alongside the epoch protocols.
// Call it in any harness that mints past an era boundary and expects the drain
// to pay anyone.
func registerIIP59EraFreezer(r *require.Assertions, test *e2etest, tier perfTier) {
	r.NoError((&iip59EraFreezer{
		numDelegates: tier.numDelegates,
		epochsPerEra: tier.epochsPerEra,
	}).ForceRegister(test.cs.Registry()))
}

// mintOne wraps createAndCommitBlock with a wall-clock measurement so
// the drain-loop can pull per-block latency out of the same call the
// harness already relies on.
func mintOne(bc blockchain.Blockchain, ap actpool.ActPool, blkTime time.Time) (time.Duration, error) {
	start := time.Now()
	blk, err := bc.MintNewBlock(blkTime)
	if err != nil {
		return time.Since(start), err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return time.Since(start), err
	}
	ap.Reset()
	return time.Since(start), nil
}

// drainSnapshot queries the rewarding protocol for chunked-drain state
// mid-flight. Returns (shardsDone, resumeVoterLen, totalDelegates,
// targetEra, present, error). resumeVoterLen is non-zero when the per-block
// voter cap stopped payout part-way through a key-space shard.
func drainSnapshot(
	cs *chainservice.ChainService,
	g genesis.Genesis,
	p *rewarding.Protocol,
	height uint64,
) (uint32, uint32, uint32, uint64, bool, error) {
	ctx := protocol.WithRegistry(context.Background(), cs.Registry())
	ctx = genesis.WithGenesisContext(ctx, g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	ctx = protocol.WithFeatureCtx(ctx)
	return p.TestOnlyEpochDrainSnapshot(ctx, cs.StateFactory())
}

// mockAutoDepositBucketReader reports every voter as "unregistered", so
// the drain routes each voter share to unclaimed balance — exercising the
// per-voter unclaimed balance credit path without depending on planted
// bucket IDs.
type mockAutoDepositBucketReader struct{}

func (mockAutoDepositBucketReader) LookupBucket(_ address.Address) (uint64, bool, error) {
	return 0, false, nil
}

// newMockAutoDepositReader returns a BucketReader that reports every
// voter as unregistered.
func newMockAutoDepositReader(_ *testing.T) autodeposit.BucketReader {
	return mockAutoDepositBucketReader{}
}

// autoDepositMinimalABI mirrors autodeposit/abi.go's abiJSON. Duplicated
// (rather than exported from the bridge package) so the bench doesn't force
// the production ABI constant into an exported surface — this is a test-only
// artifact.
const autoDepositMinimalABI = `[
	{
		"constant": true,
		"inputs": [
			{ "internalType": "address", "name": "owner", "type": "address" }
		],
		"name": "bucket",
		"outputs": [
			{ "internalType": "int256", "name": "", "type": "int256" }
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`
