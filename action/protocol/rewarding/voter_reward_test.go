// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rewarding/autodeposit"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/action/protocol/staking"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/testutil/testdb"
)

// TestSplitCommission covers the pure basis-points helper. The rewarding
// path relies on splitCommission truncating in favour of the voter pool and
// clamping malformed on-chain rates so a corrupted DelegateProfile value
// cannot over-pay commission.
func TestSplitCommission(t *testing.T) {
	cases := []struct {
		name       string
		total      *big.Int
		bps        uint64
		wantComm   *big.Int
		wantVoters *big.Int
	}{
		{"zero total", big.NewInt(0), 1000, big.NewInt(0), big.NewInt(0)},
		{"nil total", nil, 1000, big.NewInt(0), big.NewInt(0)},
		{"zero bps", big.NewInt(1000), 0, big.NewInt(0), big.NewInt(1000)},
		{"ten percent", big.NewInt(1000), 1000, big.NewInt(100), big.NewInt(900)},
		{"truncation favours voters", big.NewInt(3), 1000, big.NewInt(0), big.NewInt(3)},
		{"exactly 100 percent clamps", big.NewInt(1000), 10_000, big.NewInt(1000), big.NewInt(0)},
		{"over 100 percent clamps", big.NewInt(1000), 20_000, big.NewInt(1000), big.NewInt(0)},
		{"one bp", big.NewInt(10_000), 1, big.NewInt(1), big.NewInt(9_999)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			comm, voters := splitCommission(tc.total, tc.bps)
			r.Equal(0, comm.Cmp(tc.wantComm), "commission: got %s want %s", comm.String(), tc.wantComm.String())
			r.Equal(0, voters.Cmp(tc.wantVoters), "voter pool: got %s want %s", voters.String(), tc.wantVoters.String())
		})
	}
}

// TestSplitDelegateEpochReward covers the fallback branches that route the
// full amount to commission (voter share = 0), and the happy-path split.
// Fallback cases must return (amount, 0) so GrantEpochReward's caller runs
// the legacy per-delegate grant unchanged.
func TestSplitDelegateEpochReward(t *testing.T) {
	amount := big.NewInt(1_000)

	t.Run("fork off", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, false /* iip59On */)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("nil candidate", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, nil, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("zero amount", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, true)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, big.NewInt(0))
		r.NoError(err)
		r.Equal(0, c.Sign())
		r.Equal(0, v.Sign())
	})

	t.Run("negative amount rejected", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, true)
		_, _, err := p.splitDelegateEpochReward(ctx, sm, cand, big.NewInt(-1))
		r.Error(err)
	})

	t.Run("no snapshot fallback", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, _ := newVoterRewardCtx(t, true)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("opted out fallback", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, false /* optIn */, true /* registered */, 2000, []voterEntry{{identityset.Address(3), big.NewInt(100)}})
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("unregistered fallback", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, true, false, 2000, []voterEntry{{identityset.Address(3), big.NewInt(100)}})
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("empty voters fallback", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, true, true, 2000, nil)
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(amount))
		r.Equal(0, v.Sign())
	})

	t.Run("happy path 20 percent commission", func(t *testing.T) {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true)
		writeSnapshot(t, sm, candAddr, true, true, 2000, []voterEntry{{identityset.Address(3), big.NewInt(100)}})
		c, v, err := p.splitDelegateEpochReward(ctx, sm, cand, amount)
		r.NoError(err)
		r.Equal(0, c.Cmp(big.NewInt(200)))
		r.Equal(0, v.Cmp(big.NewInt(800)))
	})
}

type voterEntry struct {
	addr   address.Address
	weight *big.Int
}

func writeSnapshot(
	t *testing.T,
	sm protocol.StateManager,
	candAddr address.Address,
	optIn bool,
	registered bool,
	epochBps uint64,
	voters []voterEntry,
) {
	t.Helper()
	entries := make([]staking.VoterWeight, len(voters))
	for i, v := range voters {
		entries[i] = staking.VoterWeight{Voter: v.addr, Weight: v.weight}
	}
	snap := &staking.CandidatePollSnapshot{
		VoterRewardOnchainOptIn:    optIn,
		Registered:                 registered,
		BlockCommissionBasisPoints: epochBps,
		EpochCommissionBasisPoints: epochBps,
		Entries:                    entries,
	}
	require.NoError(t, staking.TestOnlyPutPollSnapshotFor(sm, candAddr, snap))
}

