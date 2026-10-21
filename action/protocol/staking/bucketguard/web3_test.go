package bucketguard

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// Selectors are checked against the Keccak-256 of the canonical signatures,
// independently of the ABI JSON, so an accidental ABI change is caught.
func TestWeb3Selectors(t *testing.T) {
	for method, sig := range map[string]string{
		"bindBucketGuard":        "bindBucketGuard(uint64,uint32,bytes,bytes)",
		"guardedUnstake":         "guardedUnstake(uint64,uint64,uint64,uint64,bytes)",
		"guardedWithdraw":        "guardedWithdraw(uint64,uint64,uint64,uint64,address,bytes)",
		"guardedTransfer":        "guardedTransfer(uint64,uint64,uint64,uint64,address,bool,uint32,bytes,bytes,bytes)",
		"guardedChangeCandidate": "guardedChangeCandidate(uint64,uint64,uint64,uint64,string,address,bytes)",
		"guardedRestake":         "guardedRestake(uint64,uint64,uint64,uint64,uint32,bool,bytes)",
		"guardedRotate":          "guardedRotate(uint64,uint64,uint64,uint64,uint32,bytes,bytes,bytes)",
	} {
		if got, want := web3Contract.Methods[method].ID, crypto.Keccak256([]byte(sig))[:4]; !bytes.Equal(got, want) {
			t.Errorf("%s selector = %x, want %x (%s)", method, got, want, sig)
		}
	}
}

func TestWeb3RoundTrip(t *testing.T) {
	p := ParamsV1()
	addr := bytes.Repeat([]byte{0xab}, 20)
	bind := &BindBucketGuard{BucketIndex: 9, AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()}
	data, err := bind.EthData()
	if err != nil {
		t.Fatal(err)
	}
	back, err := BindBucketGuardFromEthData(data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back.canonicalBytes(), bind.canonicalBytes()) || back.IntrinsicGas(p) != bind.IntrinsicGas(p) {
		t.Fatal("bind changed through the web3 envelope")
	}

	op := func(o Op, f func(*GuardedBucketOp)) *GuardedBucketOp {
		m := &GuardedBucketOp{BucketIndex: 9, Generation: 3, Nonce: 4, Deadline: 1010, Op: o, Sig: zeroSig()}
		f(m)
		return m
	}
	for name, m := range map[string]*GuardedBucketOp{
		"unstake":        op(OpUnstake, func(*GuardedBucketOp) {}),
		"withdraw":       op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient = addr }),
		"transfer+clear": op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.ClearGuard = addr, true }),
		"transfer+guard": op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = addr, zeroNewGuard() }),
		"candidate":      op(OpChangeCandidate, func(m *GuardedBucketOp) { m.CandidateName, m.ExpectedCandidateID = "delegateab", addr }),
		"restake":        op(OpRestake, func(m *GuardedBucketOp) { m.Duration, m.AutoStake = 91, true }),
		"restake-zero":   op(OpRestake, func(*GuardedBucketOp) {}),
		"rotate":         op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard = zeroNewGuard() }),
	} {
		t.Run(name, func(t *testing.T) {
			data, err := m.EthData()
			if err != nil {
				t.Fatal(err)
			}
			back, err := GuardedBucketOpFromEthData(data)
			if err != nil {
				t.Fatal(err)
			}
			if err := back.Validate(p); err != nil {
				t.Fatalf("decoded op invalid: %v", err)
			}
			if !bytes.Equal(back.canonicalBytes(), m.canonicalBytes()) {
				t.Fatal("op changed through the web3 envelope")
			}
			if back.IntrinsicGas(p) != m.IntrinsicGas(p) {
				t.Fatal("web3 and native entries price differently")
			}
		})
	}
}

func TestWeb3Rejects(t *testing.T) {
	if _, err := GuardedBucketOpFromEthData([]byte{1, 2}); !errors.Is(err, ErrWeb3Decode) {
		t.Errorf("short data: %v", err)
	}
	if _, err := GuardedBucketOpFromEthData([]byte{0xde, 0xad, 0xbe, 0xef, 0}); !errors.Is(err, ErrWeb3Decode) {
		t.Errorf("unknown selector: %v", err)
	}
	bind, _ := (&BindBucketGuard{BucketIndex: 1, AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()}).EthData()
	if _, err := GuardedBucketOpFromEthData(bind); !errors.Is(err, ErrWeb3Decode) {
		t.Errorf("bind decoded as op: %v", err)
	}
	unstake, _ := (&GuardedBucketOp{BucketIndex: 1, Deadline: 1, Op: OpUnstake, Sig: zeroSig()}).EthData()
	if _, err := GuardedBucketOpFromEthData(unstake[:len(unstake)-32]); !errors.Is(err, ErrWeb3Decode) {
		t.Errorf("truncated call: %v", err)
	}
	if _, err := (&GuardedBucketOp{Op: OpWithdraw, Recipient: make([]byte, 19)}).EthData(); !errors.Is(err, ErrMalformed) {
		t.Errorf("19-byte recipient encoded: %v", err)
	}
	// Both a new key and clearGuard: decodes, then fails Validate like the native form.
	m := &GuardedBucketOp{BucketIndex: 1, Deadline: 1, Op: OpTransfer, NewOwner: make([]byte, 20), ClearGuard: true, NewGuard: zeroNewGuard(), Sig: zeroSig()}
	data, err := m.EthData()
	if err != nil {
		t.Fatal(err)
	}
	back, err := GuardedBucketOpFromEthData(data)
	if err != nil {
		t.Fatal(err)
	}
	if err := back.Validate(ParamsV1()); !errors.Is(err, ErrMalformed) {
		t.Errorf("transfer with newGuard and clearGuard: %v", err)
	}
}
