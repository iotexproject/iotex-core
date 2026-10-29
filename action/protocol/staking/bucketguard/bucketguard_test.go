package bucketguard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// Transcript vectors of docs/qsb-spec-and-vectors.md §4.3, on the mainnet
// domain (native chain ID 1, mainnet genesis hash).
func TestTranscriptVectors(t *testing.T) {
	var genesis [32]byte
	copy(genesis[:], mustHex(t, "b337983730981c2d50f114eed5da9dd20b83c8c5e130beefdb3001dc858cfe8b"))
	d := NewDomain(1, genesis)
	if got := hex.EncodeToString(d[:]); got != "49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871" {
		t.Fatalf("mainnet domain = %s", got)
	}
	owner := testAddr(t, 0x11)
	op := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: OpUnstake}
	wantCall := "49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed871000000000000000111111111111111111111111111111111111111110000000000000001000000000000000000000000000003f200000001000000000000000000000000000000000000000000000000000000"
	if got := hex.EncodeToString(EncodeCall(d, owner, op)); got != wantCall {
		t.Fatalf("CALL transcript mismatch\n got %s\nwant %s", got, wantCall)
	}

	pub := mustHex(t, "c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a")
	wantAccept := "49d87941a7e817ba842f0f3ec9252840a56d0d090ec50f1d6938fc8a82bed8710000000000000001000000000000000111111111111111111111111111111111111111110000001200000030c3530472b05ab2c631028bf08d7fa740f87ccbe26c8290b286f8317dbc30787febe5366d8be38345ec93d0fca288459a0000000000000000000000000000000000000000000000000000000000000000"
	if got := hex.EncodeToString(EncodeAccept(d, 1, 1, owner, alg, pub, [32]byte{})); got != wantAccept {
		t.Fatalf("ACCEPT transcript mismatch\n got %s\nwant %s", got, wantAccept)
	}

	// ROTATE with a bound sponsor: the incoming key accepts SHA-256(CALL).
	rot := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 2, Deadline: 1010, Op: OpRotate,
		NewGuard: &NewGuard{AlgID: alg, PublicKey: pub}, Sponsor: bytes.Repeat([]byte{0x22}, addressLen)}
	h := CallHash(d, owner, rot)
	if got := hex.EncodeToString(h[:]); got != "bb4e4bdeb3b03dee7cc5e6822fa2b13a23ca4cdca1615087039f55cff185fec6" {
		t.Fatalf("ROTATE callHash = %s", got)
	}
	acc := sha256.Sum256(EncodeAccept(d, 1, 2, owner, alg, pub, h))
	if got := hex.EncodeToString(acc[:]); got != "93375a34350d3b8e0045ff9b6c9f440dc963d7b5afa4d0269f8c3316cc47a408" {
		t.Fatalf("ROTATE ACCEPT sha256 = %s", got)
	}
}

// Size, gas and C(m) digest vectors of docs/qsb-spec-and-vectors.md, under ParamsV1.
func TestCanonicalVectors(t *testing.T) {
	type action interface {
		canonicalBytes() []byte
		CanonicalSize() int
		IntrinsicGas(Params) uint64
	}
	op := func(o Op, f func(*GuardedBucketOp)) *GuardedBucketOp {
		m := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Nonce: 0, Deadline: 1010, Op: o, Sig: zeroSig()}
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
			16236, 1663600, "15f253feb5642b8b3976bd822a83a4d1ca22f9bbdd2eaa4ddc44c59b975f2b85"},
		{"WITHDRAW", op(OpWithdraw, func(m *GuardedBucketOp) { m.Recipient = make([]byte, 20) }),
			16258, 1665800, "ed7811a696608574581c53db032d0ec1972375e83f73a2de64adadb122459856"},
		{"TRANSFER+clear", op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.ClearGuard = make([]byte, 20), true }),
			16260, 1666000, "cb8bee995cd6ec0088914ce227831453f85291d0db683e04a0087adeac565c9e"},
		{"CHANGE_CANDIDATE", op(OpChangeCandidate, func(m *GuardedBucketOp) {
			m.CandidateName, m.ExpectedCandidateID = "delegateab", make([]byte, 20)
		}), 16270, 1667000, "34419e212ee93d25331bb8f108795e85a9902b0f74b4a0a9bd7d38688ec98c80"},
		{"RESTAKE", op(OpRestake, func(m *GuardedBucketOp) { m.Duration, m.AutoStake = 7, true }),
			16240, 1664000, "27c100e25c9665f6d7dbf35461d8f17138ef36359e81e99e8cfaaf30f33a0679"},
		{"ROTATE", op(OpRotate, func(m *GuardedBucketOp) { m.NewGuard = zeroNewGuard() }),
			32518, 3321800, "4435a0e0f21a208232a0ef38f6a733d1207b1e731452ff40aadf19fca3379ec0"},
		{"TRANSFER+newGuard", op(OpTransfer, func(m *GuardedBucketOp) { m.NewOwner, m.NewGuard = make([]byte, 20), zeroNewGuard() }),
			32540, 3324000, "1d29df46e4678ad90b6a7578ca4a1caef6f62fccd8ee23824f72a35fe36834b3"},
		{"UNSTAKE+sponsor", op(OpUnstake, func(m *GuardedBucketOp) { m.Sponsor = bytes.Repeat([]byte{0x22}, addressLen) }),
			16258, 1665800, "3a588d82cff23452522fe0c392a23a68d516dff90b7276f6b597c800347e8c71"},
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
		m := &GuardedBucketOp{BucketIndex: 1, Generation: 1, Deadline: 1010, Op: o, Sig: zeroSig()}
		f(m)
		return m
	}
	addr := make([]byte, 20)
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
	if q := ParamsV1(); q.Algs[0] != alg || q.GasPerByte != 100 {
		t.Fatal("ParamsV1 shares state between callers")
	}
}