// newVoterRewardCtx wires the minimum context splitDelegateEpochReward reads:
// a StateManager, registered rolldpos+staking protocols, and feature ctx
// toggled by iip59On.
func newVoterRewardCtx(
	t *testing.T,
	iip59On bool,
) (context.Context, protocol.StateManager, *Protocol, *state.Candidate, address.Address) {
	t.Helper()
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	if iip59On {
		g.ToBeEnabledBlockHeight = 1
	} else {
		g.ToBeEnabledBlockHeight = 1_000_000_000
	}

	registry := protocol.NewRegistry()
	rp := rolldpos.NewProtocol(g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs)
	r.NoError(rp.Register(registry))

	stakingCfg := &staking.BuilderConfig{
		Staking: g.Staking,
		Revise: staking.ReviseConfig{
			VoteWeight: g.VoteWeightCalConsts,
		},
	}
	stakingProtocol, err := staking.NewProtocol(
		staking.HelperCtx{}, stakingCfg, nil, nil, nil, nil,
	)
	r.NoError(err)
	r.NoError(stakingProtocol.Register(registry))

	p := NewProtocol(g.Rewarding)
	r.NoError(p.Register(registry))

	candAddr := identityset.Address(1)
	cand := &state.Candidate{
		Address:       candAddr.String(),
		RewardAddress: identityset.Address(2).String(),
		Votes:         big.NewInt(1_000_000),
	}

	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithRegistry(ctx, registry)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 100})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{Caller: identityset.Address(0)})
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(ctx)

	view, err := stakingProtocol.Start(ctx, sm)
	r.NoError(err)
	r.NoError(sm.WriteView("staking", view))

	return ctx, sm, p, cand, candAddr
}

