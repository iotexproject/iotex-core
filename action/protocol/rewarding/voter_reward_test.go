// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/account"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/rewardingpb"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// setupVoterRewardCtx wires up a fresh testdb-backed SM + a registered
// rewarding Protocol + a context whose FeatureCtx has NoVoterRewardDistribution
// flipped by toBeEnabledHeight (0 → post-fork on, MaxUint64 → pre-fork).
// Nothing here touches the fund; distributeVoterReward is a pure per-address
// grantToAccount path, so we don't need Deposit.
func setupVoterRewardCtx(
	t *testing.T,
	toBeEnabledHeight uint64,
) (context.Context, protocol.StateManager, *Protocol) {
	t.Helper()
	ctrl := gomock.NewController(t)
	registry := protocol.NewRegistry()
	sm := testdb.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = toBeEnabledHeight
	g.Rewarding.InitBalanceStr = "0"

	rp := rolldpos.NewProtocol(
		g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(g.DardanellesBlockHeight, g.DardanellesNumSubEpochs),
	)
	require.NoError(t, rp.Register(registry))
	ap := account.NewProtocol(DepositGas)
	require.NoError(t, ap.Register(registry))
	p := NewProtocol(g.Rewarding)
	require.NoError(t, p.Register(registry))

	// Genesis / feature ctx wired at block 0 first, so CreateGenesisStates
	// runs against a pre-fork feature ctx (matches production genesis).
	ctx := protocol.WithBlockCtx(context.Background(), protocol.BlockCtx{BlockHeight: 0})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
	ctx = genesis.WithGenesisContext(protocol.WithRegistry(ctx, registry), g)
	ctx = protocol.WithFeatureCtx(ctx)
	require.NoError(t, ap.CreateGenesisStates(ctx, sm))
	require.NoError(t, p.CreateGenesisStates(ctx, sm))

	// Now switch to an execution block context. When toBeEnabledHeight == 0
	// the FeatureCtx at this height has NoVoterRewardDistribution=false.
	execHeight := uint64(1)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight: execHeight,
		Producer:    identityset.Address(0),
	})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller: identityset.Address(0),
	})
	ctx = protocol.WithFeatureCtx(ctx)
	return ctx, sm, p
}

// decodeRewardLog re-parses a receipt Data blob so tests can assert on the
// enum + address + amount without duplicating rewardingpb import in every case.
func decodeRewardLog(t *testing.T, data []byte) *rewardingpb.RewardLog {
	t.Helper()
	rl := &rewardingpb.RewardLog{}
	require.NoError(t, proto.Unmarshal(data, rl))
	return rl
}

func unclaimed(t *testing.T, ctx context.Context, p *Protocol, sm protocol.StateManager, addr address.Address) *big.Int {
	t.Helper()
	bal, _, err := p.UnclaimedBalance(ctx, sm, addr)
	require.NoError(t, err)
	return bal
}

// TestDistributeVoterReward_LegacyFallback covers every path where
// distributeVoterReward must return (nil, false, nil) so the caller runs the
// pre-IIP-59 EPOCH_REWARD grant.
func TestDistributeVoterReward_LegacyFallback(t *testing.T) {
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)

	cases := []struct {
		name string
		// mutator lets each case tweak the base state.Candidate.
		mutate func(c *state.Candidate)
		// override lets a case override ToBeEnabledBlockHeight (default 0 = flag on).
		toBeEnabledHeight uint64
	}{
		{
			name:              "feature flag off",
			mutate:            func(c *state.Candidate) {},
			toBeEnabledHeight: math.MaxUint64, // pre-fork
		},
		{
			name:              "nil candidate",
			mutate:            func(c *state.Candidate) { *c = state.Candidate{} }, // will be nil-checked as zero rate
			toBeEnabledHeight: 0,
		},
		{
			name:              "zero commission rate",
			mutate:            func(c *state.Candidate) { c.CommissionRate = 0 },
			toBeEnabledHeight: 0,
		},
		{
			name:              "empty identity",
			mutate:            func(c *state.Candidate) { c.CommissionRate = 500; c.Identity = "" },
			toBeEnabledHeight: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, sm, p := setupVoterRewardCtx(t, tc.toBeEnabledHeight)
			cand := &state.Candidate{
				Address:        identityset.Address(2).String(),
				RewardAddress:  rewardAddr.String(),
				Identity:       candIdentity.String(),
				CommissionRate: 500, // 5%
			}
			tc.mutate(cand)
			logs, handled, err := p.distributeVoterReward(
				ctx, sm, cand, rewardAddr, big.NewInt(1000), 42, hash.ZeroHash256,
			)
			require.NoError(t, err)
			require.False(t, handled)
			require.Nil(t, logs)
			// Nothing was granted — caller runs legacy path.
			require.Equal(t, big.NewInt(0), unclaimed(t, ctx, p, sm, rewardAddr))
		})
	}
}

