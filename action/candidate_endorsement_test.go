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

func TestCandidateEndorsement_NewInvalidOp(t *testing.T) {
	r := require.New(t)
	// the non-legacy constructor must reject the legacy sentinel op
	_, err := NewCandidateEndorsement(1, CandidateEndorsementOpLegacy)
	r.ErrorContains(err, "invalid operation")

	act, err := NewCandidateEndorsement(5, CandidateEndorsementOpEndorse)
	r.NoError(err)
	r.EqualValues(5, act.BucketIndex())
	r.False(act.IsLegacy())
	r.Equal(CandidateEndorsementOpEndorse, act.Op())

	gas, err := act.IntrinsicGas()
	r.NoError(err)
	r.Equal(CandidateEndorsementBaseIntrinsicGas, gas)
	r.NoError(act.SanityCheck())
}

func TestCandidateEndorsement_LegacyOpResolution(t *testing.T) {
	r := require.New(t)
	// legacy endorse resolves to Endorse
	endorse := NewCandidateEndorsementLegacy(1, true)
	r.True(endorse.IsLegacy())
	r.Equal(CandidateEndorsementOpEndorse, endorse.Op())

	// legacy un-endorse resolves to IntentToRevoke
	unendorse := NewCandidateEndorsementLegacy(1, false)
	r.True(unendorse.IsLegacy())
	r.Equal(CandidateEndorsementOpIntentToRevoke, unendorse.Op())
}

func TestCandidateEndorsement_ProtoRoundTrip(t *testing.T) {
	r := require.New(t)
	act, err := NewCandidateEndorsement(9, CandidateEndorsementOpRevoke)
	r.NoError(err)
	pb := act.Proto()
	r.EqualValues(9, pb.GetBucketIndex())
	r.EqualValues(CandidateEndorsementOpRevoke, pb.GetOp())
	r.False(pb.GetEndorse())

	var got CandidateEndorsement
	r.NoError(got.LoadProto(pb))
	r.EqualValues(9, got.BucketIndex())
	r.Equal(CandidateEndorsementOpRevoke, got.Op())
	r.False(got.IsLegacy())

	// legacy path carries the endorse boolean through the proto
	legacy := NewCandidateEndorsementLegacy(2, true)
	var gotLegacy CandidateEndorsement
	r.NoError(gotLegacy.LoadProto(legacy.Proto()))
	r.True(gotLegacy.IsLegacy())
	r.Equal(CandidateEndorsementOpEndorse, gotLegacy.Op())
}

func TestCandidateEndorsement_LoadProtoNil(t *testing.T) {
	r := require.New(t)
	var act CandidateEndorsement
	r.Equal(ErrNilProto, act.LoadProto(nil))
}

func TestCandidateEndorsement_FillAction(t *testing.T) {
	r := require.New(t)
	act, err := NewCandidateEndorsement(4, CandidateEndorsementOpEndorse)
	r.NoError(err)
	core := &iotextypes.ActionCore{}
	act.FillAction(core)
	inner, ok := core.GetAction().(*iotextypes.ActionCore_CandidateEndorsement)
	r.True(ok)
	r.EqualValues(4, inner.CandidateEndorsement.GetBucketIndex())
}

func TestCandidateEndorsement_EthDataRoundTrip(t *testing.T) {
	cases := []struct {
		name   string
		op     CandidateEndorsementOp
		method func() []byte
	}{
		{"endorse", CandidateEndorsementOpEndorse, func() []byte { return candidateEndorsementEndorseMethod.ID }},
		{"intentToRevoke", CandidateEndorsementOpIntentToRevoke, func() []byte { return candidateEndorsementIntentToRevokeMethod.ID }},
		{"revoke", CandidateEndorsementOpRevoke, func() []byte { return candidateEndorsementRevokeMethod.ID }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := require.New(t)
			act, err := NewCandidateEndorsement(11, c.op)
			r.NoError(err)
			data, err := act.EthData()
			r.NoError(err)
			r.True(bytes.Equal(data[:4], c.method()))

			parsed, err := NewCandidateEndorsementFromABIBinary(data)
			r.NoError(err)
			r.EqualValues(11, parsed.BucketIndex())
			r.Equal(c.op, parsed.Op())
		})
	}
}

func TestCandidateEndorsement_EthDataLegacyRoundTrip(t *testing.T) {
	r := require.New(t)
	act := NewCandidateEndorsementLegacy(6, true)
	data, err := act.EthData()
	r.NoError(err)
	r.True(bytes.Equal(data[:4], candidateEndorsementLegacyMethod.ID))

	parsed, err := NewCandidateEndorsementFromABIBinary(data)
	r.NoError(err)
	r.EqualValues(6, parsed.BucketIndex())
	r.True(parsed.IsLegacy())
	// endorse flag must survive the ABI round trip
	r.Equal(CandidateEndorsementOpEndorse, parsed.Op())
}

func TestCandidateEndorsement_ConstructorOpValidation(t *testing.T) {
	r := require.New(t)
	// The public constructor only rejects the legacy sentinel op; it does NOT
	// range-check other values, so an out-of-range op is accepted at
	// construction and is only rejected later at EthData encoding time.
	act, err := NewCandidateEndorsement(1, CandidateEndorsementOp(99))
	r.NoError(err)
	r.EqualValues(99, act.Op())
	_, err = act.EthData()
	r.ErrorContains(err, "invalid operation")
}

func TestCandidateEndorsement_FromABIBinaryErrors(t *testing.T) {
	r := require.New(t)
	// too short
	_, err := NewCandidateEndorsementFromABIBinary([]byte{0x1, 0x2})
	r.Equal(errDecodeFailure, err)

	// unknown selector of valid length
	bad := make([]byte, 36)
	copy(bad[:4], []byte{0xde, 0xad, 0xbe, 0xef})
	_, err = NewCandidateEndorsementFromABIBinary(bad)
	r.Equal(errDecodeFailure, err)
}