// TestDistributeVoterOnly_WindowedDeterminism is IIP-59 PR 5.5b's core
// determinism claim: splitting one delegate's voter payout across
// multiple blocks via VoterBudgetPerBlock produces byte-identical
// per-voter payouts vs a single unbounded call.
//
// Setup: 20 voters with weights 1..20 (total 210), voterAmount=100_000
// → varying per-voter shares plus dust on the last-weighted voter.
// Reference: one distributeVoterOnly call with voterBudget=0 (unbounded).
// Chunked: three distributeVoterOnly calls with voterBudget=7 →
// windows (7, 7, 6). Both fixtures use identical snapshots.
//
// Invariants:
//   - Each chunked call reports consumed matching its window size.
//   - totalVoters=20 across all calls.
//   - Sum of chunked `paid` values equals reference `paid` (which equals
//     voterAmount exactly — allocation is dust-conserving).
//   - Per-voter unclaimed balance after the chunked run matches the
//     reference run's balance byte-for-byte (allocation is deterministic
//     across windows: dust lands in the same voter regardless of which
//     chunk contains them).
func TestDistributeVoterOnly_WindowedDeterminism(t *testing.T) {
	const numVoters = 20
	voterAmount := big.NewInt(100_000)
	epochCommission := big.NewInt(5_000)
	epochBps := uint64(500) // 5% — irrelevant for distributeVoterOnly but keeps the snapshot valid

	voters := make([]voterEntry, numVoters)
	for i := 0; i < numVoters; i++ {
		voters[i] = voterEntry{
			addr:   identityset.Address(3 + i),
			weight: big.NewInt(int64(i + 1)),
		}
	}

	// dumpBalances reads each voter's unclaimed balance so the caller can
	// compare per-voter payouts across two independent fixtures.
	dumpBalances := func(t *testing.T, ctx context.Context, sm protocol.StateReader, p *Protocol) []*big.Int {
		t.Helper()
		r := require.New(t)
		out := make([]*big.Int, numVoters)
		for i, v := range voters {
			bal, _, err := p.UnclaimedBalance(ctx, sm, v.addr)
			r.NoError(err, "read voter %d balance", i)
			out[i] = bal
		}
		return out
	}

	// Reference fixture: single unbounded call.
	var refPaid *big.Int
	var refBalances []*big.Int
	func() {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true /* iip59On */)
		writeSnapshot(t, sm, candAddr, true /* optIn */, true /* registered */, epochBps, voters)
		rewardAddr, err := address.FromString(cand.RewardAddress)
		r.NoError(err)

		logs, routed, paid, consumed, total, err := p.distributeVoterOnly(
			ctx, sm, cand, rewardAddr,
			voterAmount, epochCommission,
			0 /* startVoter */, 0, /* voterBudget=unbounded */
			100 /* blkHeight */, hash.ZeroHash256,
		)
		r.NoError(err)
		r.True(routed, "reference: distributeVoterOnly must route the frozen amount")
		r.Len(logs, 1, "reference: exactly one DelegateDistributed log")
		r.Equal(uint32(numVoters), consumed,
			"reference: unbounded call must consume all voters")
		r.Equal(uint32(numVoters), total,
			"reference: totalVoters must equal the snapshot entry count")
		r.NotNil(paid)
		r.Equal(0, paid.Cmp(voterAmount),
			"reference: unbounded payout must exactly equal voterAmount (dust included)")

		refPaid = paid
		refBalances = dumpBalances(t, ctx, sm, p)
	}()

	// Chunked fixture: three calls with voterBudget=7 → windows (7,7,6).
	chunkedPaid := new(big.Int)
	var chunkedBalances []*big.Int
	func() {
		r := require.New(t)
		ctx, sm, p, cand, candAddr := newVoterRewardCtx(t, true /* iip59On */)
		writeSnapshot(t, sm, candAddr, true /* optIn */, true /* registered */, epochBps, voters)
		rewardAddr, err := address.FromString(cand.RewardAddress)
		r.NoError(err)

		windows := []struct {
			start, budget, want uint32
		}{
			{0, 7, 7},
			{7, 7, 7},
			{14, 7, 6},
		}
		for chunkIdx, w := range windows {
			logs, routed, paid, consumed, total, err := p.distributeVoterOnly(
				ctx, sm, cand, rewardAddr,
				voterAmount, epochCommission,
				w.start, w.budget,
				100 /* blkHeight */, hash.ZeroHash256,
			)
			r.NoError(err, "chunk %d", chunkIdx)
			r.True(routed, "chunk %d: distributeVoterOnly must route", chunkIdx)
			r.Len(logs, 1, "chunk %d: one log per chunk", chunkIdx)
			r.Equal(w.want, consumed,
				"chunk %d: consumed must match window size", chunkIdx)
			r.Equal(uint32(numVoters), total,
				"chunk %d: totalVoters unchanged across windowed calls", chunkIdx)
			r.NotNil(paid)
			chunkedPaid.Add(chunkedPaid, paid)
		}
		chunkedBalances = dumpBalances(t, ctx, sm, p)
	}()

	r := require.New(t)
	r.Equal(0, refPaid.Cmp(chunkedPaid),
		"sum of chunked paid amounts must equal the reference single-call paid amount (ref=%s chunked=%s)",
		refPaid.String(), chunkedPaid.String())
	r.Equal(len(refBalances), len(chunkedBalances))
	for i := range refBalances {
		r.Equal(0, refBalances[i].Cmp(chunkedBalances[i]),
			"voter %d unclaimed balance mismatch (ref=%s chunked=%s) — allocation is not deterministic across windows",
			i, refBalances[i].String(), chunkedBalances[i].String())
	}
}

// TestProtocolOptions verifies WithAutoDepositBridge/WithAutoDepositReader
// install onto the Protocol so downstream distributeVoterOnly can consume
// them.
func TestProtocolOptions(t *testing.T) {
	r := require.New(t)
	g := genesis.TestDefault()
	bridge, err := autodeposit.New(identityset.Address(0).String())
	r.NoError(err)

	called := false
	reader := func(protocol.StateManager) autodeposit.ContractReader {
		called = true
		return autodeposit.ContractReaderFunc(func(_ context.Context, _ string, _ []byte) ([]byte, error) {
			return nil, errors.New("unused")
		})
	}

	p := NewProtocol(g.Rewarding, WithAutoDepositBridge(bridge), WithAutoDepositReader(reader))
	r.NotNil(p.autoDepositBridge)
	r.NotNil(p.autoDepositReader)

	_ = p.resolveAutoDepositReader(nil)
	r.True(called)
}
