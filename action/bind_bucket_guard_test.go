// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
)

func TestBindBucketGuard(t *testing.T) {
	r := require.New(t)
	pub := make([]byte, bucketguard.PubKeySize)
	sig := make([]byte, bucketguard.SigSize)
	act := NewBindBucketGuard(7, bucketguard.AlgSLHDSA192s, pub, sig)

	gas, err := act.IntrinsicGas()
	r.NoError(err)
	r.EqualValues(300496, gas)
	r.NoError(act.SanityCheck())

	core := &iotextypes.ActionCore{}
	act.FillAction(core)
	inner, ok := core.GetAction().(*iotextypes.ActionCore_BindBucketGuard)
	r.True(ok)
	r.EqualValues(7, inner.BindBucketGuard.GetBucketIndex())

	pb := act.Proto()
	var got BindBucketGuard
	r.NoError(got.LoadProto(pb))
	r.EqualValues(7, got.BucketIndex)
	r.Equal(bucketguard.AlgSLHDSA192s, got.AlgID)
	r.Equal(pub, got.PublicKey)
	r.Equal(sig, got.AcceptSig)
	r.Equal(act.Serialize(), got.Serialize())

	r.Equal(ErrNilProto, got.LoadProto(nil))
	bad := NewBindBucketGuard(7, 0xff, pub, sig)
	r.ErrorIs(bad.SanityCheck(), ErrInvalidAct)
}

func TestBindBucketGuardEnvelope(t *testing.T) {
	r := require.New(t)
	pub := make([]byte, bucketguard.PubKeySize)
	sig := make([]byte, bucketguard.SigSize)
	act := NewBindBucketGuard(7, bucketguard.AlgSLHDSA192s, pub, sig)
	core := &iotextypes.ActionCore{}
	act.FillAction(core)

	elp := &envelope{}
	r.NoError(elp.LoadProto(core))
	got, ok := elp.Action().(*BindBucketGuard)
	r.True(ok)
	r.Equal(act, got)
}
