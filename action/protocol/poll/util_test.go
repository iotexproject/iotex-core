// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
	"github.com/iotexproject/iotex-core/v2/test/mock/mock_chainmanager"
)

type emptyStateManager struct{}

func (emptyStateManager) Height() (uint64, error) { return 0, nil }

func (emptyStateManager) State(interface{}, ...protocol.StateOption) (uint64, error) {
	return 0, state.ErrStateNotExist
}

func (emptyStateManager) States(...protocol.StateOption) (uint64, state.Iterator, error) {
	return 0, nil, state.ErrStateNotExist
}

func (emptyStateManager) ReadView(string) (protocol.View, error) {
	return nil, state.ErrStateNotExist
}

func (emptyStateManager) Snapshot() int { return 0 }

func (emptyStateManager) Revert(int) error { return nil }

func (emptyStateManager) PutState(interface{}, ...protocol.StateOption) (uint64, error) {
	return 0, nil
}

func (emptyStateManager) DelState(...protocol.StateOption) (uint64, error) { return 0, nil }

func (emptyStateManager) WriteView(string, protocol.View) error { return nil }

type snapshotTrackingStateManager struct {
	emptyStateManager
	snapshotID  int
	snapshots   int
	reverts     []int
	revertError error
	account     *state.Account
}

func (sm *snapshotTrackingStateManager) State(value interface{}, opts ...protocol.StateOption) (uint64, error) {
	if account, ok := value.(*state.Account); ok && sm.account != nil {
		*account = *sm.account
		return 0, nil
	}
	return sm.emptyStateManager.State(value, opts...)
}

func (sm *snapshotTrackingStateManager) Snapshot() int {
	snapshot := sm.snapshotID + sm.snapshots
	sm.snapshots++
	return snapshot
}

func (sm *snapshotTrackingStateManager) Revert(snapshot int) error {
	sm.reverts = append(sm.reverts, snapshot)
	return sm.revertError
}

// freezeSnapshotEraGateCtx builds the minimum ctx freezeIIP59RewardState
// needs: post-fork FeatureCtx (so the fork gate lets us through) plus a
// genesis with the given EpochsPerRewardEra. DelegateProfileContractAddress
// stays empty so the bridge is nil and the code path that would call
// FreezeCandidateRewardSnapshots on the state manager is exercised only via the small
// nil-bridge branch (single PutState per candidate under _stakingNameSpace).
func freezeSnapshotEraGateCtx(t *testing.T, epochsPerEra uint64, height uint64) context.Context {
	t.Helper()
	g := genesis.TestDefault()
	// Turn IIP-59 fork on unconditionally: 0 <= any height means feature is on.
	g.ZanzibarBlockHeight = 0
	g.ZanzibarBetaBlockHeight = 0
	g.Rewarding.EpochsPerRewardEra = epochsPerEra

	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	return protocol.WithFeatureCtx(ctx)
}

func delegateProfileReaderCtx() context.Context {
	g := genesis.TestDefault()
	ctx := genesis.WithGenesisContext(context.Background(), g)
	return protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{
		ChainID:      1,
		EvmNetworkID: 1,
		Tip: protocol.TipInfo{
			Height:    1,
			Timestamp: time.Unix(g.Timestamp, 0),
		},
		GetBlockHash: func(uint64) (hash.Hash256, error) { return hash.ZeroHash256, nil },
		GetBlockTime: func(uint64) (time.Time, error) { return time.Unix(g.Timestamp, 0), nil },
	})
}

func TestFreezeIIP59RewardState_NonEraBoundarySkipsWrite(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// Strict mock: any PutState / State call fails the test. If the gate
	// works, the function returns nil without touching the state manager.
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	// Non-boundary: epochsPerEra=24, epochNum=25.
	ctx := freezeSnapshotEraGateCtx(t, 24, 1)
	_, ferr1 := freezeIIP59RewardState(ctx, sm, 25)
	r.NoError(ferr1)
}

