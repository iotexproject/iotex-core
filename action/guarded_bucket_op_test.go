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

func TestGuardedBucketOp(t *testing.T) {
	r := require.New(t)
	sig := make([]byte, bucketguard.SigSize)
	op := &bucketguard.GuardedBucketOp{
		BucketIndex: 1,
		Generation:  1,
		Nonce:       0,
		Deadline:    1010,
		Op:          bucketguard.OpUnstake,
		Sig:         sig,
	}
	act := NewGuardedBucketOp(op)

	gas, err := act.IntrinsicGas()
	r.NoError(err)
	r.EqualValues(299776, gas)
	r.NoError(act.SanityCheck())

	core := &iotextypes.ActionCore{}
	act.FillAction(core)
	inner, ok := core.GetAction().(*iotextypes.ActionCore_GuardedBucketOp)
	r.True(ok)
	r.EqualValues(1, inner.GuardedBucketOp.GetBucketIndex())

	pb := act.Proto()
	var got GuardedBucketOp
	r.NoError(got.LoadProto(pb))
	r.Equal(op.BucketIndex, got.BucketIndex)
	r.Equal(op.Generation, got.Generation)
	r.Equal(op.Nonce, got.Nonce)
	r.Equal(op.Deadline, got.Deadline)
	r.Equal(op.Op, got.Op)
	r.Equal(op.Sig, got.Sig)
	r.Equal(act.Serialize(), got.Serialize())

	r.Equal(ErrNilProto, got.LoadProto(nil))
	bad := NewGuardedBucketOp(&bucketguard.GuardedBucketOp{
		BucketIndex: 1,
		Deadline:    1,
		Op:          bucketguard.OpUnstake,
		Sig:         sig,
		Recipient:   make([]byte, 20),
	})
	r.ErrorIs(bad.SanityCheck(), ErrInvalidAct)
}

func TestGuardedBucketOpNewGuardProto(t *testing.T) {
	r := require.New(t)
	op := &bucketguard.GuardedBucketOp{
		BucketIndex: 2,
		Generation:  1,
		Nonce:       3,
		Deadline:    9,
		Op:          bucketguard.OpRotate,
		Sig:         make([]byte, bucketguard.SigSize),
		NewGuard: &bucketguard.NewGuard{
			AlgID:     bucketguard.AlgSLHDSA192s,
			PublicKey: make([]byte, bucketguard.PubKeySize),
			AcceptSig: make([]byte, bucketguard.SigSize),
		},
	}
	act := NewGuardedBucketOp(op)
	r.NoError(act.SanityCheck())

	var got GuardedBucketOp
	r.NoError(got.LoadProto(act.Proto()))
	r.NotNil(got.NewGuard)
	r.Equal(op.NewGuard.AlgID, got.NewGuard.AlgID)
	r.Equal(op.NewGuard.PublicKey, got.NewGuard.PublicKey)
	r.Equal(op.NewGuard.AcceptSig, got.NewGuard.AcceptSig)
}
