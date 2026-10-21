// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package action

import (
	"bytes"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/iotexproject/iotex-core/v2/action/protocol/staking/bucketguard"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
)

func testGuardedOps() map[string]*bucketguard.GuardedBucketOp {
	alg := pqcrypto.AlgSLHDSA192s
	sig := make([]byte, alg.SignatureSize())
	addr := bytes.Repeat([]byte{0xab}, 20)
	newGuard := &bucketguard.NewGuard{AlgID: alg, PublicKey: make([]byte, alg.PublicKeySize()), AcceptSig: sig}
	op := func(o bucketguard.Op, f func(*bucketguard.GuardedBucketOp)) *bucketguard.GuardedBucketOp {
		m := &bucketguard.GuardedBucketOp{BucketIndex: 7, Generation: 2, Nonce: 3, Deadline: 100, Op: o, Sig: sig}
		f(m)
		return m
	}
	return map[string]*bucketguard.GuardedBucketOp{
		"unstake":        op(bucketguard.OpUnstake, func(*bucketguard.GuardedBucketOp) {}),
		"withdraw":       op(bucketguard.OpWithdraw, func(m *bucketguard.GuardedBucketOp) { m.Recipient = addr }),
		"transfer+guard": op(bucketguard.OpTransfer, func(m *bucketguard.GuardedBucketOp) { m.NewOwner, m.NewGuard = addr, newGuard }),
		"transfer+clear": op(bucketguard.OpTransfer, func(m *bucketguard.GuardedBucketOp) { m.NewOwner, m.ClearGuard = addr, true }),
		"candidate":      op(bucketguard.OpChangeCandidate, func(m *bucketguard.GuardedBucketOp) { m.CandidateName, m.ExpectedCandidateID = "cand1", addr }),
		"restake":        op(bucketguard.OpRestake, func(m *bucketguard.GuardedBucketOp) { m.Duration, m.AutoStake = 91, true }),
		"rotate":         op(bucketguard.OpRotate, func(m *bucketguard.GuardedBucketOp) { m.NewGuard = newGuard }),
	}
}

func TestGuardedBucketOpProtoRoundTrip(t *testing.T) {
	r := require.New(t)
	for name, m := range testGuardedOps() {
		act := NewGuardedBucketOp(m)
		r.NoError(act.SanityCheck(), name)
		core := &iotextypes.ActionCore{}
		act.FillAction(core)
		wire, err := proto.Marshal(core)
		r.NoError(err)
		decoded := &iotextypes.ActionCore{}
		r.NoError(proto.Unmarshal(wire, decoded))
		elp := &envelope{}
		r.NoError(elp.loadProtoActionPayload(decoded), name)
		back, ok := elp.payload.(*GuardedBucketOp)
		r.True(ok, name)
		r.Equal(act.Guard().CanonicalSize(), back.Guard().CanonicalSize(), name)
		g1, _ := act.IntrinsicGas()
		g2, _ := back.IntrinsicGas()
		r.Equal(g1, g2, name)
		r.True(proto.Equal(act.Proto(), back.Proto()), name)
	}
}

func TestBindBucketGuardProtoRoundTrip(t *testing.T) {
	r := require.New(t)
	alg := pqcrypto.AlgSLHDSA192s
	act := NewBindBucketGuard(&bucketguard.BindBucketGuard{BucketIndex: 7, AlgID: alg, PublicKey: make([]byte, alg.PublicKeySize()), AcceptSig: make([]byte, alg.SignatureSize())})
	r.NoError(act.SanityCheck())
	gas, err := act.IntrinsicGas()
	r.NoError(err)
	r.EqualValues(300496, gas) // docs/qsb-spec-and-vectors.md, BIND vector
	core := &iotextypes.ActionCore{}
	act.FillAction(core)
	elp := &envelope{}
	r.NoError(elp.loadProtoActionPayload(core))
	back := elp.payload.(*BindBucketGuard)
	r.True(proto.Equal(act.Proto(), back.Proto()))

	bad := NewBindBucketGuard(&bucketguard.BindBucketGuard{BucketIndex: 7, AlgID: alg, PublicKey: make([]byte, 3)})
	r.True(errors.Is(bad.SanityCheck(), ErrInvalidAct))
}

// The web3 entry must yield the same action and the same price as the native
// encoding, whatever the eth data length.
func TestBucketGuardWeb3Entry(t *testing.T) {
	r := require.New(t)
	build := func(data []byte) Envelope {
		tx := types.NewTx(&types.LegacyTx{Nonce: 1, GasPrice: big.NewInt(1), Gas: 1_000_000, To: &_stakingProtocolEthAddr, Data: data})
		elp, err := (&EnvelopeBuilder{}).BuildStakingAction(tx)
		r.NoError(err)
		return elp
	}
	for name, m := range testGuardedOps() {
		act := NewGuardedBucketOp(m)
		data, err := act.EthData()
		r.NoError(err)
		got, ok := build(data).Action().(*GuardedBucketOp)
		r.True(ok, name)
		r.True(proto.Equal(act.Proto(), got.Proto()), name)
		want, _ := act.IntrinsicGas()
		have, _ := got.IntrinsicGas()
		r.Equal(want, have, name)
	}
	alg := pqcrypto.AlgSLHDSA192s
	bind := NewBindBucketGuard(&bucketguard.BindBucketGuard{BucketIndex: 7, AlgID: alg, PublicKey: make([]byte, alg.PublicKeySize()), AcceptSig: make([]byte, alg.SignatureSize())})
	data, err := bind.EthData()
	r.NoError(err)
	got, ok := build(data).Action().(*BindBucketGuard)
	r.True(ok)
	r.True(proto.Equal(bind.Proto(), got.Proto()))
}
