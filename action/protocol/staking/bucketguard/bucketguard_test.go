package bucketguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto"
	"github.com/iotexproject/iotex-core/v2/pkg/pqcrypto/signer"
)

const alg = pqcrypto.AlgSLHDSA192s

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func testAddr(t testing.TB, fill byte) address.Address {
	t.Helper()
	a, err := address.FromBytes(bytes.Repeat([]byte{fill}, addressLen))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func newTestKey(t testing.TB, fill byte) *signer.PrivateKey {
	t.Helper()
	k, err := signer.NewKeyFromSeeds(alg, bytes.Repeat([]byte{fill}, signer.SeedSize(alg)))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func zeroSig() []byte { return make([]byte, alg.SignatureSize()) }
func zeroPub() []byte { return make([]byte, alg.PublicKeySize()) }

func zeroNewGuard() *NewGuard {
	return &NewGuard{AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()}
}

// testAnchor is the anchor of the vectors: block 1000 with hash 0x33 x 32.
const testAnchorHeight = 1000

func testAnchorHash() []byte { return bytes.Repeat([]byte{0x33}, AnchorHashLen) }

// Transcript vectors of docs/qsb-spec-and-vectors.md §4.3, on the mainnet
// domain (native chain ID 1, mainnet genesis hash). They were computed
// independently with Python (standard library only).
func TestTranscriptVectors(t *testing.T) {
	var genesis [32]byte
	copy(genesis[:], mustHex(t, "b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b"))
	d := NewDomain(1, genesis)
	if got := hex.EncodeToString(d[:]); got != "49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871" {
		t.Fatalf("mainnet domain = %s", got)
	}
	owner := testAddr(t, 0x11)
	op := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010,
		AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Op: OpUnstake}
	wantCall := "49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871000000000000000111111111111111111111111111111111111111110000000000000001000000000000000000000000000003f200000000000003e8333333333333333333333333333333333333333333333333333333333333333300000001000000000000000000000000000000000000000000000000000000"
	if got := hex.EncodeToString(EncodeCall(d, owner, op)); got != wantCall {
		t.Fatalf("CALL transcript mismatch\n got %s\nwant %s", got, wantCall)
	}

	pub := mustHex(t, "c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a")
	wantAccept := "49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed8710000000000000001000000000000000111111111111111111111111111111111111111110000001200000030c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a" + strings.Repeat("00", CallHashLen)
	if got := hex.EncodeToString(EncodeAccept(d, 1, 1, owner, alg, pub, [CallHashLen]byte{})); got != wantAccept {
		t.Fatalf("ACCEPT transcript mismatch\n got %s\nwant %s", got, wantAccept)
	}

	// ROTATE with a bound sponsor: the incoming key accepts SHA-512(CALL).
	rot := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 2, Deadline: 1010,
		AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Op: OpRotate,
		NewGuard: &NewGuard{AlgID: alg, PublicKey: pub}, Sponsor: bytes.Repeat([]byte{0x22}, addressLen)}
	h := CallHash(d, owner, rot)
	if got := hex.EncodeToString(h[:]); got != "f098e3c9f256a7293c8f1e3c155d16011c9b74286435a9c015803f9c5221e0dd7b3c6a31032eba2a1ccc471a4e708e90af5349b6c829381765350cf93aae51bd" {
		t.Fatalf("ROTATE callHash = %s", got)
	}
	acc := sha256.Sum256(EncodeAccept(d, 1, 2, owner, alg, pub, h))
	if got := hex.EncodeToString(acc[:]); got != "98ace1e9e138f66d6beab41858ef6e20f69e28f1ab9e0417ba5170e87241167c" {
		t.Fatalf("ROTATE ACCEPT sha256 = %s", got)
	}

	// guardKeyID of the incoming key, as the BucketGuard event reports it.
	id := (&Guard{AlgID: alg, PublicKey: pub}).KeyID()
	if got := hex.EncodeToString(id[:]); got != "5822fceae8513928ac3b49fc3f87696a3ec93bcaf812bcdcf43d83b750dbe653" {
		t.Fatalf("guardKeyID = %s", got)
	}
}

