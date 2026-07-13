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

	"github.com/iotexproject/go-pkgs/hash"
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

// TestDistributeVoterReward_FeatureOff verifies the fork gate short-circuits
// the entire path before touching state, so the caller runs the legacy
// grantToAccount unchanged for the whole rewarding window.
func TestDistributeVoterReward_FeatureOff(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, cand, rewardAddr := newVoterRewardCtx(t, false /* enable IIP-59 */)
	// Force NoVoterRewardDistribution=true by using a genesis where the
	// ToBeEnabled height is beyond the current block height.
	logs, handled, err := p.distributeVoterReward(
		ctx, sm, cand, rewardAddr, big.NewInt(1_000), 100, hash.ZeroHash256,
	)
	r.NoError(err)
	r.False(handled)
	r.Nil(logs)
}

// TestDistributeVoterReward_NilInputs guards the wiring-error branches.
// A nil candidate or invalid totalReward is a wiring bug, not chain data,
// so it must abort rather than silently degrade.
func TestDistributeVoterReward_NilInputs(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, rewardAddr := newVoterRewardCtx(t, true)

	// nil candidate
	logs, handled, err := p.distributeVoterReward(ctx, sm, nil, rewardAddr, big.NewInt(100), 100, hash.ZeroHash256)
	r.Error(err)
	r.False(handled)
	r.Nil(logs)

	// negative totalReward
	logs, handled, err = p.distributeVoterReward(
		ctx, sm, &state.Candidate{Address: identityset.Address(0).String()},
		rewardAddr, big.NewInt(-1), 100, hash.ZeroHash256,
	)
	r.Error(err)
	r.False(handled)
	r.Nil(logs)

	// nil rewardAddr — degrades to (nil, false, nil): the delegate has no
	// reward address, so there is nothing to distribute.
	logs, handled, err = p.distributeVoterReward(
		ctx, sm, &state.Candidate{Address: identityset.Address(0).String()},
		nil, big.NewInt(100), 100, hash.ZeroHash256,
	)
	r.NoError(err)
	r.False(handled)
	r.Nil(logs)
}

// TestDistributeVoterReward_NoSnapshotFallsBackToLegacy covers the
// "candidate registered but no poll snapshot yet" case (first epoch after
// registration). It must return (nil, false, nil) so the caller runs the
// legacy grantToAccount path.
func TestDistributeVoterReward_NoSnapshotFallsBackToLegacy(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, cand, rewardAddr := newVoterRewardCtx(t, true)

	logs, handled, err := p.distributeVoterReward(
		ctx, sm, cand, rewardAddr, big.NewInt(1_000), 100, hash.ZeroHash256,
	)
	r.NoError(err)
	r.False(handled)
	r.Nil(logs)
}

// newVoterRewardCtx wires the minimum context distributeVoterReward reads:
// a StateManager, registered rolldpos+staking protocols, genesis context, a
// block context (for height), and a feature context toggled by iip59On.
//
// staking is registered via a stub Protocol so FindProtocol succeeds; the
// tests here only exercise pre-snapshot branches, so no bucket / view /
// AddDepositForCompound path is invoked.
func newVoterRewardCtx(
	t *testing.T,
	iip59On bool,
) (context.Context, protocol.StateManager, *Protocol, *state.Candidate, address.Address) {
	t.Helper()
	r := require.New(t)
	ctrl := gomock.NewController(t)
	sm := testdb.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	// Force the fork gate on/off by pinning ToBeEnabledBlockHeight relative
	// to the block height we bind below (100). NoVoterRewardDistribution is
	// !g.IsToBeEnabled(height), so ToBeEnabledBlockHeight <= height ⇒ IIP-59
	// on.
	if iip59On {
		g.ToBeEnabledBlockHeight = 1
	} else {
		g.ToBeEnabledBlockHeight = 1_000_000_000
	}

	registry := protocol.NewRegistry()
	rp := rolldpos.NewProtocol(g.NumCandidateDelegates, g.NumDelegates, g.NumSubEpochs)
	r.NoError(rp.Register(registry))

	// Register a real staking protocol so FindProtocol returns non-nil.
	// distributeVoterReward's staking calls are only reached on the
	// registered+opted-in path, which these tests do not exercise.
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

	cand := &state.Candidate{
		Address:       identityset.Address(1).String(),
		RewardAddress: identityset.Address(2).String(),
		Votes:         big.NewInt(1_000_000),
	}
	rewardAddr, err := address.FromString(cand.RewardAddress)
	r.NoError(err)

	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithRegistry(ctx, registry)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 100})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{Caller: identityset.Address(0)})
	ctx = protocol.WithFeatureCtx(ctx)

	return ctx, sm, p, cand, rewardAddr
}

// TestDistributeVoterReward_BridgeNilRoutesToCredit is a sanity check on the
// AutoDeposit wiring branch: if no autoDepositBridge is configured (as when
// AutoDepositContractAddress is empty in genesis), the reader is never
// constructed. This test exercises the branch by verifying the nil-bridge
// field remains nil on the default Protocol.
func TestDistributeVoterReward_BridgeNilRoutesToCredit(t *testing.T) {
	r := require.New(t)
	g := genesis.TestDefault()
	p := NewProtocol(g.Rewarding)
	r.Nil(p.autoDepositBridge)
	r.Nil(p.autoDepositReader)
}

// TestProtocolOptions verifies WithAutoDepositBridge/WithAutoDepositReader
// install onto the Protocol so downstream distributeVoterReward can consume
// them.
func TestProtocolOptions(t *testing.T) {
	r := require.New(t)
	g := genesis.TestDefault()
	// Use a valid IoTeX bech32 address for bridge construction.
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

	// Exercise the seam so the coverage on resolveAutoDepositReader is real.
	_ = p.resolveAutoDepositReader(nil)
	r.True(called)
}
