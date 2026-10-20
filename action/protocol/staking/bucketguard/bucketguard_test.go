package bucketguard

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/cloudflare/circl/sign/slhdsa"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func testOwner() Address {
	var a Address
	for i := range a {
		a[i] = 0x11
	}
	return a
}

func zeroSig() []byte { return make([]byte, SigSize) }
func zeroPub() []byte { return make([]byte, PubKeySize) }

func TestTranscriptVectors(t *testing.T) {
	owner := testOwner()
	op := &GuardedBucketOp{
		BucketIndex: 1,
		Generation:  1,
		Nonce:       0,
		Deadline:    1010,
		Op:          OpUnstake,
	}
	gotCall := hex.EncodeToString(EncodeCall(4689, owner, op))
	wantCall := "494f54582d5153422d43414c4c00001251000000000000000111111111111111111111111111111111111111110000000000000001000000000000000000000000000003f2000000010000000000000000000000000000000000000000000000"
	if gotCall != wantCall {
		t.Fatalf("CALL transcript mismatch\n got %s\nwant %s", gotCall, wantCall)
	}

	pub := mustHex(t, "c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a")
	gotAccept := hex.EncodeToString(EncodeAccept(4689, 1, 1, owner, AlgSLHDSA192s, pub, 0))
	wantAccept := "494f54582d5153422d414343455054000012510000000000000001000000000000000111111111111111111111111111111111111111110000001200000030c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a0000000000000000"
	if gotAccept != wantAccept {
		t.Fatalf("ACCEPT transcript mismatch\n got %s\nwant %s", gotAccept, wantAccept)
	}
}

func TestCanonicalSizeAndGasVectors(t *testing.T) {
	tests := []struct {
		name string
		size int
		gas  uint64
		got  interface {
			CanonicalSize() int
			IntrinsicGas() uint64
		}
	}{
		{
			name: "BindBucketGuard",
			size: 16281,
			gas:  300496,
			got:  &BindBucketGuard{BucketIndex: 1, AlgID: AlgSLHDSA192s, PublicKey: zeroPub(), AcceptSig: zeroSig()},
		},
		{
			name: "UNSTAKE",
			size: 16236,
			gas:  299776,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpUnstake, Sig: zeroSig()},
		},
		{
			name: "WITHDRAW",
			size: 16258,
			gas:  300128,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpWithdraw, Recipient: make([]byte, 20), Sig: zeroSig()},
		},
		{
			name: "TRANSFER+clear",
			size: 16260,
			gas:  300160,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpTransfer, NewOwner: make([]byte, 20), ClearGuard: true, Sig: zeroSig()},
		},
		{
			name: "CHANGE_CANDIDATE",
			size: 16270,
			gas:  300320,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpChangeCandidate, CandidateName: "delegateab", ExpectedCandidateID: make([]byte, 20), Sig: zeroSig()},
		},
		{
			name: "RESTAKE",
			size: 16240,
			gas:  299840,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpRestake, Duration: 7, AutoStake: true, Sig: zeroSig()},
		},
		{
			name: "ROTATE",
			size: 32518,
			gas:  590288,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpRotate, NewGuard: &NewGuard{AlgID: AlgSLHDSA192s, PublicKey: zeroPub(), AcceptSig: zeroSig()}, Sig: zeroSig()},
		},
		{
			name: "TRANSFER+newGuard",
			size: 32540,
			gas:  590640,
			got:  &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpTransfer, NewOwner: make([]byte, 20), NewGuard: &NewGuard{AlgID: AlgSLHDSA192s, PublicKey: zeroPub(), AcceptSig: zeroSig()}, Sig: zeroSig()},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got.CanonicalSize(); got != tc.size {
				t.Fatalf("size = %d, want %d", got, tc.size)
			}
			if got := tc.got.IntrinsicGas(); got != tc.gas {
				t.Fatalf("gas = %d, want %d", got, tc.gas)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	validBind := &BindBucketGuard{BucketIndex: 1, AlgID: AlgSLHDSA192s, PublicKey: zeroPub(), AcceptSig: zeroSig()}
	if err := validBind.Validate(); err != nil {
		t.Fatalf("valid bind rejected: %v", err)
	}
	for name, bind := range map[string]*BindBucketGuard{
		"algID":     {BucketIndex: 1, AlgID: 0x13, PublicKey: zeroPub(), AcceptSig: zeroSig()},
		"publicKey": {BucketIndex: 1, AlgID: AlgSLHDSA192s, PublicKey: make([]byte, PubKeySize-1), AcceptSig: zeroSig()},
		"acceptSig": {BucketIndex: 1, AlgID: AlgSLHDSA192s, PublicKey: zeroPub(), AcceptSig: make([]byte, SigSize-1)},
	} {
		t.Run("bind/"+name, func(t *testing.T) {
			if err := bind.Validate(); err == nil {
				t.Fatal("invalid bind accepted")
			}
		})
	}

	validOp := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpUnstake, Sig: zeroSig()}
	if err := validOp.Validate(); err != nil {
		t.Fatalf("valid op rejected: %v", err)
	}
	badOps := map[string]*GuardedBucketOp{
		"deadline":      {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 0, Op: OpUnstake, Sig: zeroSig()},
		"sig":           {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpUnstake, Sig: make([]byte, SigSize-1)},
		"unknownOp":     {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: 99, Sig: zeroSig()},
		"unstakeFields": {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpUnstake, Recipient: make([]byte, 20), Sig: zeroSig()},
		"transferBoth":  {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpTransfer, NewOwner: make([]byte, 20), NewGuard: &NewGuard{AlgID: AlgSLHDSA192s, PublicKey: zeroPub(), AcceptSig: zeroSig()}, ClearGuard: true, Sig: zeroSig()},
		"transferNone":  {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpTransfer, NewOwner: make([]byte, 20), Sig: zeroSig()},
		"candidate":     {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpChangeCandidate, CandidateName: "delegate-a", ExpectedCandidateID: make([]byte, 20), Sig: zeroSig()},
		"restake":       {BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpRestake, Duration: MaxRestakeDurationDays + 1, Sig: zeroSig()},
	}
	for name, op := range badOps {
		t.Run("op/"+name, func(t *testing.T) {
			if err := op.Validate(); err == nil {
				t.Fatal("invalid op accepted")
			}
		})
	}
}