// Size, gas and C(m) digest vectors of docs/qsb-spec-and-vectors.md, under
// ParamsV1. They were computed independently with Python (standard library
// only). The anchor fields 16 and 17 take two-byte tags.
func TestCanonicalVectors(t *testing.T) {
	type action interface {
		canonicalBytes() []byte
		CanonicalSize() int
		IntrinsicGas(Params) uint64
	}
	addr := bytes.Repeat([]byte{0x44}, addressLen)
	op := func(o Op, f func(*GuardedBucketOp)) *GuardedBucketOp {
		m := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010,
			AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Op: o, Sig: zeroSig()}
		f(m)
		return m
	}
	for _, tc := range []struct {
		name string
		got  action
		size int
		gas  uint64
		hash string
	}{
		{"BindBucketGuard", &BindBucketGuard{BucketIndex: 1, AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()},
			16281, 1668100, "e39fec5a6a1739ff165ab0c0ffe59878fcec475778da8bdddd08f4e4aad09ce4"},
		{"UNSTAKE", op(OpUnstake, func(*GuardedBucketOp) {}),
			16275, 1667500, "5e036b59d04be808968a4831c5eff1259959c7e42889fa4b66050055d06bf1ee"},
		{"WITHDRAW", op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient = addr }),
			16297, 1669700, "a29d9725e9d92e65752889227036b86bc66944060b75fc3e86efecd7083f179d"},
		{"TRANSFER+clear", op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.ClearGuard = addr, true }),
			16299, 1669900, "d7aa8801752cb5f8bb2042781c1fd4409f2f1b597f114e57eccbbc31df4cad93"},
		{"CHANGE_CANDIDATE", op(OpChangeCandidate, func(m *GuardedBucketOp) {
			m.CandidateName, m.ExpectedCandidateID = "delegateab", addr
		}), 16309, 1670900, "8c8fd5613a39f05403f804add7068d30db525c7837adfe89419268c9fc37e726"},
		{"RESTAKE", op(OpRestake, func(m *GuardedBucketOp) { m.Duration, m.AutoStake = 7, true }),
			16279, 1667900, "9c869032738758f64a7ab884a436174d3bbad40daf75c960d286c624f1d50783"},
		{"ROTATE", op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard = zeroNewGuard() }),
			32557, 3325700, "e7836c0e0df88bf9f6348a1d2840f9055a9383358ae4273647f814fef0b5ec17"},
		{"TRANSFER+newGuard", op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = addr, zeroNewGuard() }),
			32579, 3327900, "d831db5b5e2d3038ca27ddd3cccda2476e3d0a239eba9142a90808359e5170e3"},
		{"UNSTAKE+sponsor", op(OpUnstake, func(m *GuardedBucketOp) { m.Sponsor = bytes.Repeat([]byte{0x22}, addressLen) }),
			16297, 1669700, "2d14d32b020768b6a3e273e10a57fbbe16cb6e9c7b8ffc5ec09705440dbb64d5"},
		// anchorHeight 2^28 takes a five-byte varint.
		{"UNSTAKE+anchor2^28", op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHeight = 1 << 28 }),
			16278, 1667800, "e34256c46d6ea409dd0c815faf38c9f1c5cd7e902c870d717fb4abb79455c19d"},
		// A set but empty newGuard is emitted (and later fails Validate).
		{"ROTATE+emptyNewGuard", op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard = &NewGuard{} }),
			16277, 1697700, "2565c01dd96ee1fad524b7bd9c0aa07d7eb346b73b2519861ecd4db7374b6d9a"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got.CanonicalSize(); got != tc.size {
				t.Fatalf("size = %d, want %d", got, tc.size)
			}
			if got := tc.got.IntrinsicGas(ParamsV1()); got != tc.gas {
				t.Fatalf("gas = %d, want %d", got, tc.gas)
			}
			sum := sha256.Sum256(tc.got.canonicalBytes())
			if got := hex.EncodeToString(sum[:]); got != tc.hash {
				t.Fatalf("C(m) sha256 = %s, want %s", got, tc.hash)
			}
		})
	}
}

// The anchor fields take the two-byte protobuf wire tags of fields 16 and 17,
// which QSBSizeV1 counts as tlen = 2.
func TestCanonicalAnchorTags(t *testing.T) {
	m := &GuardedBucketOp{AnchorHeight: 1, AnchorHash: []byte{0xaa}}
	if got := hex.EncodeToString(m.canonicalBytes()); got != "800101"+"8a0101aa" {
		t.Fatalf("anchor fields encode as %s", got)
	}
}

