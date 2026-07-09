// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
)

func TestCandidateActivate_Getters(t *testing.T) {
	r := require.New(t)
	act := NewCandidateActivate(7)
	r.EqualValues(7, act.BucketID())

	gas, err := act.IntrinsicGas()
	r.NoError(err)
	r.Equal(CandidateActivateBaseIntrinsicGas, gas)

	// SanityCheck has no constraints and must never reject
	r.NoError(act.SanityCheck())
}

func TestCandidateActivate_ProtoRoundTrip(t *testing.T) {
	r := require.New(t)
	act := NewCandidateActivate(42)
	pb := act.Proto()
	r.EqualValues(42, pb.GetBucketIndex())

	var got CandidateActivate
	r.NoError(got.LoadProto(pb))
	r.EqualValues(42, got.BucketID())
}

func TestCandidateActivate_LoadProtoNil(t *testing.T) {
	r := require.New(t)
	var act CandidateActivate
	r.Equal(ErrNilProto, act.LoadProto(nil))
}

func TestCandidateActivate_FillAction(t *testing.T) {
	r := require.New(t)
	act := NewCandidateActivate(3)
	core := &iotextypes.ActionCore{}
	act.FillAction(core)
	inner, ok := core.GetAction().(*iotextypes.ActionCore_CandidateActivate)
	r.True(ok)
	r.EqualValues(3, inner.CandidateActivate.GetBucketIndex())
}

func TestCandidateActivate_EthDataRoundTrip(t *testing.T) {
	r := require.New(t)
	act := NewCandidateActivate(99)
	data, err := act.EthData()
	r.NoError(err)
	r.True(len(data) >= 4)
	r.True(bytes.Equal(data[:4], candidateActivateMethod.ID))

	parsed, err := NewCandidateActivateFromABIBinary(data)
	r.NoError(err)
	r.EqualValues(99, parsed.BucketID())
}

func TestCandidateActivate_FromABIBinaryErrors(t *testing.T) {
	r := require.New(t)
	// too short: no room for a 4-byte selector + payload
	_, err := NewCandidateActivateFromABIBinary([]byte{0x1, 0x2, 0x3})
	r.Equal(errDecodeFailure, err)

	// correct length but wrong selector
	bad := make([]byte, 36)
	bad[0] = candidateActivateMethod.ID[0] ^ 0xff
	_, err = NewCandidateActivateFromABIBinary(bad)
	r.Equal(errDecodeFailure, err)

	// valid selector but truncated/garbage payload fails ABI unpack
	_, err = NewCandidateActivateFromABIBinary(append(append([]byte{}, candidateActivateMethod.ID...), 0x1))
	r.Error(err)
	r.NotEqual(errDecodeFailure, err)
}
