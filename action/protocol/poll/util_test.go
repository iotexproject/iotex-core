// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"math/big"
	"testing"

	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

// freezeSnapshotEraGateCtx builds the minimum ctx freezeIIP59PollSnapshot
// needs: post-fork FeatureCtx (so the fork gate lets us through) plus a
// genesis with the given EpochsPerRewardEra. DelegateProfileContractAddress
// stays empty so the bridge is nil and the code path that would call
// FreezePollSnapshot on the state manager is exercised only via the small
// nil-bridge branch (single PutState per candidate under _stakingNameSpace).
func freezeSnapshotEraGateCtx(t *testing.T, epochsPerEra uint64, height uint64) context.Context {
	t.Helper()
	g := genesis.TestDefault()
	// Turn IIP-59 fork on unconditionally: 0 <= any height means feature is on.
	g.ToBeEnabledBlockHeight = 0
	g.Rewarding.EpochsPerRewardEra = epochsPerEra

	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(ctx)
}

func fakeCandidatesForGate() state.CandidateList {
	return state.CandidateList{
		&state.Candidate{
			Address:       identityset.Address(1).String(),
			Votes:         big.NewInt(30),
			RewardAddress: identityset.Address(1).String(),
		},
	}
}

func TestFreezeIIP59PollSnapshot_NonEraBoundarySkipsWrite(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// Strict mock: any PutState / State call fails the test. If the gate
	// works, the function returns nil without touching the state manager.
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	// Non-boundary: epochsPerEra=24, epochNum=25.
	ctx := freezeSnapshotEraGateCtx(t, 24, 1)
	r.NoError(freezeIIP59PollSnapshot(ctx, sm, fakeCandidatesForGate(), 25))
}

func TestFreezeIIP59PollSnapshot_EraBoundaryProceeds(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// Boundary epoch: the function must proceed through the gate and reach
	// staking.FreezePollSnapshot. That path unconditionally issues at least
	// one State/PutState call per candidate. AnyTimes() is fine — we only
	// care that the era gate did not short-circuit.
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(uint64(0), state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(uint64(0), nil).MinTimes(1)

	ctx := freezeSnapshotEraGateCtx(t, 24, 1)
	r.NoError(freezeIIP59PollSnapshot(ctx, sm, fakeCandidatesForGate(), 24))
}

func TestFreezeIIP59PollSnapshot_EpochsPerRewardEraZeroDisables(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// EpochsPerRewardEra=0 must disable the write for every non-zero
	// epoch — matches IsEraBoundary's own zero-cadence semantics — so the
	// legacy per-epoch cadence stays off. Strict mock catches any PutState.
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	ctx := freezeSnapshotEraGateCtx(t, 0, 1)
	r.NoError(freezeIIP59PollSnapshot(ctx, sm, fakeCandidatesForGate(), 1))
	r.NoError(freezeIIP59PollSnapshot(ctx, sm, fakeCandidatesForGate(), 24))
	r.NoError(freezeIIP59PollSnapshot(ctx, sm, fakeCandidatesForGate(), 1_000_000))
}

func TestFreezeIIP59PollSnapshot_PreForkGateStillWins(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// Even on an era boundary, if the IIP-59 fork gate is closed the write
	// must be skipped. This asserts the ordering: fork gate first, era gate
	// second.
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	// ToBeEnabledBlockHeight sits above the current block height, so
	// NoVoterRewardDistribution=true.
	g.ToBeEnabledBlockHeight = 1_000_000
	g.Rewarding.EpochsPerRewardEra = 24
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1})
	ctx = protocol.WithFeatureCtx(ctx)

	r.NoError(freezeIIP59PollSnapshot(ctx, sm, fakeCandidatesForGate(), 24))
}