// TestDistributeVoterReward_InvalidInput covers hard errors.
func TestDistributeVoterReward_InvalidInput(t *testing.T) {
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)

	t.Run("commission rate above denominator", func(t *testing.T) {
		ctx, sm, p := setupVoterRewardCtx(t, 0)
		cand := &state.Candidate{
			RewardAddress:  rewardAddr.String(),
			Identity:       candIdentity.String(),
			CommissionRate: commissionRateDenominator + 1,
		}
		_, _, err := p.distributeVoterReward(
			ctx, sm, cand, rewardAddr, big.NewInt(1000), 1, hash.ZeroHash256,
		)
		require.Error(t, err)
	})

	t.Run("unparseable identity", func(t *testing.T) {
		ctx, sm, p := setupVoterRewardCtx(t, 0)
		cand := &state.Candidate{
			RewardAddress:  rewardAddr.String(),
			Identity:       "not-a-real-address",
			CommissionRate: 100,
		}
		_, _, err := p.distributeVoterReward(
			ctx, sm, cand, rewardAddr, big.NewInt(1000), 1, hash.ZeroHash256,
		)
		require.Error(t, err)
	})

	t.Run("nil reward address", func(t *testing.T) {
		ctx, sm, p := setupVoterRewardCtx(t, 0)
		cand := &state.Candidate{
			Identity:       candIdentity.String(),
			CommissionRate: 100,
		}
		_, _, err := p.distributeVoterReward(
			ctx, sm, cand, nil, big.NewInt(1000), 1, hash.ZeroHash256,
		)
		require.Error(t, err)
	})
}

// TestDistributeVoterReward_NoVoters exercises both "no snapshot" and
// "snapshot with all-zero weights". In both cases the delegate gets the full
// totalReward and a single DELEGATE_COMMISSION log is emitted.
func TestDistributeVoterReward_NoVoters(t *testing.T) {
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	cand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 500, // 5%; irrelevant when totalWeight==0
	}

	t.Run("no snapshot at all", func(t *testing.T) {
		ctx, sm, p := setupVoterRewardCtx(t, 0)
		logs, handled, err := p.distributeVoterReward(
			ctx, sm, cand, rewardAddr, big.NewInt(1000), 1, hash.ZeroHash256,
		)
		require.NoError(t, err)
		require.True(t, handled)
		require.Len(t, logs, 1)
		rl := decodeRewardLog(t, logs[0].Data)
		require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, rl.Type)
		require.Equal(t, rewardAddr.String(), rl.Addr)
		require.Equal(t, "1000", rl.Amount)
		require.Equal(t, big.NewInt(1000), unclaimed(t, ctx, p, sm, rewardAddr))
	})

	t.Run("snapshot with all zero weights", func(t *testing.T) {
		ctx, sm, p := setupVoterRewardCtx(t, 0)
		voters := []address.Address{identityset.Address(20), identityset.Address(21)}
		weights := []*big.Int{big.NewInt(0), big.NewInt(0)}
		require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

		logs, handled, err := p.distributeVoterReward(
			ctx, sm, cand, rewardAddr, big.NewInt(1000), 1, hash.ZeroHash256,
		)
		require.NoError(t, err)
		require.True(t, handled)
		require.Len(t, logs, 1)
		require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, decodeRewardLog(t, logs[0].Data).Type)
		require.Equal(t, big.NewInt(1000), unclaimed(t, ctx, p, sm, rewardAddr))
	})
}

