// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"math/big"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/iotexproject/iotex-address/address"
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/poll"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/delegateprofile"
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
	epochsPerEra        uint64
	voterBudgetPerBlock uint64
}

var iip59PerfTiers = map[string]perfTier{
	"small":   {numDelegates: 3, numVoters: 100, epochsPerEra: 2, voterBudgetPerBlock: 50},
	"medium":  {numDelegates: 10, numVoters: 1_000, epochsPerEra: 4, voterBudgetPerBlock: 250},
	"mainnet": {numDelegates: 24, numVoters: 27_020, epochsPerEra: 24, voterBudgetPerBlock: 4_504},
}

const iip59PerfTierEnv = "IIP59_PERF_TIER"

// The stake parameters every seeded bucket gets. They are package-level rather
// than inline in installIIP59PerfHooks because the payout assertions recompute
// vote weights from them: if the two ever disagreed, the assertions would be
// checking the drain against a fixture that does not exist.
const iip59PerfVoterStakeDurationDays uint32 = 30

func iip59PerfDelegateSelfStake() *big.Int { return unit.ConvertIotxToRau(1_200_000) }
func iip59PerfVoterStake() *big.Int        { return unit.ConvertIotxToRau(1_000) }

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
		t.Fatalf("unknown %s=%q; want small|medium|mainnet", iip59PerfTierEnv, tierName)
	}
	if tierName == "mainnet" && testing.Short() {
		t.Skipf("skipping mainnet tier under -short")
	}

	cfg := newIIP59PerfCfg(r, tier)
	defer clearDBPaths(&cfg)

	// Install the three test-only injection seams. All three are
	// package-level globals; reset them on cleanup so subsequent tests
	// see a virgin state.
	installIIP59PerfHooks(t, tier, cfg.Genesis)

	test := newE2ETest(t, cfg)
	defer test.teardown()
	registerEpochProtocols(r, test)
	registerIIP59EraFreezer(r, test, tier)

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
		wall, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())
		minted++

		height := bc.TipHeight()
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
			chunkTimes = append(chunkTimes, wall)
			t.Logf("h=%d epoch=%d present=%v idx=%d/%d voterIdx=%d era=%d wall=%v",
				height, rp.GetEpochNum(height), present, delegateIndex, totalDelegates, voterIndex, tEra, wall)
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

	// Latency is only worth reporting for a settlement that was correct. The
	// bench mints a real era freeze and a real drain, so the same run can carry
	// the per-voter payout assertions -- and until it did, nothing here would
	// have noticed the harness paying nobody at all.
	plan, completed, planPresent, err := drainPlan(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.True(planPresent, "the completed settlement's plan must be readable at tip")
	r.True(completed, "the settlement must be marked completed once the cursor goes absent")
	assertIIP59PerVoterPayouts(t, iip59DrainRun{
		tier:     tier,
		g:        cfg.Genesis,
		plan:     plan,
		balances: iip59VoterBalances(t, test.cs, cfg.Genesis, seededStressAddrs(tier)),
	})

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

	t.Logf("iip59 perf: tier=%s delegates=%d voters=%d era_epochs=%d voter_budget=%d drain_blocks=%d p50=%v p95=%v max=%v total=%v",
		tierName, tier.numDelegates, tier.numVoters, tier.epochsPerEra, tier.voterBudgetPerBlock,
		len(chunkTimes), p50, p95, maxD, total)
}

// newIIP59PerfCfg derives a config from initCfg(), then overrides the
// knobs the bench needs: fork on at height 1, tier-sized delegate set,
// era cadence, chunk size, mock contract addresses (so the DelegateProfile
// bridge is exercised end-to-end even though the reads are hooked out).
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

	// Point IIP-59's on-chain contract fields at real bech32 addresses.
	// The reader stubs installed below intercept every call, so the
	// bytecode never actually needs to exist — the addresses only have
	// to parse.
	cfg.Genesis.Blockchain.AutoDepositContractAddress = identityset.Address(25).String()
	cfg.Genesis.Poll.DelegateProfileContractAddress = identityset.Address(26).String()

	testutil.NormalizeGenesisHeights(&cfg.Genesis.Blockchain)
	return cfg
}

