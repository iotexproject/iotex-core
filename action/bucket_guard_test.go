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

	"github.com/ethereum/go-ethereum/common"
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
	r.EqualValues(1668100, gas) // docs/qsb-spec-and-vectors.md, BIND vector
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

// The guarded methods are published in the native staking ABI so clients can
// encode them; bucketguard decodes with its own copy of the ABI because it
// cannot import this package. Every call bucketguard encodes must be a call
// of the published method with the same selector, encoding and argument
// names, so the two cannot drift.
func TestNativeStakingABIGuardedMethods(t *testing.T) {
	r := require.New(t)
	alg := pqcrypto.AlgSLHDSA192s
	fill := func(b byte, n int) []byte { return bytes.Repeat([]byte{b}, n) }
	// Distinct values per field, so an argument published under the wrong
	// name is caught.
	sig, pub, accept := fill(1, alg.SignatureSize()), fill(2, alg.PublicKeySize()), fill(3, alg.SignatureSize())
	addrA, addrB := fill(0xaa, 20), fill(0xbb, 20)
	newGuard := &bucketguard.NewGuard{AlgID: alg, PublicKey: pub, AcceptSig: accept}
	type testCase struct {
		method string
		act    interface{ EthData() ([]byte, error) }
		args   map[string]any // the published arguments the call must decode to
	}
	guarded := func(method string, o bucketguard.Op, f func(*bucketguard.GuardedBucketOp), args map[string]any) testCase {
		m := &bucketguard.GuardedBucketOp{BucketIndex: 7, Generation: 2, Nonce: 3, Deadline: 100, Op: o, Sig: sig}
		f(m)
		args["bucketIndex"], args["generation"], args["nonce"], args["deadline"], args["sig"] =
			uint64(7), uint64(2), uint64(3), uint64(100), sig
		return testCase{method, NewGuardedBucketOp(m), args}
	}
	tests := []testCase{
		{"bindBucketGuard",
			NewBindBucketGuard(&bucketguard.BindBucketGuard{BucketIndex: 7, AlgID: alg, PublicKey: pub, AcceptSig: accept}),
			map[string]any{"bucketIndex": uint64(7), "algID": uint32(alg), "publicKey": pub, "acceptSig": accept}},
		guarded("guardedUnstake", bucketguard.OpUnstake, func(*bucketguard.GuardedBucketOp) {}, map[string]any{}),
		guarded("guardedWithdraw", bucketguard.OpWithdraw,
			func(m *bucketguard.GuardedBucketOp) { m.Recipient = addrA },
			map[string]any{"recipient": common.BytesToAddress(addrA)}),
		guarded("guardedTransfer", bucketguard.OpTransfer,
			func(m *bucketguard.GuardedBucketOp) { m.NewOwner, m.NewGuard = addrA, newGuard },
			map[string]any{"newOwner": common.BytesToAddress(addrA), "clearGuard": false,
				"newAlgID": uint32(alg), "newPublicKey": pub, "newAcceptSig": accept}),
		guarded("guardedTransfer", bucketguard.OpTransfer,
			func(m *bucketguard.GuardedBucketOp) { m.NewOwner, m.ClearGuard = addrA, true },
			map[string]any{"newOwner": common.BytesToAddress(addrA), "clearGuard": true,
				"newAlgID": uint32(0), "newPublicKey": []byte{}, "newAcceptSig": []byte{}}),
		guarded("guardedChangeCandidate", bucketguard.OpChangeCandidate,
			func(m *bucketguard.GuardedBucketOp) { m.CandidateName, m.ExpectedCandidateID = "cand1", addrB },
			map[string]any{"candidateName": "cand1", "expectedCandidateID": common.BytesToAddress(addrB)}),
		guarded("guardedRestake", bucketguard.OpRestake,
			func(m *bucketguard.GuardedBucketOp) { m.Duration, m.AutoStake = 91, true },
			map[string]any{"duration": uint32(91), "autoStake": true}),
		guarded("guardedRotate", bucketguard.OpRotate,
			func(m *bucketguard.GuardedBucketOp) { m.NewGuard = newGuard },
			map[string]any{"newAlgID": uint32(alg), "newPublicKey": pub, "newAcceptSig": accept}),
	}

	published := NativeStakingContractABI()
	covered := map[string]bool{}
	for _, tc := range tests {
		method, ok := published.Methods[tc.method]
		r.True(ok, "%s is not in the native staking ABI", tc.method)
		r.Equal("nonpayable", method.StateMutability, tc.method)
		data, err := tc.act.EthData()
		r.NoError(err, tc.method)
		r.Equal(method.ID, data[:4], "%s: selector differs from %s", tc.method, method.Sig)

		// Same encoding: the published ABI repacks the decoded arguments to
		// the bytes bucketguard produced.
		vals, err := method.Inputs.Unpack(data[4:])
		r.NoError(err, tc.method)
		repacked, err := published.Pack(tc.method, vals...)
		r.NoError(err, tc.method)
		r.Equal(data, repacked, tc.method)

		// Same names: each published argument carries the field it names.
		got := map[string]any{}
		r.NoError(method.Inputs.UnpackIntoMap(got, data[4:]), tc.method)
		r.Len(got, len(tc.args), tc.method)
		for name, want := range tc.args {
			r.Contains(got, name, tc.method)
			r.Equal(want, got[name], "%s.%s", tc.method, name)
		}
		covered[tc.method] = true
	}
	r.Len(covered, 7, "every guarded method is checked")
}
