// Copyright (c) 2026 IoTeX Foundation
// This source code is provided 'as is' and no warranties are given as to title or non-infringement, merchantability
// or fitness for purpose and, to the extent permitted by law, all liability for your use of the code is disclaimed.
// This source code is governed by Apache License 2.0 that can be found in the LICENSE file.

package bucketguard

import (
	"bytes"
	"testing"

	"github.com/iotexproject/iotex-address/address"
)

// fuzzSeedOps are valid ops of every kind, the seeds of the web3 fuzz target.
func fuzzSeedOps() []*GuardedBucketOp {
	addr := bytes.Repeat([]byte{0xab}, 20)
	op := func(o Op, f func(*GuardedBucketOp)) *GuardedBucketOp {
		m := &GuardedBucketOp{BucketIndex: 9, Generation: 3, Nonce: 4, Deadline: 1010,
			AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Op: o, Sig: zeroSig()}
		f(m)
		return m
	}
	return []*GuardedBucketOp{
		op(OpUnstake, func(*GuardedBucketOp) {}),
		op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient, m.Sponsor = addr, bytes.Repeat([]byte{0x22}, 20) }),
		op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.ClearGuard = addr, true }),
		op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = addr, zeroNewGuard() }),
		op(OpChangeCandidate, func(m *GuardedBucketOp) { m.CandidateName, m.ExpectedCandidateID = "delegateab", addr }),
		op(OpRestake, func(m *GuardedBucketOp) { m.Duration, m.AutoStake = 91, true }),
		op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard = zeroNewGuard() }),
	}
}

// FuzzWeb3Decode feeds arbitrary calldata to the web3 entry (IIP-65 §6). It
// must never panic, and a call it accepts must survive the checks a node runs
// on it: Validate, pricing, the CALL transcript, and re-encoding, which must
// give the same canonical value and price.
func FuzzWeb3Decode(f *testing.F) {
	bind := &BindBucketGuard{BucketIndex: 9, AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()}
	if data, err := bind.EthData(); err == nil {
		f.Add(data)
	}
	for _, m := range fuzzSeedOps() {
		if data, err := m.EthData(); err == nil {
			f.Add(data)
		}
	}
	p := ParamsV1()
	owner, _ := address.FromBytes(bytes.Repeat([]byte{0x11}, 20))
	f.Fuzz(func(t *testing.T, data []byte) {
		if b, err := BindBucketGuardFromEthData(data); err == nil {
			_ = b.Validate(p)
			_ = b.IntrinsicGas(p)
			out, err := b.EthData()
			if err != nil {
				t.Fatalf("decoded bind does not re-encode: %v", err)
			}
			back, err := BindBucketGuardFromEthData(out)
			if err != nil || !bytes.Equal(back.canonicalBytes(), b.canonicalBytes()) {
				t.Fatalf("bind changed through re-encoding (%v)", err)
			}
		}
		m, err := GuardedBucketOpFromEthData(data)
		if err != nil {
			return
		}
		valid := m.Validate(p) == nil
		_ = m.IntrinsicGas(p)
		_ = EncodeCall(testDomain, owner, m)
		if !valid {
			return
		}
		out, err := m.EthData()
		if err != nil {
			t.Fatalf("valid op does not re-encode: %v", err)
		}
		back, err := GuardedBucketOpFromEthData(out)
		if err != nil {
			t.Fatalf("re-encoded op does not decode: %v", err)
		}
		if !bytes.Equal(back.canonicalBytes(), m.canonicalBytes()) || back.IntrinsicGas(p) != m.IntrinsicGas(p) {
			t.Fatal("op changed through re-encoding")
		}
	})
}