// TestDistributeVoterReward_FullCommission covers a 100% commission rate: the
// delegate takes everything, no voter logs.
func TestDistributeVoterReward_FullCommission(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	cand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: commissionRateDenominator, // 100%
	}
	voters := []address.Address{identityset.Address(20), identityset.Address(21)}
	weights := []*big.Int{big.NewInt(100), big.NewInt(200)}
	require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

	logs, handled, err := p.distributeVoterReward(
		ctx, sm, cand, rewardAddr, big.NewInt(1000), 1, hash.ZeroHash256,
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, logs, 1)
	rl := decodeRewardLog(t, logs[0].Data)
	require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, rl.Type)
	require.Equal(t, "1000", rl.Amount)
	require.Equal(t, big.NewInt(1000), unclaimed(t, ctx, p, sm, rewardAddr))
	require.Equal(t, big.NewInt(0), unclaimed(t, ctx, p, sm, voters[0]))
	require.Equal(t, big.NewInt(0), unclaimed(t, ctx, p, sm, voters[1]))
}

// TestDistributeVoterReward_EvenSplit exercises the happy path with a clean
// division: commissionRate=1000 (10%), totalReward=1_000_000, three equal
// voters. Verifies:
//   - one VOTER_REWARD log per voter, in snapshot (sorted) order
//   - a single DELEGATE_COMMISSION log with the full commission + dust
//   - per-address balances match the split
//   - conservation: commission + Σ(voter shares) == totalReward
func TestDistributeVoterReward_EvenSplit(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	cand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 1000, // 10%
	}
	// weights sum to 300 → each voter gets exactly voterPool/3 = 300_000.
	// commission = 1_000_000 * 1000 / 10000 = 100_000.
	// voterPool  = 900_000; distributed = 900_000; dust = 0.
	voters := []address.Address{identityset.Address(20), identityset.Address(21), identityset.Address(22)}
	weights := []*big.Int{big.NewInt(100), big.NewInt(100), big.NewInt(100)}
	require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

	totalReward := big.NewInt(1_000_000)
	logs, handled, err := p.distributeVoterReward(
		ctx, sm, cand, rewardAddr, totalReward, 42, hash.ZeroHash256,
	)
	require.NoError(t, err)
	require.True(t, handled)

	// 3 VOTER_REWARD + 1 DELEGATE_COMMISSION
	require.Len(t, logs, 4)
	sum := big.NewInt(0)
	for i := 0; i < 3; i++ {
		rl := decodeRewardLog(t, logs[i].Data)
		require.Equal(t, rewardingpb.RewardLog_VOTER_REWARD, rl.Type)
		require.Equal(t, "300000", rl.Amount)
		amt, ok := new(big.Int).SetString(rl.Amount, 10)
		require.True(t, ok)
		sum = new(big.Int).Add(sum, amt)
	}
	commLog := decodeRewardLog(t, logs[3].Data)
	require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, commLog.Type)
	require.Equal(t, rewardAddr.String(), commLog.Addr)
	require.Equal(t, "100000", commLog.Amount)
	commAmt, _ := new(big.Int).SetString(commLog.Amount, 10)
	require.Equal(t, totalReward, new(big.Int).Add(sum, commAmt))

	// balances match
	require.Equal(t, big.NewInt(100_000), unclaimed(t, ctx, p, sm, rewardAddr))
	require.Equal(t, big.NewInt(300_000), unclaimed(t, ctx, p, sm, voters[0]))
	require.Equal(t, big.NewInt(300_000), unclaimed(t, ctx, p, sm, voters[1]))
	require.Equal(t, big.NewInt(300_000), unclaimed(t, ctx, p, sm, voters[2]))
}