// installIIP59PerfHooks wires the three test-only injection seams:
//   - staking.TestOnlyGenesisStateSeeder — plants the perf-bench candidates + voter buckets
//     directly in genesis, bypassing the action pool.
//   - poll.TestOnlyDelegateProfileReaderFactory — returns a canned
//     commission-portion payload so PutPollResult's DelegateProfile
//     freeze doesn't need real contract bytecode.
//   - chainservice.TestOnlyRewardingOptions — swaps the AutoDeposit
//     ContractReader for one that always reports "unregistered", which
//     routes every voter share through the credit path (no compound
//     dependency on planted bucket IDs).
func installIIP59PerfHooks(t *testing.T, tier perfTier, g genesis.Genesis) {
	spec := staking.TestOnlyPerfBenchSpec{
		NumDelegates:            tier.numDelegates,
		NumVoters:               tier.numVoters,
		DelegateSelfStake:       iip59PerfDelegateSelfStake(),
		VoterStake:              iip59PerfVoterStake(),
		VoterStakedDurationDays: iip59PerfVoterStakeDurationDays,
		VoteWeightCalConsts:     g.Staking.VoteWeightCalConsts,
		// Match the mock DelegateProfile reader below: raw payload = 1000 bp
		// (voter portion) → commission = 10000 - 1000 = 9000 bp for both
		// block-side and epoch-side streams.
		BlockCommissionBasisPoints: 9000,
		EpochCommissionBasisPoints: 9000,
	}
	staking.TestOnlyGenesisStateSeeder = func(ctx context.Context, csm staking.CandidateStateManager) error {
		_, err := staking.TestOnlySeedPerfBenchState(ctx, csm, spec)
		return err
	}
	t.Cleanup(func() { staking.TestOnlyGenesisStateSeeder = nil })

	dpReader := newMockDelegateProfileReader(t)
	poll.TestOnlyDelegateProfileReaderFactory = func(_ protocol.StateManager) delegateprofile.ContractReader {
		return dpReader
	}
	t.Cleanup(func() { poll.TestOnlyDelegateProfileReaderFactory = nil })

	adReader := newMockAutoDepositReader(t)
	chainservice.TestOnlyRewardingOptions = []rewarding.Option{
		rewarding.WithAutoDepositBucketReader(func(autodeposit.SlotReader) autodeposit.BucketReader {
			return adReader
		}),
	}
	t.Cleanup(func() { chainservice.TestOnlyRewardingOptions = nil })
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

// newMockDelegateProfileReader returns a ContractReader that answers
// every getProfileByField(_, _) call with an ABI-encoded 1000 (10 %
// voter take → 90 % commission). Constant across all delegates and
// both portion fields — enough for the drain-chunking cost we're
// measuring; per-delegate variance isn't the point of this bench.
func newMockDelegateProfileReader(t *testing.T) delegateprofile.ContractReader {
	parsed, err := abi.JSON(strings.NewReader(delegateProfileMinimalABI))
	require.NoError(t, err)
	// Encode raw payload as big-endian uint256 with basis points = 1000.
	payload := new(big.Int).SetUint64(1000).Bytes()
	method, ok := parsed.Methods["getProfileByField"]
	require.True(t, ok, "getProfileByField must be present in mock ABI")
	packed, err := method.Outputs.Pack(payload)
	require.NoError(t, err)
	return delegateprofile.ContractReaderFunc(func(_ context.Context, _ string, _ []byte) ([]byte, error) {
		return packed, nil
	})
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

// delegateProfileMinimalABI mirrors delegateprofile/abi.go's abiJSON.
// Duplicated (rather than exported from the bridge package) so the
// bench doesn't force the production ABI constant into an exported
// surface — this is a test-only artifact.
const delegateProfileMinimalABI = `[
	{
		"constant": true,
		"inputs": [
			{ "name": "_delegate", "type": "address" },
			{ "name": "_field", "type": "string" }
		],
		"name": "getProfileByField",
		"outputs": [
			{ "name": "", "type": "bytes" }
		],
		"payable": false,
		"stateMutability": "view",
		"type": "function"
	}
]`

// autoDepositMinimalABI mirrors autodeposit/abi.go's abiJSON. Same
// duplication rationale as delegateProfileMinimalABI.
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
