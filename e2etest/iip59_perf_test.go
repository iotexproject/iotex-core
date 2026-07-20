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
	"github.com/mohae/deepcopy"
	"github.com/stretchr/testify/require"

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
// scale (delegates × voters), era cadence (epochs per era), and the two
// per-block caps: compoundBatchSize (delegates paid per block) and
// voterBudgetPerBlock (voters paid per block, mid-delegate resumable).
// Both caps are picked deliberately low so the drain spans multiple
// blocks — otherwise the bench collapses to a single-block run and
// stops measuring what we care about (chunking behaviour).
//
// voterBudgetPerBlock=0 keeps the pre-5.5b behaviour (delegate-count
// cap only, entire voter list paid inside one block for each delegate).
type perfTier struct {
	numDelegates        int
	numVoters           int
	epochsPerEra        uint64
	compoundBatchSize   uint64
	voterBudgetPerBlock uint64
}

var iip59PerfTiers = map[string]perfTier{
	"small":   {numDelegates: 3, numVoters: 100, epochsPerEra: 2, compoundBatchSize: 2},
	"medium":  {numDelegates: 10, numVoters: 1_000, epochsPerEra: 4, compoundBatchSize: 4},
	"mainnet": {numDelegates: 24, numVoters: 27_020, epochsPerEra: 24, compoundBatchSize: 4},
}

const iip59PerfTierEnv = "IIP59_PERF_TIER"

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
	// Guard: bound total block count so a broken test can't spin
	// forever. small tier era = 3 delegates × 2 sub-epochs × 2 epochs
	// = 12 blocks per era; mainnet is 24 × 15 × 24 = 8640 blocks per
	// era. Give each tier a comfortable ceiling.
	maxBlocks := 500
	if tier.numVoters > 500 {
		maxBlocks = 20 * tier.numVoters / int(tier.compoundBatchSize)
		if maxBlocks < 1000 {
			maxBlocks = 1000
		}
	}

	minted := 0
	for minted < maxBlocks {
		blkTime = blkTime.Add(step)
		wall, err := mintOne(bc, ap, blkTime)
		r.NoErrorf(err, "mint at height %d", bc.TipHeight())
		minted++

		height := bc.TipHeight()
		delegateIndex, voterIndex, totalDelegates, tEra, present, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, height)
		r.NoErrorf(err, "drainSnapshot at height %d", height)

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
		"drain must span ≥ 2 blocks to exercise chunking; got %d (tier=%s, batch=%d, voters=%d)",
		len(chunkTimes), tierName, tier.compoundBatchSize, tier.numVoters)

	// Confirm the drain completed cleanly: the era boundary the cursor
	// was targeting has flipped back to "no drain in progress."
	tipHeight := bc.TipHeight()
	_, _, _, _, stillDraining, err := drainSnapshot(test.cs, cfg.Genesis, rewardProto, tipHeight)
	r.NoError(err)
	r.Falsef(stillDraining, "drain still in progress at tip height %d", tipHeight)

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

	t.Logf("iip59 perf: tier=%s delegates=%d voters=%d era_epochs=%d batch=%d drain_blocks=%d p50=%v p95=%v max=%v total=%v",
		tierName, tier.numDelegates, tier.numVoters, tier.epochsPerEra, tier.compoundBatchSize,
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
	cfg.Genesis.Rewarding.CompoundBatchSize = tier.compoundBatchSize
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
		DelegateSelfStake:       unit.ConvertIotxToRau(1_200_000),
		VoterStake:              unit.ConvertIotxToRau(1_000),
		VoterStakedDurationDays: 30,
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
		rewarding.WithAutoDepositReader(func(_ protocol.StateManager) autodeposit.ContractReader {
			return adReader
		}),
	}
	t.Cleanup(func() { chainservice.TestOnlyRewardingOptions = nil })
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
// mid-flight. Returns (delegateIndex, voterIndex, totalDelegates,
// targetEra, present, error). voterIndex is the mid-delegate resume
// position when the per-block voter cap stops payout inside a delegate;
// 0 otherwise.
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

// newMockAutoDepositReader returns a ContractReader that reports every
// voter as "unregistered" (bucket = 0). The drain then routes each
// voter share via RouteCredit, exercising the per-voter unclaimed
// balance credit path without depending on planted bucket IDs.
func newMockAutoDepositReader(t *testing.T) autodeposit.ContractReader {
	parsed, err := abi.JSON(strings.NewReader(autoDepositMinimalABI))
	require.NoError(t, err)
	method, ok := parsed.Methods["bucket"]
	require.True(t, ok, "bucket must be present in mock ABI")
	packed, err := method.Outputs.Pack(big.NewInt(0))
	require.NoError(t, err)
	return autodeposit.ContractReaderFunc(func(_ context.Context, _ string, _ []byte) ([]byte, error) {
		return packed, nil
	})
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