// TestDistributeVoterReward_DustFolding: unequal weights that don't divide
// cleanly. Dust from truncation must flow back to the delegate, and total
// reward must be conserved to the wei.
func TestDistributeVoterReward_DustFolding(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	cand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 500, // 5%
	}
	// commission = 1000 * 500 / 10000 = 50
	// voterPool  = 950
	// weights sum = 7. 950/7 = 135 rem 5.
	// voter shares (floor(950 * w / 7)):
	//   w=1: 135, w=2: 271, w=4: 542  → sum = 948, dust = 2
	// delegate payout = 50 + 2 = 52
	voters := []address.Address{identityset.Address(20), identityset.Address(21), identityset.Address(22)}
	weights := []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(4)}
	require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

	totalReward := big.NewInt(1000)
	logs, handled, err := p.distributeVoterReward(
		ctx, sm, cand, rewardAddr, totalReward, 1, hash.ZeroHash256,
	)
	require.NoError(t, err)
	require.True(t, handled)
	require.Len(t, logs, 4)

	// Voter reward logs come out in snapshot (sorted-by-voter-bytes) order.
	// Snapshot sorts by voter.Bytes(); we don't hard-code which weight lands
	// where by index — instead we index the logs by address string and check
	// their amounts against the computed shares.
	voterPool := big.NewInt(950)
	totalWeight := big.NewInt(7)
	distributed := big.NewInt(0)
	got := map[string]*big.Int{}
	for i := 0; i < 3; i++ {
		rl := decodeRewardLog(t, logs[i].Data)
		require.Equal(t, rewardingpb.RewardLog_VOTER_REWARD, rl.Type)
		amt, ok := new(big.Int).SetString(rl.Amount, 10)
		require.True(t, ok)
		got[rl.Addr] = amt
		distributed = new(big.Int).Add(distributed, amt)
	}
	for i, v := range voters {
		expected := new(big.Int).Div(new(big.Int).Mul(voterPool, weights[i]), totalWeight)
		require.Equal(t, expected, got[v.String()], "voter %d share", i)
	}
	// Conservation: totalReward == commission + Σ voter shares + dust; and
	// dust is folded into the single DELEGATE_COMMISSION log.
	commLog := decodeRewardLog(t, logs[3].Data)
	require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, commLog.Type)
	require.Equal(t, rewardAddr.String(), commLog.Addr)
	commAmt, _ := new(big.Int).SetString(commLog.Amount, 10)
	total := new(big.Int).Add(distributed, commAmt)
	require.Equal(t, totalReward, total)
	// commission = totalReward*rate/denom = 50; dust = 2 → delegate = 52.
	require.Equal(t, big.NewInt(52), commAmt)
	require.Equal(t, big.NewInt(2), new(big.Int).Sub(commAmt, big.NewInt(50)))
	// balances match log amounts
	require.Equal(t, commAmt, unclaimed(t, ctx, p, sm, rewardAddr))
	for i, v := range voters {
		require.Equal(t, got[v.String()], unclaimed(t, ctx, p, sm, v), "voter %d balance", i)
	}
}

// TestDistributeVoterReward_ZeroWeightVoterSkipped: a voter with weight 0
// sitting inside a snapshot that also has positive-weight entries must be
// skipped (no VOTER_REWARD log for it) but the split among the remaining
// positive-weight voters must still add up to voterPool.
func TestDistributeVoterReward_ZeroWeightVoterSkipped(t *testing.T) {
	ctx, sm, p := setupVoterRewardCtx(t, 0)
	rewardAddr := identityset.Address(1)
	candIdentity := identityset.Address(10)
	cand := &state.Candidate{
		RewardAddress:  rewardAddr.String(),
		Identity:       candIdentity.String(),
		CommissionRate: 0, // 0% wouldn't hit IIP-59 path… so use 100 (1%).
	}
	cand.CommissionRate = 100
	voters := []address.Address{identityset.Address(20), identityset.Address(21), identityset.Address(22)}
	weights := []*big.Int{big.NewInt(50), big.NewInt(0), big.NewInt(50)}
	require.NoError(t, staking.WriteVoterWeightSnapshotForTest(sm, candIdentity, voters, weights))

	totalReward := big.NewInt(10_000)
	logs, handled, err := p.distributeVoterReward(
		ctx, sm, cand, rewardAddr, totalReward, 1, hash.ZeroHash256,
	)
	require.NoError(t, err)
	require.True(t, handled)
	// 2 non-zero voters + 1 delegate commission log.
	require.Len(t, logs, 3)
	for i := 0; i < 2; i++ {
		rl := decodeRewardLog(t, logs[i].Data)
		require.Equal(t, rewardingpb.RewardLog_VOTER_REWARD, rl.Type)
		require.NotEqual(t, identityset.Address(21).String(), rl.Addr, "zero-weight voter must be skipped")
	}
	commLog := decodeRewardLog(t, logs[2].Data)
	require.Equal(t, rewardingpb.RewardLog_DELEGATE_COMMISSION, commLog.Type)
	// commission = 100; voterPool = 9900; each of 2 voters gets 4950.
	require.Equal(t, big.NewInt(4950), unclaimed(t, ctx, p, sm, voters[0]))
	require.Equal(t, big.NewInt(0), unclaimed(t, ctx, p, sm, voters[1]))
	require.Equal(t, big.NewInt(4950), unclaimed(t, ctx, p, sm, voters[2]))
	require.Equal(t, big.NewInt(100), unclaimed(t, ctx, p, sm, rewardAddr))
}
