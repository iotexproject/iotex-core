// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/test/identityset"
)

func TestSetCommissionRate_SanityCheck(t *testing.T) {
	r := require.New(t)

	r.NoError(NewSetCommissionRate(0).SanityCheck(), "0 (legacy mode) is valid")
	r.NoError(NewSetCommissionRate(1).SanityCheck())
	r.NoError(NewSetCommissionRate(1000).SanityCheck(), "10% is valid")
	r.NoError(NewSetCommissionRate(MaxCommissionRate).SanityCheck(), "exactly 100% is valid")

	err := NewSetCommissionRate(MaxCommissionRate + 1).SanityCheck()
	r.Error(err)
	r.ErrorIs(err, ErrInvalidCommissionRate)
}

func TestSetCommissionRate_IntrinsicGas(t *testing.T) {
	r := require.New(t)
	g, err := NewSetCommissionRate(500).IntrinsicGas()
	r.NoError(err)
	r.Equal(SetCommissionRateBaseIntrinsicGas, g)
}

// Proto roundtrip: a SetCommissionRate action must serialize to its
// protobuf form and back without losing the rate. This also exercises
// the ActionCore oneof field wiring (FillAction).
func TestSetCommissionRate_ProtoRoundTrip(t *testing.T) {
	r := require.New(t)
	original := NewSetCommissionRate(1500)

	core := &iotextypes.ActionCore{}
	original.FillAction(core)
	wrapper, ok := core.Action.(*iotextypes.ActionCore_SetCommissionRate)
	r.True(ok, "FillAction must install the SetCommissionRate oneof")
	r.Equal(uint64(1500), wrapper.SetCommissionRate.GetRate())

	restored := &SetCommissionRate{}
	r.NoError(restored.LoadProto(wrapper.SetCommissionRate))
	r.Equal(uint64(1500), restored.Rate())
}

func TestSetCommissionRate_LoadProtoNil(t *testing.T) {
	r := require.New(t)
	r.ErrorIs((&SetCommissionRate{}).LoadProto(nil), ErrNilProto)
}

// EthData should pack the ABI selector + rate so the action can be
// submitted through the same path as other staking actions reachable from
// MetaMask / hardhat tooling.
func TestSetCommissionRate_EthDataRoundTrip(t *testing.T) {
	r := require.New(t)
	original := NewSetCommissionRate(2500)
	data, err := original.EthData()
	r.NoError(err)
	r.True(len(data) >= 4, "EthData must at least contain the 4-byte selector")
	r.True(bytes.Equal(data[:4], _setCommissionRateMethod.ID))

	restored, err := NewSetCommissionRateFromABIBinary(data)
	r.NoError(err)
	r.Equal(uint64(2500), restored.Rate())
}

func TestSetCommissionRate_FromABIBinaryRejectsGarbage(t *testing.T) {
	r := require.New(t)

	// Empty / too-short payloads must error rather than panic.
	_, err := NewSetCommissionRateFromABIBinary(nil)
	r.ErrorIs(err, errDecodeFailure)

	_, err = NewSetCommissionRateFromABIBinary([]byte{0x01, 0x02, 0x03})
	r.ErrorIs(err, errDecodeFailure)

	// Wrong method selector (use candidateActivate's) should be rejected.
	wrong := append([]byte{}, candidateActivateMethod.ID...)
	wrong = append(wrong, make([]byte, 32)...) // dummy arg
	_, err = NewSetCommissionRateFromABIBinary(wrong)
	r.ErrorIs(err, errDecodeFailure)
}

// The CommissionRateSet event ABI is what indexers will subscribe to —
// verify the topic-0 keccak is stable (matches the human-readable signature)
// and the indexed candidate address ends up in topics[1].
func TestPackCommissionRateSetEvent(t *testing.T) {
	r := require.New(t)
	cand := identityset.Address(3)

	topics, data, err := PackCommissionRateSetEvent(cand, 1500)
	r.NoError(err)
	r.Len(topics, 2, "topic-0 + indexed candidate")

	// Topic 0 = keccak256("CommissionRateSet(address,uint64)")
	r.Equal(hash.Hash256(_commissionRateSetEvent.ID), topics[0])

	// Topic 1 = padded candidate address bytes
	r.Equal(hash.BytesToHash256(cand.Bytes()), topics[1])

	// Non-indexed args: the new rate, packed as uint64 (32-byte ABI slot).
	r.NotEmpty(data)
	unpacked, err := _commissionRateSetEvent.Inputs.NonIndexed().Unpack(data)
	r.NoError(err)
	r.Len(unpacked, 1)
	r.Equal(uint64(1500), unpacked[0])
}
