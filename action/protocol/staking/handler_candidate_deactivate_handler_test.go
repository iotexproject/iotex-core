// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

// deactivateHandlerCtx builds a context with the action caller and block height
// that p.handleCandidateDeactivate depends on.
func deactivateHandlerCtx(caller address.Address, height uint64) context.Context {
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{Caller: caller})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

func TestProtocol_HandleCandidateDeactivateHandler(t *testing.T) {
	owner := identityset.Address(1)
	// self-stake bucket owned by the candidate at index 0
	bucketCfgs := []*bucketConfig{
		{owner, owner, "1200000000000000000000000", 30, true, true, nil, 0},
	}
	candCfgs := []*candidateConfig{
		{owner, identityset.Address(7), owner, "test1"},
	}

	newCand := func(t *testing.T) (*Protocol, CandidateStateManager, *Candidate) {
		ctrl := gomock.NewController(t)
		sm, p, _, cands := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
		require.NoError(t, err)
		return p, csm, cands[0]
	}

	t.Run("caller is not a candidate owner", func(t *testing.T) {
		r := require.New(t)
		p, csm, _ := newCand(t)
		act := action.NewCandidateDeactivate(action.CandidateDeactivateOpRequest)
		ctx := deactivateHandlerCtx(identityset.Address(9), 100)
		_, _, err := p.handleCandidateDeactivate(ctx, act, csm)
		r.Equal(errCandNotExist, err)
	})

	t.Run("candidate has no self-stake bucket", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		cand.SelfStakeBucketIdx = candidateNoSelfStakeBucketIndex
		r.NoError(csm.Upsert(cand))
		act := action.NewCandidateDeactivate(action.CandidateDeactivateOpRequest)
		ctx := deactivateHandlerCtx(owner, 100)
		_, _, err := p.handleCandidateDeactivate(ctx, act, csm)
		r.ErrorContains(err, ErrInvalidSelfStkIndex.Error())
	})

	t.Run("request exit success", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		r.EqualValues(0, cand.DeactivatedAt)
		act := action.NewCandidateDeactivate(action.CandidateDeactivateOpRequest)
		ctx := deactivateHandlerCtx(owner, 100)
		rLog, tLogs, err := p.handleCandidateDeactivate(ctx, act, csm)
		r.NoError(err)
		r.Nil(tLogs)
		r.NotNil(rLog)
		// DeactivatedAt flips to the requested sentinel
		r.Equal(candidateExitRequested, csm.GetByOwner(owner).DeactivatedAt)
	})

	t.Run("request exit twice fails", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		cand.DeactivatedAt = candidateExitRequested
		r.NoError(csm.Upsert(cand))
		act := action.NewCandidateDeactivate(action.CandidateDeactivateOpRequest)
		ctx := deactivateHandlerCtx(owner, 100)
		_, _, err := p.handleCandidateDeactivate(ctx, act, csm)
		r.ErrorContains(err, ErrExitAlreadyRequested.Error())
	})

	t.Run("confirm exit success", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		// scheduled to exit at height 90, current block 100 -> ready
		cand.DeactivatedAt = 90
		r.NoError(csm.Upsert(cand))
		act := action.NewCandidateDeactivate(action.CandidateDeactivateOpConfirm)
		ctx := deactivateHandlerCtx(owner, 100)
		rLog, _, err := p.handleCandidateDeactivate(ctx, act, csm)
		r.NoError(err)
		r.NotNil(rLog)
		after := csm.GetByOwner(owner)
		r.EqualValues(0, after.SelfStake.Uint64())
		r.Equal(uint64(candidateNoSelfStakeBucketIndex), after.SelfStakeBucketIdx)
	})

	t.Run("confirm exit not ready", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		// scheduled to exit at height 110, current block 100 -> not ready
		cand.DeactivatedAt = 110
		r.NoError(csm.Upsert(cand))
		act := action.NewCandidateDeactivate(action.CandidateDeactivateOpConfirm)
		ctx := deactivateHandlerCtx(owner, 100)
		_, _, err := p.handleCandidateDeactivate(ctx, act, csm)
		r.ErrorContains(err, ErrExitNotReady.Error())
	})
}

func TestProtocol_HandleScheduleCandidateDeactivation(t *testing.T) {
	owner := identityset.Address(1)
	bucketCfgs := []*bucketConfig{
		{owner, owner, "1200000000000000000000000", 30, true, true, nil, 0},
	}
	candCfgs := []*candidateConfig{
		{owner, identityset.Address(7), owner, "test1"},
	}

	// buildCtx registers rolldpos (unless registry is empty) and sets the block height.
	buildCtx := func(withRolldpos bool, height uint64) (context.Context, *rolldpos.Protocol) {
		reg := protocol.NewRegistry()
		rp := rolldpos.NewProtocol(10, 10, 10)
		if withRolldpos {
			require.NoError(t, rp.Register(reg))
		}
		ctx := protocol.WithRegistry(context.Background(), reg)
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
		ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())
		ctx = protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
		return ctx, rp
	}

	newCand := func(t *testing.T) (*Protocol, CandidateStateManager, *Candidate) {
		ctrl := gomock.NewController(t)
		sm, p, _, cands := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
		require.NoError(t, err)
		return p, csm, cands[0]
	}

	t.Run("candidate not exist", func(t *testing.T) {
		r := require.New(t)
		p, csm, _ := newCand(t)
		ctx, _ := buildCtx(true, 100)
		act := action.NewScheduleCandidateDeactivation(identityset.Address(9))
		_, _, err := p.handleScheduleCandidateDeactivation(ctx, act, csm)
		r.Equal(errCandNotExist, err)
	})

	t.Run("rolldpos protocol not found", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		ctx, _ := buildCtx(false, 100)
		act := action.NewScheduleCandidateDeactivation(cand.GetIdentifier())
		_, _, err := p.handleScheduleCandidateDeactivation(ctx, act, csm)
		r.ErrorContains(err, "rolldpos protocol not found")
	})

	t.Run("invalid epoch number", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		ctx, _ := buildCtx(true, 0)
		act := action.NewScheduleCandidateDeactivation(cand.GetIdentifier())
		_, _, err := p.handleScheduleCandidateDeactivation(ctx, act, csm)
		r.ErrorContains(err, "invalid epoch number")
	})

	t.Run("success", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		height := uint64(100)
		ctx, rp := buildCtx(true, height)
		g := genesis.TestDefault()
		act := action.NewScheduleCandidateDeactivation(cand.GetIdentifier())
		rLog, tLogs, err := p.handleScheduleCandidateDeactivation(ctx, act, csm)
		r.NoError(err)
		r.Nil(tLogs)
		r.NotNil(rLog)

		epochNum := rp.GetEpochNum(height)
		want := height + g.ExitAdmissionInterval*rp.NumBlocksByEpoch(epochNum)
		updated := csm.GetByIdentifier(cand.GetIdentifier())
		r.Equal(want, updated.DeactivatedAt)
		r.Greater(updated.DeactivatedAt, height)

		// lastExitEpoch must be persisted under the CandsMap namespace
		var last lastExitEpoch
		_, err = csm.SM().State(&last, protocol.NamespaceOption(CandsMapNS), protocol.KeyOption(_lastExitEpoch))
		r.NoError(err)
		r.Equal(epochNum, last.epoch)
	})
}
