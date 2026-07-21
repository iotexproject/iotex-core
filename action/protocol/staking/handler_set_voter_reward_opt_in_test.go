// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func optInHandlerCtx(caller address.Address, height uint64) context.Context {
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{Caller: caller})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
	ctx = genesis.WithGenesisContext(ctx, genesis.TestDefault())
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

func TestProtocol_HandleSetVoterRewardOptIn(t *testing.T) {
	owner := identityset.Address(1)
	bucketCfgs := []*bucketConfig{
		{owner, owner, "1200000000000000000000000", 30, true, true, nil, 0},
	}
	candCfgs := []*candidateConfig{
		{owner, identityset.Address(7), owner, "test1"},
	}

	newCand := func(t *testing.T) (*Protocol, CandidateStateManager, *Candidate) {
		ctrl := gomock.NewController(t)
		sm, p, _, cands := initTestState(t, ctrl, bucketCfgs, candCfgs)
		csm, err := NewCandidateStateManager(sm)
		require.NoError(t, err)
		return p, csm, cands[0]
	}

	t.Run("owner opts in flips the flag", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		r.False(cand.VoterRewardOnchainOptIn)

		act := action.NewSetVoterRewardOptIn(cand.GetIdentifier().Bytes(), true)
		ctx := optInHandlerCtx(owner, 100)

		rLog, tLogs, err := p.handleSetVoterRewardOptIn(ctx, act, csm)
		r.NoError(err)
		r.Nil(tLogs)
		r.NotNil(rLog)

		updated := csm.GetByIdentifier(cand.GetIdentifier())
		r.True(updated.VoterRewardOnchainOptIn)
	})

	t.Run("owner opts out clears the flag", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		cand.VoterRewardOnchainOptIn = true
		r.NoError(csm.Upsert(cand))

		act := action.NewSetVoterRewardOptIn(cand.GetIdentifier().Bytes(), false)
		ctx := optInHandlerCtx(owner, 100)

		rLog, _, err := p.handleSetVoterRewardOptIn(ctx, act, csm)
		r.NoError(err)
		r.NotNil(rLog)

		updated := csm.GetByIdentifier(cand.GetIdentifier())
		r.False(updated.VoterRewardOnchainOptIn)
	})

	t.Run("idempotent set to current value succeeds and re-emits log", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)
		cand.VoterRewardOnchainOptIn = true
		r.NoError(csm.Upsert(cand))

		act := action.NewSetVoterRewardOptIn(cand.GetIdentifier().Bytes(), true)
		ctx := optInHandlerCtx(owner, 100)

		rLog, _, err := p.handleSetVoterRewardOptIn(ctx, act, csm)
		r.NoError(err)
		r.NotNil(rLog)
		r.True(csm.GetByIdentifier(cand.GetIdentifier()).VoterRewardOnchainOptIn)
	})

	t.Run("non-owner caller is rejected", func(t *testing.T) {
		r := require.New(t)
		p, csm, cand := newCand(t)

		act := action.NewSetVoterRewardOptIn(cand.GetIdentifier().Bytes(), true)
		ctx := optInHandlerCtx(identityset.Address(9), 100)

		_, _, err := p.handleSetVoterRewardOptIn(ctx, act, csm)
		r.Error(err)
		he, ok := err.(*handleError)
		r.True(ok)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), he.ReceiptStatus())
		// State must not have flipped.
		r.False(csm.GetByIdentifier(cand.GetIdentifier()).VoterRewardOnchainOptIn)
	})

	t.Run("unknown candidate is rejected", func(t *testing.T) {
		r := require.New(t)
		p, csm, _ := newCand(t)

		unknown := identityset.Address(15)
		act := action.NewSetVoterRewardOptIn(unknown.Bytes(), true)
		ctx := optInHandlerCtx(owner, 100)

		_, _, err := p.handleSetVoterRewardOptIn(ctx, act, csm)
		r.Equal(errCandNotExist, err)
	})

	t.Run("invalid identifier bytes is rejected", func(t *testing.T) {
		r := require.New(t)
		p, csm, _ := newCand(t)

		act := action.NewSetVoterRewardOptIn([]byte{0xff}, true)
		ctx := optInHandlerCtx(owner, 100)

		_, _, err := p.handleSetVoterRewardOptIn(ctx, act, csm)
		r.Error(err)
		he, ok := err.(*handleError)
		r.True(ok)
		r.Equal(uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist), he.ReceiptStatus())
	})
}
