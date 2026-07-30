// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package rewarding

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action"
	"github.com/iotexproject/iotex-core/v2/state"
	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestVoterRewardDestinationState(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	voter := identityset.Address(7)
	recipient := identityset.Address(8)

	effective, explicitlySet, updatedHeight, err := p.resolveVoterRewardDestination(ctx, sm, voter)
	r.NoError(err)
	r.Equal(voter.Bytes(), effective.Bytes())
	r.False(explicitlySet)
	r.Zero(updatedHeight)

	oldRecipient, newRecipient, err := p.setVoterRewardDestination(ctx, sm, voter, recipient.Bytes())
	r.NoError(err)
	r.Equal(voter.Bytes(), oldRecipient.Bytes())
	r.Equal(recipient.Bytes(), newRecipient.Bytes())

	effective, explicitlySet, updatedHeight, err = p.resolveVoterRewardDestination(ctx, sm, voter)
	r.NoError(err)
	r.Equal(recipient.Bytes(), effective.Bytes())
	r.True(explicitlySet)
	r.Equal(uint64(100), updatedHeight)

	oldRecipient, newRecipient, err = p.setVoterRewardDestination(ctx, sm, voter, voter.Bytes())
	r.NoError(err)
	r.Equal(recipient.Bytes(), oldRecipient.Bytes())
	r.Equal(voter.Bytes(), newRecipient.Bytes())

	effective, explicitlySet, updatedHeight, err = p.resolveVoterRewardDestination(ctx, sm, voter)
	r.NoError(err)
	r.Equal(voter.Bytes(), effective.Bytes())
	r.False(explicitlySet)
	r.Zero(updatedHeight)
	_, err = p.state(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{})
	r.True(errors.Is(err, state.ErrStateNotExist), "reset must delete sparse override state")

	r.NoError(p.putState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{recipient: recipient}))
	_, _, err = p.setVoterRewardDestination(ctx, sm, voter, nil)
	r.NoError(err)
	effective, explicitlySet, _, err = p.resolveVoterRewardDestination(ctx, sm, voter)
	r.NoError(err)
	r.Equal(voter.Bytes(), effective.Bytes())
	r.False(explicitlySet)
}

func TestVoterRewardDestinationValidationGate(t *testing.T) {
	r := require.New(t)
	act := action.NewSetVoterRewardDestination(identityset.Address(8).Bytes())
	elp := (&action.EnvelopeBuilder{}).SetAction(act).Build()

	preForkCtx, preForkSM, preForkProtocol, _, _ := newVoterRewardCtx(t, false)
	r.Error(preForkProtocol.Validate(preForkCtx, elp, preForkSM))

	postForkCtx, postForkSM, postForkProtocol, _, _ := newVoterRewardCtx(t, true)
	r.NoError(postForkProtocol.Validate(postForkCtx, elp, postForkSM))

	invalid := (&action.EnvelopeBuilder{}).
		SetAction(action.NewSetVoterRewardDestination([]byte{1, 2, 3})).Build()
	r.Error(postForkProtocol.Validate(postForkCtx, invalid, postForkSM))
}

func TestVoterRewardDestinationRejectsMalformedStoredState(t *testing.T) {
	r := require.New(t)
	ctx, sm, p, _, _ := newVoterRewardCtx(t, true)
	voter := identityset.Address(7)
	zero := mustTestAddress(t, make([]byte, common.AddressLength))

	for _, destination := range []*voterRewardDestination{
		{},
		{recipient: zero, updatedHeight: 1},
	} {
		data, err := destination.Serialize()
		r.NoError(err)
		decoded := &voterRewardDestination{}
		err = decoded.Deserialize(data)
		r.Error(err)
	}

	r.NoError(p.putState(ctx, sm, voterRewardDestinationKey(voter), &voterRewardDestination{
		recipient: voter, updatedHeight: 1,
	}))
	_, _, _, err := p.resolveVoterRewardDestination(ctx, sm, voter)
	r.Error(err)
}

func mustTestAddress(t *testing.T, raw []byte) address.Address {
	t.Helper()
	addr, err := address.FromBytes(raw)
	require.NoError(t, err)
	return addr
}
