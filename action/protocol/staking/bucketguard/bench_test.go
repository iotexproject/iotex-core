// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bucketguard

import (
	"errors"
	"testing"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

// The benchmarks below time the signature work of a guarded action, the part
// G_VERIFY prices (IIP-65 §8): the transcripts are built and every required
// signature is verified, as in the handlers. Run them with
//
//	go test -run '^$' -bench . -benchtime 200x ./action/protocol/staking/bucketguard/
//
// On amd64, GODEBUG=cpu.sha=off measures the same code without the SHA
// extensions.

func benchSign(b *testing.B, k *signer.PrivateKey, ctx string, msg []byte) []byte {
	b.Helper()
	sig, err := k.Sign([]byte(ctx), msg)
	if err != nil {
		b.Fatal(err)
	}
	return sig
}

// benchOp returns a signed op on bucket 1 at generation 1, nonce 2, and the
// guard it is checked against. With next, the op installs next's key.
func benchOp(b *testing.B, o Op, cur, next *signer.PrivateKey, owner address.Address, f func(*GuardedBucketOp)) (*Guard, *GuardedBucketOp) {
	b.Helper()
	g := &Guard{AlgID: alg, PublicKey: cur.PublicKey().Bytes(), Generation: 1, Nonce: 2, BoundHeight: 1}
	op := anchored(&GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 2, Deadline: 1010, Op: o}, 1000)
	if f != nil {
		f(op)
	}
	resultingOwner := owner
	if len(op.NewOwner) > 0 {
		resultingOwner, _ = address.FromBytes(op.NewOwner)
	}
	if next != nil {
		pub := next.PublicKey().Bytes()
		op.NewGuard = &NewGuard{AlgID: alg, PublicKey: pub}
		h := CallHash(testDomain, owner, op)
		op.NewGuard.AcceptSig = benchSign(b, next, AcceptContext, EncodeAccept(testDomain, op.BucketIndex, op.Generation+1, resultingOwner, alg, pub, h))
	}
	op.Sig = benchSign(b, cur, CallContext, EncodeCall(testDomain, owner, op))
	if err := op.Validate(ParamsV1()); err != nil {
		b.Fatal(err)
	}
	return g, op
}

// BenchmarkVerifyOp times VerifyOp, which builds CALL (and ACCEPT) and
// verifies the signatures, for one and two verifications, the largest op,
// and the failing cases a block can be filled with.
func BenchmarkVerifyOp(b *testing.B) {
	owner, newOwner := testAddr(b, 0x11), testAddr(b, 0x22)
	cur, next := newTestKey(b, 1), newTestKey(b, 2)
	transfer := func(op *GuardedBucketOp) { op.NewOwner = newOwner.Bytes() }

	unstakeG, unstake := benchOp(b, OpUnstake, cur, nil, owner, nil)
	rotateG, rotate := benchOp(b, OpRotate, cur, next, owner, nil)
	transferG, transferOp := benchOp(b, OpTransfer, cur, next, owner, transfer)

	// A CALL signature over another message fails only at the final
	// comparison, so it costs a full verification.
	badCall := *unstake
	badCall.Sig = rotate.Sig
	// A valid CALL with an ACCEPT for another CALL: both are verified.
	_, other := benchOp(b, OpRotate, cur, next, owner, func(op *GuardedBucketOp) { op.Deadline = 1009 })
	badAccept := *rotate
	ng := *rotate.NewGuard
	ng.AcceptSig = other.NewGuard.AcceptSig
	badAccept.NewGuard = &ng
	badAccept.Sig = benchSign(b, cur, CallContext, EncodeCall(testDomain, owner, &badAccept))

	for _, bc := range []struct {
		name string
		g    *Guard
		op   *GuardedBucketOp
		want error
	}{
		{"unstake/1-verify", unstakeG, unstake, nil},
		{"unstake/bad-call", unstakeG, &badCall, ErrGuardSignature},
		{"rotate/2-verify", rotateG, rotate, nil},
		{"rotate/bad-accept", rotateG, &badAccept, ErrGuardSignature},
		{"transfer-new-guard/2-verify-max-size", transferG, transferOp, nil},
	} {
		b.Run(bc.name, func(b *testing.B) {
			p := ParamsV1()
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := VerifyOp(p, testDomain, owner, bc.g, bc.op); !errors.Is(err, bc.want) {
					b.Fatalf("want %v, got %v", bc.want, err)
				}
			}
		})
	}
}

// BenchmarkAuthorizeBind times the bind check, one ACCEPT verification.
func BenchmarkAuthorizeBind(b *testing.B) {
	owner := testAddr(b, 0x11)
	k := newTestKey(b, 1)
	pub := k.PublicKey().Bytes()
	m := &BindBucketGuard{BucketIndex: 1, AlgID: alg, PublicKey: pub,
		AcceptSig: benchSign(b, k, AcceptContext, EncodeAccept(testDomain, 1, 1, owner, alg, pub, [CallHashLen]byte{}))}
	p := ParamsV1()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := AuthorizeBind(p, testDomain, owner, BindScope{}, nil, m, 10); err != nil {
			b.Fatal(err)
		}
	}
}
