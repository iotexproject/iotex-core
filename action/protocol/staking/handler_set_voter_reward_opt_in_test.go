// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose, and in no event shall the authors be liable for any claim, damages or other liability.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/action/protocol"
	"github.com/iotexproject/iotex-core/v2/blockchain/genesis"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func voterRewardOptInContext(callerIndex int) context.Context {
	g := genesis.TestDefault()
	g.ToBeEnabledBlockHeight = 1
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: 1})
	ctx = protocol.WithActionCtx(ctx, protocol.ActionCtx{Caller: identityset.Address(callerIndex)})
	return protocol.WithFeatureCtx(protocol.WithFeatureWithHeightCtx(ctx))
}

func TestHandleSetVoterRewardOptIn(t *testing.T) {
	r := require.New(t)
	owner := identityset.Address(1)
	ctrl := gomock.NewController(t)
	sm, p, _, candidates := initTestState(t, ctrl,
		[]*bucketConfig{{owner, owner, "1200000000000000000000000", 30, true, true, nil, 0}},
		[]*candidateConfig{{owner, identityset.Address(7), owner, "test1"}},
	)
	csm, err := NewCandidateStateManager(sm)
	r.NoError(err)
	candidate := candidates[0]
	act := action.NewSetVoterRewardOptIn(candidate.GetIdentifier().Bytes(), true)

	rLog, txLogs, err := p.handleSetVoterRewardOptIn(voterRewardOptInContext(1), act, csm)
	r.NoError(err)
	r.NotNil(rLog)
	r.Nil(txLogs)
	r.True(csm.GetByIdentifier(candidate.GetIdentifier()).VoterRewardOnchainOptIn)

	// The one-way transition is idempotent.
	_, _, err = p.handleSetVoterRewardOptIn(voterRewardOptInContext(1), act, csm)
	r.NoError(err)
}

func TestSetVoterRewardOptInValidationAndAuthorization(t *testing.T) {
	r := require.New(t)
	owner := identityset.Address(1)
	ctrl := gomock.NewController(t)
	sm, p, _, candidates := initTestState(t, ctrl,
		[]*bucketConfig{{owner, owner, "1200000000000000000000000", 30, true, true, nil, 0}},
		[]*candidateConfig{{owner, identityset.Address(7), owner, "test1"}},
	)
	csm, err := NewCandidateStateManager(sm)
	r.NoError(err)
	id := candidates[0].GetIdentifier().Bytes()

	r.Error(p.validateSetVoterRewardOptIn(voterRewardOptInContext(1), action.NewSetVoterRewardOptIn(id, false)))
	_, _, err = p.handleSetVoterRewardOptIn(
		voterRewardOptInContext(9), action.NewSetVoterRewardOptIn(id, true), csm,
	)
	r.Error(err)
	handleErr, ok := err.(*handleError)
	r.True(ok)
	r.Equal(uint64(iotextypes.ReceiptStatus_ErrUnauthorizedOperator), handleErr.ReceiptStatus())
}
