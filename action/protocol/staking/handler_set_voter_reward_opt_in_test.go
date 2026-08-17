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

func voterRewardOptInContext(callerIndex int, height uint64) context.Context {
	g := genesis.TestDefault()
	g.ZanzibarBlockHeight = 1
	ctx := genesis.WithGenesisContext(context.Background(), g)
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{BlockHeight: height})
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
	csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
	r.NoError(err)
	candidate := candidates[0]
	act := action.NewSetVoterRewardOptIn()

	rLog, txLogs, err := p.handleSetVoterRewardOptIn(voterRewardOptInContext(1, 1), act, csm)
	r.NoError(err)
	r.NotNil(rLog)
	r.Nil(txLogs)
	r.Len(rLog.events, 1)
	r.Len(rLog.events[0].topics, 2)
	r.Empty(rLog.events[0].data)
	r.True(csm.GetByIdentifier(candidate.GetIdentifier()).VoterRewardOnchainOptIn)

	// The one-way transition is idempotent.
	_, _, err = p.handleSetVoterRewardOptIn(voterRewardOptInContext(1, 1), act, csm)
	r.NoError(err)
}

func TestSetVoterRewardOptInValidationAndOwnerLookup(t *testing.T) {
	r := require.New(t)
	owner := identityset.Address(1)
	ctrl := gomock.NewController(t)
	sm, p, _, _ := initTestState(t, ctrl,
		[]*bucketConfig{{owner, owner, "1200000000000000000000000", 30, true, true, nil, 0}},
		[]*candidateConfig{{owner, identityset.Address(7), owner, "test1"}},
	)
	csm, err := NewCandidateStateManagerWithContext(context.Background(), sm)
	r.NoError(err)
	r.Error(p.validateSetVoterRewardOptIn(voterRewardOptInContext(1, 0), action.NewSetVoterRewardOptIn()))
	r.NoError(p.validateSetVoterRewardOptIn(voterRewardOptInContext(1, 1), action.NewSetVoterRewardOptIn()))
	_, _, err = p.handleSetVoterRewardOptIn(
		voterRewardOptInContext(9, 1), action.NewSetVoterRewardOptIn(), csm,
	)
	r.Error(err)
	handleErr, ok := err.(*handleError)
	r.True(ok)
	r.Equal(uint64(iotextypes.ReceiptStatus_ErrCandidateNotExist), handleErr.ReceiptStatus())
}
