// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/mohae/deepcopy"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// setCommissionTestCtx builds a context plumbed with all the protocol
// contexts handleSetCommissionRate / validateSetCommissionRate read from.
// `enabled` controls whether the IIP-59 feature flag is active at the
// tests' fixed block height (10). When true, ToBeEnabledBlockHeight is
// set to 1 so IsToBeEnabled(10) is true; when false, the default keeps
// the flag off at height 10.
func setCommissionTestCtx(t *testing.T, enabled bool, caller, _ address.Address) context.Context {
	t.Helper()
	cfg := deepcopy.Copy(genesis.TestDefault()).(genesis.Genesis)
	if enabled {
		cfg.ToBeEnabledBlockHeight = 1
	}
	// Some unrelated forks need to be on at height 1 so that the test setup
	// (initTestState below) doesn't trip over pre-fork sanity checks.
	cfg.GreenlandBlockHeight = 1
	cfg.TsunamiBlockHeight = 1
	cfg.UpernavikBlockHeight = 1
	cfg.YapBlockHeight = 1

	ctx := genesis.WithGenesisContext(context.Background(), cfg)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    10,
		BlockTimeStamp: timeBlock,
		GasLimit:       1000000,
	})
	ctx = protocol.WithBlockchainCtx(ctx, protocol.BlockchainCtx{Tip: protocol.TipInfo{}})
	ctx = protocol.WithFeatureWithHeightCtx(ctx)
	ctx = protocol.WithFeatureCtx(ctx)
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       caller,
		GasPrice:     big.NewInt(1000),
		IntrinsicGas: action.SetCommissionRateBaseIntrinsicGas,
		Nonce:        1,
	})
	return ctx
}

func TestProtocol_validateSetCommissionRate(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	_, p, _, _ := initTestState(t, ctrl, nil, nil)

	owner := identityset.Address(1)

	// Pre-fork (default): NoVoterRewardDistribution = true. The validator
	// must reject every rate, including 0, so the action is dropped at
	// mempool admission rather than wasting block space.
	preCtx := setCommissionTestCtx(t, false, owner, owner)
	err := p.validateSetCommissionRate(preCtx, action.NewSetCommissionRate(0))
	r.Error(err)
	r.ErrorIs(err, action.ErrInvalidAct)

	// Post-fork: rate within range is OK.
	postCtx := setCommissionTestCtx(t, true, owner, owner)
	r.NoError(p.validateSetCommissionRate(postCtx, action.NewSetCommissionRate(0)))
	r.NoError(p.validateSetCommissionRate(postCtx, action.NewSetCommissionRate(action.MaxCommissionRate)))

	// Post-fork: rate above MaxCommissionRate is rejected.
	err = p.validateSetCommissionRate(postCtx, action.NewSetCommissionRate(action.MaxCommissionRate+1))
	r.Error(err)
	r.True(errors.Is(err, action.ErrInvalidCommissionRate) || errors.Is(err, action.ErrInvalidAct),
		"expected an invalid-rate error, got %v", err)
}

