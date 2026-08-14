// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestSetVoterRewardDestinationRoundTrip(t *testing.T) {
	r := require.New(t)
	contractABI := NativeStakingContractABI()
	r.Contains(contractABI.Methods, "setVoterRewardDestination")
	r.Contains(contractABI.Events, "VoterRewardDestinationSet")

	recipient := identityset.Address(7)
	act := NewSetVoterRewardDestination(recipient.Bytes())

	r.NoError(act.SanityCheck())
	r.Equal(SetVoterRewardDestinationBaseIntrinsicGas, mustIntrinsicGas(t, act))
	r.Equal(recipient.Bytes(), act.Recipient())

	fromProto := &SetVoterRewardDestination{}
	r.NoError(fromProto.LoadProto(act.Proto()))
	r.Equal(recipient.Bytes(), fromProto.Recipient())

	elp := (&EnvelopeBuilder{}).SetNonce(1).SetAction(act).Build()
	core := elp.Proto()
	r.NotNil(core.GetSetVoterRewardDestination())
	loaded := &envelope{}
	r.NoError(loaded.LoadProto(core))
	loadedAction, ok := loaded.Action().(*SetVoterRewardDestination)
	r.True(ok)
	r.Equal(recipient.Bytes(), loadedAction.Recipient())

	data, err := act.EthData()
	r.NoError(err)
	fromABI, err := NewSetVoterRewardDestinationFromABIBinary(data)
	r.NoError(err)
	r.Equal(recipient.Bytes(), fromABI.Recipient())

	resetData, err := NewSetVoterRewardDestination(nil).EthData()
	r.NoError(err)
	reset, err := NewSetVoterRewardDestinationFromABIBinary(resetData)
	r.NoError(err)
	r.Empty(reset.Recipient())
}

func TestSetVoterRewardDestinationValidation(t *testing.T) {
	r := require.New(t)
	r.NoError(NewSetVoterRewardDestination(nil).SanityCheck())
	r.NoError(NewSetVoterRewardDestination(make([]byte, common.AddressLength)).SanityCheck())
	r.Error(NewSetVoterRewardDestination([]byte{1, 2, 3}).SanityCheck())

	_, err := NewSetVoterRewardDestinationFromABIBinary([]byte{1, 2, 3})
	r.ErrorIs(err, errDecodeFailure)
	r.Error((&SetVoterRewardDestination{}).LoadProto(nil))

	recipient := identityset.Address(8).Bytes()
	act := NewSetVoterRewardDestination(recipient)
	recipient[0] ^= 0xff
	r.NotEqual(recipient, act.Recipient(), "constructor must not retain caller-owned bytes")
	copyOut := act.Recipient()
	copyOut[0] ^= 0xff
	r.NotEqual(copyOut, act.Recipient(), "getter must return a defensive copy")
}

func TestPackVoterRewardDestinationSetEvent(t *testing.T) {
	r := require.New(t)
	voter := identityset.Address(1)
	oldRecipient := identityset.Address(2)
	newRecipient := identityset.Address(3)

	topics, data, err := PackVoterRewardDestinationSetEvent(voter, oldRecipient, newRecipient)
	r.NoError(err)
	r.Len(topics, 2)
	r.Equal(hash.Hash256(voterRewardDestinationSetEvent.ID), topics[0])
	r.Equal(hash.BytesToHash256(voter.Bytes()), topics[1])

	values, err := voterRewardDestinationSetEvent.Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Equal(common.BytesToAddress(oldRecipient.Bytes()), values[0])
	r.Equal(common.BytesToAddress(newRecipient.Bytes()), values[1])

	_, _, err = PackVoterRewardDestinationSetEvent(nil, oldRecipient, newRecipient)
	r.Error(err)
}

func mustIntrinsicGas(t *testing.T, act *SetVoterRewardDestination) uint64 {
	t.Helper()
	gas, err := act.IntrinsicGas()
	require.NoError(t, err)
	return gas
}