func TestActive(t *testing.T) {
	if Active(0, 0) || Active(99, 0) || Active(99, 100) {
		t.Fatal("inactive fork reported active")
	}
	if !Active(100, 100) || !Active(101, 100) {
		t.Fatal("active fork reported inactive")
	}
}

func TestVerifyRoundTrip(t *testing.T) {
	pub, priv, err := slhdsa.GenerateKey(rand.Reader, slhdsa.SHA2_192s)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := pub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	op := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpUnstake}
	call := EncodeCall(4689, testOwner(), op)
	sig, err := slhdsa.SignDeterministic(&priv, slhdsa.NewMessage(call), []byte(Context))
	if err != nil {
		t.Fatal(err)
	}
	if !Verify(pubBytes, call, sig) {
		t.Fatal("valid signature rejected")
	}

	tamperedMsg := append([]byte(nil), call...)
	tamperedMsg[0] ^= 1
	if Verify(pubBytes, tamperedMsg, sig) {
		t.Fatal("tampered message accepted")
	}
	tamperedSig := append([]byte(nil), sig...)
	tamperedSig[0] ^= 1
	if Verify(pubBytes, call, tamperedSig) {
		t.Fatal("tampered signature accepted")
	}
	otherPub, _, err := slhdsa.GenerateKey(rand.Reader, slhdsa.SHA2_192s)
	if err != nil {
		t.Fatal(err)
	}
	otherBytes, err := otherPub.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	if Verify(otherBytes, call, sig) {
		t.Fatal("wrong public key accepted")
	}
	if Verify(pubBytes[:PubKeySize-1], call, sig) || Verify(pubBytes, call, sig[:SigSize-1]) {
		t.Fatal("malformed sizes accepted")
	}
}