func TestValidate(t *testing.T) {
	p := ParamsV1()
	bind := func(f func(*BindBucketGuard)) *BindBucketGuard {
		m := &BindBucketGuard{BucketIndex: 1, AlgID: alg, PublicKey: zeroPub(), AcceptSig: zeroSig()}
		f(m)
		return m
	}
	if err := bind(func(*BindBucketGuard) {}).Validate(p); err != nil {
		t.Fatalf("valid bind rejected: %v", err)
	}
	for name, m := range map[string]*BindBucketGuard{
		"algID":     bind(func(m *BindBucketGuard) { m.AlgID = 0x13 }),
		"publicKey": bind(func(m *BindBucketGuard) { m.PublicKey = m.PublicKey[1:] }),
		"acceptSig": bind(func(m *BindBucketGuard) { m.AcceptSig = m.AcceptSig[1:] }),
	} {
		if err := m.Validate(p); !errors.Is(err, ErrMalformed) {
			t.Errorf("bind/%s: want ErrMalformed, got %v", name, err)
		}
	}

	op := func(o Op, f func(*GuardedBucketOp)) *GuardedBucketOp {
		m := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Deadline: 1010,
			AnchorHeight: testAnchorHeight, AnchorHash: testAnchorHash(), Op: o, Sig: zeroSig()}
		f(m)
		return m
	}
	addr := bytes.Repeat([]byte{0x44}, addressLen)
	zero := make([]byte, addressLen)
	valid := map[string]*GuardedBucketOp{
		"unstake":        op(OpUnstake, func(*GuardedBucketOp) {}),
		"withdraw":       op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient = addr }),
		"transfer+clear": op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.ClearGuard = addr, true }),
		"transfer+guard": op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = addr, zeroNewGuard() }),
		"candidate":      op(OpChangeCandidate, func(m *GuardedBucketOp) { m.CandidateName, m.ExpectedCandidateID = "abc", addr }),
		"restake":        op(OpRestake, func(m *GuardedBucketOp) { m.Duration, m.AutoStake = 91, true }),
		// Duration 0 is a staking-level question (legacy Restake allows it),
		// so the guard layer accepts it.
		"restake-zero": op(OpRestake, func(*GuardedBucketOp) {}),
		"rotate":       op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard = zeroNewGuard() }),
		"sponsored":    op(OpUnstake, func(m *GuardedBucketOp) { m.Sponsor = addr }),
		// The anchor block may be the deadline block, and the window is inclusive.
		"anchor=deadline": op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHeight = m.Deadline }),
		"full window":     op(OpUnstake, func(m *GuardedBucketOp) { m.Deadline = m.AnchorHeight + p.MaxDeadlineWindow }),
		// Only recipient, newOwner and sponsor are account addresses.
		"candidate-zeroID": op(OpChangeCandidate, func(m *GuardedBucketOp) { m.CandidateName, m.ExpectedCandidateID = "abc", zero }),
	}
	for name, m := range valid {
		if err := m.Validate(p); err != nil {
			t.Errorf("op/%s: valid op rejected: %v", name, err)
		}
	}
	invalid := map[string]*GuardedBucketOp{
		"deadline":          op(OpUnstake, func(m *GuardedBucketOp) { m.Deadline = 0 }),
		"sig":               op(OpUnstake, func(m *GuardedBucketOp) { m.Sig = m.Sig[1:] }),
		"unknownOp":         op(99, func(*GuardedBucketOp) {}),
		"unstake+recipient": op(OpUnstake, func(m *GuardedBucketOp) { m.Recipient = addr }),
		"unstake+duration":  op(OpUnstake, func(m *GuardedBucketOp) { m.Duration = 1 }),
		"withdraw-short":    op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient = addr[1:] }),
		"transfer-both":     op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard, m.ClearGuard = addr, zeroNewGuard(), true }),
		"transfer-none":     op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner = addr }),
		"transfer-badKey":   op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = addr, &NewGuard{AlgID: alg, PublicKey: zeroPub()} }),
		"candidate-noName":  op(OpChangeCandidate, func(m *GuardedBucketOp) { m.ExpectedCandidateID = addr }),
		"candidate-noID":    op(OpChangeCandidate, func(m *GuardedBucketOp) { m.CandidateName = "abc" }),
		"rotate-noKey":      op(OpRotate, func(*GuardedBucketOp) {}),
		"rotate+clear":      op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard, m.ClearGuard = zeroNewGuard(), true }),
		"restake+newOwner":  op(OpRestake, func(m *GuardedBucketOp) { m.NewOwner = addr }),
		"sponsor-short":     op(OpUnstake, func(m *GuardedBucketOp) { m.Sponsor = addr[1:] }),
		"sponsor-zero":      op(OpUnstake, func(m *GuardedBucketOp) { m.Sponsor = zero }),
		"recipient-zero":    op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient = zero }),
		"newOwner-zero":     op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.ClearGuard = zero, true }),
		"newOwner-zero+key": op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = zero, zeroNewGuard() }),
		"no anchor height":  op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHeight = 0 }),
		"no anchor hash":    op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHash = nil }),
		"anchor hash short": op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHash = m.AnchorHash[1:] }),
		"anchor hash long":  op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHash = append(m.AnchorHash, 0) }),
		"anchor>deadline":   op(OpUnstake, func(m *GuardedBucketOp) { m.AnchorHeight = m.Deadline + 1 }),
		"window exceeded":   op(OpUnstake, func(m *GuardedBucketOp) { m.Deadline = m.AnchorHeight + p.MaxDeadlineWindow + 1 }),
	}
	for name, m := range invalid {
		if err := m.Validate(p); !errors.Is(err, ErrMalformed) {
			t.Errorf("op/%s: want ErrMalformed, got %v", name, err)
		}
	}
}

func TestParamsV1IsACopy(t *testing.T) {
	p := ParamsV1()
	p.Algs[0] = 0x13
	p.GasPerByte = 1
	if q := ParamsV1(); q.Algs[0] != alg || q.GasPerByte != 100 || q.MaxDeadlineWindow != 241_920 {
		t.Fatal("ParamsV1 shares state between callers")
	}
}