func TestFreezeIIP59RewardState_EraBoundaryProceeds(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// Boundary epoch: the function must proceed through the gate and reach
	// staking.FreezeCandidateRewardSnapshots.
	//
	// What proves it got there is the error. The frozen set now comes from the
	// candidate center, so the first thing FreezeCandidateRewardSnapshots does is construct
	// the base view -- reading the height, then the view -- and a view that
	// cannot be read is fatal there rather than degraded: a boundary that
	// cannot enumerate candidates would freeze an empty era and put every
	// delegate on the 100%-commission fallback for a full day.
	//
	// protocol.ErrNoName is what the real views.Read returns for an
	// unregistered name (protocol.go:197), i.e. "staking installed no view",
	// which is the exact shape a mock state manager presents.
	sm := mock_chainmanager.NewMockStateManager(ctrl)
	sm.EXPECT().Height().Return(uint64(1), nil).AnyTimes()
	sm.EXPECT().ReadView(gomock.Any()).Return(nil, protocol.ErrNoName).AnyTimes()

	ctx := freezeSnapshotEraGateCtx(t, 24, 1)
	_, err := freezeIIP59RewardState(ctx, sm, 24)
	r.ErrorIs(err, protocol.ErrNoName)
	r.Contains(err.Error(), "construct candidate view for reward snapshot")
}

func TestFreezeIIP59RewardState_EpochsPerRewardEraZeroDisables(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// EpochsPerRewardEra=0 must disable the write for every non-zero
	// epoch — matches IsEraBoundary's own zero-cadence semantics — so the
	// legacy per-epoch cadence stays off. Strict mock catches any PutState.
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	ctx := freezeSnapshotEraGateCtx(t, 0, 1)
	_, ferr2 := freezeIIP59RewardState(ctx, sm, 1)
	r.NoError(ferr2)
	_, ferr3 := freezeIIP59RewardState(ctx, sm, 24)
	r.NoError(ferr3)
	_, ferr4 := freezeIIP59RewardState(ctx, sm, 1_000_000)
	r.NoError(ferr4)
}

func TestFreezeIIP59RewardState_PreForkGateStillWins(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)

	// Even on an era boundary, if the IIP-59 fork gate is closed the write
	// must be skipped. This asserts the ordering: fork gate first, era gate
	// second.
	sm := mock_chainmanager.NewMockStateManager(ctrl)

	g := genesis.TestDefault()
	// ZanzibarBlockHeight sits above the current block height, so
	// NoVoterRewardDistribution=true.
	g.ZanzibarBlockHeight = 1_000_000
	g.ZanzibarBetaBlockHeight = 1_000_000
	g.Rewarding.EpochsPerRewardEra = 24
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1})
	ctx = protocol.WithFeatureCtx(ctx)

	_, ferr5 := freezeIIP59RewardState(ctx, sm, 24)

	r.NoError(ferr5)
}

func TestDelegateProfileContractReaderInstallsEVMHelperContext(t *testing.T) {
	ctx := delegateProfileReaderCtx()
	sm := &snapshotTrackingStateManager{snapshotID: 42}
	reader := delegateProfileContractReader(sm)

	require.NotPanics(t, func() {
		result, err := reader.Read(ctx, identityset.Address(1).String(), []byte{0, 0, 0, 0})
		require.NoError(t, err)
		require.Empty(t, result)
	})
	require.GreaterOrEqual(t, sm.snapshots, 1)
	require.Equal(t, []int{42}, sm.reverts)
}

func TestDelegateProfileContractReaderReturnsRevertError(t *testing.T) {
	revertErr := errors.New("revert failed")
	ctx := delegateProfileReaderCtx()
	sm := &snapshotTrackingStateManager{
		snapshotID:  7,
		revertError: revertErr,
	}
	reader := delegateProfileContractReader(sm)

	_, err := reader.Read(ctx, identityset.Address(1).String(), []byte{0, 0, 0, 0})
	require.ErrorContains(t, err, "failed to revert DelegateProfile simulation")
	require.ErrorIs(t, err, revertErr)
	require.GreaterOrEqual(t, sm.snapshots, 1)
	require.Equal(t, []int{7}, sm.reverts)
}

func TestDelegateProfileContractReaderUsesCurrentCallerNonce(t *testing.T) {
	ctx := delegateProfileReaderCtx()
	account, err := state.NewAccount()
	require.NoError(t, err)
	for nonce := uint64(1); nonce <= 5; nonce++ {
		require.NoError(t, account.SetPendingNonce(nonce))
	}
	sm := &snapshotTrackingStateManager{snapshotID: 11, account: account}
	reader := delegateProfileContractReader(sm)

	core, logs := observer.New(zapcore.ErrorLevel)
	previousLogger := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	defer zap.ReplaceGlobals(previousLogger)

	_, err = reader.Read(ctx, identityset.Address(1).String(), []byte{0, 0, 0, 0})
	require.NoError(t, err)
	require.Equal(t, 0, logs.FilterMessage("Inconsistent nonce.").Len())
	require.Equal(t, []int{11}, sm.reverts)
}
