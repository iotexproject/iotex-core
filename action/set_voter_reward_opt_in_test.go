// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestSetVoterRewardOptInRoundTrip(t *testing.T) {
	r := require.New(t)
	act := NewSetVoterRewardOptIn()

	r.NoError(act.SanityCheck())
	gas, err := act.IntrinsicGas()
	r.NoError(err)
	r.Equal(SetVoterRewardOptInBaseIntrinsicGas, gas)
	pb, err := proto.Marshal(act.Proto())
	r.NoError(err)
	r.Empty(pb)

	fromProto := &SetVoterRewardOptIn{}
	r.NoError(fromProto.LoadProto(act.Proto()))

	elp := (&EnvelopeBuilder{}).SetNonce(1).SetAction(act).Build()
	loaded := &envelope{}
	r.NoError(loaded.LoadProto(elp.Proto()))
	loadedAction, ok := loaded.Action().(*SetVoterRewardOptIn)
	r.True(ok)
	r.NotNil(loadedAction)

	data, err := act.EthData()
	r.NoError(err)
	r.Len(data, 4)
	fromABI, err := NewSetVoterRewardOptInFromABIBinary(data)
	r.NoError(err)
	r.NotNil(fromABI)

	_, err = NewSetVoterRewardOptInFromABIBinary([]byte{1, 2, 3})
	r.ErrorIs(err, errDecodeFailure)
	_, err = NewSetVoterRewardOptInFromABIBinary(append(data, 0))
	r.ErrorIs(err, errDecodeFailure)
	r.Error((&SetVoterRewardOptIn{}).LoadProto(nil))
}