func TestProtocol_handleSetCommissionRate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	owner := identityset.Address(1)
	operator := identityset.Address(7)
	reward := identityset.Address(1)
	candidateCfgs := []*candidateConfig{
		{owner, operator, reward, "testcand"},
	}
	sm, p, _, candidates := initTestState(t, ctrl, nil, candidateCfgs)
	cand := candidates[0]

	t.Run("non-owner caller is rejected with errCandNotExist", func(t *testing.T) {
		r := require.New(t)
		stranger := identityset.Address(9)
		ctx := setCommissionTestCtx(t, true, stranger, stranger)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		_, err = p.handleSetCommissionRate(ctx, action.NewSetCommissionRate(1500), csm)
		r.ErrorIs(err, errCandNotExist)
	})

	t.Run("owner write persists the new rate", func(t *testing.T) {
		r := require.New(t)
		ctx := setCommissionTestCtx(t, true, owner, owner)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)

		// Sanity: starting rate is 0 (legacy).
		r.Equal(uint64(0), csm.GetByOwner(owner).CommissionRate)

		rLog, err := p.handleSetCommissionRate(ctx, action.NewSetCommissionRate(1500), csm)
		r.NoError(err)
		r.NotNil(rLog)

		// Build the receipt log slice and verify the CommissionRateSet
		// event was emitted with the candidate's identifier in topic-1.
		logs := rLog.Build(ctx, nil)
		r.NotEmpty(logs)
		evtTopics, _, err := action.PackCommissionRateSetEvent(cand.GetIdentifier(), 1500)
		r.NoError(err)
		r.Equal(evtTopics[0][:], logs[0].Topics[0][:], "topic-0 must be the CommissionRateSet event ID")
		r.Equal(evtTopics[1][:], logs[0].Topics[1][:], "topic-1 must be the candidate identifier")

		// The handler writes through csm.Upsert into the dirty view;
		// re-reading the candidate from a fresh csm must see the new rate.
		r.NoError(csm.Commit(ctx))
		fresh, err := NewCandidateStateManager(sm)
		r.NoError(err)
		r.Equal(uint64(1500), fresh.GetByOwner(owner).CommissionRate)
	})

	t.Run("owner can update an existing rate", func(t *testing.T) {
		r := require.New(t)
		ctx := setCommissionTestCtx(t, true, owner, owner)
		csm, err := NewCandidateStateManager(sm)
		r.NoError(err)
		// Previous subtest left rate = 1500.
		r.Equal(uint64(1500), csm.GetByOwner(owner).CommissionRate)

		_, err = p.handleSetCommissionRate(ctx, action.NewSetCommissionRate(0), csm)
		r.NoError(err)
		r.NoError(csm.Commit(ctx))

		fresh, err := NewCandidateStateManager(sm)
		r.NoError(err)
		r.Equal(uint64(0), fresh.GetByOwner(owner).CommissionRate,
			"falling back to legacy must persist as 0 — no implicit floor")
	})
}

// TestProtocol_HandleSetCommissionRate_FullEnvelope exercises the action
// through p.Handle (the public Protocol entry point) to verify it's wired
// into the handle switch and that a real receipt is returned.
func TestProtocol_HandleSetCommissionRate_FullEnvelope(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	owner := identityset.Address(1)
	operator := identityset.Address(7)
	reward := identityset.Address(1)
	sm, p, _, _ := initTestState(t, ctrl, nil, []*candidateConfig{
		{owner, operator, reward, "testcand"},
	})

	// Fresh state — owner's account pending nonce is 0; the envelope and
	// ActionCtx must agree on nonce 0 for settleAction's SetPendingNonce(0+1)
	// to succeed.
	const callerNonce = uint64(0)
	ctx := setCommissionTestCtx(t, true, owner, owner)
	// Replace the ActionCtx Nonce with the actual envelope nonce.
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{
		Caller:       owner,
		GasPrice:     big.NewInt(1000),
		IntrinsicGas: action.SetCommissionRateBaseIntrinsicGas,
		Nonce:        callerNonce,
	})
	elp := (&action.EnvelopeBuilder{}).
		SetNonce(callerNonce).
		SetGasLimit(1000000).
		SetGasPrice(big.NewInt(1000)).
		SetAction(action.NewSetCommissionRate(2500)).Build()

	receipt, err := p.Handle(ctx, elp, sm)
	r.NoError(err)
	r.NotNil(receipt)
	r.Equal(iotextypes.ReceiptStatus_Success, iotextypes.ReceiptStatus(receipt.Status))
	r.Len(receipt.Logs(), 1, "exactly one log: the CommissionRateSet event")

	csm, err := NewCandidateStateManager(sm)
	r.NoError(err)
	r.Equal(uint64(2500), csm.GetByOwner(owner).CommissionRate)
}

// TestProtocol_HandleSetCommissionRate_PreFlagRejectedByValidate is the
// pre-fork end-to-end safety net: even if a tx somehow reached p.Handle
// directly, the Validate hook should be the gate. Here we verify Validate
// rejects the action under the default (pre-fork) feature flag state.
func TestProtocol_HandleSetCommissionRate_PreFlagRejectedByValidate(t *testing.T) {
	r := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	owner := identityset.Address(1)
	sm, p, _, _ := initTestState(t, ctrl, nil, []*candidateConfig{
		{owner, identityset.Address(7), identityset.Address(1), "testcand"},
	})

	ctx := setCommissionTestCtx(t, false, owner, owner)
	elp := (&action.EnvelopeBuilder{}).
		SetNonce(1).
		SetGasLimit(1000000).
		SetGasPrice(big.NewInt(1000)).
		SetAction(action.NewSetCommissionRate(1500)).Build()

	err := p.Validate(ctx, elp, sm)
	r.ErrorIs(err, action.ErrInvalidAct)
}
