// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"context"
	"math/big"
	"testing"

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
